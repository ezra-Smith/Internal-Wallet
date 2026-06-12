package logic

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"internalwallet/common/security"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
)

const (
	// TradePasswordMaxAttempts 交易密码最大尝试次数
	TradePasswordMaxAttempts = 3
	// TradePasswordLockDuration 锁定时长（失败次数记录的过期时间）
	TradePasswordLockDuration = 24 * time.Hour
	// TradePasswordAttemptsKeyPrefix Redis key 前缀
	TradePasswordAttemptsKeyPrefix = "trade_pwd_attempts:"
)

// TradePasswordVerifyResult 交易密码验证结果
type TradePasswordVerifyResult struct {
	Valid             bool   // 密码是否正确
	RemainingAttempts int    // 剩余尝试次数
	AccountLocked     bool   // 账户是否已被锁定
	Message           string // 提示信息
}

// VerifyTradePasswordWithLimit 验证交易密码（带尝试次数限制）
// 如果密码错误且达到最大尝试次数，会自动冻结用户账户
func VerifyTradePasswordWithLimit(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	userID int64,
	tradePassword string,
	tradePasswordHash string,
	scene string, // 场景：internal_transfer, withdraw, verify 等
) (*TradePasswordVerifyResult, error) {
	logger := logx.WithContext(ctx)

	// 1. 检查当前失败次数
	attemptsKey := fmt.Sprintf("%s%d", TradePasswordAttemptsKeyPrefix, userID)
	currentAttempts := 0

	if svcCtx.RedisClient != nil {
		val, err := svcCtx.RedisClient.Get(ctx, attemptsKey).Result()
		if err == nil {
			currentAttempts, _ = strconv.Atoi(val)
		} else if err != redis.Nil {
			logger.Errorf("Redis get trade password attempts failed: %v", err)
		}
	}

	// 2. 如果已达到最大尝试次数，直接返回锁定状态
	if currentAttempts >= TradePasswordMaxAttempts {
		logger.Infof("User %d trade password already locked (attempts=%d)", userID, currentAttempts)
		return &TradePasswordVerifyResult{
			Valid:             false,
			RemainingAttempts: 0,
			AccountLocked:     true,
			Message:           "交易密码已锁定，请联系客服",
		}, nil
	}

	// 3. 验证密码
	isValid := security.VerifyPassword(tradePasswordHash, tradePassword)

	if isValid {
		// 密码正确，清除失败次数
		if svcCtx.RedisClient != nil {
			_ = svcCtx.RedisClient.Del(ctx, attemptsKey)
		}
		logger.Infof("User %d trade password verified successfully (scene=%s)", userID, scene)
		return &TradePasswordVerifyResult{
			Valid:             true,
			RemainingAttempts: TradePasswordMaxAttempts,
			AccountLocked:     false,
			Message:           "ok",
		}, nil
	}

	// 4. 密码错误，增加失败次数
	newAttempts := currentAttempts + 1
	if svcCtx.RedisClient != nil {
		pipe := svcCtx.RedisClient.Pipeline()
		pipe.Incr(ctx, attemptsKey)
		pipe.Expire(ctx, attemptsKey, TradePasswordLockDuration)
		_, err := pipe.Exec(ctx)
		if err != nil {
			logger.Errorf("Redis incr trade password attempts failed: %v", err)
		}
	}

	remainingAttempts := TradePasswordMaxAttempts - newAttempts
	if remainingAttempts < 0 {
		remainingAttempts = 0
	}

	logger.Infof("User %d trade password verification failed (scene=%s, attempts=%d/%d)",
		userID, scene, newAttempts, TradePasswordMaxAttempts)

	// 5. 达到最大尝试次数，冻结账户
	if newAttempts >= TradePasswordMaxAttempts {
		logger.Errorf("User %d reached max trade password attempts, freezing account", userID)

		// 调用 Admin RPC 冻结用户
		if svcCtx.AdminRpc != nil {
			freezeResp, err := svcCtx.AdminRpc.FreezeUser(ctx, &pb.FreezeUserRequest{
				Uid:          strconv.FormatInt(userID, 10),
				FreezeAssets: true, // 同时冻结资产
				Reason:       fmt.Sprintf("交易密码连续错误%d次，系统自动冻结", TradePasswordMaxAttempts),
			})
			if err != nil {
				logger.Errorf("Failed to freeze user %d: %v", userID, err)
			} else if freezeResp != nil && freezeResp.Success {
				logger.Infof("User %d account frozen due to trade password attempts exceeded", userID)
			}
		}

		return &TradePasswordVerifyResult{
			Valid:             false,
			RemainingAttempts: 0,
			AccountLocked:     true,
			Message:           "交易密码错误次数过多，账户已被锁定，请联系客服",
		}, nil
	}

	// 6. 返回错误信息（包含剩余次数）
	return &TradePasswordVerifyResult{
		Valid:             false,
		RemainingAttempts: remainingAttempts,
		AccountLocked:     false,
		Message:           fmt.Sprintf("交易密码错误，请输入正确密码，再输错%d次账户将会锁定", remainingAttempts),
	}, nil
}

// GetTradePasswordRemainingAttempts 获取剩余尝试次数
func GetTradePasswordRemainingAttempts(ctx context.Context, svcCtx *svc.ServiceContext, userID int64) int {
	if svcCtx.RedisClient == nil {
		return TradePasswordMaxAttempts
	}

	attemptsKey := fmt.Sprintf("%s%d", TradePasswordAttemptsKeyPrefix, userID)
	val, err := svcCtx.RedisClient.Get(ctx, attemptsKey).Result()
	if err != nil {
		return TradePasswordMaxAttempts
	}

	currentAttempts, _ := strconv.Atoi(val)
	remaining := TradePasswordMaxAttempts - currentAttempts
	if remaining < 0 {
		return 0
	}
	return remaining
}

// ResetTradePasswordAttempts 重置交易密码尝试次数（管理员解锁时调用）
func ResetTradePasswordAttempts(ctx context.Context, svcCtx *svc.ServiceContext, userID int64) error {
	if svcCtx.RedisClient == nil {
		return nil
	}

	attemptsKey := fmt.Sprintf("%s%d", TradePasswordAttemptsKeyPrefix, userID)
	return svcCtx.RedisClient.Del(ctx, attemptsKey).Err()
}

// VerifyTradePasswordSimple 简化版交易密码验证（返回 error）
// 用于替换现有的简单验证逻辑
func VerifyTradePasswordSimple(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	userID int64,
	tradePassword string,
	tradePasswordHash string,
	scene string,
) error {
	result, err := VerifyTradePasswordWithLimit(ctx, svcCtx, userID, tradePassword, tradePasswordHash, scene)
	if err != nil {
		return err
	}

	if result.AccountLocked {
		return errx.TradePasswordLocked()
	}

	if !result.Valid {
		return errx.TradePasswordWrongWithRemaining(result.RemainingAttempts)
	}

	return nil
}
