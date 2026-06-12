package logic

import (
	"context"
	"time"

	"internalwallet/services/admin/rpc/internal/repository"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

// VaultBalanceSyncScheduler 定期同步金库地址余额的定时任务
type VaultBalanceSyncScheduler struct {
	svcCtx       *svc.ServiceContext
	syncInterval time.Duration
}

// NewVaultBalanceSyncScheduler 创建新的金库余额同步调度器
func NewVaultBalanceSyncScheduler(svcCtx *svc.ServiceContext, syncInterval time.Duration) *VaultBalanceSyncScheduler {
	if syncInterval <= 0 {
		syncInterval = 5 * time.Minute // 默认5分钟
	}
	return &VaultBalanceSyncScheduler{
		svcCtx:       svcCtx,
		syncInterval: syncInterval,
	}
}

// Run 启动定时同步任务
func (s *VaultBalanceSyncScheduler) Run(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	if s == nil || s.svcCtx == nil {
		logx.WithContext(ctx).Error("vault balance sync scheduler disabled: svcCtx not configured")
		return
	}
	if s.svcCtx.DB == nil || s.svcCtx.VaultNetworkRepo == nil || s.svcCtx.VaultAddressRepo == nil {
		logx.WithContext(ctx).Error("vault balance sync scheduler disabled: db/repositories not configured")
		return
	}
	if s.svcCtx.ChainSyncClient == nil {
		logx.WithContext(ctx).Error("vault balance sync scheduler disabled: ChainSyncClient not configured")
		return
	}

	t := time.NewTicker(s.syncInterval)
	defer t.Stop()

	logx.WithContext(ctx).Infof("vault balance sync scheduler started (interval=%s)", s.syncInterval)

	// 启动时立即执行一次同步
	go s.syncAllNetworks(ctx)

	for {
		select {
		case <-ctx.Done():
			logx.WithContext(ctx).Info("vault balance sync scheduler stopped")
			return
		case <-t.C:
			s.syncAllNetworks(ctx)
		}
	}
}

// syncAllNetworks 同步所有网络的金库地址余额
func (s *VaultBalanceSyncScheduler) syncAllNetworks(ctx context.Context) {
	logger := logx.WithContext(ctx)
	logger.Info("开始定时同步所有网络的金库地址余额...")

	// 查询所有启用的网络（status为空字符串表示查询所有）
	networks, err := s.svcCtx.VaultNetworkRepo.List(ctx, "")
	if err != nil {
		logger.Errorf("查询 vault 网络失败: %v", err)
		return
	}

	if len(networks) == 0 {
		logger.Info("没有配置的 vault 网络，跳过同步")
		return
	}

	logger.Infof("找到 %d 个网络需要同步", len(networks))

	successCount := 0
	failCount := 0

	for _, network := range networks {
		if network == nil {
			continue
		}

		// 查询该网络下的所有地址
		filter := &repository.VaultAddressFilter{
			NetworkID: network.ID,
		}
		addresses, _, err := s.svcCtx.VaultAddressRepo.List(ctx, filter, 1, 1000)
		if err != nil {
			logger.Errorf("查询网络 %s (ID=%d) 的地址失败: %v", network.Network, network.ID, err)
			failCount++
			continue
		}

		if len(addresses) == 0 {
			logger.Infof("网络 %s (ID=%d) 没有配置地址，跳过", network.Network, network.ID)
			continue
		}

		logger.Infof("同步网络 %s (ID=%d) 的 %d 个地址", network.Network, network.ID, len(addresses))

		networkSuccessCount := 0
		networkFailCount := 0

		// 同步每个地址的余额
		for _, addr := range addresses {
			if addr == nil || addr.Address == "" {
				continue
			}

			syncCtx, syncCancel := context.WithTimeout(ctx, 30*time.Second)
			if err := SyncVaultAddressBalancesHelper(syncCtx, s.svcCtx, logger, addr); err != nil {
				logger.Errorf("同步地址余额失败: network=%s address=%s address_id=%d error=%v",
					network.Network, maskAddress(addr.Address), addr.ID, err)
				failCount++
				networkFailCount++
			} else {
				logger.Debugf("✓ 地址余额同步成功: network=%s address=%s address_id=%d",
					network.Network, maskAddress(addr.Address), addr.ID)
				successCount++
				networkSuccessCount++
			}
			syncCancel()
		}

		// 同步完该网络的所有地址后，统一更新 vault_balances 汇总表
		// 即使某些地址同步失败，也要更新汇总表（基于已成功同步的地址数据）
		if networkSuccessCount > 0 || networkFailCount > 0 {
			logger.Infof("网络 %s (ID=%d) 地址同步完成，开始更新 vault_balances 汇总表...", network.Network, network.ID)
			syncCtx, syncCancel := context.WithTimeout(ctx, 30*time.Second)
			if err := syncVaultBalancesFromAddresses(syncCtx, s.svcCtx, logger, network.ID); err != nil {
				logger.Errorf("更新 vault_balances 汇总表失败: network=%s network_id=%d error=%v",
					network.Network, network.ID, err)
			} else {
				logger.Infof("✓ vault_balances 汇总表更新成功: network=%s network_id=%d", network.Network, network.ID)
			}
			syncCancel()
		}
	}

	logger.Infof("定时同步完成: 成功=%d, 失败=%d, 网络数=%d", successCount, failCount, len(networks))
}
