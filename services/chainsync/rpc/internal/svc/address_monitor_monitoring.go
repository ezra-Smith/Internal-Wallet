package svc

import (
	"context"
	"fmt"
	"sync"
	"time"

	"internalwallet/proto/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

// monitoringLoop 监控循环
func (am *AddressMonitor) monitoringLoop() {
	defer am.wg.Done()

	// 根据配置设置检查间隔
	checkInterval := 10 * time.Second
	if am.config != nil && am.config.Monitoring.AddressMonitoring != nil {
		checkInterval = time.Duration(am.config.Monitoring.AddressMonitoring.CheckInterval) * time.Second
	}

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-am.stopCh:
			logx.Info("Address monitoring loop stopped")
			return
		case <-ticker.C:
			am.checkAllChainActivity()
		}
	}
}

// checkAllChainActivity 检查所有链的活动（基于区块的增量扫描）
func (am *AddressMonitor) checkAllChainActivity() {
	if am.registry == nil {
		return
	}

	var wg sync.WaitGroup
	for _, chainType := range []pb.BlockChainType{
		pb.BlockChainType_CHAIN_TYPE_ETHEREUM,
		pb.BlockChainType_CHAIN_TYPE_BSC,
		pb.BlockChainType_CHAIN_TYPE_TRON,
	} {
		addressSet := am.registry.BuildMonitoredAddressSet(chainType)
		if chainType == pb.BlockChainType_CHAIN_TYPE_ETHEREUM {
			// #region agent log
			writeDebugLog(map[string]interface{}{
				"sessionId":    "debug-session",
				"runId":        "pre-fix",
				"hypothesisId": "H2",
				"location":     "address_monitor.go:checkAllChainActivity",
				"message":      "built monitored address set",
				"data": map[string]interface{}{
					"chain":         chainType.String(),
					"addressSetLen": len(addressSet),
				},
				"timestamp": time.Now().UnixMilli(),
			})
			// #endregion
		}
		if len(addressSet) == 0 {
			continue
		}
		wg.Add(1)
		go func(ct pb.BlockChainType, set map[string]bool) {
			defer wg.Done()
			am.checkChainActivity(ct, set)
		}(chainType, addressSet)
	}
	wg.Wait()
}

// checkChainActivity 检查特定链的地址活动（优化：区块中心批量扫描）
func (am *AddressMonitor) checkChainActivity(chainType pb.BlockChainType, addressSet map[string]bool) {
	if len(addressSet) == 0 {
		return
	}

	if chainType == pb.BlockChainType_CHAIN_TYPE_ETHEREUM {
		// #region agent log
		writeDebugLog(map[string]interface{}{
			"sessionId":    "debug-session",
			"runId":        "pre-fix",
			"hypothesisId": "H2",
			"location":     "address_monitor.go:checkChainActivity:entry",
			"message":      "checkChainActivity invoked",
			"data": map[string]interface{}{
				"chain":         chainType.String(),
				"addressSetLen": len(addressSet),
			},
			"timestamp": time.Now().UnixMilli(),
		})
		// #endregion
	}

	// Guard against overlapping scans per chain.
	// The monitoring loop ticks frequently and starts goroutines; without a guard,
	// multiple scans for the same chain can run concurrently and overwrite the
	// Redis cursor out-of-order (regression/flapping), delaying deposits.
	if am.scanGuards != nil {
		if guard, ok := am.scanGuards[chainType]; ok && guard != nil {
			select {
			case guard <- struct{}{}:
				defer func() { <-guard }()
			default:
				logx.Debugf("⏭️ Skipping scan for chain %v: previous scan still running", chainType)
				return
			}
		}
	}

	logx.Infof("🔍 Starting block-based scanning for %d monitored addresses on chain %v", len(addressSet), chainType)

	provider, err := am.providerPool.GetProvider(chainType)
	if err != nil {
		logx.Errorf("Failed to get provider for chain %v address monitoring: %v", chainType, err)
		return
	}

	if chainType == pb.BlockChainType_CHAIN_TYPE_ETHEREUM {
		// #region agent log
		writeDebugLog(map[string]interface{}{
			"sessionId":    "debug-session",
			"runId":        "pre-fix",
			"hypothesisId": "H6",
			"location":     "address_monitor.go:checkChainActivity:provider",
			"message":      "selected provider for chain scan",
			"data": map[string]interface{}{
				"chain":        chainType.String(),
				"providerId":   provider.GetID(),
				"providerType": fmt.Sprintf("%v", provider.GetType()),
				"endpoint":     provider.GetEndpoint(),
			},
			"timestamp": time.Now().UnixMilli(),
		})
		// #endregion
	}

	// 使用更长的超时时间，避免网络请求超时
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 执行区块中心扫描
	err = am.scanNewBlocksBatch(ctx, provider, chainType, addressSet)
	if err != nil {
		logx.Errorf("Failed to scan blocks for chain %v: %v", chainType, err)
		if chainType == pb.BlockChainType_CHAIN_TYPE_ETHEREUM {
			// #region agent log
			writeDebugLog(map[string]interface{}{
				"sessionId":    "debug-session",
				"runId":        "pre-fix",
				"hypothesisId": "H2",
				"location":     "address_monitor.go:checkChainActivity:scanError",
				"message":      "scanNewBlocksBatch error",
				"data": map[string]interface{}{
					"chain": chainType.String(),
					"error": err.Error(),
				},
				"timestamp": time.Now().UnixMilli(),
			})
			// #endregion
		}
	} else {
		logx.Infof("✅ Completed block-based scanning for %d addresses on chain %v", len(addressSet), chainType)
	}
}
