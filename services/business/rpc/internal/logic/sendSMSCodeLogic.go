package logic

import (
	"context"
	"strconv"
	"strings"
	"time"

	"internalwallet/common/code"
	"internalwallet/common/middleware"
	"internalwallet/common/utils"
	"internalwallet/pkg/notify"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type SendSMSCodeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSendSMSCodeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SendSMSCodeLogic {
	return &SendSMSCodeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 发送短信验证码
func (l *SendSMSCodeLogic) SendSMSCode(in *pb.SendSMSCodeReq) (*pb.SendSMSCodeResp, error) {
	if in == nil {
		l.Infof("SendSMSCode failed: request is nil")
		return nil, errx.InvalidParam("invalid params")
	}

	// 参数验证日志
	l.Infof("SendSMSCode request: phone=%s, country_code=%s, type=%d", in.Phone, in.CountryCode, in.Type)

	// 验证 CodeType 是否有效
	if in.Type == pb.CodeType_CODE_TYPE_UNSPECIFIED {
		l.Infof("SendSMSCode failed: code type is unspecified (type=%d)", in.Type)
		return nil, errx.InvalidParam("code type is required")
	}

	if in.Phone == "" {
		uidStr := middleware.GetUserID(l.ctx)
		if uidStr == "" {
			l.Infof("SendSMSCode failed: phone is empty and no user_id in context")
			return nil, errx.InvalidParam("phone is required")
		}
		id, err := strconv.ParseInt(uidStr, 10, 64)
		if err != nil {
			l.Infof("SendSMSCode failed: invalid user_id format: %v", err)
			return nil, errx.InvalidUser()
		}
		accountRepo := l.svcCtx.UserAccountRepository
		if p, _ := accountRepo.GetByID(l.ctx, id); p != nil {
			in.Phone = p.Phone
			in.CountryCode = p.CountryCode
			l.Infof("SendSMSCode: retrieved phone from user profile: phone=%s, country_code=%s", in.Phone, in.CountryCode)
		}
	}

	phone := strings.TrimSpace(in.Phone)
	countryCode, err := normalizeAndValidateCountryCode(l.svcCtx, in.CountryCode)
	if err != nil {
		l.Infof("SendSMSCode failed: invalid country_code=%s, error=%v", in.CountryCode, err)
		return nil, err
	}

	if phone == "" {
		return nil, errx.PhoneRequired()
	}

	if countryCode == "" {
		return nil, errx.InvalidParam("country code is required")
	}

	if !utils.ValidatePhone(phone) {
		return nil, errx.InvalidPhoneFormat()
	}

	// Geetest 校验（发送短信验证码前）
	fullPhone := countryCode + phone
	if err := validateGeetest(l.ctx, l.svcCtx, geetestSceneSendSMSCode, in.LotNumber, in.CaptchaOutput, in.PassToken, in.GenTime, fullPhone, 0); err != nil {
		return nil, err
	}

	// 黑名单检查：发送验证码前检查手机号是否在黑名单中
	blacklistResult, err := CheckPhoneBlacklist(l.ctx, l.svcCtx, fullPhone)
	if err != nil {
		l.Logger.Errorf("check phone blacklist failed: %v", err)
		// 黑名单检查失败不阻止发送，只记录日志
	} else if blacklistResult.IsBlacklisted {
		return nil, errx.Blacklisted(FormatSendCodeBlockedMessage())
	}

	expire := l.svcCtx.Config.Code.ExpireSeconds
	if expire <= 0 {
		expire = 600
	}
	scene := "auth"
	switch in.Type {
	case pb.CodeType_CODE_TYPE_AUTH:
		scene = "auth"
	case pb.CodeType_CODE_TYPE_RESET_PASSWORD:
		scene = "reset_password"
	case pb.CodeType_CODE_TYPE_RESET_TRADE_PASSWORD:
		scene = "reset_trade_password"
	case pb.CodeType_CODE_TYPE_WITHDRAW:
		scene = "withdraw"
	case pb.CodeType_CODE_TYPE_BIND_PHONE:
		scene = "bind_phone"
	case pb.CodeType_CODE_TYPE_2FA:
		scene = "2fa"
	}
	length := l.svcCtx.Config.Code.Length
	if length <= 0 {
		length = 6
	}
	gen := code.NewGenerator(l.svcCtx.RedisClient, int(length), time.Duration(expire)*time.Second)
	recipient := countryCode + phone
	codeStr, err := gen.Generate(l.ctx, recipient, scene, "sms")
	if err != nil {
		return nil, errx.GenerateFailed()
	}
	result, err := notify.SendVerificationCode(recipient, codeStr)
	if err != nil {
		l.Logger.Errorf("send sms code failed: %v", err)
		return nil, errx.SMSSendFailed()
	}
	if !result.Success {
		l.Logger.Errorf("send sms code failed: %s", result.Message)
		// 检查是否是手机号格式错误
		if strings.Contains(result.Message, "无效手机号") || strings.Contains(result.Message, "invalid phone") ||
			strings.Contains(result.Message, "手机号格式") || strings.Contains(result.Message, "phone format") {
			return nil, errx.SMSInvalidPhone()
		}
		return nil, errx.SMSSendFailed()
	}
	l.Logger.Infof("send sms code success: %v", result)
	return &pb.SendSMSCodeResp{Success: true, Message: "ok", ExpireSeconds: expire}, nil
}
