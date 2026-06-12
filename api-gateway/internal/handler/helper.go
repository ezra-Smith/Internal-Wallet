package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"internalwallet/common/errcode"
	"internalwallet/common/httpx"
	"internalwallet/common/i18n"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// RPCHandler 通用的 RPC 调用包装器（使用泛型）
type RPCHandler[Req any, Resp any] struct {
	Method     string
	Call       func(context.Context, *Req) (*Resp, error)
	PathParams []string // 路径参数名称，如 ["id", "type"]
}

// ServeHTTP 实现 http.Handler 接口
func (h *RPCHandler[Req, Resp]) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 1. 验证 HTTP Method
	if r.Method != h.Method {
		WriteError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 2. 解析请求参数
	var req Req

	// 解析路径参数（如果有）
	if len(h.PathParams) > 0 {
		pathVars := ExtractPathParams(r.Context())
		if pathVars != nil {
			if err := SetStructFields(&req, pathVars); err != nil {
				WriteError(w, fmt.Sprintf("Invalid path params: %v", err), http.StatusBadRequest)
				return
			}
		}
	}

	// 根据请求方法解析参数
	if h.Method == http.MethodGet || h.Method == http.MethodDelete {
		// GET/DELETE 从 Query 参数解析
		if err := ParseQuery(r, &req); err != nil {
			WriteError(w, fmt.Sprintf("Invalid query params: %v", err), http.StatusBadRequest)
			return
		}
	} else {
		// POST/PUT/PATCH 从 Body 解析
		body, err := io.ReadAll(r.Body)
		if err != nil {
			WriteError(w, "Failed to read request body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		if len(body) > 0 {
			if err := json.Unmarshal(body, &req); err != nil {
				WriteError(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
				return
			}
		}
	}

	// 3. 调用 RPC
	resp, err := h.Call(r.Context(), &req)
	if err != nil {
		WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 4. 返回响应
	WriteJSON(w, resp)
}

// RegisterRPC 简化的路由注册函数
func RegisterRPC[Req any, Resp any](
	mux *http.ServeMux,
	path string,
	method string,
	call func(context.Context, *Req) (*Resp, error),
) {
	handler := &RPCHandler[Req, Resp]{
		Method: method,
		Call:   call,
	}
	mux.Handle(path, handler)
}

// RegisterRPCWithPath 支持路径参数的路由注册
func RegisterRPCWithPath[Req any, Resp any](
	mux *http.ServeMux,
	path string,
	method string,
	call func(context.Context, *Req) (*Resp, error),
	pathParams []string,
) {
	handler := &RPCHandler[Req, Resp]{
		Method:     method,
		Call:       call,
		PathParams: pathParams,
	}
	mux.Handle(path, handler)
}

// ParseQuery 从 URL Query 参数解析到结构体
func ParseQuery(r *http.Request, v interface{}) error {
	query := r.URL.Query()
	val := reflect.ValueOf(v)

	if val.Kind() != reflect.Ptr {
		return fmt.Errorf("v must be a pointer")
	}

	val = val.Elem()
	typ := val.Type()

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := typ.Field(i)

		// 获取 json tag 作为参数名
		tag := fieldType.Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}

		// 处理 tag 中的逗号（如 `json:"id,omitempty"`）
		paramName := strings.Split(tag, ",")[0]

		// 从 query 中获取值
		queryValue := query.Get(paramName)
		if queryValue == "" {
			continue
		}

		// 根据字段类型设置值
		if err := setFieldValue(field, queryValue); err != nil {
			return fmt.Errorf("failed to set field %s: %v", paramName, err)
		}
	}

	return nil
}

// SetStructFields 从 map 设置结构体字段
func SetStructFields(v interface{}, values map[string]string) error {
	val := reflect.ValueOf(v)
	if val.Kind() != reflect.Ptr {
		return fmt.Errorf("v must be a pointer")
	}

	val = val.Elem()
	typ := val.Type()

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := typ.Field(i)

		// 获取 json tag
		tag := fieldType.Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}

		paramName := strings.Split(tag, ",")[0]

		// 从 values 中获取值
		value, ok := values[paramName]
		if !ok {
			continue
		}

		// 设置字段值
		if err := setFieldValue(field, value); err != nil {
			return fmt.Errorf("failed to set field %s: %v", paramName, err)
		}
	}

	return nil
}

// setFieldValue 根据字段类型设置值
func setFieldValue(field reflect.Value, value string) error {
	if !field.CanSet() {
		return fmt.Errorf("field cannot be set")
	}

	switch field.Kind() {
	case reflect.Ptr:
		// Support protobuf wrapper types in query/path params (google.protobuf.*Value)
		switch field.Type() {
		case reflect.TypeOf((*wrapperspb.BoolValue)(nil)):
			boolVal, err := strconv.ParseBool(value)
			if err != nil {
				return err
			}
			field.Set(reflect.ValueOf(wrapperspb.Bool(boolVal)))
			return nil
		case reflect.TypeOf((*wrapperspb.Int32Value)(nil)):
			intVal, err := strconv.ParseInt(value, 10, 32)
			if err != nil {
				return err
			}
			field.Set(reflect.ValueOf(wrapperspb.Int32(int32(intVal))))
			return nil
		case reflect.TypeOf((*wrapperspb.Int64Value)(nil)):
			intVal, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return err
			}
			field.Set(reflect.ValueOf(wrapperspb.Int64(intVal)))
			return nil
		case reflect.TypeOf((*wrapperspb.UInt32Value)(nil)):
			uintVal, err := strconv.ParseUint(value, 10, 32)
			if err != nil {
				return err
			}
			field.Set(reflect.ValueOf(wrapperspb.UInt32(uint32(uintVal))))
			return nil
		case reflect.TypeOf((*wrapperspb.UInt64Value)(nil)):
			uintVal, err := strconv.ParseUint(value, 10, 64)
			if err != nil {
				return err
			}
			field.Set(reflect.ValueOf(wrapperspb.UInt64(uintVal)))
			return nil
		case reflect.TypeOf((*wrapperspb.StringValue)(nil)):
			field.Set(reflect.ValueOf(wrapperspb.String(value)))
			return nil
		case reflect.TypeOf((*wrapperspb.DoubleValue)(nil)):
			floatVal, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return err
			}
			field.Set(reflect.ValueOf(wrapperspb.Double(floatVal)))
			return nil
		case reflect.TypeOf((*wrapperspb.FloatValue)(nil)):
			floatVal, err := strconv.ParseFloat(value, 32)
			if err != nil {
				return err
			}
			field.Set(reflect.ValueOf(wrapperspb.Float(float32(floatVal))))
			return nil
		default:
			// Generic pointer support (e.g. proto3 optional scalars generate *int64/*bool/...).
			// Allocate element, set it using existing scalar logic, then assign pointer.
			elemType := field.Type().Elem()
			elem := reflect.New(elemType)
			if err := setFieldValue(elem.Elem(), value); err != nil {
				return err
			}
			field.Set(elem)
			return nil
		}

	case reflect.String:
		field.SetString(value)

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		intVal, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return err
		}
		field.SetInt(intVal)

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		uintVal, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return err
		}
		field.SetUint(uintVal)

	case reflect.Float32, reflect.Float64:
		floatVal, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return err
		}
		field.SetFloat(floatVal)

	case reflect.Bool:
		boolVal, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		field.SetBool(boolVal)

	default:
		return fmt.Errorf("unsupported field type: %v", field.Kind())
	}

	return nil
}

// protoJSONMarshaler 用于序列化 protobuf 消息，确保零值字段也被输出
var protoJSONMarshaler = protojson.MarshalOptions{
	EmitUnpopulated: true, // 输出所有字段，包括零值
	UseProtoNames:   true, // 使用 snake_case (与 proto 定义一致)
}

func parseLegacyCode(v interface{}) (int, bool) {
	switch t := v.(type) {
	case float64:
		i := int(t)
		if float64(i) != t {
			return 0, false
		}
		return i, true
	case float32:
		i := int(t)
		if float32(i) != t {
			return 0, false
		}
		return i, true
	case int:
		return t, true
	case int32:
		return int(t), true
	case int64:
		return int(t), true
	case uint:
		return int(t), true
	case uint32:
		return int(t), true
	case uint64:
		return int(t), true
	case string:
		i, err := strconv.Atoi(strings.TrimSpace(t))
		if err != nil {
			return 0, false
		}
		return i, true
	default:
		return 0, false
	}
}

func isRFC3339String(v interface{}) bool {
	s, ok := v.(string)
	if !ok {
		return false
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	_, err := time.Parse(time.RFC3339, s)
	return err == nil
}

func looksLikeLegacyEnvelope(m map[string]interface{}) bool {
	if _, ok := m["success"].(bool); ok {
		return true
	}
	if _, ok := m["code"]; ok {
		return true
	}
	if _, ok := m["message"].(string); ok {
		return true
	}
	if _, ok := m["msg"].(string); ok {
		return true
	}
	if _, ok := m["request_id"]; ok {
		return true
	}
	if _, ok := m["timestamp"]; ok {
		return true
	}
	if _, ok := m["details"]; ok {
		return true
	}
	return false
}

func shouldUnwrapDataWrapper(m map[string]interface{}) bool {
	if _, ok := m["data"]; !ok {
		return false
	}
	// If it already looks like a legacy envelope, we always unwrap.
	if looksLikeLegacyEnvelope(m) {
		return true
	}
	// Best-effort: common wrapper style: {data} or {data, pagination}.
	if len(m) == 1 {
		return true
	}
	if len(m) == 2 {
		if _, ok := m["pagination"]; ok {
			return true
		}
	}
	return false
}

func deriveBizCodeFromLegacyMessage(message string) int {
	s := strings.ToLower(strings.TrimSpace(message))
	switch {
	case s == "":
		return int(errcode.CodeInternalError)
	case strings.Contains(s, "unauthorized") || strings.Contains(s, "token"):
		return int(errcode.CodeUnauthorized)
	case strings.Contains(s, "forbidden"):
		return int(errcode.CodeForbidden)
	case strings.Contains(s, "not found"):
		return int(errcode.CodeNotFound)
	case strings.Contains(s, "already") && strings.Contains(s, "exist"):
		return int(errcode.CodeConflict)
	case strings.Contains(s, "too many") || strings.Contains(s, "rate limit"):
		return int(errcode.CodeTooManyRequests)
	case strings.Contains(s, "invalid") || strings.Contains(s, "required") || strings.Contains(s, "missing"):
		return int(errcode.CodeInvalidParam)
	case strings.Contains(s, "timeout"):
		return int(errcode.CodeTimeout)
	case strings.Contains(s, "unavailable"):
		return int(errcode.CodeServiceUnavail)
	case strings.Contains(s, "internal"):
		return int(errcode.CodeInternalError)
	default:
		return int(errcode.CodeInvalidOperation)
	}
}

// WriteJSON 返回成功的 JSON 响应（统一格式）
func WriteJSON(w http.ResponseWriter, data interface{}) {
	requestID := w.Header().Get(httpx.RequestIDHeader)
	ts := time.Now().UTC().Format(time.RFC3339)

	httpStatus := http.StatusOK
	success := true
	code := 0
	message := "success"
	var details interface{}

	payload := data

	// If it is a protobuf response, marshal to JSON first.
	if msg, ok := data.(proto.Message); ok {
		jsonBytes, err := protoJSONMarshaler.Marshal(msg)
		if err != nil {
			WriteErrorWithDetails(w, int(errcode.CodeInternalError), "Internal server error", http.StatusInternalServerError, nil)
			return
		}

		var decoded interface{}
		if err := json.Unmarshal(jsonBytes, &decoded); err != nil {
			decoded = json.RawMessage(jsonBytes)
		}
		payload = decoded
	}

	// Scheme A normalization (best-effort):
	// - Extract legacy envelope fields from payload (success/code/message/request_id/timestamp/details)
	// - Unwrap legacy `data` wrapper to avoid `data.data.*`
	// - Ensure invariants: code==0 <=> success==true
	if m, ok := payload.(map[string]interface{}); ok {
		isLegacy := looksLikeLegacyEnvelope(m)
		unwrapData := shouldUnwrapDataWrapper(m)

		var successFound bool
		if v, ok := m["success"].(bool); ok {
			successFound = true
			success = v
			delete(m, "success")
		}

		var codeFound bool
		if v, ok := m["code"]; ok {
			if parsed, ok := parseLegacyCode(v); ok {
				codeFound = true
				code = parsed
			}
			delete(m, "code")
		}

		if v, ok := m["message"].(string); ok && strings.TrimSpace(v) != "" {
			message = v
			delete(m, "message")
		}
		if v, ok := m["msg"].(string); ok && strings.TrimSpace(v) != "" {
			// Compatibility: some legacy payloads use `msg`.
			if strings.TrimSpace(message) == "" || strings.TrimSpace(message) == "success" {
				message = v
			}
			delete(m, "msg")
		}

		// Transport-only fields must never leak into HTTP payload.
		if isLegacy {
			delete(m, "request_id")
			// IMPORTANT:
			// `timestamp` can be a legitimate business field in protobuf payloads (e.g. tx timestamp, unix seconds).
			// Only treat it as a legacy envelope field when it looks like an RFC3339 string.
			if v, ok := m["timestamp"]; ok && isRFC3339String(v) {
				delete(m, "timestamp")
			}
			delete(m, "details")
		}

		// Enforce invariants and derive missing fields.
		switch {
		case codeFound:
			success = code == 0
			if success && strings.TrimSpace(message) == "" {
				message = "success"
			}
			if !success && strings.TrimSpace(message) == "" {
				message = errcode.ErrorCode(code).Message()
			}
		case successFound:
			if success {
				code = 0
				if strings.TrimSpace(message) == "" {
					message = "success"
				}
			} else {
				code = deriveBizCodeFromLegacyMessage(message)
				if strings.TrimSpace(message) == "" {
					message = errcode.ErrorCode(code).Message()
				}
			}
		default:
			// Pure payload / data-wrapper without success/code: treat as success.
			success = true
			code = 0
			if strings.TrimSpace(message) == "" {
				message = "success"
			}
		}

		if unwrapData {
			if raw, ok := m["data"]; ok {
				switch dv := raw.(type) {
				case map[string]interface{}:
					for k, v := range dv {
						if _, exists := m[k]; exists {
							continue
						}
						m[k] = v
					}
					delete(m, "data")
					payload = m
				default:
					delete(m, "data")
					if len(m) == 0 {
						payload = dv
					} else {
						m["value"] = dv
						payload = m
					}
				}
			} else {
				payload = m
			}
		} else {
			payload = m
		}

		if !success {
			httpStatus = httpx.HTTPStatusFromBizCode(code)
		}
	}

	resp := httpx.UnifiedResponse{
		Success:   success,
		Code:      code,
		Message:   message,
		Data:      payload,
		RequestID: requestID,
		Timestamp: ts,
		Details:   details,
	}
	if !success {
		// Keep error response concise unless caller explicitly attaches details.
		resp.Data = nil
	}
	if err := httpx.Write(w, httpStatus, resp); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// WriteJSONRaw 返回原始 JSON 响应（不包装，用于特殊场景）
func WriteJSONRaw(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// 如果是 protobuf 消息，使用 protojson 序列化以确保零值字段也被输出
	if msg, ok := data.(proto.Message); ok {
		jsonBytes, err := protoJSONMarshaler.Marshal(msg)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_, _ = w.Write(jsonBytes)
		return
	}

	// 非 protobuf 消息使用标准 json 序列化
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// WriteError 返回错误响应
func WriteError(w http.ResponseWriter, message string, statusCode int) {
	WriteErrorWithDetails(w, httpx.BizCodeFromHTTPStatus(statusCode), message, statusCode, nil)
}

// WriteErrorWithDetails 返回带 details 的错误响应（同时补齐 request_id/timestamp）
func WriteErrorWithDetails(w http.ResponseWriter, code int, message string, statusCode int, details interface{}) {
	requestID := w.Header().Get(httpx.RequestIDHeader)
	ts := time.Now().UTC().Format(time.RFC3339)

	resp := httpx.UnifiedResponse{
		Success:   false,
		Code:      code,
		Message:   message,
		RequestID: requestID,
		Timestamp: ts,
		Details:   details,
	}
	_ = httpx.Write(w, statusCode, resp)
}

// WriteRPCError 将 gRPC error 映射为 HTTP 状态码与统一错误响应
//
// 设计说明：
// - 业务错误统一返回 HTTP 200，前端通过响应体中的 code 字段判断是否成功
// - 只有真正的服务器内部错误（panic、服务不可达等）才返回 HTTP 5xx
// - 如果 ErrorInfo.metadata["http_status"] 显式指定了状态码，则使用该状态码
func WriteRPCError(w http.ResponseWriter, r *http.Request, err error) {
	st, ok := status.FromError(err)
	if !ok {
		// 无法解析的错误视为内部错误，返回 500
		WriteErrorWithDetails(w, int(errcode.CodeInternalError), err.Error(), http.StatusInternalServerError, nil)
		return
	}

	// 默认使用 HTTP 200（业务错误）
	// 只有 Internal/Unknown/Unavailable 等服务级别错误才使用 5xx
	httpStatus := http.StatusOK
	if isServerError(st.Code()) {
		httpStatus = grpcCodeToHTTPStatus(st.Code())
	}

	// Default business code is derived from gRPC code (and may be overridden by ErrorInfo.metadata["code"]).
	bizCode := grpcCodeToBizCode(st.Code())
	bizCodeOverridden := false
	detailPayload := map[string]interface{}{
		"grpc_code": st.Code().String(),
	}
	var reason string
	var reasonMetadata map[string]string

	for _, d := range st.Details() {
		switch v := d.(type) {
		case *errdetails.ErrorInfo:
			if v.Reason != "" {
				detailPayload["reason"] = v.Reason
				reason = v.Reason
			}
			if len(v.Metadata) > 0 {
				// Filter reserved keys from metadata for HTTP details output.
				filtered := map[string]string{}
				for k, vv := range v.Metadata {
					k = strings.TrimSpace(k)
					vv = strings.TrimSpace(vv)
					if k == "" || vv == "" {
						continue
					}
					if k == "code" || k == "http_status" {
						continue
					}
					filtered[k] = vv
				}
				if len(filtered) > 0 {
					detailPayload["metadata"] = filtered
				}
				reasonMetadata = v.Metadata
				if raw, exists := v.Metadata["code"]; exists {
					if parsed, parseErr := strconv.Atoi(raw); parseErr == nil && parsed > 0 {
						bizCode = parsed
						bizCodeOverridden = true
					}
				}
				// 只有服务器错误（5xx）才使用 metadata 中的 http_status
				// 业务错误统一返回 HTTP 200，前端通过 code 字段判断
				if raw, exists := v.Metadata["http_status"]; exists {
					if parsed, parseErr := strconv.Atoi(raw); parseErr == nil && parsed >= 500 && parsed <= 599 {
						httpStatus = parsed
					}
				}
			}
		case *errdetails.BadRequest:
			if len(v.FieldViolations) > 0 {
				items := make([]map[string]string, 0, len(v.FieldViolations))
				for _, fv := range v.FieldViolations {
					desc := fv.Description
					if strings.TrimSpace(desc) != "" {
						desc = i18n.Validation(r.Context(), desc)
					}
					items = append(items, map[string]string{
						"field":       fv.Field,
						"description": desc,
					})
				}
				detailPayload["field_violations"] = items
			}
		case *errdetails.RetryInfo:
			// 可选：用于前端提示重试
			if v.RetryDelay != nil {
				detailPayload["retry_delay_seconds"] = v.RetryDelay.Seconds
			}
		default:
			// 忽略其他类型
		}
	}

	if !bizCodeOverridden {
		// 使用 gRPC code 映射的业务码
		bizCode = grpcCodeToBizCode(st.Code())
	}

	// 如果 details 只有 grpc_code，则不输出 details（保持响应简洁）
	var details interface{}
	if len(detailPayload) > 1 {
		details = detailPayload
	}

	msg := st.Message()
	if strings.TrimSpace(reason) != "" {
		data := map[string]interface{}{}
		// 如果 metadata 中有 error_detail，将其加入到国际化数据中
		var errorDetail string
		for k, v := range reasonMetadata {
			k = strings.TrimSpace(k)
			v = strings.TrimSpace(v)
			if k == "" || v == "" {
				continue
			}
			// Reserved transport metadata.
			if k == "code" || k == "http_status" {
				continue
			}
			if k == "error_detail" {
				errorDetail = v
			}
			data[k] = v
		}
		localized := i18n.T(r.Context(), reason, data)
		// 如果找到了国际化翻译，使用它；但如果有 error_detail，则优先使用具体的错误详情
		if localized != "" && localized != reason {
			if errorDetail != "" {
				// 有具体的错误详情，使用它而不是通用的国际化消息
				msg = errorDetail
			} else {
				// 没有具体错误详情，使用国际化消息
				msg = localized
			}
		}
	}

	_ = r // 预留：后续可从请求中提取更多信息
	WriteErrorWithDetails(w, bizCode, msg, httpStatus, details)
}

// isServerError 判断是否为服务器级别错误（应返回 5xx）
func isServerError(code codes.Code) bool {
	switch code {
	case codes.Internal, codes.Unknown, codes.Unavailable, codes.DataLoss:
		return true
	default:
		return false
	}
}

// grpcCodeToBizCode 将 gRPC 状态码映射为业务错误码
func grpcCodeToBizCode(code codes.Code) int {
	switch code {
	case codes.OK:
		return int(errcode.CodeSuccess)
	case codes.InvalidArgument:
		return int(errcode.CodeInvalidParam)
	case codes.Unauthenticated:
		return int(errcode.CodeUnauthorized)
	case codes.PermissionDenied:
		return int(errcode.CodeForbidden)
	case codes.NotFound:
		return int(errcode.CodeNotFound)
	case codes.AlreadyExists:
		return int(errcode.CodeConflict)
	case codes.ResourceExhausted:
		return int(errcode.CodeTooManyRequests)
	case codes.Unavailable:
		return int(errcode.CodeServiceUnavail)
	case codes.DeadlineExceeded:
		return int(errcode.CodeTimeout)
	case codes.FailedPrecondition:
		return int(errcode.CodeInvalidOperation)
	default:
		return int(errcode.CodeInternalError)
	}
}

func grpcCodeToHTTPStatus(code codes.Code) int {
	switch code {
	case codes.OK:
		return http.StatusOK
	case codes.InvalidArgument:
		return http.StatusBadRequest
	case codes.Unauthenticated:
		return http.StatusUnauthorized
	case codes.PermissionDenied:
		return http.StatusForbidden
	case codes.NotFound:
		return http.StatusNotFound
	case codes.AlreadyExists:
		return http.StatusConflict
	case codes.FailedPrecondition:
		return http.StatusUnprocessableEntity
	case codes.ResourceExhausted:
		return http.StatusTooManyRequests
	case codes.Unavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// ExtractPathParams 从 context 中提取路径参数
func ExtractPathParams(ctx context.Context) map[string]string {
	if params, ok := ctx.Value("path_params").(map[string]string); ok {
		return params
	}
	return nil
}

// PathParamMiddleware 路径参数解析中间件
func PathParamMiddleware(pattern string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			params := parsePathParams(r.URL.Path, pattern)
			ctx := context.WithValue(r.Context(), "path_params", params)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// parsePathParams 解析路径参数
// 例如: pattern="/api/v1/account/{id}", path="/api/v1/account/123" => {"id": "123"}
func parsePathParams(path, pattern string) map[string]string {
	params := make(map[string]string)

	pathParts := strings.Split(strings.Trim(path, "/"), "/")
	patternParts := strings.Split(strings.Trim(pattern, "/"), "/")

	if len(pathParts) != len(patternParts) {
		return params
	}

	for i, part := range patternParts {
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			paramName := strings.Trim(part, "{}")
			params[paramName] = pathParts[i]
			continue
		}
		// 静态段必须完全匹配
		if part != pathParts[i] {
			return map[string]string{}
		}
	}

	return params
}
