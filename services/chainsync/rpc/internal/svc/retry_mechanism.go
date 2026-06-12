package svc

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
)

// RetryPolicy 重试策略
type RetryPolicy struct {
	// MaxAttempts 最大重试次数
	MaxAttempts int
	// InitialDelay 初始延迟
	InitialDelay time.Duration
	// MaxDelay 最大延迟
	MaxDelay time.Duration
	// Multiplier 延迟倍数
	Multiplier float64
	// RandomizationFactor 随机化因子 (0-1)
	RandomizationFactor float64
	// BackoffType 退避类型
	BackoffType BackoffType
}

// BackoffType 退避类型
type BackoffType string

const (
	// BackoffTypeFixed 固定延迟
	BackoffTypeFixed BackoffType = "fixed"
	// BackoffTypeLinear 线性退避
	BackoffTypeLinear BackoffType = "linear"
	// BackoffTypeExponential 指数退避
	BackoffTypeExponential BackoffType = "exponential"
	// BackoffTypeFibonacci 斐波那契退避
	BackoffTypeFibonacci BackoffType = "fibonacci"
)

// DefaultRetryPolicy 默认重试策略
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts:         3,
		InitialDelay:        100 * time.Millisecond,
		MaxDelay:            5 * time.Second,
		Multiplier:          2.0,
		RandomizationFactor: 0.1,
		BackoffType:         BackoffTypeExponential,
	}
}

// RetryableError 可重试的错误
type RetryableError interface {
	error
	IsRetryable() bool
}

// retryableError 可重试错误实现
type retryableError struct {
	err       error
	retryable bool
}

func (e *retryableError) Error() string {
	return e.err.Error()
}

func (e *retryableError) IsRetryable() bool {
	return e.retryable
}

// NewRetryableError 创建可重试错误
func NewRetryableError(err error, retryable bool) RetryableError {
	return &retryableError{
		err:       err,
		retryable: retryable,
	}
}

// RetryFunc 重试函数类型
type RetryFunc func(ctx context.Context) error

// RetryMechanism 重试机制
type RetryMechanism struct {
	policy RetryPolicy
	rng    *rand.Rand
}

// NewRetryMechanism 创建重试机制
func NewRetryMechanism(policy RetryPolicy) *RetryMechanism {
	return &RetryMechanism{
		policy: policy,
		rng:    rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Execute 执行带重试的操作
func (rm *RetryMechanism) Execute(ctx context.Context, fn RetryFunc) error {
	var lastErr error

	for attempt := 0; attempt < rm.policy.MaxAttempts; attempt++ {
		if attempt > 0 {
			// 计算延迟时间
			delay := rm.calculateDelay(attempt)

			// 等待延迟
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
				// 继续执行
			}

			logx.Infof("Retrying operation (attempt %d/%d) after %v delay",
				attempt+1, rm.policy.MaxAttempts, delay)
		}

		// 执行函数
		err := fn(ctx)
		if err == nil {
			if attempt > 0 {
				logx.Infof("Operation succeeded after %d attempts", attempt+1)
			}
			return nil
		}

		lastErr = err

		// 检查是否可重试
		if !rm.isRetryable(err) {
			logx.Errorf("Operation failed with non-retryable error: %v", err)
			return err
		}

		// 检查上下文是否已取消
		if ctx.Err() != nil {
			return ctx.Err()
		}

		logx.Errorf("Operation attempt %d failed: %v", attempt+1, err)
	}

	return fmt.Errorf("operation failed after %d attempts, last error: %v",
		rm.policy.MaxAttempts, lastErr)
}

// calculateDelay 计算延迟时间
func (rm *RetryMechanism) calculateDelay(attempt int) time.Duration {
	var delay time.Duration

	switch rm.policy.BackoffType {
	case BackoffTypeFixed:
		delay = rm.policy.InitialDelay
	case BackoffTypeLinear:
		delay = time.Duration(attempt) * rm.policy.InitialDelay
	case BackoffTypeExponential:
		delay = time.Duration(float64(rm.policy.InitialDelay) *
			math.Pow(rm.policy.Multiplier, float64(attempt-1)))
	case BackoffTypeFibonacci:
		delay = rm.fibonacciDelay(attempt)
	default:
		delay = rm.policy.InitialDelay
	}

	// 应用随机化因子
	if rm.policy.RandomizationFactor > 0 {
		randomization := rm.policy.RandomizationFactor * float64(delay)
		delay += time.Duration(rm.rng.Float64()*2*randomization - randomization)
	}

	// 确保不超过最大延迟
	if delay > rm.policy.MaxDelay {
		delay = rm.policy.MaxDelay
	}

	return delay
}

// fibonacciDelay 计算斐波那契延迟
func (rm *RetryMechanism) fibonacciDelay(attempt int) time.Duration {
	if attempt <= 1 {
		return rm.policy.InitialDelay
	}

	// 计算斐波那契数
	fib := rm.fibonacci(attempt)
	return time.Duration(float64(rm.policy.InitialDelay) * float64(fib))
}

// fibonacci 计算斐波那契数
func (rm *RetryMechanism) fibonacci(n int) int {
	if n <= 1 {
		return n
	}

	a, b := 0, 1
	for i := 2; i <= n; i++ {
		a, b = b, a+b
	}
	return b
}

// isRetryable 检查错误是否可重试
func (rm *RetryMechanism) isRetryable(err error) bool {
	if retryableErr, ok := err.(RetryableError); ok {
		return retryableErr.IsRetryable()
	}

	// 检查常见可重试错误
	errStr := strings.ToLower(err.Error())

	// 网络相关错误
	networkErrors := []string{
		"connection refused",
		"connection reset",
		"connection timeout",
		"network is unreachable",
		"temporary failure",
		"timeout",
		"deadline exceeded",
	}

	for _, netErr := range networkErrors {
		if strings.Contains(errStr, netErr) {
			return true
		}
	}

	// HTTP 状态码相关错误
	if strings.Contains(errStr, "status:") {
		retryableStatusCodes := []string{
			"status: 408", // Request Timeout
			"status: 429", // Too Many Requests
			"status: 500", // Internal Server Error
			"status: 502", // Bad Gateway
			"status: 503", // Service Unavailable
			"status: 504", // Gateway Timeout
			"status: 507", // Insufficient Storage
			"status: 509", // Bandwidth Limit Exceeded
		}

		for _, code := range retryableStatusCodes {
			if strings.Contains(errStr, code) {
				return true
			}
		}
	}

	// 上下文取消错误不可重试
	if strings.Contains(errStr, "context canceled") {
		return false
	}

	return true
}

// RetryConfig 重试配置
type RetryConfig struct {
	// GetLatestBlock 获取最新区块的重试配置
	GetLatestBlock RetryPolicy
	// GetBlock 获取区块信息的重试配置
	GetBlock RetryPolicy
	// GetTransaction 获取交易信息的重试配置
	GetTransaction RetryPolicy
	// GetAddressBalance 获取地址余额的重试配置
	GetAddressBalance RetryPolicy
	// GetAddressTransactions 获取地址交易历史的重试配置
	GetAddressTransactions RetryPolicy
	// HealthCheck 健康检查的重试配置
	HealthCheck RetryPolicy
}

// DefaultRetryConfig 默认重试配置
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		GetLatestBlock: RetryPolicy{
			MaxAttempts:         3,
			InitialDelay:        200 * time.Millisecond,
			MaxDelay:            2 * time.Second,
			Multiplier:          2.0,
			RandomizationFactor: 0.1,
			BackoffType:         BackoffTypeExponential,
		},
		GetBlock: RetryPolicy{
			MaxAttempts:         3,
			InitialDelay:        300 * time.Millisecond,
			MaxDelay:            3 * time.Second,
			Multiplier:          2.0,
			RandomizationFactor: 0.1,
			BackoffType:         BackoffTypeExponential,
		},
		GetTransaction: RetryPolicy{
			MaxAttempts:         3,
			InitialDelay:        200 * time.Millisecond,
			MaxDelay:            2 * time.Second,
			Multiplier:          2.0,
			RandomizationFactor: 0.1,
			BackoffType:         BackoffTypeExponential,
		},
		GetAddressBalance: RetryPolicy{
			MaxAttempts:         3,
			InitialDelay:        300 * time.Millisecond,
			MaxDelay:            3 * time.Second,
			Multiplier:          2.0,
			RandomizationFactor: 0.1,
			BackoffType:         BackoffTypeExponential,
		},
		GetAddressTransactions: RetryPolicy{
			MaxAttempts:         2, // 批量操作减少重试次数
			InitialDelay:        500 * time.Millisecond,
			MaxDelay:            5 * time.Second,
			Multiplier:          1.5,
			RandomizationFactor: 0.1,
			BackoffType:         BackoffTypeExponential,
		},
		HealthCheck: RetryPolicy{
			MaxAttempts:         2,
			InitialDelay:        1 * time.Second,
			MaxDelay:            10 * time.Second,
			Multiplier:          2.0,
			RandomizationFactor: 0.1,
			BackoffType:         BackoffTypeExponential,
		},
	}
}
