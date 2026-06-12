package httpx

import (
	"encoding/json"
	"net/http"
	"time"

	"internalwallet/common/errcode"
)

const RequestIDHeader = "X-Request-ID"

// UnifiedResponse is the only HTTP envelope allowed to be returned by API Gateway.
//
// Field names must stay stable; do not rename without a versioned migration.
type UnifiedResponse struct {
	Success   bool        `json:"success"`
	Code      int         `json:"code"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data,omitempty"`
	RequestID string      `json:"request_id"`
	Timestamp string      `json:"timestamp"`
	Details   interface{} `json:"details,omitempty"`
}

func NowTimestamp() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func BizCodeFromHTTPStatus(status int) int {
	switch status {
	case http.StatusBadRequest:
		return int(errcode.CodeInvalidParam)
	case http.StatusUnauthorized:
		return int(errcode.CodeUnauthorized)
	case http.StatusForbidden:
		return int(errcode.CodeForbidden)
	case http.StatusNotFound:
		return int(errcode.CodeNotFound)
	case http.StatusConflict:
		return int(errcode.CodeConflict)
	case http.StatusTooManyRequests:
		return int(errcode.CodeTooManyRequests)
	case http.StatusServiceUnavailable:
		return int(errcode.CodeServiceUnavail)
	case http.StatusRequestTimeout:
		return int(errcode.CodeTimeout)
	case http.StatusMethodNotAllowed, http.StatusUnprocessableEntity:
		return int(errcode.CodeInvalidOperation)
	default:
		return int(errcode.CodeInternalError)
	}
}

// HTTPStatusFromBizCode 根据业务错误码返回 HTTP 状态码
//
// 设计说明：
// - 业务错误统一返回 HTTP 200，前端通过响应体中的 code 字段判断是否成功
// - 只有真正的服务器内部错误（内部错误、服务不可用、超时）才返回 HTTP 5xx
// - 这样前端处理更简单，避免在 HTTP 层和业务层分别处理错误
func HTTPStatusFromBizCode(code int) int {
	switch errcode.ErrorCode(code) {
	// 服务器内部错误 - 返回 5xx
	case errcode.CodeInternalError, errcode.CodeDatabaseError, errcode.CodeCacheError:
		return http.StatusInternalServerError
	case errcode.CodeServiceUnavail:
		return http.StatusServiceUnavailable
	case errcode.CodeTimeout:
		return http.StatusGatewayTimeout
	// 所有业务错误 - 统一返回 200
	default:
		return http.StatusOK
	}
}

func Write(w http.ResponseWriter, status int, resp UnifiedResponse) error {
	if resp.Timestamp == "" {
		resp.Timestamp = NowTimestamp()
	}

	if ct := w.Header().Get("Content-Type"); ct == "" {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
	}
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(resp)
}
