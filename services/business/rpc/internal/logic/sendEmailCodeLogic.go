package logic

import (
	"context"
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

type SendEmailCodeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSendEmailCodeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SendEmailCodeLogic {
	return &SendEmailCodeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SendEmailCodeLogic) SendEmailCode(in *pb.SendEmailCodeReq) (*pb.SendEmailCodeResp, error) {
	if in == nil {
		return nil, errx.InvalidParam("invalid params")
	}
	email := strings.TrimSpace(in.Email)
	if email == "" {
		// 兼容已登录场景：从 JWT claims 注入的 context 里取
		email = middleware.GetEmail(l.ctx)
	}
	
	// 邮箱为空检查
	if email == "" {
		return nil, errx.EmailRequired()
	}
	
	// Normalize email to lowercase for consistent Redis key matching
	email = strings.ToLower(email)
	
	// 邮箱格式校验
	if !utils.ValidateEmail(email) {
		return nil, errx.InvalidEmailFormat()
	}

	// Geetest 校验（发送邮箱验证码前）
	if err := validateGeetest(l.ctx, l.svcCtx, geetestSceneSendEmail, in.LotNumber, in.CaptchaOutput, in.PassToken, in.GenTime, email, 0); err != nil {
		return nil, err
	}

	// 黑名单检查：发送验证码前检查邮箱是否在黑名单中
	blacklistResult, err := CheckEmailBlacklist(l.ctx, l.svcCtx, email)
	if err != nil {
		l.Logger.Errorf("check email blacklist failed: %v", err)
		// 黑名单检查失败不阻止发送，只记录日志
	} else if blacklistResult.IsBlacklisted {
		return nil, errx.Blacklisted(FormatSendCodeBlockedMessage())
	}

	expire := l.svcCtx.Config.Code.ExpireSeconds
	//如果没有拿到配置，则使用默认值 5分钟
	if expire <= 0 {
		expire = 300
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
	case pb.CodeType_CODE_TYPE_BIND_EMAIL:
		scene = "bind_email"
	case pb.CodeType_CODE_TYPE_2FA:
		scene = "2fa"
	}
	gen := code.NewGenerator(l.svcCtx.RedisClient, 6, time.Duration(expire)*time.Second)
	codeStr, err := gen.Generate(l.ctx, email, scene, "email")

	if err != nil {
		l.Logger.Errorf("generate verification code failed: %v", err)
		return nil, errx.GenerateFailed()
	}

	// 使用异步发送邮件，立即返回，避免超时
	_, err = notify.SendVerificationEmailAsync(email, codeStr)
	if err != nil {
		l.Logger.Errorf("submit verification email to async queue failed: %v", err)
		// 检查是否是配置错误
		errMsg := err.Error()
		if strings.Contains(errMsg, "required") || strings.Contains(errMsg, "EMAIL_") || strings.Contains(errMsg, "配置错误") || strings.Contains(errMsg, "SMTP") {
			// 提供更详细的错误信息
			if strings.Contains(errMsg, "EMAIL_SMTP_HOST") || strings.Contains(errMsg, "SMTPHost") {
				return nil, errx.Internal("邮件服务配置错误：SMTP 服务器地址未配置")
			}
			if strings.Contains(errMsg, "EMAIL_FROM_ADDRESS") || strings.Contains(errMsg, "FromAddress") {
				return nil, errx.Internal("邮件服务配置错误：发件人邮箱地址未配置")
			}
			return nil, errx.Internal("邮件服务配置错误，请联系管理员")
		}
		if strings.Contains(errMsg, "队列已满") {
			return nil, errx.Internal("邮件发送队列繁忙，请稍后重试")
		}
		return nil, errx.SendFailed("提交验证码发送任务失败，请稍后重试")
	}

	l.Logger.Infof("verification email submitted to async queue successfully [email=%s, scene=%s]", email, scene)
	return &pb.SendEmailCodeResp{Success: true, Message: "验证码已发送，请查收邮件", ExpireSeconds: expire}, nil
}
