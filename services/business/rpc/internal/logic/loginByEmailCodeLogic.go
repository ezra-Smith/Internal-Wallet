package logic

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"internalwallet/common/code"
	"internalwallet/common/middleware"
	"internalwallet/common/utils"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/model"
	"internalwallet/services/business/rpc/internal/repository"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type LoginByEmailCodeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewLoginByEmailCodeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LoginByEmailCodeLogic {
	return &LoginByEmailCodeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 登录（邮箱+验证码）- 支持自动注册
func (l *LoginByEmailCodeLogic) LoginByEmailCode(in *pb.LoginByEmailCodeRequest) (*pb.LoginByEmailCodeResponse, error) {
	if in == nil {
		return nil, errx.InvalidParam("invalid params")
	}

	email := strings.TrimSpace(in.Email)
	codeStr := strings.TrimSpace(in.Code)
	if !utils.ValidateEmail(email) {
		return nil, errx.InvalidParam("invalid email")
	}
	if codeStr == "" {
		return nil, errx.InvalidParam("invalid code")
	}

	// 验证码校验
	length := l.svcCtx.Config.Code.Length
	if length <= 0 {
		length = 6
	}
	expire := l.svcCtx.Config.Code.ExpireSeconds
	if expire <= 0 {
		expire = 300
	}
	gen := code.NewGenerator(l.svcCtx.RedisClient, int(length), time.Duration(expire)*time.Second)

	scene := "auth"
	codeType := "email"
	recipient := email
	bypass := l.svcCtx.Config.Code.BypassCode

	var ok bool
	var err error
	if bypass != "" && codeStr == bypass {
		ok = true
	} else {
		ok, err = gen.Verify(l.ctx, recipient, scene, codeType, codeStr)
		if err != nil {
			return nil, errx.VerifyFailed("verify failed")
		}
	}
	if !ok {
		return nil, errx.InvalidCode("email")
	}

	// 查询用户是否存在
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

		// 用户存在，检查黑名单（登录场景）
		blacklistResult, blErr := CheckEmailBlacklist(l.ctx, l.svcCtx, email)
		if blErr != nil {
			l.Logger.Errorf("check email blacklist failed: %v", blErr)
		} else if blacklistResult.IsBlacklisted {
			return nil, errx.Blacklisted(blacklistResult.Message)
		}
	} else if err != nil && !errors.Is(err, repository.ErrProfileNotFound) {
		// 数据库/连接等错误：不要走注册分支，直接返回 DB_ERROR
		l.Logger.Errorf("get user by email failed: request_id=%s, email=%s, err=%v",
			middleware.GetRequestID(l.ctx), email, err,
		)
		return nil, errx.DBError()
	} else {
		// 用户不存在，先检查黑名单（注册场景）
		blacklistResult, blErr := CheckEmailBlacklist(l.ctx, l.svcCtx, email)
		if blErr != nil {
			l.Logger.Errorf("check email blacklist failed: %v", blErr)
		} else if blacklistResult.IsBlacklisted {
			return nil, errx.Blacklisted(FormatRegisterBlockedMessage())
		}

		// 自动注册
		profile, err = l.autoRegisterByEmail(email)
		if err != nil {
			// 常见线上故障：DB schema 未迁移（例如 users.id 非 AUTO_INCREMENT）
			// 见迁移文件：deploy/docker/init-db/63-users-auto-increment-id.sql
			if isMySQLNoDefaultValue(err) {
				l.Logger.Errorf("auto register by email failed (db schema not migrated?): request_id=%s, email=%s, err=%v",
					middleware.GetRequestID(l.ctx), email, err,
				)
				return nil, errx.DBSchemaNotMigrated("63-users-auto-increment-id.sql")
			}
			l.Logger.Errorf("auto register by email failed: request_id=%s, email=%s, err=%v",
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

	// 获取当前IP（用于异常登录检测和更新登录信息）
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

	// 删除已使用的验证码
	_ = gen.Delete(l.ctx, recipient, scene, codeType)

	message := "登录成功"
	if isNewUser {
		message = "注册并登录成功"
	}

	return &pb.LoginByEmailCodeResponse{
		Success:     true,
		Message:     message,
		UserId:      strconv.FormatInt(profile.ID, 10),
		AccessToken: token,
		ExpiresIn:   expires,
	}, nil
}

// autoRegisterByEmail 自动注册邮箱用户
func (l *LoginByEmailCodeLogic) autoRegisterByEmail(email string) (*model.UserModel, error) {
	profile := &model.UserModel{
		Email:      email,
		Nickname:   email,
		Source:     "email_code",
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
	go l.initializeUserWallet(profile.ID, "business_email_register")

	return profile, nil
}

// initializeUserWallet 初始化用户钱包（异步调用）
func (l *LoginByEmailCodeLogic) initializeUserWallet(userID int64, requester string) {
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
