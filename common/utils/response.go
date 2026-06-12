package utils

import (
	"encoding/json"
	"net/http"
)

// 错误码常量
const (
	CodeSuccess            = 0   // 成功
	CodeInvalidParam       = 400 // 参数错误
	CodeUnauthorized       = 401 // 未授权
	CodeForbidden          = 403 // 禁止访问
	CodeNotFound           = 404 // 资源不存在
	CodeInternalError      = 500 // 内部错误
	CodeServiceUnavailable = 503 // 服务不可用

	// 业务错误码 (10000+)
	CodeUserNotFound        = 10001 // 用户不存在
	CodePasswordError       = 10002 // 密码错误
	CodeVerifyCodeError     = 10003 // 验证码错误
	CodeTokenExpired        = 10004 // Token过期
	Code2FARequired         = 10005 // 需要2FA验证
	Code2FAInvalid          = 10006 // 2FA验证失败
	CodeInsufficientBalance = 20001 // 余额不足
)

// Response 统一响应结构
type Response struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data,omitempty"`
}

// Success 成功响应
func Success(w http.ResponseWriter, data interface{}) {
	resp := Response{
		Code: CodeSuccess,
		Msg:  "success",
		Data: data,
	}
	writeJSON(w, http.StatusOK, resp)
}

// SuccessWithMsg 成功响应（自定义消息）
func SuccessWithMsg(w http.ResponseWriter, msg string, data interface{}) {
	resp := Response{
		Code: CodeSuccess,
		Msg:  msg,
		Data: data,
	}
	writeJSON(w, http.StatusOK, resp)
}

// Error 错误响应
func Error(w http.ResponseWriter, code int, msg string) {
	resp := Response{
		Code: code,
		Msg:  msg,
	}
	writeJSON(w, http.StatusOK, resp)
}

// ErrorWithData 错误响应（带数据）
func ErrorWithData(w http.ResponseWriter, code int, msg string, data interface{}) {
	resp := Response{
		Code: code,
		Msg:  msg,
		Data: data,
	}
	writeJSON(w, http.StatusOK, resp)
}

// 常用错误响应快捷方法
func InvalidParam(w http.ResponseWriter, msg string) {
	Error(w, CodeInvalidParam, msg)
}

func Unauthorized(w http.ResponseWriter, msg string) {
	Error(w, CodeUnauthorized, msg)
}

func InternalError(w http.ResponseWriter, msg string) {
	Error(w, CodeInternalError, msg)
}

// writeJSON 写入JSON响应
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
