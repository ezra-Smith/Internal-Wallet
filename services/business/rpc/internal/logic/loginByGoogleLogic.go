package logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"internalwallet/common/middleware"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/model"
	"internalwallet/services/business/rpc/internal/repository"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

// GoogleTokenInfo Google ID Token 验证后返回的用户信息
type GoogleTokenInfo struct {
	Iss           string `json:"iss"`            // 签发者
	Azp           string `json:"azp"`            // 授权方
	Aud           string `json:"aud"`            // 受众（Client ID）
	Sub           string `json:"sub"`            // 用户唯一标识（Google User ID）
	Email         string `json:"email"`          // 邮箱
	EmailVerified string `json:"email_verified"` // 邮箱是否验证
	Name          string `json:"name"`           // 姓名
	Picture       string `json:"picture"`        // 头像URL
	GivenName     string `json:"given_name"`     // 名
	FamilyName    string `json:"family_name"`    // 姓
	Locale        string `json:"locale"`         // 语言区域
	Iat           string `json:"iat"`            // 签发时间
	Exp           string `json:"exp"`            // 过期时间
}

type LoginByGoogleLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewLoginByGoogleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LoginByGoogleLogic {
	return &LoginByGoogleLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// LoginByGoogle Google OAuth 登录 - 支持自动注册
func (l *LoginByGoogleLogic) LoginByGoogle(in *pb.LoginByGoogleRequest) (*pb.LoginByGoogleResponse, error) {
	if in == nil || in.IdToken == "" {
		return nil, errx.InvalidParam("invalid params: id_token is required")
	}

	// 验证 Google ID Token
	tokenInfo, err := l.verifyGoogleIDToken(in.IdToken)
	if err != nil {
		l.Logger.Errorf("verify google id token failed: %v", err)
		return nil, errx.InvalidParam("invalid google token")
	}

	// 验证邮箱是否存在
	if tokenInfo.Email == "" {
		return nil, errx.InvalidParam("google account has no email")
	}

	googleID := tokenInfo.Sub
	email := strings.ToLower(strings.TrimSpace(tokenInfo.Email))

	// 检查 Google ID 黑名单
	blacklistResult, blErr := CheckGoogleBlacklist(l.ctx, l.svcCtx, googleID)
	if blErr != nil {
		l.Logger.Errorf("check google blacklist failed: %v", blErr)
	} else if blacklistResult.IsBlacklisted {
		return nil, errx.Blacklisted(blacklistResult.Message)
	}

	// 检查邮箱黑名单
	emailBlacklist, blErr := CheckEmailBlacklist(l.ctx, l.svcCtx, email)
	if blErr != nil {
		l.Logger.Errorf("check email blacklist failed: %v", blErr)
	} else if emailBlacklist.IsBlacklisted {
		return nil, errx.Blacklisted(emailBlacklist.Message)
	}

	// 查询用户是否存在（优先通过邮箱查询）
	accountRepo := l.svcCtx.UserAccountRepository
	profile, err := accountRepo.GetByEmail(l.ctx, email)

	var isNewUser bool
	if err == nil && profile != nil {
		// 检查用户状态（冻结/终止用户不能登录）
		if profile.Status == 2 {
			return nil, errx.AccountFrozen()
		}
		if profile.Status == 3 {
			return nil, errx.AccountTerminated()
		}

		// 更新用户头像（如果有变化）
		if tokenInfo.Picture != "" && profile.Avatar != tokenInfo.Picture {
			_ = accountRepo.UpdateFields(l.ctx, profile.ID, map[string]interface{}{
				"avatar": tokenInfo.Picture,
			})
		}
	} else if err != nil && !errors.Is(err, repository.ErrProfileNotFound) {
		// 数据库/连接等错误：不要走注册分支，直接返回 DB_ERROR
		l.Logger.Errorf("get user by email failed: request_id=%s, email=%s, err=%v",
			middleware.GetRequestID(l.ctx), email, err,
		)
		return nil, errx.DBError()
	} else {
		// 用户不存在，自动注册
		profile, err = l.autoRegisterByGoogle(tokenInfo)
		if err != nil {
			// 常见线上故障：DB schema 未迁移（例如 users.id 非 AUTO_INCREMENT）
			// 见迁移文件：deploy/docker/init-db/63-users-auto-increment-id.sql
			if isMySQLNoDefaultValue(err) {
				l.Logger.Errorf("auto register by google failed (db schema not migrated?): request_id=%s, email=%s, err=%v",
					middleware.GetRequestID(l.ctx), email, err,
				)
				return nil, errx.DBSchemaNotMigrated("63-users-auto-increment-id.sql")
			}
			l.Logger.Errorf("auto register by google failed: request_id=%s, email=%s, err=%v",
				middleware.GetRequestID(l.ctx), email, err,
			)
			return nil, errx.RegistrationFailed()
		}
		isNewUser = true
	}

	// 生成 JWT Token
	expires := int64(3600)
	if l.svcCtx.Config.JWT.AccessExpire > 0 {
		expires = l.svcCtx.Config.JWT.AccessExpire
	}
	token, err := middleware.GenerateToken(strconv.FormatInt(profile.ID, 10), email, l.svcCtx.Config.JWT.AccessSecret, time.Second*time.Duration(expires))
	if err != nil {
		l.Logger.Errorf("failed to generate token: %v", err)
		return nil, errx.TokenError()
	}

	// 获取当前IP
	ip := middleware.GetClientIP(l.ctx)
	// 先查询上次登录信息（在更新数据之前，这样才能获取到旧数据）
	lastLoginInfo, _ := GetLastLoginInfo(l.ctx, l.svcCtx, profile.ID)

	// 异步检测并通知异常登录
	go CheckAndNotifyAbnormalLogin(l.ctx, l.svcCtx, profile.ID, lastLoginInfo)

	// 更新最后登录时间和IP
	if err := accountRepo.UpdateLastLoginTime(l.ctx, profile.ID, ip); err != nil {
		l.Logger.Errorf("failed to update last login time and ip: %v", err)
	}

	// 更新设备登录记录（包含IP地理位置）
	if err := UpdateUserDeviceOnLogin(l.ctx, l.svcCtx.DB, profile.ID); err != nil {
		// 设备记录更新失败不影响登录流程
		l.Logger.Errorf("failed to update user device: user_id=%d, error=%v", profile.ID, err)
	}

	message := "登录成功"
	if isNewUser {
		message = "注册并登录成功"
	}

	return &pb.LoginByGoogleResponse{
		Success:     true,
		Message:     message,
		UserId:      strconv.FormatInt(profile.ID, 10),
		AccessToken: token,
		ExpiresIn:   expires,
		IsNewUser:   isNewUser,
		Email:       email,
		Name:        tokenInfo.Name,
		Avatar:      tokenInfo.Picture,
	}, nil
}

// verifyGoogleIDToken 验证 Google ID Token
// 通过调用 Google tokeninfo API 验证 token 有效性
func (l *LoginByGoogleLogic) verifyGoogleIDToken(idToken string) (*GoogleTokenInfo, error) {
	// Google 官方 tokeninfo 验证端点
	url := fmt.Sprintf("https://oauth2.googleapis.com/tokeninfo?id_token=%s", idToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to verify token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token verification failed: status=%d, body=%s", resp.StatusCode, string(body))
	}

	var tokenInfo GoogleTokenInfo
	if err := json.Unmarshal(body, &tokenInfo); err != nil {
		return nil, fmt.Errorf("failed to parse token info: %w", err)
	}

	// 可选：验证 aud（Client ID）是否匹配
	// 这里可以添加对 l.svcCtx.Config.Google.ClientID 的验证

	return &tokenInfo, nil
}

// autoRegisterByGoogle 自动注册 Google 用户
func (l *LoginByGoogleLogic) autoRegisterByGoogle(tokenInfo *GoogleTokenInfo) (*model.UserModel, error) {
	email := strings.ToLower(strings.TrimSpace(tokenInfo.Email))
	nickname := tokenInfo.Name
	if nickname == "" {
		nickname = email
	}

	profile := &model.UserModel{
		Email:      email,
		Nickname:   nickname,
		Avatar:     tokenInfo.Picture,
		Source:     "google_oauth",
		RegisterIp: middleware.GetClientIP(l.ctx),
	}

	if l.svcCtx.DB == nil {
		return nil, gorm.ErrInvalidDB
	}

	if err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		accountRepo := repository.NewUserAccountRepository(tx)
		if err := accountRepo.CreateMember(l.ctx, profile); err != nil {
			return err
		}
		return setupNewUserWithTx(l.ctx, l.svcCtx, tx, profile)
	}); err != nil {
		return nil, err
	}

	// 事务成功后，异步调用 Admin 初始化用户钱包
	go l.initializeUserWallet(profile.ID, "business_google_register")

	return profile, nil
}

// initializeUserWallet 初始化用户钱包（异步调用）
func (l *LoginByGoogleLogic) initializeUserWallet(userID int64, requester string) {
	if l.svcCtx.AdminRpc == nil {
		l.Logger.Infof("AdminRpc not available, skip wallet initialization for user_id=%d", userID)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := l.svcCtx.AdminRpc.InitializeUserWallet(ctx, &pb.InitializeUserWalletRequest{
		UserId:    userID,
		Requester: requester,
	})
	if err != nil {
		l.Logger.Errorf("Failed to initialize user wallet: user_id=%d, err=%v", userID, err)
	} else {
		l.Logger.Infof("✓ User wallet initialized successfully: user_id=%d", userID)
	}
}
