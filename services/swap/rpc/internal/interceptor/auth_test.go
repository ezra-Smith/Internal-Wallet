package interceptor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"internalwallet/services/swap/rpc/internal/config"
	"internalwallet/services/swap/rpc/internal/security"
	"internalwallet/services/swap/rpc/internal/svc"
)

func TestAPIKeyAuthUnaryInterceptor_SwapMethods(t *testing.T) {
	// Non-admin methods should pass without authentication (internal service calls)
	svcCtx := &svc.ServiceContext{
		Config: config.Config{},
	}

	interceptor := APIKeyAuthUnaryInterceptor(svcCtx)
	info := &grpc.UnaryServerInfo{FullMethod: "/swap.Swap/GetQuote"}
	handler := func(ctx context.Context, req any) (any, error) {
		return "ok", nil
	}

	t.Run("no auth required for swap methods", func(t *testing.T) {
		out, err := interceptor(context.Background(), nil, info, handler)
		require.NoError(t, err)
		require.Equal(t, "ok", out)
	})

	t.Run("with metadata also works", func(t *testing.T) {
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-request-id", "test123"))
		out, err := interceptor(ctx, nil, info, handler)
		require.NoError(t, err)
		require.Equal(t, "ok", out)
	})
}

func TestAPIKeyAuthUnaryInterceptor_AdminMethods(t *testing.T) {
	adminToken := "adm-token"

	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			Swap: config.SwapConfig{
				Auth: config.AuthConfig{
					AdminTokenHash: security.HashAdminToken(adminToken),
				},
			},
		},
	}

	interceptor := APIKeyAuthUnaryInterceptor(svcCtx)
	info := &grpc.UnaryServerInfo{FullMethod: "/swap.Swap/CreateSwapApiKey"}
	handler := func(ctx context.Context, req any) (any, error) { return "ok", nil }

	t.Run("missing admin token", func(t *testing.T) {
		_, err := interceptor(context.Background(), nil, info, handler)
		require.Error(t, err)
		require.Equal(t, codes.Unauthenticated, status.Code(err))
	})

	t.Run("invalid admin token", func(t *testing.T) {
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-swap-admin-token", "bad"))
		_, err := interceptor(ctx, nil, info, handler)
		require.Error(t, err)
		require.Equal(t, codes.PermissionDenied, status.Code(err))
	})

	t.Run("valid admin token", func(t *testing.T) {
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-swap-admin-token", adminToken))
		out, err := interceptor(ctx, nil, info, handler)
		require.NoError(t, err)
		require.Equal(t, "ok", out)
	})
}
