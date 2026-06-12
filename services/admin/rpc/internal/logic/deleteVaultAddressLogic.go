package logic

import (
	"context"
	"fmt"
	"strings"
	"time"

	"internalwallet/common/mq"
	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteVaultAddressLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteVaultAddressLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteVaultAddressLogic {
	return &DeleteVaultAddressLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *DeleteVaultAddressLogic) DeleteVaultAddress(in *pb.DeleteVaultAddressRequest) (*pb.DeleteVaultAddressResponse, error) {

	// 1. 参数验证
	if in.Id <= 0 {
		return &pb.DeleteVaultAddressResponse{
			Success:   false,
			Message:   "Invalid address ID",
			RequestId: resp.RequestID(l.ctx),
			Timestamp: time.Now().Format(time.RFC3339),
		}, nil
	}

	// 2. 检查地址是否存在
	addr, err := l.svcCtx.VaultAddressRepo.FindByID(l.ctx, in.Id)
	if err != nil {
		return &pb.DeleteVaultAddressResponse{
			Success:   false,
			Message:   "Address not found",
			RequestId: resp.RequestID(l.ctx),
			Timestamp: time.Now().Format(time.RFC3339),
		}, nil
	}

	// 3. 检查是否是活跃钱包
	if addr.IsActiveWallet == 1 {
		return &pb.DeleteVaultAddressResponse{
			Success:   false,
			Message:   fmt.Sprintf("Cannot delete active wallet. Please set another address as active wallet first."),
			RequestId: resp.RequestID(l.ctx),
			Timestamp: time.Now().Format(time.RFC3339),
		}, nil
	}

	// 4. 检查是否有余额 (可选：如果有余额，可能需要警告)
	totalBalanceUSD, err := l.svcCtx.VaultAddressBalanceRepo.GetTotalBalanceUSD(l.ctx, in.Id)
	if err != nil {
		logx.Errorf("Failed to get total balance for address %d: %v", in.Id, err)
	}
	if totalBalanceUSD > 0 {
		logx.Infof("Warning: Deleting address %d with non-zero balance (USD: %s)", in.Id, centsToUSDString(totalBalanceUSD))
		// 可选：阻止删除有余额的地址
		// return &pb.DeleteVaultAddressResponse{
		// 	Success:   false,
		// 	Message:   fmt.Sprintf("Cannot delete address with non-zero balance (USD: %s)", centsToUSDString(totalBalanceUSD)),
		// 	RequestId: resp.RequestID(l.ctx),
		// 	Timestamp: time.Now().Format(time.RFC3339),
		// }, nil
	}

	// 5. 执行软删除
	if err := l.svcCtx.VaultAddressRepo.Delete(l.ctx, in.Id); err != nil {
		logx.Errorf("Failed to delete vault address: %v", err)
		return &pb.DeleteVaultAddressResponse{
			Success:   false,
			Message:   "Failed to delete address",
			RequestId: resp.RequestID(l.ctx),
			Timestamp: time.Now().Format(time.RFC3339),
		}, err
	}

	// 0. Best-effort: publish address monitor event to Kafka for chainsync incremental updates.
	l.removeVaultAddressMonitor(addr)

	// 6. 记录审计日志
	logx.Infof("Vault address deleted: id=%d, address=%s, network_id=%d", addr.ID, addr.Address, addr.NetworkID)

	// 7. 返回成功
	return &pb.DeleteVaultAddressResponse{
		Success:   true,
		Message:   "Address deleted successfully",
		RequestId: resp.RequestID(l.ctx),
		Timestamp: time.Now().Format(time.RFC3339),
	}, nil
}

func (l *DeleteVaultAddressLogic) removeVaultAddressMonitor(addr *model.VaultAddressModel) {
	if addr == nil || l.svcCtx == nil || l.svcCtx.VaultNetworkRepo == nil {
		return
	}
	// Only active addresses should be monitored.
	if strings.TrimSpace(addr.Status) != vaultAddressStatusActive {
		return
	}

	network, err := l.svcCtx.VaultNetworkRepo.FindByID(l.ctx, addr.NetworkID)
	if err != nil || network == nil {
		logx.WithContext(l.ctx).Errorf("remove vault address monitor: network not found (network_id=%d, err=%v)", addr.NetworkID, err)
		return
	}

	address := strings.TrimSpace(addr.Address)
	if address == "" {
		return
	}

	chain := strings.TrimSpace(network.ChainType)
	if chain == "" {
		chain = strings.TrimSpace(network.Network)
	}
	if chain == "" {
		return
	}

	l.svcCtx.PublishAddressMonitorEvent(l.ctx, mq.AddressMonitorEvent{
		Action:  mq.AddressMonitorActionRemove,
		Source:  mq.AddressMonitorSourceVault,
		Chain:   chain,
		Address: address,
		Reason:  "admin.vault_address.deleted",
		Metadata: map[string]string{
			"vault_address_id": fmt.Sprintf("%d", addr.ID),
			"vault_network_id": fmt.Sprintf("%d", addr.NetworkID),
			"address_type":     strings.TrimSpace(addr.AddressType),
		},
	})
}
