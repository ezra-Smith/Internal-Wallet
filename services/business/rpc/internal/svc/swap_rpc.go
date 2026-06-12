package svc

import (
	"context"

	commoninterceptor "internalwallet/common/interceptor"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// swapClientInterceptor wraps common metadata propagation and adds Swap API key authentication.
func swapClientInterceptor(apiKey string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		// Debug log
		logx.Infof("=== [SWAP CLIENT] Calling Swap RPC method: %s ===", method)
		logx.Infof("=== [SWAP CLIENT] API Key configured: %v ===", apiKey != "")
		logx.Infof("=== [SWAP CLIENT] API Key length: %d ===", len(apiKey))

		// Step 1: Extract metadata from context (使用公共函数)
		extractedMd := commoninterceptor.ExtractMetadataFromContext(ctx)

		// Step 2: Add authorization header
		if apiKey != "" {
			extractedMd.Set("authorization", "Bearer "+apiKey)
			logx.Info("=== [SWAP CLIENT] Set authorization header ===")
		} else {
			logx.Errorf("=== [SWAP CLIENT] API Key is empty, Swap calls will likely fail! ===")
		}

		logx.Infof("=== [SWAP CLIENT] Final metadata to send: %v ===", extractedMd)

		// Step 3: Create new context with metadata
		ctx = metadata.NewOutgoingContext(ctx, extractedMd)

		// Step 4: Invoke the RPC
		err := invoker(ctx, method, req, reply, cc, opts...)

		if err != nil {
			logx.Errorf("=== [SWAP CLIENT] RPC call failed: %v ===", err)
		} else {
			logx.Infof("=== [SWAP CLIENT] RPC call succeeded ===")
		}

		return err
	}
}
