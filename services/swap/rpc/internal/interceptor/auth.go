package interceptor

import (
	"context"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"internalwallet/services/swap/rpc/internal/model"
	"internalwallet/services/swap/rpc/internal/security"
	"internalwallet/services/swap/rpc/internal/svc"
)

type contextKey string

const (
	ContextKeyAPIKeyID    contextKey = "api_key_id"
	ContextKeyProjectName contextKey = "project_name"
)

// APIKeyAuthUnaryInterceptor enforces swap-service auth:
// - API key management APIs + Configuration management: require `x-swap-admin-token: <token>`
// - Other swap APIs: require API key authentication (multi-tenant)
func APIKeyAuthUnaryInterceptor(svcCtx *svc.ServiceContext) grpc.UnaryServerInterceptor {
	adminMethods := map[string]bool{
		// API Key 管理
		"CreateSwapApiKey": true,
		"RevokeSwapApiKey": true,
		"ListSwapApiKeys":  true,

		// Provider 管理
		"CreateSwapProvider":           true,
		"UpdateSwapProvider":           true,
		"DeleteSwapProvider":           true,
		"GetSwapProviderDetail":        true, // Provider 详情
		"ListSwapProviders":            true,
		"ListSwapProvidersForDropdown": true, // Admin 下拉框接口
		"EnableSwapProvider":           true, // 启用 Provider
		"DisableSwapProvider":          true, // 禁用 Provider

		// Config 管理
		"ListSwapConfigs":        true,
		"GetSwapConfigDetail":    true,
		"CreateSwapConfig":       true,
		"UpdateSwapConfigDetail": true,
		"DeleteSwapConfig":       true,

		// Project Config 管理
		"ListProjectSwapConfigs":      true,
		"EnableSwapConfigForProject":  true,
		"DisableSwapConfigForProject": true,
	}

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		methodName := extractMethodName(info.FullMethod)

		// Admin methods: require admin token
		if adminMethods[methodName] {
			if err := requireAdminToken(ctx, svcCtx); err != nil {
				return nil, err
			}
			return handler(ctx, req)
		}

		// Business methods: require API Key
		apiKey, err := extractAndValidateAPIKey(ctx, svcCtx)
		if err != nil {
			return nil, err
		}

		// Inject auth context
		ctx = context.WithValue(ctx, ContextKeyAPIKeyID, apiKey.ID)
		ctx = context.WithValue(ctx, ContextKeyProjectName, apiKey.ProjectName)

		return handler(ctx, req)
	}
}

func extractAndValidateAPIKey(ctx context.Context, svcCtx *svc.ServiceContext) (*model.SwapSvcApiKeyModel, error) {
	if svcCtx.ApiKeyRepo == nil {
		return nil, status.Error(codes.Internal, "api key repository not configured")
	}

	pepper := strings.TrimSpace(svcCtx.Config.Swap.Auth.ApiKeyPepper)
	if pepper == "" {
		return nil, status.Error(codes.Internal, "api key pepper not configured")
	}

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		logx.Info("=== [SWAP SERVER] No metadata in incoming context ===")
		return nil, status.Error(codes.Unauthenticated, "missing metadata")
	}

	logx.Infof("=== [SWAP SERVER] Incoming metadata keys: %v ===", md)

	authHeader := ""
	if vals := md.Get("authorization"); len(vals) > 0 {
		authHeader = vals[0]
		logx.Infof("=== [SWAP SERVER] Found authorization header: %s ===", authHeader[:20]+"...")
	} else {
		logx.Errorf("=== [SWAP SERVER] No authorization header found in metadata ===")
		return nil, status.Error(codes.Unauthenticated, "missing or invalid authorization header")
	}

	if !strings.HasPrefix(authHeader, "Bearer ") {
		logx.Errorf("=== [SWAP SERVER] Authorization header missing 'Bearer ' prefix: %s ===", authHeader)
		return nil, status.Error(codes.Unauthenticated, "missing or invalid authorization header")
	}

	apiKey := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
	if apiKey == "" {
		logx.Errorf("=== [SWAP SERVER] API key is empty after trimming ===")
		return nil, status.Error(codes.Unauthenticated, "empty API key")
	}

	logx.Infof("=== [SWAP SERVER] Extracted API key (length: %d) ===", len(apiKey))

	keyHash, err := security.HashAPIKey(pepper, apiKey)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to hash api key")
	}
	// Query database
	keyModel, err := svcCtx.ApiKeyRepo.FindByKeyHash(ctx, keyHash)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid API key")
	}

	if !keyModel.IsActive {
		return nil, status.Error(codes.PermissionDenied, "API key is disabled")
	}

	if keyModel.ExpiresAt != nil && time.Now().After(*keyModel.ExpiresAt) {
		return nil, status.Error(codes.PermissionDenied, "API key expired")
	}

	return keyModel, nil
}

// GetAPIKeyID extracts API key ID from context
func GetAPIKeyID(ctx context.Context) int64 {
	if v := ctx.Value(ContextKeyAPIKeyID); v != nil {
		if id, ok := v.(int64); ok {
			return id
		}
	}
	return 0
}

// GetProjectName extracts project name from context
func GetProjectName(ctx context.Context) string {
	if v := ctx.Value(ContextKeyProjectName); v != nil {
		if name, ok := v.(string); ok {
			return name
		}
	}
	return ""
}

func requireAdminToken(ctx context.Context, svcCtx *svc.ServiceContext) error {
	cfgHash := strings.TrimSpace(svcCtx.Config.Swap.Auth.AdminTokenHash)
	if cfgHash == "" {
		return status.Error(codes.Internal, "swap admin token not configured")
	}

	pepper := strings.TrimSpace(svcCtx.Config.Swap.Auth.ApiKeyPepper)
	if pepper == "" {
		return status.Error(codes.Internal, "swap api key pepper not configured")
	}

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "missing metadata")
	}
	token := ""
	if vals := md.Get("x-swap-admin-token"); len(vals) > 0 {
		token = strings.TrimSpace(vals[0])
	}
	if token == "" {
		return status.Error(codes.Unauthenticated, "missing x-swap-admin-token")
	}

	// 使用与 API Key 相同的哈希逻辑：HMAC-SHA256(pepper, adminToken)
	hash, err := security.HashAPIKey(pepper, token)
	if err != nil {
		return status.Error(codes.Internal, "failed to hash admin token")
	}

	if !strings.EqualFold(hash, cfgHash) {
		logx.Errorf("Admin token hash mismatch: expected=%s, got=%s", cfgHash, hash)
		return status.Error(codes.PermissionDenied, "invalid admin token")
	}
	return nil
}

func extractMethodName(fullMethod string) string {
	// fullMethod: /{package}.{service}/{method}
	if fullMethod == "" {
		return ""
	}
	parts := strings.Split(fullMethod, "/")
	if len(parts) == 0 {
		return fullMethod
	}
	return parts[len(parts)-1]
}
