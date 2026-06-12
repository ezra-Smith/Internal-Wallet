package errx

import (
	"strconv"
	"strings"

	"internalwallet/common/errcode"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// BizCode 业务错误码类型别名，使用 common/errcode 中的定义
// 为了兼容现有代码，保留此类型别名
type BizCode = errcode.ErrorCode

// 错误码常量别名，使用 common/errcode 中的统一定义
const (
	// 通用
	CodeInvalidParam  = errcode.CodeInvalidParam
	CodeUnauthorized  = errcode.CodeUnauthorized
	CodeForbidden     = errcode.CodeForbidden
	CodeNotFound      = errcode.CodeNotFound
	CodeConflict      = errcode.CodeConflict
	CodeTooManyReq    = errcode.CodeTooManyRequests
	CodeInternalError = errcode.CodeInternalError

	// Auth
	CodeAuthInvalidCredentials = errcode.CodeAuthInvalidCredentials
	CodeAuthAccountLocked      = errcode.CodeAuthAccountLocked
	CodeAuthCaptchaRequired    = errcode.CodeAuthCaptchaRequired
	CodeAuthCaptchaInvalid     = errcode.CodeAuthCaptchaInvalid
	CodeAuth2FATokenExpired    = errcode.CodeAuth2FATokenExpired
	CodeAuth2FACodeInvalid     = errcode.CodeAuth2FACodeInvalid
	CodeAuth2FATooManyAttempts = errcode.CodeAuth2FATooManyAttempts
	CodeAuthRefreshNotAllowed  = errcode.CodeAuthRefreshNotAllowed
	CodeAuthTokenRevoked       = errcode.CodeAuthTokenRevoked
	CodeAuthPasswordChangeReq  = errcode.CodeAuthPasswordChangeReq
	CodeAuthTwoFABindReq       = errcode.CodeAuthTwoFABindReq
	CodeAuthWalletInitReq      = errcode.CodeAuthWalletInitReq

	// Admin Users / RBAC
	CodeCannotDisableSelf    = errcode.CodeCannotDisableSelf
	CodeLastSuperAdmin       = errcode.CodeLastSuperAdmin
	CodePasswordTooWeak      = errcode.CodePasswordTooWeak
	CodePasswordMismatch     = errcode.CodePasswordMismatch
	CodePasswordRecentlyUsed = errcode.CodePasswordRecentlyUsed
)

// New 创建带 ErrorInfo/BadRequest details 的 gRPC error
// httpStatus: 目标 HTTP 状态码（由 api-gateway 根据 metadata 覆盖）
func New(grpcCode codes.Code, httpStatus int, bizCode BizCode, reason string, message string, fieldViolations map[string]string) error {
	st := status.New(grpcCode, message)

	meta := map[string]string{
		"code":        strconv.Itoa(int(bizCode)),
		"http_status": strconv.Itoa(httpStatus),
	}
	// Optional structured metadata for localization / debugging.
	if reason == "DB_SCHEMA_MISSING" {
		// Expected message example:
		//   "rbac schema not initialized (missing table: admin_menu)"
		if table := extractMissingTable(message); table != "" {
			meta["table"] = table
		}
		// Expected message example:
		//   "security schema not initialized (missing column: two_factor_pending_secret)"
		if column := extractMissingColumn(message); column != "" {
			meta["column"] = column
		}
	}

	info := &errdetails.ErrorInfo{
		Reason:   reason,
		Metadata: meta,
	}
	withDetails, err := st.WithDetails(info)
	if err != nil {
		return st.Err()
	}

	if len(fieldViolations) > 0 {
		br := &errdetails.BadRequest{}
		for field, desc := range fieldViolations {
			br.FieldViolations = append(br.FieldViolations, &errdetails.BadRequest_FieldViolation{
				Field:       field,
				Description: desc,
			})
		}
		withDetails, _ = withDetails.WithDetails(br)
	}

	return withDetails.Err()
}

// NewWithMetadata is like New, but allows attaching additional ErrorInfo metadata.
// Reserved keys "code" / "http_status" are ignored from extraMetadata.
func NewWithMetadata(grpcCode codes.Code, httpStatus int, bizCode BizCode, reason string, message string, fieldViolations map[string]string, extraMetadata map[string]string) error {
	st := status.New(grpcCode, message)

	meta := map[string]string{
		"code":        strconv.Itoa(int(bizCode)),
		"http_status": strconv.Itoa(httpStatus),
	}
	for k, v := range extraMetadata {
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if k == "" || v == "" {
			continue
		}
		if k == "code" || k == "http_status" {
			continue
		}
		meta[k] = v
	}

	// Optional structured metadata for localization / debugging.
	if reason == "DB_SCHEMA_MISSING" {
		// Expected message example:
		//   "rbac schema not initialized (missing table: admin_menu)"
		if _, ok := meta["table"]; !ok {
			if table := extractMissingTable(message); table != "" {
				meta["table"] = table
			}
		}
		// Expected message example:
		//   "security schema not initialized (missing column: two_factor_pending_secret)"
		if _, ok := meta["column"]; !ok {
			if column := extractMissingColumn(message); column != "" {
				meta["column"] = column
			}
		}
	}

	info := &errdetails.ErrorInfo{
		Reason:   reason,
		Metadata: meta,
	}
	withDetails, err := st.WithDetails(info)
	if err != nil {
		return st.Err()
	}

	if len(fieldViolations) > 0 {
		br := &errdetails.BadRequest{}
		for field, desc := range fieldViolations {
			br.FieldViolations = append(br.FieldViolations, &errdetails.BadRequest_FieldViolation{
				Field:       field,
				Description: desc,
			})
		}
		withDetails, _ = withDetails.WithDetails(br)
	}

	return withDetails.Err()
}

func extractMissingTable(message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return ""
	}
	const marker = "missing table:"
	idx := strings.Index(strings.ToLower(message), marker)
	if idx < 0 {
		return ""
	}
	s := message[idx+len(marker):]
	// Trim common wrappers: ")" / "]" / "." etc.
	s = strings.TrimSpace(s)
	s = strings.TrimRight(s, ").] ,")
	// Keep only the last token to avoid returning full sentences.
	if fields := strings.Fields(s); len(fields) > 0 {
		return fields[0]
	}
	return s
}

func extractMissingColumn(message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return ""
	}
	const marker = "missing column:"
	idx := strings.Index(strings.ToLower(message), marker)
	if idx < 0 {
		return ""
	}
	s := message[idx+len(marker):]
	s = strings.TrimSpace(s)
	s = strings.TrimRight(s, ").] ,")
	if fields := strings.Fields(s); len(fields) > 0 {
		return fields[0]
	}
	return s
}

// ========================
// 快捷错误创建函数
// ========================
//
// 设计说明：
// - 业务错误统一返回 HTTP 200，前端通过 code 字段判断是否成功
// - 只有真正的服务器内部错误（panic、服务不可达等）才返回 HTTP 5xx
// - 推荐新代码使用这些快捷函数，而不是直接调用 New()

// InvalidParam 创建参数错误
func InvalidParam(reason string, message string, fields map[string]string) error {
	return New(codes.InvalidArgument, 200, CodeInvalidParam, reason, message, fields)
}

// Unauthorized 创建未授权错误
func Unauthorized(reason string, message string) error {
	return New(codes.Unauthenticated, 200, CodeUnauthorized, reason, message, nil)
}

// Forbidden 创建禁止访问错误
func Forbidden(reason string, message string) error {
	return New(codes.PermissionDenied, 200, CodeForbidden, reason, message, nil)
}

// NotFound 创建资源不存在错误
func NotFound(reason string, message string) error {
	return New(codes.NotFound, 200, CodeNotFound, reason, message, nil)
}

// Conflict 创建资源冲突错误
func Conflict(reason string, message string) error {
	return New(codes.AlreadyExists, 200, CodeConflict, reason, message, nil)
}

// Internal 创建内部错误（保持 500，因为这是真正的服务器异常）
func Internal(reason string, message string) error {
	return New(codes.Internal, 500, CodeInternalError, reason, message, nil)
}

// TooManyRequests 创建请求频率限制错误
func TooManyRequests(reason string, message string) error {
	return New(codes.ResourceExhausted, 200, CodeTooManyReq, reason, message, nil)
}
