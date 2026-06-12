package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"internalwallet/common/middleware"
	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	admininterceptor "internalwallet/services/admin/rpc/internal/interceptor"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/repository"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"gorm.io/gorm"
)

type SyncVaultLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSyncVaultLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SyncVaultLogic {
	return &SyncVaultLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SyncVaultLogic) SyncVault(in *pb.SyncVaultRequest) (*pb.SyncVaultResponse, error) {
	if in == nil {
		in = &pb.SyncVaultRequest{}
	}
	if in.ChainId == 0 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid chain_id", map[string]string{"chain_id": "required"})
	}
	if l.svcCtx.DB == nil ||
		l.svcCtx.VaultNetworkRepo == nil ||
		l.svcCtx.VaultSyncTaskRepo == nil ||
		l.svcCtx.VaultBalanceRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	current, ok := admininterceptor.GetCurrentAdmin(l.ctx)
	if !ok || current == nil {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}

	network, err := l.svcCtx.VaultNetworkRepo.FindByChainID(l.ctx, in.ChainId)
	if err != nil || network == nil {
		return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "NETWORK_NOT_FOUND", "network not found", nil)
	}

	// 查找该网络下的所有 vault 地址
	filter := &repository.VaultAddressFilter{
		NetworkID: network.ID,
	}
	vaultAddresses, _, err := l.svcCtx.VaultAddressRepo.List(l.ctx, filter, 1, 1000)
	if err != nil {
		l.Logger.Errorf("查询 vault 地址失败: network_id=%d, error=%v", network.ID, err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "failed to query vault addresses", nil)
	}
	if len(vaultAddresses) == 0 {
		return nil, errx.New(codes.FailedPrecondition, 428, errx.CodeInvalidParam, "VAULT_ADDRESS_NOT_CONFIGURED", "no vault addresses configured for this network", nil)
	}

	l.Logger.Infof("找到 %d 个 vault 地址需要同步: network=%s, network_id=%d", len(vaultAddresses), network.Network, network.ID)

	// 在同步前确保所有热钱包地址都在 vault_addresses 中
	l.Logger.Info("检查并同步公司热钱包到 vault_addresses...")
	if err := EnsureCompanyHotWalletsInVault(l.ctx, l.svcCtx, l.Logger); err != nil {
		l.Logger.Errorf("同步热钱包到 vault_addresses 失败: %v (继续执行余额同步)", err)
		// 不阻塞余额同步流程，仅记录错误
	}

	now := time.Now()
	ip := middleware.GetClientIP(l.ctx)
	ua := middleware.GetUserAgent(l.ctx)

	var task *model.VaultSyncTaskModel
	if err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		taskRepo := l.svcCtx.VaultSyncTaskRepo.WithTx(tx)
		networkRepo := l.svcCtx.VaultNetworkRepo.WithTx(tx)
		auditRepo := repository.NewAdminAuditLogRepository(tx)

		m := &model.VaultSyncTaskModel{
			NetworkID: network.ID,
			ChainID:   network.ChainID,
			Network:   network.Network,
			Status:    "syncing",
			StartedAt: &now,
			CreatedBy: current.ID,
			CreatedAt: &now,
			UpdatedAt: &now,
		}
		if err := taskRepo.Create(l.ctx, m); err != nil {
			return err
		}
		task = m

		_ = networkRepo.UpdateFields(l.ctx, network.ID, map[string]interface{}{
			"sync_status": "syncing",
		})

		addressList := make([]string, 0, len(vaultAddresses))
		for _, addr := range vaultAddresses {
			if addr != nil && addr.Address != "" {
				addressList = append(addressList, maskAddress(addr.Address))
			}
		}
		detailsBytes, _ := json.Marshal(map[string]interface{}{
			"chain_id":       network.ChainID,
			"network":        network.Network,
			"vault_addresses": addressList,
			"address_count":  len(vaultAddresses),
			"sync_task_id":   m.ID,
		})
		_ = auditRepo.CreateLog(l.ctx, &model.AdminAuditLogModel{
			AdminID:     current.ID,
			Action:      "vault.sync.start",
			TargetType:  "vault_network",
			TargetID:    fmt.Sprintf("%d", network.ID),
			Description: "触发 Vault 手动同步",
			Details:     detailsBytes,
			IP:          ip,
			UserAgent:   ua,
		})
		return nil
	}); err != nil {
		l.Logger.Errorf("create vault sync task failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "sync failed", nil)
	}

	// Best-effort synchronous sync if ChainSync client is configured.
	if l.svcCtx.ChainSyncClient != nil {
		// 同步所有 vault 地址的余额
		successCount := 0
		failCount := 0
		for _, vaultAddr := range vaultAddresses {
			if vaultAddr == nil || vaultAddr.Address == "" {
				continue
			}
			
			l.Logger.Infof("开始同步地址余额: address=%s, address_id=%d, network=%s", 
				vaultAddr.Address, vaultAddr.ID, network.Network)
			
			// 使用 helper 函数同步单个地址的余额
			if err := SyncVaultAddressBalancesHelper(l.ctx, l.svcCtx, l.Logger, vaultAddr); err != nil {
				l.Logger.Errorf("同步地址余额失败: address=%s, address_id=%d, error=%v", 
					vaultAddr.Address, vaultAddr.ID, err)
				failCount++
				continue
			}
			
			l.Logger.Infof("✓ 地址余额同步成功: address=%s, address_id=%d", 
				vaultAddr.Address, vaultAddr.ID)
			successCount++
		}
		
		l.Logger.Infof("地址余额同步完成: 成功=%d, 失败=%d, 总数=%d", 
			successCount, failCount, len(vaultAddresses))
		
		// 如果所有地址都失败了，返回错误
		if successCount == 0 && failCount > 0 {
			_ = l.markSyncFailed(task.ID, network.ID, fmt.Sprintf("all %d addresses sync failed", failCount))
			return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "SYNC_FAILED", 
				fmt.Sprintf("failed to sync all %d vault addresses", failCount), nil)
		}
		
		// 成功同步后，标记任务完成
		if successCount > 0 {
			_ = l.markSyncSuccess(task.ID, network.ID)
		}
	}

	return &pb.SyncVaultResponse{
		Success: true,
		Message: resp.Msg(l.ctx, "SYNC_TASK_STARTED"),
		Data: &pb.SyncVaultData{
			Network:   network.Network,
			SyncId:    fmt.Sprintf("sync-%d", task.ID),
			Status:    "syncing",
			StartedAt: formatTime(now),
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}

func (l *SyncVaultLogic) markSyncSuccess(taskID int64, networkID int64) error {
	now := time.Now()
	if l.svcCtx.VaultSyncTaskRepo != nil {
		_ = l.svcCtx.VaultSyncTaskRepo.UpdateFields(l.ctx, taskID, map[string]interface{}{
			"status":      "completed",
			"finished_at": &now,
			"updated_at":  &now,
		})
	}
	if l.svcCtx.VaultNetworkRepo != nil {
		_ = l.svcCtx.VaultNetworkRepo.UpdateFields(l.ctx, networkID, map[string]interface{}{
			"sync_status":  "synced",
			"last_sync_at": &now,
		})
	}
	return nil
}

func (l *SyncVaultLogic) markSyncFailed(taskID int64, networkID int64, msg string) error {
	now := time.Now()
	if l.svcCtx.VaultSyncTaskRepo != nil {
		_ = l.svcCtx.VaultSyncTaskRepo.UpdateFields(l.ctx, taskID, map[string]interface{}{
			"status":        "failed",
			"finished_at":   &now,
			"error_message": msg,
			"updated_at":    &now,
		})
	}
	if l.svcCtx.VaultNetworkRepo != nil {
		_ = l.svcCtx.VaultNetworkRepo.UpdateFields(l.ctx, networkID, map[string]interface{}{
			"sync_status": "error",
		})
	}
	return nil
}

func timePtr(t time.Time) *time.Time { return &t }
