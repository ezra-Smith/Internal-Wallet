package logic

import (
	"context"
	"fmt"
	"time"

	"internalwallet/services/business/rpc/internal/repository"
	"internalwallet/services/business/rpc/internal/svc"
)

// BlacklistCheckResult 黑名单检查结果
type BlacklistCheckResult struct {
	IsBlacklisted bool
	Message       string    // 用户友好的错误消息
	BlockedAt     time.Time // 被拉黑时间
}

// CheckEmailBlacklist 检查邮箱是否在黑名单中
func CheckEmailBlacklist(ctx context.Context, svcCtx *svc.ServiceContext, email string) (*BlacklistCheckResult, error) {
	return checkBlacklist(ctx, svcCtx, email, repository.BlacklistIdentifierTypeEmail)
}

// CheckPhoneBlacklist 检查手机号是否在黑名单中
// phone 应该包含完整格式（如 +8613800000000）
func CheckPhoneBlacklist(ctx context.Context, svcCtx *svc.ServiceContext, phone string) (*BlacklistCheckResult, error) {
	return checkBlacklist(ctx, svcCtx, phone, repository.BlacklistIdentifierTypePhone)
}

// CheckGoogleBlacklist 检查 Google ID 是否在黑名单中
func CheckGoogleBlacklist(ctx context.Context, svcCtx *svc.ServiceContext, googleID string) (*BlacklistCheckResult, error) {
	return checkBlacklist(ctx, svcCtx, googleID, repository.BlacklistIdentifierTypeGoogle)
}

// checkBlacklist 通用黑名单检查
func checkBlacklist(ctx context.Context, svcCtx *svc.ServiceContext, identifier string, identifierType int32) (*BlacklistCheckResult, error) {
	if svcCtx.BlacklistRepository == nil {
		// 如果没有配置黑名单仓库，默认不在黑名单中
		return &BlacklistCheckResult{IsBlacklisted: false}, nil
	}

	info, err := svcCtx.BlacklistRepository.GetBlacklistInfo(ctx, identifier, identifierType)
	if err != nil {
		return nil, err
	}

	if !info.IsBlacklisted {
		return &BlacklistCheckResult{IsBlacklisted: false}, nil
	}

	return &BlacklistCheckResult{
		IsBlacklisted: true,
		Message:       formatLoginBlockedMessage(info.BlockedAt),
		BlockedAt:     info.BlockedAt,
	}, nil
}

// formatLoginBlockedMessage 格式化登录被阻止的消息
// 登录流程：您的账号已于{时间}被风控锁定，目前不可登录，请联系钱包管理员
func formatLoginBlockedMessage(blockedAt time.Time) string {
	timeStr := blockedAt.Format("2006-01-02 15:04:05")
	return fmt.Sprintf("您的账号已于%s被风控锁定，目前不可登录，请联系钱包管理员", timeStr)
}

// FormatRegisterBlockedMessage 格式化注册被阻止的消息
// 注册流程：该手机账号/邮箱存在异常，目前不可注册，请联系钱包管理员
func FormatRegisterBlockedMessage() string {
	return "该手机账号/邮箱存在异常，目前不可注册，请联系钱包管理员"
}

// FormatSendCodeBlockedMessage 格式化发送验证码被阻止的消息
func FormatSendCodeBlockedMessage() string {
	return "该账号存在异常，无法发送验证码，请联系钱包管理员"
}
