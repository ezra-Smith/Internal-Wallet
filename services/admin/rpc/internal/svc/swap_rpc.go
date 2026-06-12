package svc

import (
	"context"
	"strings"

	commoninterceptor "internalwallet/common/interceptor"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// swapAdminMethods 定义需要使用 admin token 认证的 Swap RPC 方法
var swapAdminMethods = map[string]bool{
	// API Key 管理 (3个方法)
	"CreateSwapApiKey": true,
	"RevokeSwapApiKey": true,
	"ListSwapApiKeys":  true,

	// Provider 管理 (12个方法)
	"CreateSwapProvider":            true,
	"UpdateSwapProvider":            true,
	"DeleteSwapProvider":            true,
	"GetSwapProviderDetail":         true, // Provider 详情
	"ListSwapProviders":             true,
	"ListSwapProvidersForDropdown":  true, // Admin 下拉框接口
	"EnableSwapProvider":            true, // 启用 Provider
	"DisableSwapProvider":           true, // 禁用 Provider
	"EnableSwapProviderForProject":  true,
	"DisableSwapProviderForProject": true,
	"GetSwapProviderByProject":      true,

	// Config 管理 (5个方法)
	"ListSwapConfigs":        true,
	"GetSwapConfigDetail":    true,
	"CreateSwapConfig":       true,
	"UpdateSwapConfigDetail": true,
	"DeleteSwapConfig":       true,

	// Project Config 管理 (3个方法)
	"ListProjectSwapConfigs":      true,
	"EnableSwapConfigForProject":  true,
	"DisableSwapConfigForProject": true,

	// Swap 交易查询 (1个方法)
	"ListSwapTransactions": true,
}

// swapClientInterceptor wraps common metadata propagation and adds Swap service authentication.
// Admin 方法使用 x-swap-admin-token，业务方法使用 Authorization Bearer token.
func swapClientInterceptor(adminToken string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		// Debug log
		logx.Infof("=== [ADMIN -> SWAP] Calling Swap RPC method: %s ===", method)
		logx.Infof("=== [ADMIN -> SWAP] Admin token configured: %v ===", adminToken != "")
		logx.Infof("=== [ADMIN -> SWAP] Admin token length: %d ===", len(adminToken))

		// 提取方法名：/swap.Swap/CreateSwapConfig -> CreateSwapConfig
		methodName := extractMethodName(method)
		logx.Infof("=== [ADMIN -> SWAP] Extracted method name: %s ===", methodName)

		// Step 1: Extract metadata from context (使用公共函数)
		extractedMd := commoninterceptor.ExtractMetadataFromContext(ctx)

		// Step 2: 根据方法类型设置认证 header
		if swapAdminMethods[methodName] {
			// Admin 方法：使用 x-swap-admin-token
			if adminToken != "" {
				extractedMd.Set("x-swap-admin-token", adminToken)
				logx.Infof("=== [ADMIN -> SWAP] Using admin token authentication for method: %s ===", methodName)
			} else {
				logx.Errorf("=== [ADMIN -> SWAP] Admin token is empty, admin method calls will fail! ===")
			}
		} else {
			// 业务方法：使用 Authorization Bearer token（当前 Admin 不调用业务方法，但预留结构）
			logx.Infof("=== [ADMIN -> SWAP] Method %s is not an admin method, would use Bearer token ===", methodName)
		}

		logx.Infof("=== [ADMIN -> SWAP] Final metadata to send: %v ===", extractedMd)

		// Step 3: Create new context with metadata
		ctx = metadata.NewOutgoingContext(ctx, extractedMd)

		// Step 4: Invoke the RPC
		err := invoker(ctx, method, req, reply, cc, opts...)

		if err != nil {
			logx.Errorf("=== [ADMIN -> SWAP] RPC call failed for %s: %v ===", methodName, err)
		} else {
			logx.Infof("=== [ADMIN -> SWAP] RPC call succeeded for %s ===", methodName)
		}

		return err
	}
}

// extractMethodName 从完整的 RPC 方法路径中提取方法名
// 例如：/swap.Swap/CreateSwapConfig -> CreateSwapConfig
func extractMethodName(fullMethod string) string {
	parts := strings.Split(fullMethod, "/")
	if len(parts) >= 3 {
		return parts[2]
	}
	return fullMethod
}
