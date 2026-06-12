package logic

import (
	"context"
	"strings"

	"internalwallet/common/captcha"
	"internalwallet/common/middleware"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/model"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

const (
	geetestSceneLoginEmail  = "login_email"
	geetestSceneLoginPhone  = "login_phone"
	geetestSceneLoginGoogle = "login_google"
	geetestSceneForgotEmail = "forgot_email"
	geetestSceneForgotPhone = "forgot_phone"
	geetestSceneValidate    = "geetest_validate"
	geetestSceneSendSMSCode = "send_sms_code"
	geetestSceneSendEmail   = "send_email_code"
)

func validateGeetest(ctx context.Context, svcCtx *svc.ServiceContext, scene string, lotNumber, captchaOutput, passToken, genTime, identifier string, userID int64) error {
	if svcCtx == nil || svcCtx.GeetestClient == nil {
		return nil
	}
	if !svcCtx.Config.Geetest.Enabled {
		return nil
	}

	result := svcCtx.GeetestClient.ValidateGeetestDetailed(ctx, lotNumber, captchaOutput, passToken, genTime)
	recordGeetestValidation(ctx, svcCtx, scene, lotNumber, identifier, userID, result)

	if result.FailOpen {
		if result.Err != nil {
			// NOTE: go-zero logx may not expose Warnf in some versions; use Errorf for visibility.
			logx.WithContext(ctx).Errorf("geetest fail-open: scene=%s reason=%s err=%v", scene, result.Reason, result.Err)
		}
		return nil
	}

	if result.Err != nil {
		if result.Reason == "missing_params" {
			return errx.CaptchaRequired()
		}
		return errx.VerifyFailed("geetest verify failed")
	}

	if !result.Success {
		// 区分用户取消和验证失败
		// Geetest 返回的 reason 可能包含 "user_cancel" 或 "cancelled" 等标识
		reasonLower := strings.ToLower(strings.TrimSpace(result.Reason))
		if strings.Contains(reasonLower, "cancel") || strings.Contains(reasonLower, "user_cancel") {
			// 用户主动取消验证码，返回更友好的错误
			return errx.CaptchaCancelled()
		}
		// 验证失败（包括验证码错误、过期等）
		return errx.CaptchaInvalid()
	}

	return nil
}

func recordGeetestValidation(ctx context.Context, svcCtx *svc.ServiceContext, scene, lotNumber, identifier string, userID int64, result captcha.GeetestValidationResult) {
	if svcCtx == nil || svcCtx.GeetestValidationLogRepository == nil {
		return
	}

	requestID := middleware.GetRequestID(ctx)
	ip := middleware.GetClientIP(ctx)
	userAgent := middleware.GetUserAgent(ctx)

	errMsg := ""
	if result.Err != nil {
		errMsg = truncateString(result.Err.Error(), 512)
	}

	logEntry := &model.GeetestValidationLogModel{
		UserId:     userID,
		Scene:      strings.TrimSpace(scene),
		Identifier: strings.TrimSpace(identifier),
		LotNumber:  strings.TrimSpace(lotNumber),
		IpAddress:  ip,
		UserAgent:  truncateString(userAgent, 512),
		RequestId:  truncateString(requestID, 64),
		Success:    result.Success,
		FailOpen:   result.FailOpen,
		Reason:     truncateString(result.Reason, 255),
		Error:      errMsg,
	}

	if err := svcCtx.GeetestValidationLogRepository.Create(ctx, logEntry); err != nil {
		logx.WithContext(ctx).Errorf("failed to save geetest validation log: %v", err)
	}
}

func truncateString(value string, max int) string {
	if max <= 0 {
		return ""
	}
	v := strings.TrimSpace(value)
	if len(v) <= max {
		return v
	}
	return v[:max]
}
