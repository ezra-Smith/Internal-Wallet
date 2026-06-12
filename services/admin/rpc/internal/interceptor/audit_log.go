package interceptor

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"internalwallet/common/middleware"
	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/auditctx"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
)

const (
	maxAuditPayloadBytes = 8 * 1024
	maxAuditDetailsBytes = 32 * 1024
)

// AuditLogUnaryInterceptor creates a request-level audit log for every Admin RPC call (success/failed).
//
// Notes:
// - Must run after ServerMetadataUnaryInterceptor (to read metadata from context).
// - Should run before AdminAuthUnaryInterceptor (to capture RBAC/idempotency failures too).
// - Business logic can "enrich" this row by calling AdminAuditLogRepo.CreateLog(ctx, ...) (repo will update-in-place).
func AuditLogUnaryInterceptor(svcCtx *svc.ServiceContext) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if svcCtx == nil || svcCtx.AdminAuditLogRepo == nil {
			return handler(ctx, req)
		}

		start := time.Now()
		methodName := extractMethodName(info.FullMethod)

		adminID := parseInt64(middleware.GetUserID(ctx))
		email := strings.TrimSpace(middleware.GetEmail(ctx))
		ip := strings.TrimSpace(middleware.GetClientIP(ctx))
		ua := strings.TrimSpace(middleware.GetUserAgent(ctx))
		requestID := strings.TrimSpace(middleware.GetRequestID(ctx))
		httpMethod := strings.TrimSpace(middleware.GetHTTPMethod(ctx))
		httpPath := strings.TrimSpace(middleware.GetHTTPPath(ctx))
		idempotencyKey := strings.TrimSpace(middleware.GetIdempotencyKey(ctx))

		module := moduleForMethod(methodName)
		targetType, targetID := inferTarget(methodName, req)
		description := getMethodDescription(methodName)

		detailsBytes := buildAuditDetails(ctx, req, info.FullMethod, methodName, httpMethod, httpPath)

		row := &model.AdminAuditLogModel{
			AdminID: adminID,

			Action:     methodName,
			TargetType: targetType,
			TargetID:   targetID,

			Description: description,
			Details:     detailsBytes,

			IP:        ip,
			UserAgent: ua,

			Success:        true,
			GrpcCode:       int32(codes.OK),
			ErrorMessage:   "",
			RequestID:      requestID,
			RpcMethod:      info.FullMethod,
			HttpMethod:     httpMethod,
			HttpPath:       httpPath,
			DurationMs:     0,
			OperatorEmail:  email,
			IdempotencyKey: idempotencyKey,
			Module:         module,
		}

		if err := svcCtx.AdminAuditLogRepo.CreateLog(ctx, row); err != nil {
			logx.WithContext(ctx).Errorw("audit log create failed", logx.Field("error", err))
			return handler(ctx, req)
		}

		ctx = auditctx.WithAuditLogID(ctx, row.ID)
		resp, err := handler(ctx, req)

		durationMs := int32(time.Since(start).Milliseconds())
		success := err == nil
		grpcCode, errMsg, errDetails := grpcErrorInfo(err)
		errMsg = trimString(errMsg, 1024)

		updates := map[string]interface{}{
			"success":       success,
			"grpc_code":     grpcCode,
			"error_message": errMsg,
			"duration_ms":   durationMs,
		}

		if merged, mErr := mergeAndAppendDetails(ctx, svcCtx.AdminAuditLogRepo.GetDB(), row.ID, success, grpcCode, durationMs, resp, errDetails); mErr == nil && len(merged) > 0 {
			updates["details"] = merged
		}

		if uErr := svcCtx.AdminAuditLogRepo.UpdateFields(ctx, row.ID, updates); uErr != nil {
			logx.WithContext(ctx).Errorw("audit log update failed", logx.Field("error", uErr))
		}

		return resp, err
	}
}

func grpcErrorInfo(err error) (grpcCode int32, msg string, details interface{}) {
	if err == nil {
		return int32(codes.OK), "", nil
	}

	st, ok := status.FromError(err)
	if !ok {
		return int32(codes.Unknown), err.Error(), map[string]interface{}{
			"type":    fmt.Sprintf("%T", err),
			"message": err.Error(),
		}
	}

	out := map[string]interface{}{
		"grpc_code":     st.Code().String(),
		"grpc_code_int": int(st.Code()),
		"message":       st.Message(),
	}

	rawDetails := st.Details()
	if len(rawDetails) > 0 {
		items := make([]interface{}, 0, len(rawDetails))
		for _, d := range rawDetails {
			item := map[string]interface{}{
				"type": fmt.Sprintf("%T", d),
			}
			if pm, ok := d.(proto.Message); ok {
				if v, ok := messageToInterface(pm).(map[string]interface{}); ok && len(v) > 0 {
					item["data"] = v
				}
			}
			items = append(items, item)
		}
		out["details"] = items
	}

	return int32(st.Code()), st.Message(), out
}

func mergeAndAppendDetails(ctx context.Context, db *gorm.DB, id int64, success bool, grpcCode int32, durationMs int32, resp interface{}, errDetails interface{}) ([]byte, error) {
	type detailsRow struct {
		Details []byte `gorm:"column:details"`
	}
	var row detailsRow

	if err := db.WithContext(ctx).
		Model(&model.AdminAuditLogModel{}).
		Select("details").
		Where("id = ?", id).
		First(&row).Error; err != nil {
		return nil, err
	}

	base := map[string]interface{}{}
	if len(row.Details) > 0 {
		_ = json.Unmarshal(row.Details, &base)
	}
	if base == nil {
		base = map[string]interface{}{}
	}

	base["result"] = map[string]interface{}{
		"success":     success,
		"grpc_code":   grpcCode,
		"duration_ms": durationMs,
	}

	if errDetails != nil {
		base["error"] = errDetails
	}

	if summary := responseSummary(resp); summary != nil {
		base["response"] = summary
	}

	out, err := json.Marshal(base)
	if err != nil {
		return nil, err
	}
	if len(out) <= maxAuditDetailsBytes {
		return out, nil
	}

	// Best-effort shrink: drop request payload if still too big.
	if _, ok := base["request"]; ok {
		base["request"] = map[string]interface{}{
			"_truncated": true,
			"note":       "omitted due to size limit",
		}
	}
	out, err = json.Marshal(base)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func buildAuditDetails(ctx context.Context, req interface{}, fullMethod, methodName, httpMethod, httpPath string) []byte {
	requestPayload := sanitize(messageToInterface(req))
	requestBytes, _ := json.Marshal(requestPayload)
	if len(requestBytes) > maxAuditPayloadBytes {
		requestPayload = map[string]interface{}{
			"_truncated":     true,
			"original_bytes": len(requestBytes),
			"content":        string(requestBytes[:maxAuditPayloadBytes]),
		}
	}

	jwt := map[string]interface{}{}
	if v := strings.TrimSpace(middleware.GetJWTID(ctx)); v != "" {
		jwt["jti"] = v
	}
	if exp := middleware.GetJWTExpiresAt(ctx); exp > 0 {
		jwt["exp_unix"] = exp
	}
	if ver := middleware.GetJWTTokenVersion(ctx); ver > 0 {
		jwt["ver"] = ver
	}

	meta := map[string]interface{}{}
	if requestID := strings.TrimSpace(middleware.GetRequestID(ctx)); requestID != "" {
		meta["request_id"] = requestID
	}
	if k := strings.TrimSpace(middleware.GetIdempotencyKey(ctx)); k != "" {
		meta["idempotency_key"] = k
	}
	if locale := strings.TrimSpace(middleware.GetLocale(ctx)); locale != "" {
		meta["locale"] = locale
	}
	if len(jwt) > 0 {
		meta["jwt"] = jwt
	}

	details := map[string]interface{}{
		"rpc": map[string]interface{}{
			"full_method": fullMethod,
			"method":      methodName,
		},
		"http": map[string]interface{}{
			"method": httpMethod,
			"path":   httpPath,
		},
		"request":  requestPayload,
		"metadata": meta,
	}

	out, err := json.Marshal(details)
	if err != nil {
		return nil
	}
	if len(out) <= maxAuditDetailsBytes {
		return out
	}
	delete(details, "request")
	details["request"] = map[string]interface{}{
		"_truncated": true,
		"note":       "omitted due to size limit",
	}
	out, _ = json.Marshal(details)
	return out
}

func responseSummary(resp interface{}) interface{} {
	if resp == nil {
		return nil
	}
	v := messageToInterface(resp)
	m, ok := v.(map[string]interface{})
	if !ok || len(m) == 0 {
		return nil
	}
	out := map[string]interface{}{}
	copyIfExists := func(key string) {
		if val, ok := m[key]; ok {
			out[key] = val
		}
	}
	copyIfExists("success")
	copyIfExists("message")
	copyIfExists("request_id")
	copyIfExists("timestamp")
	copyIfExists("pagination")
	if len(out) == 0 {
		return nil
	}
	return out
}

func moduleForMethod(methodName string) string {
	name := strings.TrimSpace(methodName)
	if name == "" {
		return ""
	}

	// audit logs
	if name == "GetAuditLogs" {
		return "audit"
	}

	// account/admin auth
	if strings.HasPrefix(name, "AdminMFA") ||
		strings.Contains(name, "TwoFA") ||
		strings.Contains(name, "Password") ||
		strings.Contains(name, "Login") ||
		strings.Contains(name, "Logout") ||
		strings.Contains(name, "RefreshToken") ||
		strings.Contains(name, "GetMe") ||
		strings.Contains(name, "Admin") {
		return "account"
	}

	// RBAC
	if strings.Contains(name, "Role") ||
		strings.Contains(name, "Permission") ||
		strings.Contains(name, "Menu") ||
		strings.Contains(name, "RBAC") {
		return "role"
	}

	// swap
	if strings.Contains(name, "Swap") {
		return "swap"
	}

	// vault
	if strings.Contains(name, "Vault") {
		return "vault"
	}

	// transfer
	if strings.Contains(name, "Transfer") {
		return "transfer"
	}

	// withdrawals approvals
	if strings.Contains(name, "Withdrawal") || strings.Contains(name, "Withdraw") {
		return "approval"
	}

	// web3 blacklist
	if strings.Contains(name, "Web3") || strings.Contains(name, "Blacklist") {
		return "blacklist"
	}

	// user management
	if strings.Contains(name, "User") {
		return "user"
	}

	// assets/currency/settings
	if strings.Contains(name, "Currency") || strings.Contains(name, "Chain") || strings.Contains(name, "Deposit") {
		return "currency"
	}

	if strings.Contains(name, "Config") || strings.Contains(name, "Setting") || strings.Contains(name, "System") {
		return "system"
	}

	return "system"
}

func inferTarget(methodName string, req interface{}) (targetType string, targetID string) {
	// Special-cases where request type is known.
	switch r := req.(type) {
	case *pb.UpdateSystemConfigRequest:
		return "config", strings.TrimSpace(r.Category)
	}

	v := messageToInterface(req)
	m, ok := v.(map[string]interface{})
	if !ok || len(m) == 0 {
		return "", ""
	}

	getStr := func(keys ...string) string {
		for _, k := range keys {
			val, ok := m[k]
			if !ok || val == nil {
				continue
			}
			switch t := val.(type) {
			case string:
				if strings.TrimSpace(t) != "" {
					return strings.TrimSpace(t)
				}
			case float64:
				if t > 0 {
					return strconv.FormatInt(int64(t), 10)
				}
			case int64:
				if t > 0 {
					return strconv.FormatInt(t, 10)
				}
			case json.Number:
				if i, err := t.Int64(); err == nil && i > 0 {
					return strconv.FormatInt(i, 10)
				}
			}
		}
		return ""
	}

	// Infer by method category.
	switch {
	case strings.Contains(methodName, "Role"):
		return "role", getStr("role_id", "id")
	case strings.Contains(methodName, "Admin"):
		return "admin", getStr("admin_id", "id")
	case strings.Contains(methodName, "User"):
		return "user", getStr("uid", "user_id", "id")
	case strings.Contains(methodName, "Transfer"):
		return "transfer", getStr("batch_id", "id")
	case strings.Contains(methodName, "Vault"):
		return "vault", getStr("id")
	case strings.Contains(methodName, "Currency"):
		return "currency", getStr("symbol", "currency", "id")
	default:
		return "", getStr("id")
	}
}

func messageToInterface(v interface{}) interface{} {
	if v == nil {
		return nil
	}
	if pm, ok := v.(proto.Message); ok {
		b, err := protojson.MarshalOptions{UseProtoNames: true, EmitUnpopulated: false}.Marshal(pm)
		if err != nil {
			return nil
		}
		var out interface{}
		if err := json.Unmarshal(b, &out); err != nil {
			return nil
		}
		return out
	}

	// Unwrap pointers and try JSON marshal
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Pointer && !rv.IsNil() {
		return messageToInterface(rv.Elem().Interface())
	}

	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var out interface{}
	if err := json.Unmarshal(b, &out); err != nil {
		return nil
	}
	return out
}

func sanitize(v interface{}) interface{} {
	switch t := v.(type) {
	case map[string]interface{}:
		out := map[string]interface{}{}
		for k, vv := range t {
			if isSensitiveKey(k) {
				out[k] = "[redacted]"
				continue
			}
			out[k] = sanitize(vv)
		}
		return out
	case []interface{}:
		out := make([]interface{}, 0, len(t))
		for _, item := range t {
			out = append(out, sanitize(item))
		}
		return out
	default:
		return v
	}
}

func isSensitiveKey(key string) bool {
	k := strings.ToLower(strings.TrimSpace(key))
	if k == "" {
		return false
	}
	normalized := strings.NewReplacer("-", "", "_", "", " ", "").Replace(k)

	safeExact := map[string]bool{
		"password":               true,
		"passwordhash":           true,
		"oldpassword":            true,
		"newpassword":            true,
		"currentpassword":        true,
		"confirmpassword":        true,
		"twofactorsecret":        true,
		"twofactorpendingsecret": true,
		"twofactorcode":          true,
		"otp":                    true,
		"otpcode":                true,
		"totp":                   true,
		"mfacode":                true,
		"captchacode":            true,
		"captchatoken":           true,
		"accesstoken":            true,
		"refreshtoken":           true,
		"idtoken":                true,
		"authorization":          true,
		"privatekey":             true,
		"mnemonic":               true,
		"seed":                   true,
		"phrase":                 true,
		"apikey":                 true,
		"secret":                 true,
		"signature":              true,
	}
	if safeExact[normalized] {
		return true
	}

	if strings.Contains(normalized, "password") ||
		strings.Contains(normalized, "secret") ||
		strings.Contains(normalized, "token") ||
		strings.Contains(normalized, "privatekey") ||
		strings.Contains(normalized, "mnemonic") ||
		strings.Contains(normalized, "twofactor") ||
		strings.Contains(normalized, "captcha") ||
		strings.Contains(normalized, "otp") ||
		strings.Contains(normalized, "mfa") {
		return true
	}
	return false
}

func trimString(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if len(s) <= max {
		return s
	}
	return s[:max]
}

func parseInt64(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return v
}

// getMethodDescription returns a Chinese description for the given RPC method name.
// If no mapping exists, it returns the original method name.
func getMethodDescription(methodName string) string {
	descriptions := map[string]string{
		// ==================== 认证相关 ====================
		"RefreshToken":            "刷新令牌",
		"Logout":                  "登出",
		"AdminMFACaptchaGenerate": "生成验证码",
		"AdminMFACaptchaValidate": "验证验证码",
		"AdminMFALogin":           "管理员MFA登录",
		"AdminMFAGetTwoFAQrCode":  "获取2FA二维码",
		"AdminMFABindTwoFA":       "绑定2FA",
		"AdminMFAVerifyTwoFA":     "验证2FA",

		// ==================== 管理员管理 ====================
		"ListAdmins":            "查询管理员列表",
		"CreateAdmin":           "创建管理员",
		"UpdateAdmin":           "更新管理员",
		"DisableAdmin":          "禁用管理员",
		"ResetAdminPassword":    "重置管理员密码",
		"ResetAdminTwoFA":       "重置管理员2FA",
		"ChangeMyPassword":      "修改我的密码",
		"GetMe":                 "获取我的信息",
		"GetMySecuritySettings": "获取我的安全设置",
		"StartTwoFASetup":       "开始设置2FA",
		"ConfirmTwoFASetup":     "确认设置2FA",
		"StartTwoFARebind":      "开始重新绑定2FA",
		"DisableTwoFA":          "关闭2FA",

		// ==================== 用户管理 ====================
		"ListUsers":                             "查询用户列表",
		"GetUsersStatistics":                    "获取用户统计",
		"ListUserWithdrawAuditWhitelistRules":   "查询用户提现审核白名单规则",
		"UpsertUserWithdrawAuditWhitelistRules": "设置用户提现审核白名单规则",
		"GetUserDetail":                         "获取用户详情",
		"FreezeUser":                            "冻结用户",
		"UnfreezeUser":                          "解冻用户",
		"TerminateUser":                         "终止用户",
		"UpdateUserRole":                        "更新用户角色",
		"UpdateUserMemberLevel":                 "更新用户会员等级",
		"ResetUserPassword":                     "重置用户密码",
		"ResetUserTradePassword":                "重置用户交易密码",
		"UpdateUserWithdrawAuditWhitelist":      "更新用户提现审核白名单",
		"UnbindUserTotp":                        "解绑用户TOTP",
		"ListUser2FAHistory":                    "查询用户2FA历史",
		"AddUserNote":                           "添加用户备注",
		"ExportUsers":                           "导出用户",

		// ==================== RBAC权限管理 ====================
		"GetMyRBAC":            "获取我的RBAC权限",
		"GetRbacPermissionMap": "获取RBAC权限映射",

		// 菜单管理
		"CreateMenu":  "创建菜单",
		"UpdateMenu":  "更新菜单",
		"DeleteMenu":  "删除菜单",
		"GetMenuById": "根据ID获取菜单",
		"ListMenus":   "查询菜单列表",
		"GetMenuTree": "获取菜单树",

		// 权限管理
		"CreatePermission":       "创建权限",
		"UpdatePermission":       "更新权限",
		"DeletePermission":       "删除权限",
		"GetPermissionById":      "根据ID获取权限",
		"ListPermissions":        "查询权限列表",
		"GetPermissionsByMenuId": "根据菜单ID获取权限",

		// 角色管理
		"CreateRole":              "创建角色",
		"UpdateRole":              "更新角色",
		"DeleteRole":              "删除角色",
		"GetRoleById":             "根据ID获取角色",
		"ListRoles":               "查询角色列表",
		"AssignMenusToRole":       "为角色分配菜单",
		"AssignPermissionsToRole": "为角色分配权限",
		"AssignRolesToUser":       "为用户分配角色",
		"GetRoleMenus":            "获取角色菜单",
		"GetRolePermissions":      "获取角色权限",
		"GetUserRoles":            "获取用户角色",

		// ==================== 系统配置 ====================
		"GetSystemConfig":    "获取系统配置",
		"UpdateSystemConfig": "更新系统配置",

		// ==================== 审计日志 ====================
		"GetAuditLogs": "查询审计日志",

		// ==================== Swap API密钥 ====================
		"CreateSwapApiKey": "创建Swap API密钥",
		"RevokeSwapApiKey": "撤销Swap API密钥",
		"ListSwapApiKeys":  "查询Swap API密钥列表",

		// ==================== Swap服务商 ====================
		"ListSwapProvidersForDropdown": "查询Swap服务商下拉列表",
		"ListSwapProviders":            "查询Swap服务商列表",
		"GetSwapProviderDetail":        "获取Swap服务商详情",
		"CreateSwapProvider":           "创建Swap服务商",
		"UpdateSwapProvider":           "更新Swap服务商",
		"DeleteSwapProvider":           "删除Swap服务商",
		"EnableSwapProvider":           "启用Swap服务商",
		"DisableSwapProvider":          "禁用Swap服务商",

		// ==================== Swap配置 ====================
		"ListSwapConfigs":             "查询Swap配置列表",
		"GetSwapConfigDetail":         "获取Swap配置详情",
		"CreateSwapConfig":            "创建Swap配置",
		"UpdateSwapConfigDetail":      "更新Swap配置详情",
		"DeleteSwapConfig":            "删除Swap配置",
		"ListProjectSwapConfigs":      "查询项目Swap配置列表",
		"EnableSwapConfigForProject":  "为项目启用Swap配置",
		"DisableSwapConfigForProject": "为项目禁用Swap配置",
		"ListSwapTransactions":        "查询Swap交易列表",

		// ==================== 币种管理 ====================
		"GetCurrencyOverview":               "获取币种概览",
		"CreateCurrency":                    "创建币种",
		"UpdateCurrency":                    "更新币种",
		"GetCurrency":                       "获取币种详情",
		"ListCurrencies":                    "查询币种列表",
		"GetCurrencyConfig":                 "获取币种配置",
		"UpdateCurrencyConfig":              "更新币种配置",
		"UpdateCurrencyStatus":              "更新币种状态",
		"GetCurrencyGlobalWithdrawFee":      "获取币种全局提现手续费",
		"UpdateCurrencyGlobalWithdrawFee":   "更新币种全局提现手续费",
		"GetCurrencyGlobalWithdrawAudit":    "获取币种全局提现审核配置",
		"UpdateCurrencyGlobalWithdrawAudit": "更新币种全局提现审核配置",
		"GetCurrencyGlobalTransferAudit":    "获取币种全局转账审核配置",
		"UpdateCurrencyGlobalTransferAudit": "更新币种全局转账审核配置",

		// ==================== 链管理 ====================
		"ListChains":      "查询链列表",
		"UpdateChainIcon": "更新链图标",

		// ==================== 财务科目管理 ====================
		"AccountingListAccountTypes":     "查询财务科目列表",
		"AccountingCreateAccountType":    "创建财务科目",
		"AccountingUpdateAccountType":    "更新财务科目",
		"AccountingDeleteAccountType":    "删除财务科目",
		"AccountingSetAccountTypeAssets": "设置财务科目资产",

		// ==================== 财务账户管理 ====================
		"AccountingEnsureUserSetup":      "初始化用户财务账户",
		"AccountingGetUserBalances":      "获取用户财务余额",
		"AccountingEnsureSystemAccounts": "初始化系统财务账户",
		"AccountingListLedgerTx":         "查询财务账交易列表",
		"AccountingGetLedgerTxDetail":    "获取财务账交易详情",

		// ==================== 充值地址 ====================
		"ListDepositAddresses":            "查询充值地址列表",
		"ListDepositAddressesWithBalance": "查询带余额的充值地址列表",
		"GetDepositAddress":               "获取充值地址",
		"CreateDepositAddress":            "创建充值地址",
		"UpdateDepositAddress":            "更新充值地址",
		"DeleteDepositAddress":            "删除充值地址",
		"InitializeUserWallet":            "初始化用户钱包",

		// ==================== 充值地址余额 ====================
		"ListDepositAddressBalances":      "查询充值地址余额列表",
		"GetDepositAddressBalance":        "获取充值地址余额",
		"GetDepositAddressBalanceStats":   "获取充值地址余额统计",
		"GetDepositAddressBalanceByAsset": "根据资产获取充值地址余额",
		"UpdateDepositAddressBalance":     "更新充值地址余额",

		// ==================== 充值管理 ====================
		"ListDeposits":  "查询充值列表",
		"GetDeposit":    "获取充值详情",
		"CreateDeposit": "创建充值",
		"UpdateDeposit": "更新充值",
		"CancelDeposit": "取消充值",

		// ==================== 提现管理 ====================
		"CreateCurrencyWithdrawal":     "创建币种提现",
		"GetCurrencyWithdrawal":        "获取币种提现详情",
		"ListCurrencyWithdrawals":      "查询币种提现列表",
		"ApproveCurrencyWithdrawal":    "审批币种提现",
		"RejectCurrencyWithdrawal":     "拒绝币种提现",
		"CancelCurrencyWithdrawal":     "取消币种提现",
		"UpdateCurrencyWithdrawal":     "更新币种提现",
		"ListCurrencyWithdrawalEvents": "查询币种提现事件列表",

		// ==================== 转账批次 ====================
		"ListTransferBatches":  "查询转账批次列表",
		"ImportTransferBatch":  "导入转账批次",
		"GetTransferBatch":     "获取转账批次",
		"SubmitTransferBatch":  "提交转账批次",
		"ApproveTransferBatch": "审批转账批次",
		"CancelTransferBatch":  "取消转账批次",
		"ExecuteTransferBatch": "执行转账批次",
		"RetryTransferBatch":   "重试转账批次",

		// ==================== Web3用户管理 ====================
		"ListWeb3Users":             "查询Web3用户列表",
		"GetWeb3User":               "获取Web3用户详情",
		"GetWeb3UsersPageList":      "获取Web3用户分页列表",
		"GetWeb3UsersSummary":       "获取Web3用户汇总",
		"ListWeb3UserTransactions":  "查询Web3用户交易列表",
		"BlacklistWeb3Address":      "拉黑Web3地址",
		"UnblacklistWeb3Address":    "解除拉黑Web3地址",
		"BatchWeb3AddressBlacklist": "批量Web3地址黑名单操作",
		"GetWeb3UserStatistics":     "获取Web3用户统计",
		"ExportWeb3Users":           "导出Web3用户",
		"ListWeb3UserAddresses":     "查询Web3用户地址列表",
		"FreezeWeb3User":            "冻结Web3用户",
		"UnfreezeWeb3User":          "解冻Web3用户",
		"AddWeb3UserNote":           "添加Web3用户备注",

		// ==================== 敏感地址黑名单 ====================
		"ListBlacklistAddresses":              "查询黑名单地址列表",
		"CreateBlacklistAddress":              "创建黑名单地址",
		"BatchCreateBlacklistAddresses":       "批量创建黑名单地址",
		"UpdateBlacklistAddressMonitorStatus": "更新黑名单地址监控状态",
		"DeleteBlacklistAddress":              "删除黑名单地址",
		"ExportBlacklistAddresses":            "导出黑名单地址",

		// ==================== Web3黑名单（旧） ====================
		"ListWeb3Blacklist":   "查询Web3黑名单列表",
		"CreateWeb3Blacklist": "创建Web3黑名单",
		"UpdateWeb3Blacklist": "更新Web3黑名单",
		"DeleteWeb3Blacklist": "删除Web3黑名单",

		// ==================== 仪表盘 ====================
		"GetDashboard":           "获取仪表盘数据",
		"GetDashboardOverview":   "获取仪表盘核心概览",
		"GetDashboardTrends":     "获取仪表盘趋势图表",
		"GetDashboardActivities": "获取仪表盘最近活动",

		// ==================== 通知管理 ====================
		"ListNotifications":  "查询通知列表",
		"CreateNotification": "创建通知",
		"UpdateNotification": "更新通知",
		"DeleteNotification": "删除通知",
		"SendNotification":   "发送通知",

		// ==================== 金库资金管理 ====================
		"GetVaultOverview":       "获取金库资金概览",
		"GetVaultDetail":         "获取金库详细信息",
		"CreateVaultAdjustment":  "创建金库资金调整",
		"ListVaultAdjustments":   "查询金库资金调整列表",
		"ApproveVaultAdjustment": "审批金库资金调整",
		"UpdateVaultThresholds":  "更新金库预警阈值",
		"SyncVault":              "同步刷新金库余额",

		// ==================== 金库地址管理 ====================
		"AddVaultAddress":          "添加金库地址",
		"ListVaultAddresses":       "查询金库地址列表",
		"UpdateVaultAddressStatus": "更新金库地址状态",
		"DeleteVaultAddress":       "删除金库地址",
		"GetVaultAddressBalances":  "获取金库地址余额",

		// ==================== 金库管理（旧） ====================
		"ListVaults":            "查询金库列表",
		"GetVault":              "获取金库详情",
		"CreateVault":           "创建金库",
		"UpdateVault":           "更新金库",
		"DeleteVault":           "删除金库",
		"GetVaultBalance":       "获取金库余额",
		"ListVaultTransactions": "查询金库交易列表",

		// ==================== 其他Admin方法（带Admin前缀的特殊方法） ====================
		"AdminCreateSwapApiKey":             "创建Swap API密钥",
		"AdminRevokeSwapApiKey":             "撤销Swap API密钥",
		"AdminListSwapApiKeys":              "查询Swap API密钥列表",
		"AdminListSwapProvidersForDropdown": "查询Swap服务商下拉列表",
		"AdminListSwapProviders":            "查询Swap服务商列表",
		"AdminGetSwapProviderDetail":        "获取Swap服务商详情",
		"AdminCreateSwapProvider":           "创建Swap服务商",
		"AdminUpdateSwapProvider":           "更新Swap服务商",
		"AdminDeleteSwapProvider":           "删除Swap服务商",
		"AdminEnableSwapProvider":           "启用Swap服务商",
		"AdminDisableSwapProvider":          "禁用Swap服务商",
		"AdminListSwapConfigs":              "查询Swap配置列表",
		"AdminGetSwapConfigDetail":          "获取Swap配置详情",
		"AdminCreateSwapConfig":             "创建Swap配置",
		"AdminUpdateSwapConfigDetail":       "更新Swap配置详情",
		"AdminDeleteSwapConfig":             "删除Swap配置",
		"AdminListProjectSwapConfigs":       "查询项目Swap配置列表",
		"AdminEnableSwapConfigForProject":   "为项目启用Swap配置",
		"AdminDisableSwapConfigForProject":  "为项目禁用Swap配置",
		"AdminListSwapTransactions":         "查询Swap交易列表",
	}

	if desc, ok := descriptions[methodName]; ok {
		return desc
	}

	// 未映射的方法返回原方法名
	return methodName
}
