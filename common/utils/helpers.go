package utils

import (
	"context"
	"fmt"
	"time"
)

// GenerateRequestID 生成请求 ID
func GenerateRequestID() string {
	id := GenerateID()
	return fmt.Sprintf("req_%d", id)
}

// StringPtrToString 将字符串指针转换为字符串
func StringPtrToString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// StringPtr 将字符串转换为字符串指针
func StringPtr(s string) *string {
	return &s
}

// TimePtr 将时间转换为时间指针
func TimePtr(t time.Time) *time.Time {
	return &t
}

// GetAdminIDFromContext 从 context 中获取管理员 ID (已废弃，请使用 admininterceptor.GetCurrentAdmin)
// Deprecated: Use admininterceptor.GetCurrentAdmin instead
func GetAdminIDFromContext(ctx context.Context) int64 {
	// 从 context 中获取 admin_id
	if ctx == nil {
		return 0
	}

	// 尝试从 metadata 或 context.Value 中获取
	if val := ctx.Value("admin_id"); val != nil {
		if adminID, ok := val.(int64); ok {
			return adminID
		}
	}

	// 尝试从字符串类型的 context value 中获取
	if val := ctx.Value("admin_id"); val != nil {
		if adminIDStr, ok := val.(string); ok {
			var adminID int64
			fmt.Sscanf(adminIDStr, "%d", &adminID)
			return adminID
		}
	}

	return 0
}

// GetUserIDFromContext 从 context 中获取用户 ID
func GetUserIDFromContext(ctx context.Context) int64 {
	if ctx == nil {
		return 0
	}

	if val := ctx.Value("user_id"); val != nil {
		if userID, ok := val.(int64); ok {
			return userID
		}
	}

	if val := ctx.Value("user_id"); val != nil {
		if userIDStr, ok := val.(string); ok {
			var userID int64
			fmt.Sscanf(userIDStr, "%d", &userID)
			return userID
		}
	}

	return 0
}
