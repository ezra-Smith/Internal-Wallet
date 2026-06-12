package svc

import (
	"context"
	"fmt"
	"strings"

	commoninterceptor "internalwallet/common/interceptor"
	"internalwallet/proto/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc/metadata"
)

// withAdminAuth ensures Admin RPC calls can pass AdminAuthUnaryInterceptor by attaching a valid x-user-id.
// Note: This is a service-to-service call; we reuse the seeded super_admin id in bootstrap SQL.
func (al *AddressLoader) withAdminAuth(ctx context.Context) context.Context {
	const adminServiceUserID = "10001"
	if ctx == nil {
		ctx = context.Background()
	}

	if md, ok := metadata.FromOutgoingContext(ctx); ok {
		if v := md.Get(commoninterceptor.GRPCUserIDKey); len(v) > 0 && strings.TrimSpace(v[0]) != "" {
			return ctx
		}
	}

	return metadata.AppendToOutgoingContext(ctx, commoninterceptor.GRPCUserIDKey, adminServiceUserID)
}

// connectToAdmin 连接到admin服务
func (al *AddressLoader) connectToAdmin() error {
	al.mu.Lock()
	defer al.mu.Unlock()

	client := zrpc.MustNewClient(al.config.Admin)
	al.adminClient = pb.NewAdminClient(client.Conn())

	logx.Info("Successfully connected to admin service via zrpc")
	return nil
}

// connectToSigner 连接到signer服务
func (al *AddressLoader) connectToSigner() error {
	al.mu.Lock()
	defer al.mu.Unlock()

	client := zrpc.MustNewClient(al.config.Signer)
	al.signerClient = pb.NewSignerServiceClient(client.Conn())

	logx.Info("Successfully connected to signer service via zrpc")
	return nil
}

// ensureAdminConnection 确保连接到 Admin 服务可用，如果断开则重连
func (al *AddressLoader) ensureAdminConnection() error {
	al.mu.RLock()
	adminClient := al.adminClient
	al.mu.RUnlock()

	if adminClient == nil {
		if err := al.connectToAdmin(); err != nil {
			return fmt.Errorf("failed to connect to admin: %w", err)
		}
	}
	return nil
}

// ensureSignerConnection 确保连接到 Signer 服务可用，如果断开则重连
func (al *AddressLoader) ensureSignerConnection() error {
	al.mu.RLock()
	signerClient := al.signerClient
	al.mu.RUnlock()

	if signerClient == nil {
		if err := al.connectToSigner(); err != nil {
			return fmt.Errorf("failed to connect to signer: %w", err)
		}
	}

	return nil
}

// ensureConnection 确保连接可用，如果断开则重连（兼容旧代码：同时依赖 admin + signer）
func (al *AddressLoader) ensureConnection() error {
	if err := al.ensureAdminConnection(); err != nil {
		return err
	}
	if err := al.ensureSignerConnection(); err != nil {
		return err
	}
	return nil
}

// ConvertChainType 转换链类型字符串为枚举
func (al *AddressLoader) ConvertChainType(chainStr string) pb.BlockChainType {
	switch strings.ToUpper(strings.TrimSpace(chainStr)) {
	case "ETH", "ETHEREUM":
		return pb.BlockChainType_CHAIN_TYPE_ETHEREUM
	case "BSC", "BNB", "BNB SMART CHAIN":
		return pb.BlockChainType_CHAIN_TYPE_BSC
	case "TRN", "TRON", "TRX":
		return pb.BlockChainType_CHAIN_TYPE_TRON
	default:
		return pb.BlockChainType_CHAIN_TYPE_UNSPECIFIED
	}
}

// Close 关闭连接
func (al *AddressLoader) Close() {
	al.mu.Lock()
	defer al.mu.Unlock()

	al.adminClient = nil
	al.signerClient = nil
	logx.Info("AddressLoader connections closed")
}

// GetConnectionStatus 获取连接状态
func (al *AddressLoader) GetConnectionStatus() bool {
	al.mu.RLock()
	defer al.mu.RUnlock()
	return al.adminClient != nil && al.signerClient != nil
}
