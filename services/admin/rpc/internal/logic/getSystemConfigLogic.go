package logic

import (
	"context"
	"encoding/json"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetSystemConfigLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetSystemConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetSystemConfigLogic {
	return &GetSystemConfigLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ==================== System Settings ====================
func (l *GetSystemConfigLogic) GetSystemConfig(in *pb.GetSystemConfigRequest) (*pb.GetSystemConfigResponse, error) {
	_ = in

	// defaults（可被 DB 覆盖）
	general := &pb.GeneralConfig{
		SystemName:      "Zink Wallet",
		Timezone:        "Asia/Shanghai",
		CurrencyDisplay: "USD",
		MaintenanceMode: false,
	}
	securityCfg := &pb.SecurityConfig{
		SessionTimeoutMinutes:  l.svcCtx.Config.Security.SessionTimeoutMinutes,
		MaxLoginAttempts:       l.svcCtx.Config.Security.MaxLoginAttempts,
		LockoutDurationMinutes: l.svcCtx.Config.Security.LockoutDurationMinutes,
		PasswordExpiryDays:     l.svcCtx.Config.Security.PasswordExpiryDays,
		Require_2Fa:            l.svcCtx.Config.Security.Require2FA,
	}
	notification := &pb.NotificationConfig{
		EmailEnabled:    true,
		WebhookEnabled:  false,
		WebhookUrl:      "",
		AlertRecipients: []string{},
	}
	limits := &pb.LimitsConfig{
		UserDailyWithdrawUsd:  "50000.00",
		UserSingleWithdrawUsd: "10000.00",
		AutoApproveMaxUsd:     "500.00",
	}

	// DB override
	if l.svcCtx.SystemConfigRepo != nil {
		applyList := func(category string, items interface{}) {
			_ = category
			_ = items
		}
		_ = applyList
		categories := []string{"general", "security", "notification", "limits"}
		for _, cat := range categories {
			items, err := l.svcCtx.SystemConfigRepo.GetByCategory(l.ctx, cat)
			if err != nil {
				continue
			}
			for _, it := range items {
				if it == nil || len(it.Value) == 0 {
					continue
				}
				var v interface{}
				if err := json.Unmarshal(it.Value, &v); err != nil {
					continue
				}
				switch cat {
				case "general":
					switch it.KeyName {
					case "system_name":
						if s, ok := v.(string); ok {
							general.SystemName = s
						}
					case "timezone":
						if s, ok := v.(string); ok {
							general.Timezone = s
						}
					case "currency_display":
						if s, ok := v.(string); ok {
							general.CurrencyDisplay = s
						}
					case "maintenance_mode":
						if b, ok := v.(bool); ok {
							general.MaintenanceMode = b
						}
					}
				case "security":
					switch it.KeyName {
					case "session_timeout_minutes":
						if n, ok := v.(float64); ok {
							securityCfg.SessionTimeoutMinutes = int32(n)
						}
					case "max_login_attempts":
						if n, ok := v.(float64); ok {
							securityCfg.MaxLoginAttempts = int32(n)
						}
					case "lockout_duration_minutes":
						if n, ok := v.(float64); ok {
							securityCfg.LockoutDurationMinutes = int32(n)
						}
					case "password_expiry_days":
						if n, ok := v.(float64); ok {
							securityCfg.PasswordExpiryDays = int32(n)
						}
					case "require_2fa":
						if b, ok := v.(bool); ok {
							securityCfg.Require_2Fa = b
						}
					}
				case "notification":
					switch it.KeyName {
					case "email_enabled":
						if b, ok := v.(bool); ok {
							notification.EmailEnabled = b
						}
					case "webhook_enabled":
						if b, ok := v.(bool); ok {
							notification.WebhookEnabled = b
						}
					case "webhook_url":
						if s, ok := v.(string); ok {
							notification.WebhookUrl = s
						}
					case "alert_recipients":
						var arr []string
						if err := json.Unmarshal(it.Value, &arr); err == nil {
							notification.AlertRecipients = arr
						}
					}
				case "limits":
					switch it.KeyName {
					case "user_daily_withdraw_usd":
						if s, ok := v.(string); ok {
							limits.UserDailyWithdrawUsd = s
						}
					case "user_single_withdraw_usd":
						if s, ok := v.(string); ok {
							limits.UserSingleWithdrawUsd = s
						}
					case "auto_approve_max_usd":
						if s, ok := v.(string); ok {
							limits.AutoApproveMaxUsd = s
						}
					}
				}
			}
		}
	}

	return &pb.GetSystemConfigResponse{
		Success: true,
		Message: "ok",
		Data: &pb.SystemConfigData{
			General:      general,
			Security:     securityCfg,
			Notification: notification,
			Limits:       limits,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
