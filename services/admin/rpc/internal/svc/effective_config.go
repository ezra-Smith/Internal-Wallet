package svc

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
)

// EffectiveRequire2FA 获取运行时生效的 Require2FA：
// - DB (system_config: security.require_2fa) 优先
// - DB 不可用/未配置时回退到 yaml（svcCtx.Config.Security.Require2FA）
func (s *ServiceContext) EffectiveRequire2FA(ctx context.Context) bool {
	fallback := s.Config.Security.Require2FA
	if s == nil || s.SystemConfigRepo == nil {
		return fallback
	}

	item, err := s.SystemConfigRepo.GetOne(ctx, "security", "require_2fa")
	if err != nil || item == nil || len(item.Value) == 0 {
		return fallback
	}

	var v interface{}
	if err := json.Unmarshal(item.Value, &v); err != nil {
		return fallback
	}

	switch vv := v.(type) {
	case bool:
		return vv
	case string:
		if b, err := strconv.ParseBool(strings.TrimSpace(vv)); err == nil {
			return b
		}
	case float64:
		return vv != 0
	}
	return fallback
}
