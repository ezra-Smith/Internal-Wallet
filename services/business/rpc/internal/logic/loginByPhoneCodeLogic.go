package logic

import (
	"context"
	"errors"
	"fmt"
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

type LoginByPhoneCodeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewLoginByPhoneCodeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LoginByPhoneCodeLogic {
	return &LoginByPhoneCodeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 登录（手机+验证码）- 支持自动注册
func (l *LoginByPhoneCodeLogic) LoginByPhoneCode(in *pb.LoginByPhoneCodeRequest) (*pb.LoginByPhoneCodeResponse, error) {
	if in == nil {
		return nil, errx.InvalidParam("invalid params")
	}

	countryCode, err := normalizeAndValidateCountryCode(l.svcCtx, in.CountryCode)
	if err != nil {
		return nil, err
	}
	phone := strings.TrimSpace(in.Phone)
	codeStr := strings.TrimSpace(in.Code)
	if !utils.ValidatePhone(phone) {
		return nil, errx.InvalidParam("invalid phone")
	}
	if codeStr == "" {
		return nil, errx.InvalidParam("invalid code")
	}

	fullPhone := fmt.Sprintf("%s%s", countryCode, phone)

	// 验证码校验
	length := l.svcCtx.Config.Code.Length
	if length <= 0 {
		length = 6
	}
	expire := l.svcCtx.Config.Code.ExpireSeconds
	if expire <= 0 {
		expire = 600
	}
	gen := code.NewGenerator(l.svcCtx.RedisClient, int(length), time.Duration(expire)*time.Second)

	scene := "auth"
	codeType := "sms"
	recipient := fullPhone
	bypass := l.svcCtx.Config.Code.BypassCode

	var ok bool
	if bypass != "" && codeStr == bypass {
		ok = true
	} else {
		ok, err = gen.Verify(l.ctx, recipient, scene, codeType, codeStr)
		if err != nil {
			return nil, errx.VerifyFailed("verify failed")
		}
	}
	if !ok {
		return nil, errx.InvalidCode("sms")
	}

	// 查询用户是否存在
	accountRepo := l.svcCtx.UserAccountRepository
	profile, err := accountRepo.GetByPhone(l.ctx, phone, countryCode)

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
		blacklistResult, blErr := CheckPhoneBlacklist(l.ctx, l.svcCtx, fullPhone)
		if blErr != nil {
			l.Logger.Errorf("check phone blacklist failed: %v", blErr)
		} else if blacklistResult.IsBlacklisted {
			return nil, errx.Blacklisted(blacklistResult.Message)
		}
	} else if err != nil && !errors.Is(err, repository.ErrProfileNotFound) {
		// 数据库/连接等错误：不要走注册分支，直接返回 DB_ERROR
		l.Logger.Errorf("get user by phone failed: request_id=%s, phone=%s, country_code=%s, err=%v",
			middleware.GetRequestID(l.ctx), phone, countryCode, err,
		)
		return nil, errx.DBError()
	} else {
		// 用户不存在，先检查黑名单（注册场景）
		blacklistResult, blErr := CheckPhoneBlacklist(l.ctx, l.svcCtx, fullPhone)
		if blErr != nil {
			l.Logger.Errorf("check phone blacklist failed: %v", blErr)
		} else if blacklistResult.IsBlacklisted {
			return nil, errx.Blacklisted(FormatRegisterBlockedMessage())
		}

		// 自动注册
		createdNew := true
		profile, err = l.autoRegisterByPhone(phone, countryCode)
		if err != nil {
			// 常见线上故障：DB schema 未迁移（例如 users.id 非 AUTO_INCREMENT）
			// 见迁移文件：deploy/docker/init-db/63-users-auto-increment-id.sql
			if isMySQLNoDefaultValue(err) {
				l.Logger.Errorf("auto register by phone failed (db schema not migrated?): request_id=%s, phone=%s, country_code=%s, err=%v",
					middleware.GetRequestID(l.ctx), phone, countryCode, err,
				)
				return nil, errx.DBSchemaNotMigrated("63-users-auto-increment-id.sql")
			}
			// 并发注册/历史数据兼容：可能出现唯一键冲突（phone UNIQUE）
			// 这种情况下直接回读用户继续登录，避免返回 500。
			if isMySQLDuplicateKey(err) {
				existing, getErr := accountRepo.GetByPhoneOnly(l.ctx, phone)
				if getErr == nil && existing != nil {
					profile = existing
					createdNew = false
				} else {
					l.Logger.Errorf("auto register by phone duplicate key but refetch failed: request_id=%s, phone=%s, country_code=%s, err=%v, refetch_err=%v",
						middleware.GetRequestID(l.ctx), phone, countryCode, err, getErr,
					)
					return nil, errx.RegistrationFailed()
				}
			} else {
				l.Logger.Errorf("auto register by phone failed: request_id=%s, phone=%s, country_code=%s, err=%v",
					middleware.GetRequestID(l.ctx), phone, countryCode, err,
				)
				return nil, errx.RegistrationFailed()
			}
		}
		isNewUser = createdNew
	}

	// 生成 JWT Token
	expires := int64(3600)
	if l.svcCtx.Config.JWT.AccessExpire > 0 {
		expires = l.svcCtx.Config.JWT.AccessExpire
	}
	token, err := middleware.GenerateToken(strconv.FormatInt(profile.ID, 10), "", l.svcCtx.Config.JWT.AccessSecret, time.Second*time.Duration(expires))
	if err != nil {
		l.Logger.Errorf("failed to generate token: %v", err)
		return nil, errx.TokenError()
	}

	// 获取当前IP和User-Agent（用于异常登录检测和更新登录信息）
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

	return &pb.LoginByPhoneCodeResponse{
		Success:     true,
		Message:     message,
		UserId:      strconv.FormatInt(profile.ID, 10),
		AccessToken: token,
		ExpiresIn:   expires,
	}, nil
}

// autoRegisterByPhone 自动注册手机用户
func (l *LoginByPhoneCodeLogic) autoRegisterByPhone(phone, countryCode string) (*model.UserModel, error) {
	profile := &model.UserModel{
		Phone:       phone,
		CountryCode: countryCode,
		Nickname:    phone,
		Source:      "phone_code",
		RegisterIp:  middleware.GetClientIP(l.ctx),
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
	go l.initializeUserWallet(profile.ID, "business_phone_register")

	return profile, nil
}

// initializeUserWallet 初始化用户钱包（异步调用）
func (l *LoginByPhoneCodeLogic) initializeUserWallet(userID int64, requester string) {
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
