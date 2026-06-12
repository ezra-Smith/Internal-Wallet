package jpush

import (
	"errors"
	"fmt"
)

var (
	// ErrInvalidAppKey 无效的AppKey
	ErrInvalidAppKey = errors.New("invalid jpush app key")

	// ErrInvalidMasterSecret 无效的MasterSecret
	ErrInvalidMasterSecret = errors.New("invalid jpush master secret")

	// ErrEmptyTarget 推送目标为空
	ErrEmptyTarget = errors.New("push target is empty")

	// ErrEmptyContent 推送内容为空
	ErrEmptyContent = errors.New("push content is empty")

	// ErrInvalidRegistrationID 无效的RegistrationID
	ErrInvalidRegistrationID = errors.New("invalid registration id")

	// ErrTooManyRegistrationIDs Registration ID数量超过限制（每次最多1000个）
	ErrTooManyRegistrationIDs = errors.New("too many registration ids (max 1000)")

	// ErrTooManyAliases 别名数量超过限制（每次最多1000个）
	ErrTooManyAliases = errors.New("too many aliases (max 1000)")

	// ErrAliasTooLong 别名长度超过限制（最大40字节）
	ErrAliasTooLong = errors.New("alias too long (max 40 bytes)")

	// ErrNetworkTimeout 网络超时
	ErrNetworkTimeout = errors.New("jpush network timeout")

	// ErrRateLimitExceeded 超过频率限制
	ErrRateLimitExceeded = errors.New("jpush rate limit exceeded")

	// ErrAuthenticationFailed 认证失败
	ErrAuthenticationFailed = errors.New("jpush authentication failed")

	// ErrInvalidDevice 无效的设备
	ErrInvalidDevice = errors.New("invalid device")
)

// JPushError 极光推送错误
type JPushError struct {
	Code       int    // 错误码
	Message    string // 错误消息
	StatusCode int    // HTTP状态码
}

func (e *JPushError) Error() string {
	return fmt.Sprintf("jpush error (code=%d, status=%d): %s", e.Code, e.StatusCode, e.Message)
}

// NewJPushError 创建极光推送错误
func NewJPushError(code, statusCode int, message string) error {
	return &JPushError{
		Code:       code,
		Message:    message,
		StatusCode: statusCode,
	}
}

// IsRetryableError 判断是否为可重试的错误
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// 网络超时错误可以重试
	if errors.Is(err, ErrNetworkTimeout) {
		return true
	}

	// 频率限制错误可以重试（延迟后）
	if errors.Is(err, ErrRateLimitExceeded) {
		return true
	}

	// 极光推送错误码判断
	var jpushErr *JPushError
	if errors.As(err, &jpushErr) {
		// 5xx服务器错误可以重试
		if jpushErr.StatusCode >= 500 && jpushErr.StatusCode < 600 {
			return true
		}
		// 429频率限制可以重试
		if jpushErr.StatusCode == 429 {
			return true
		}
	}

	return false
}

// IsAuthError 判断是否为认证错误
func IsAuthError(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, ErrAuthenticationFailed) {
		return true
	}

	var jpushErr *JPushError
	if errors.As(err, &jpushErr) {
		// 401未授权
		return jpushErr.StatusCode == 401
	}

	return false
}

// IsInvalidDeviceError 判断是否为无效设备错误
func IsInvalidDeviceError(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, ErrInvalidDevice) {
		return true
	}

	var jpushErr *JPushError
	if errors.As(err, &jpushErr) {
		// 极光错误码1011表示无效的registration_id
		return jpushErr.Code == 1011
	}

	return false
}
