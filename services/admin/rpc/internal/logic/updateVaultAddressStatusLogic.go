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

type UpdateVaultAddressStatusLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateVaultAddressStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateVaultAddressStatusLogic {
	return &UpdateVaultAddressStatusLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateVaultAddressStatusLogic) UpdateVaultAddressStatus(in *pb.UpdateVaultAddressStatusRequest) (*pb.UpdateVaultAddressStatusResponse, error) {

	// 1. 参数验证
	if in.Id <= 0 {
		return &pb.UpdateVaultAddressStatusResponse{
			Success:   false,
			Message:   "Invalid address ID",
			RequestId: resp.RequestID(l.ctx),
			Timestamp: time.Now().Format(time.RFC3339),
		}, nil
	}

	if !validateVaultAddressStatus(in.Status) {
		return &pb.UpdateVaultAddressStatusResponse{
			Success:   false,
			Message:   fmt.Sprintf("Invalid status, must be one of: active, inactive, disabled"),
			RequestId: resp.RequestID(l.ctx),
			Timestamp: time.Now().Format(time.RFC3339),
		}, nil
	}

	// 2. 检查地址是否存在
	addr, err := l.svcCtx.VaultAddressRepo.FindByID(l.ctx, in.Id)
	if err != nil {
		return &pb.UpdateVaultAddressStatusResponse{
			Success:   false,
			Message:   "Address not found",
			RequestId: resp.RequestID(l.ctx),
			Timestamp: time.Now().Format(time.RFC3339),
		}, nil
	}
	prevStatus := strings.TrimSpace(addr.Status)

	// 3. 检查是否设置为活跃钱包
	var isActiveWallet int8 = 0
	if in.SetAsActiveWallet && in.Status == vaultAddressStatusActive {
		// 只有 address_type=active 的地址才能设置为活跃钱包
		if addr.AddressType != vaultAddressTypeActive {
			return &pb.UpdateVaultAddressStatusResponse{
				Success:   false,
				Message:   "Only addresses with type 'active' can be set as active wallet",
				RequestId: resp.RequestID(l.ctx),
				Timestamp: time.Now().Format(time.RFC3339),
			}, nil
		}

		// 检查同一网络是否已有活跃钱包
		existingActive, err := l.svcCtx.VaultAddressRepo.GetActiveWallet(l.ctx, addr.NetworkID)
		if err != nil {
			logx.Errorf("Failed to get active wallet: %v", err)
			return &pb.UpdateVaultAddressStatusResponse{
				Success:   false,
				Message:   "Internal server error",
				RequestId: resp.RequestID(l.ctx),
				Timestamp: time.Now().Format(time.RFC3339),
			}, err
		}

		if existingActive != nil && existingActive.ID != in.Id {
			return &pb.UpdateVaultAddressStatusResponse{
				Success:   false,
				Message:   fmt.Sprintf("Network already has an active wallet (address: %s)", existingActive.Address),
				RequestId: resp.RequestID(l.ctx),
				Timestamp: time.Now().Format(time.RFC3339),
			}, nil
		}

		isActiveWallet = 1
	}

	// 4. 更新状态
	if err := l.svcCtx.VaultAddressRepo.UpdateStatus(l.ctx, in.Id, in.Status, isActiveWallet); err != nil {
		logx.Errorf("Failed to update vault address status: %v", err)
		return &pb.UpdateVaultAddressStatusResponse{
			Success:   false,
			Message:   "Failed to update address status",
			RequestId: resp.RequestID(l.ctx),
			Timestamp: time.Now().Format(time.RFC3339),
		}, err
	}

	// 4. Best-effort: publish address monitor event to Kafka for chainsync incremental updates.
	l.syncVaultAddressMonitor(addr, prevStatus, strings.TrimSpace(in.Status))

	// 5. 返回成功
	return &pb.UpdateVaultAddressStatusResponse{
		Success: true,
		Message: "Address status updated successfully",
		Data: &pb.UpdateVaultAddressStatusData{
			Id:             in.Id,
			Status:         in.Status,
			IsActiveWallet: isActiveWallet == 1,
			UpdatedAt:      time.Now().Format(time.RFC3339),
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: time.Now().Format(time.RFC3339),
	}, nil
}

func (l *UpdateVaultAddressStatusLogic) syncVaultAddressMonitor(addr *model.VaultAddressModel, prevStatus string, nextStatus string) {
	if addr == nil || l.svcCtx == nil || l.svcCtx.VaultNetworkRepo == nil {
		return
	}
	prevStatus = strings.TrimSpace(prevStatus)
	nextStatus = strings.TrimSpace(nextStatus)
	if prevStatus == nextStatus {
		return
	}

	var action mq.AddressMonitorAction
	if prevStatus != vaultAddressStatusActive && nextStatus == vaultAddressStatusActive {
		action = mq.AddressMonitorActionUpsert
	} else if prevStatus == vaultAddressStatusActive && nextStatus != vaultAddressStatusActive {
		action = mq.AddressMonitorActionRemove
	} else {
		return
	}

	network, err := l.svcCtx.VaultNetworkRepo.FindByID(l.ctx, addr.NetworkID)
	if err != nil || network == nil {
		logx.WithContext(l.ctx).Errorf("sync vault address monitor: network not found (network_id=%d, err=%v)", addr.NetworkID, err)
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
		Action:  action,
		Source:  mq.AddressMonitorSourceVault,
		Chain:   chain,
		Address: address,
		Reason:  fmt.Sprintf("admin.vault_address.status_%s", nextStatus),
		Metadata: map[string]string{
			"vault_address_id": fmt.Sprintf("%d", addr.ID),
			"vault_network_id": fmt.Sprintf("%d", addr.NetworkID),
			"address_type":     strings.TrimSpace(addr.AddressType),
		},
	})
}
