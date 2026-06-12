package svc

import (
	"context"
	"fmt"
	"strings"
	"time"

	"internalwallet/services/chainsync/rpc/internal/config"

	"github.com/zeromicro/go-zero/core/logx"
)

// AddressRetryConfig 地址监控器重试配置
type AddressRetryConfig struct {
	MaxRetries    int           // 最大重试次数
	InitialDelay  time.Duration // 初始重试延迟
	MaxDelay      time.Duration // 最大重试延迟
	BackoffFactor float64       // 退避因子
	TimeoutFactor int           // 超时时间倍数因子
}

// initRetryConfig 初始化重试配置
func initRetryConfig(config *config.Config) AddressRetryConfig {
	retryConfig := AddressRetryConfig{
		MaxRetries:    3,                // 默认3次重试
		InitialDelay:  1 * time.Second,  // 初始1秒延迟
		MaxDelay:      30 * time.Second, // 最大30秒延迟
		BackoffFactor: 2.0,              // 指数退避因子
		TimeoutFactor: 2,                // 超时时间倍数
	}

	// 从配置中读取重试设置
	if config != nil {
		if config.Sync.DefaultRetryCount > 0 {
			retryConfig.MaxRetries = config.Sync.DefaultRetryCount
		}
		if config.Sync.MaxRetryDelay > 0 {
			retryConfig.MaxDelay = time.Duration(config.Sync.MaxRetryDelay) * time.Second
		}

	}

	return retryConfig
}

// RetryFunction 重试函数类型
type RetryFunction func(ctx context.Context) error

// ExecuteWithRetry 带重试机制的执行函数
func (am *AddressMonitor) ExecuteWithRetry(ctx context.Context, operation string, fn RetryFunction) error {
	delay := am.retryConfig.InitialDelay

	for attempt := 1; attempt <= am.retryConfig.MaxRetries; attempt++ {
		err := fn(ctx)
		if err == nil {
			if attempt > 1 {
				logx.Infof("✅ %s succeeded on attempt %d", operation, attempt)
			}
			return nil
		}

		// 检查是否为不可重试的错误
		if am.isNonRetryableError(err) {
			logx.Errorf("❌ %s failed with non-retryable error: %v", operation, err)
			return err
		}

		if attempt < am.retryConfig.MaxRetries {
			logx.Infof("⚠️ %s failed on attempt %d/%d: %v, retrying in %v...",
				operation, attempt, am.retryConfig.MaxRetries, err, delay)

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
				// 指数退避
				delay = time.Duration(float64(delay) * am.retryConfig.BackoffFactor)
				if delay > am.retryConfig.MaxDelay {
					delay = am.retryConfig.MaxDelay
				}
			}
		}
	}

	return fmt.Errorf("%s failed after %d attempts", operation, am.retryConfig.MaxRetries)
}

// isNonRetryableError 判断是否为不可重试的错误
func (am *AddressMonitor) isNonRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// 不可重试的错误类型
	nonRetryableErrors := []string{
		"invalid address",
		"invalid parameters",
		"unauthorized",
		"forbidden",
		"not found",
		"invalid range",
	}

	for _, nonRetryable := range nonRetryableErrors {
		if strings.Contains(strings.ToLower(errStr), nonRetryable) {
			return true
		}
	}

	// context相关错误
	if strings.Contains(errStr, "context") {
		return true
	}

	return false
}

// GetTimeoutWithFactor 获取带倍数因子的超时时间
func (am *AddressMonitor) GetTimeoutWithFactor(baseTimeout time.Duration) time.Duration {
	return time.Duration(am.retryConfig.TimeoutFactor) * baseTimeout
}
