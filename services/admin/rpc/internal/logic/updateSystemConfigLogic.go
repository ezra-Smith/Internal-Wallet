package logic

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	admininterceptor "internalwallet/services/admin/rpc/internal/interceptor"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/repository"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"gorm.io/gorm"
	"internalwallet/common/middleware"
)

type UpdateSystemConfigLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateSystemConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateSystemConfigLogic {
	return &UpdateSystemConfigLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateSystemConfigLogic) UpdateSystemConfig(in *pb.UpdateSystemConfigRequest) (*pb.UpdateSystemConfigResponse, error) {
	if in == nil || strings.TrimSpace(in.Category) == "" || in.Settings == nil {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid params", map[string]string{
			"category": "required",
			"settings": "required",
		})
	}
	if l.svcCtx.DB == nil || l.svcCtx.SystemConfigRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	current, ok := admininterceptor.GetCurrentAdmin(l.ctx)
	if !ok || current == nil {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}

	category := strings.TrimSpace(in.Category)
	allowed := map[string]map[string]bool{
		"general": {
			"system_name":      true,
			"timezone":         true,
			"currency_display": true,
			"maintenance_mode": true,
		},
		"security": {
			"session_timeout_minutes":  true,
			"max_login_attempts":       true,
			"lockout_duration_minutes": true,
			"password_expiry_days":     true,
			"require_2fa":              true,
		},
		"notification": {
			"email_enabled":    true,
			"webhook_enabled":  true,
			"webhook_url":      true,
			"alert_recipients": true,
		},
		"limits": {
			"user_daily_withdraw_usd":  true,
			"user_single_withdraw_usd": true,
			"auto_approve_max_usd":     true,
		},
	}
	keysAllowed, ok := allowed[category]
	if !ok {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_CATEGORY", "无效配置分类", map[string]string{"category": "invalid"})
	}

	settings := in.Settings.AsMap()
	if len(settings) == 0 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "empty settings", map[string]string{"settings": "empty"})
	}
	normalized := make(map[string]interface{}, len(settings))
	typeViolations := map[string]string{}
	for key, val := range settings {
		if !keysAllowed[key] {
			return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid setting key", map[string]string{"settings." + key: "not allowed"})
		}
		nv, msg := normalizeSetting(category, key, val)
		if msg != "" {
			typeViolations["settings."+key] = msg
			continue
		}
		normalized[key] = nv
	}
	if len(typeViolations) > 0 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid setting value", typeViolations)
	}
	settings = normalized

	now := time.Now()
	ip := middleware.GetClientIP(l.ctx)
	ua := middleware.GetUserAgent(l.ctx)

	type kv struct {
		Key      string
		OldValue string
		NewValue string
	}
	kvs := make([]kv, 0, len(settings))

	toStr := func(b []byte) string {
		if len(b) == 0 {
			return ""
		}
		var v interface{}
		if err := json.Unmarshal(b, &v); err != nil {
			return string(b)
		}
		return toJSONString(v)
	}

	detailsBytes, _ := json.Marshal(map[string]interface{}{
		"category": category,
		"reason":   strings.TrimSpace(in.Reason),
	})

	if err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		cfgRepo := repository.NewAdminSystemConfigRepository(tx)
		auditRepo := repository.NewAdminAuditLogRepository(tx)

		for key, val := range settings {
			old := ""
			existing, err := cfgRepo.GetOne(l.ctx, category, key)
			if err == nil && existing != nil {
				old = toStr(existing.Value)
			}
			newBytes, mErr := json.Marshal(val)
			if mErr != nil {
				return mErr
			}
			newStr := toJSONString(val)

			if _, err := cfgRepo.Upsert(l.ctx, category, key, newBytes, current.ID); err != nil {
				return err
			}
			kvs = append(kvs, kv{Key: key, OldValue: old, NewValue: newStr})
		}

		_ = auditRepo.CreateLog(l.ctx, &model.AdminAuditLogModel{
			AdminID:     current.ID,
			Action:      "config.update",
			TargetType:  "config",
			TargetID:    category,
			Description: "更新系统配置: " + category,
			Details:     detailsBytes,
			IP:          ip,
			UserAgent:   ua,
		})
		return nil
	}); err != nil {
		l.Logger.Errorf("update system config failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	changes := make([]*pb.ConfigChangeItem, 0, len(kvs))
	for _, item := range kvs {
		changes = append(changes, &pb.ConfigChangeItem{
			Key:      item.Key,
			OldValue: item.OldValue,
			NewValue: item.NewValue,
		})
	}

	return &pb.UpdateSystemConfigResponse{
		Success: true,
		Message: resp.Msg(l.ctx, "CONFIG_UPDATED"),
		Data: &pb.UpdateSystemConfigData{
			Category:  category,
			Changes:   changes,
			UpdatedAt: formatTime(now),
			UpdatedBy: current.Username,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}

func normalizeSetting(category, key string, val interface{}) (interface{}, string) {
	requireBool := func(v interface{}) (bool, bool) {
		switch x := v.(type) {
		case bool:
			return x, true
		case string:
			if b, err := strconv.ParseBool(strings.TrimSpace(x)); err == nil {
				return b, true
			}
			return false, false
		default:
			return false, false
		}
	}

	requireInt := func(v interface{}) (int64, bool) {
		switch x := v.(type) {
		case float64:
			if x != float64(int64(x)) {
				return 0, false
			}
			return int64(x), true
		case string:
			if n, err := strconv.ParseInt(strings.TrimSpace(x), 10, 64); err == nil {
				return n, true
			}
			return 0, false
		default:
			return 0, false
		}
	}

	requireString := func(v interface{}) (string, bool) {
		s, ok := v.(string)
		return s, ok
	}

	requireStringSlice := func(v interface{}) ([]string, bool) {
		switch x := v.(type) {
		case []string:
			return x, true
		case []interface{}:
			out := make([]string, 0, len(x))
			for _, it := range x {
				s, ok := it.(string)
				if !ok {
					return nil, false
				}
				out = append(out, s)
			}
			return out, true
		default:
			return nil, false
		}
	}

	switch category {
	case "general":
		switch key {
		case "system_name", "timezone", "currency_display":
			if s, ok := requireString(val); ok {
				return s, ""
			}
			return nil, "must be string"
		case "maintenance_mode":
			if b, ok := requireBool(val); ok {
				return b, ""
			}
			return nil, "must be boolean"
		}
	case "security":
		switch key {
		case "require_2fa":
			if b, ok := requireBool(val); ok {
				return b, ""
			}
			return nil, "must be boolean"
		case "session_timeout_minutes", "max_login_attempts", "lockout_duration_minutes", "password_expiry_days":
			if n, ok := requireInt(val); ok {
				return n, ""
			}
			return nil, "must be integer"
		}
	case "notification":
		switch key {
		case "email_enabled", "webhook_enabled":
			if b, ok := requireBool(val); ok {
				return b, ""
			}
			return nil, "must be boolean"
		case "webhook_url":
			if s, ok := requireString(val); ok {
				return s, ""
			}
			return nil, "must be string"
		case "alert_recipients":
			if arr, ok := requireStringSlice(val); ok {
				return arr, ""
			}
			return nil, "must be array of strings"
		}
	case "limits":
		switch key {
		case "user_daily_withdraw_usd", "user_single_withdraw_usd", "auto_approve_max_usd":
			if s, ok := requireString(val); ok {
				return s, ""
			}
			return nil, "must be string"
		}
	}

	// 理论上不会走到这里（key 已被白名单过滤），保守兜底为原值。
	return val, ""
}
