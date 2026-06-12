package logic

import (
	"context"
	"fmt"
	"time"

	"internalwallet/common/i18n"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
)

const (
	// SecurityCooldownDuration 安全冷却期时长（24小时）
	// 用户修改交易密码或解绑GA后，24小时内不允许进行转账等敏感操作
	SecurityCooldownDuration = 24 * time.Hour

	// SecurityCooldownKeyPrefix Redis key 前缀
	SecurityCooldownKeyPrefix = "security_cooldown:"
)

// SecurityCooldownReason 冷却期触发原因
type SecurityCooldownReason string

const (
	CooldownReasonTradePasswordSet   SecurityCooldownReason = "trade_password_set"   // 修改交易密码
	CooldownReasonTradePasswordReset SecurityCooldownReason = "trade_password_reset" // 重置交易密码
	CooldownReasonGABind             SecurityCooldownReason = "ga_bind"              // 重新绑定GA
	CooldownReasonGAUnbind           SecurityCooldownReason = "ga_unbind"            // 解绑GA
	CooldownReasonPasswordReset      SecurityCooldownReason = "password_reset"       // 重置登录密码
)

// SecurityCooldownInfo 冷却期信息
type SecurityCooldownInfo struct {
	InCooldown       bool                   // 是否在冷却期内
	Reason           SecurityCooldownReason // 触发原因
	CooldownUntil    time.Time              // 冷却期结束时间
	RemainingSeconds int64                  // 剩余秒数
}

// SetSecurityCooldown 设置用户安全冷却期
// 当用户执行敏感安全操作（修改交易密码、解绑GA等）后调用
func SetSecurityCooldown(ctx context.Context, svcCtx *svc.ServiceContext, userID int64, reason SecurityCooldownReason) error {
	if svcCtx.RedisClient == nil {
		logx.WithContext(ctx).Errorf("RedisClient not available, skipping security cooldown set for user %d", userID)
		return nil // 降级处理，不阻塞主流程
	}

	key := fmt.Sprintf("%s%d", SecurityCooldownKeyPrefix, userID)
	value := fmt.Sprintf("%s:%d", reason, time.Now().Unix())

	err := svcCtx.RedisClient.Set(ctx, key, value, SecurityCooldownDuration).Err()
	if err != nil {
		logx.WithContext(ctx).Errorf("Failed to set security cooldown for user %d: %v", userID, err)
		return err
	}

	logx.WithContext(ctx).Infof("Security cooldown set for user %d, reason: %s, expires in 24h", userID, reason)
	return nil
}

// CheckSecurityCooldown 检查用户是否在安全冷却期内
// 在执行转账、提现等敏感操作前调用
func CheckSecurityCooldown(ctx context.Context, svcCtx *svc.ServiceContext, userID int64) (*SecurityCooldownInfo, error) {
	if svcCtx.RedisClient == nil {
		// Redis 不可用时，降级为不在冷却期（允许操作）
		logx.WithContext(ctx).Errorf("RedisClient not available, skipping security cooldown check for user %d", userID)
		return &SecurityCooldownInfo{InCooldown: false}, nil
	}

	key := fmt.Sprintf("%s%d", SecurityCooldownKeyPrefix, userID)

	// 获取冷却期值
	val, err := svcCtx.RedisClient.Get(ctx, key).Result()
	if err == redis.Nil {
		// Key 不存在，不在冷却期
		return &SecurityCooldownInfo{InCooldown: false}, nil
	}
	if err != nil {
		logx.WithContext(ctx).Errorf("Failed to check security cooldown for user %d: %v", userID, err)
		// Redis 错误时，降级为不在冷却期（允许操作）
		return &SecurityCooldownInfo{InCooldown: false}, nil
	}

	// 获取 TTL
	ttl, err := svcCtx.RedisClient.TTL(ctx, key).Result()
	if err != nil || ttl <= 0 {
		// Key 已过期或即将过期
		return &SecurityCooldownInfo{InCooldown: false}, nil
	}

	// 解析原因
	var reason SecurityCooldownReason
	if len(val) > 0 {
		// 格式: "reason:timestamp"
		for i := 0; i < len(val); i++ {
			if val[i] == ':' {
				reason = SecurityCooldownReason(val[:i])
				break
			}
		}
		if reason == "" {
			reason = SecurityCooldownReason(val) // 兼容旧格式
		}
	}

	cooldownUntil := time.Now().Add(ttl)

	return &SecurityCooldownInfo{
		InCooldown:       true,
		Reason:           reason,
		CooldownUntil:    cooldownUntil,
		RemainingSeconds: int64(ttl.Seconds()),
	}, nil
}

// ClearSecurityCooldown 清除用户安全冷却期（管理员操作）
func ClearSecurityCooldown(ctx context.Context, svcCtx *svc.ServiceContext, userID int64) error {
	if svcCtx.RedisClient == nil {
		return nil
	}

	key := fmt.Sprintf("%s%d", SecurityCooldownKeyPrefix, userID)
	err := svcCtx.RedisClient.Del(ctx, key).Err()
	if err != nil {
		logx.WithContext(ctx).Errorf("Failed to clear security cooldown for user %d: %v", userID, err)
		return err
	}

	logx.WithContext(ctx).Infof("Security cooldown cleared for user %d", userID)
	return nil
}

// GetCooldownReasonMessage 获取冷却期原因的本地化描述
func GetCooldownReasonMessage(ctx context.Context, reason SecurityCooldownReason) string {
	var key string
	switch reason {
	case CooldownReasonTradePasswordSet:
		key = "SECURITY_COOLDOWN_REASON_TRADE_PASSWORD_SET"
	case CooldownReasonTradePasswordReset:
		key = "SECURITY_COOLDOWN_REASON_TRADE_PASSWORD_RESET"
	case CooldownReasonGABind:
		key = "SECURITY_COOLDOWN_REASON_GA_BIND"
	case CooldownReasonGAUnbind:
		key = "SECURITY_COOLDOWN_REASON_GA_UNBIND"
	case CooldownReasonPasswordReset:
		key = "SECURITY_COOLDOWN_REASON_PASSWORD_RESET"
	default:
		key = "SECURITY_COOLDOWN_REASON_DEFAULT"
	}
	return i18n.T(ctx, key, nil)
}

// FormatCooldownRemainingTime 格式化剩余时间（本地化）
func FormatCooldownRemainingTime(ctx context.Context, seconds int64) string {
	if seconds <= 0 {
		return i18n.T(ctx, "SECURITY_COOLDOWN_TIME_LESS_THAN_MINUTE", nil)
	}

	hours := seconds / 3600
	minutes := (seconds % 3600) / 60

	if hours > 0 {
		if minutes > 0 {
			return i18n.T(ctx, "SECURITY_COOLDOWN_TIME_HOURS_MINUTES", map[string]interface{}{
				"hours":   hours,
				"minutes": minutes,
			})
		}
		return i18n.T(ctx, "SECURITY_COOLDOWN_TIME_HOURS", map[string]interface{}{
			"hours": hours,
		})
	}
	if minutes > 0 {
		return i18n.T(ctx, "SECURITY_COOLDOWN_TIME_MINUTES", map[string]interface{}{
			"minutes": minutes,
		})
	}
	return i18n.T(ctx, "SECURITY_COOLDOWN_TIME_LESS_THAN_MINUTE", nil)
}

// GetCooldownErrorMessage 获取完整的冷却期错误消息（本地化）
func GetCooldownErrorMessage(ctx context.Context, reason SecurityCooldownReason, remainingSeconds int64) string {
	reasonMsg := GetCooldownReasonMessage(ctx, reason)
	timeMsg := FormatCooldownRemainingTime(ctx, remainingSeconds)

	return i18n.T(ctx, "SECURITY_COOLDOWN_ACTIVE", map[string]interface{}{
		"reason":         reasonMsg,
		"remaining_time": timeMsg,
	})
}
