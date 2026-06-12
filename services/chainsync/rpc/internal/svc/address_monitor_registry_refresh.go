package svc

import (
	"context"
	"time"

	"internalwallet/proto/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

func (am *AddressMonitor) addressRegistryRefreshLoop() {
	defer am.wg.Done()

	refreshInterval := 60 * time.Second
	if am.config != nil && am.config.Monitoring.AddressRegistry.RefreshIntervalSeconds > 0 {
		refreshInterval = time.Duration(am.config.Monitoring.AddressRegistry.RefreshIntervalSeconds) * time.Second
	}

	ticker := time.NewTicker(refreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-am.stopCh:
			logx.Info("Address registry refresh loop stopped")
			return
		case <-ticker.C:
			am.refreshAddressRegistry("periodic")
		}
	}
}

func (am *AddressMonitor) refreshAddressRegistry(reason string) {
	if am.registry == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	prev := am.registry.Snapshot()
	beforeCounts := am.getSnapshotCounts(prev)

	res, err := am.registry.FullRefresh(ctx)
	if err != nil {
		logx.Errorf("❌ Address registry refresh failed (%s): %v", reason, err)
		return
	}

	next := am.registry.Snapshot()
	afterCounts := am.getSnapshotCounts(next)

	logx.Infof("🔄 Address registry refreshed (%s): added=%d removed=%d | ETH=%d BSC=%d TRON=%d | sources(deposit=%d company=%d vault=%d web3=%d manual=%d)",
		reason,
		res.added, res.removed,
		afterCounts.ethTotal, afterCounts.bscTotal, afterCounts.tronTotal,
		afterCounts.depositTotal, afterCounts.companyTotal, afterCounts.vaultTotal, afterCounts.web3Total, afterCounts.manualTotal,
	)

	{
		targetAddr := "0x81962f7083b23b2b3d2f3cd1ee7ac038591e7a90"
		normalized := normalizeMonitoredAddress(pb.BlockChainType_CHAIN_TYPE_ETHEREUM, targetAddr)
		monitored := am.registry != nil && am.registry.IsMonitored(pb.BlockChainType_CHAIN_TYPE_ETHEREUM, normalized)
		// #region agent log
		writeDebugLog(map[string]interface{}{
			"sessionId":    "debug-session",
			"runId":        "pre-fix",
			"hypothesisId": "H3",
			"location":     "address_monitor.go:refreshAddressRegistry",
			"message":      "registry contains target address",
			"data": map[string]interface{}{
				"chain":         pb.BlockChainType_CHAIN_TYPE_ETHEREUM.String(),
				"address":       normalized,
				"monitored":     monitored,
				"refreshReason": reason,
				"ethTotal":      afterCounts.ethTotal,
				"depositTotal":  afterCounts.depositTotal,
			},
			"timestamp": time.Now().UnixMilli(),
		})
		// #endregion
	}

	// Optional debug for drift
	if res.added != 0 || res.removed != 0 {
		logx.Debugf("Address registry diff (%s): before ETH=%d BSC=%d TRON=%d, after ETH=%d BSC=%d TRON=%d",
			reason,
			beforeCounts.ethTotal, beforeCounts.bscTotal, beforeCounts.tronTotal,
			afterCounts.ethTotal, afterCounts.bscTotal, afterCounts.tronTotal,
		)
	}
}

type snapshotCounts struct {
	ethTotal  int
	bscTotal  int
	tronTotal int

	depositTotal int
	companyTotal int
	vaultTotal   int
	web3Total    int
	manualTotal  int
}

func (am *AddressMonitor) getSnapshotCounts(s *addressRegistrySnapshot) snapshotCounts {
	if s == nil {
		return snapshotCounts{}
	}
	var c snapshotCounts

	if cs := s.chains[pb.BlockChainType_CHAIN_TYPE_ETHEREUM]; cs != nil {
		c.ethTotal = cs.total
		c.depositTotal += cs.depositCount
		c.companyTotal += cs.companyCount
		c.vaultTotal += cs.vaultCount
		c.web3Total += cs.web3Count
		c.manualTotal += cs.manualCount
	}
	if cs := s.chains[pb.BlockChainType_CHAIN_TYPE_BSC]; cs != nil {
		c.bscTotal = cs.total
		c.depositTotal += cs.depositCount
		c.companyTotal += cs.companyCount
		c.vaultTotal += cs.vaultCount
		c.web3Total += cs.web3Count
		c.manualTotal += cs.manualCount
	}
	if cs := s.chains[pb.BlockChainType_CHAIN_TYPE_TRON]; cs != nil {
		c.tronTotal = cs.total
		c.depositTotal += cs.depositCount
		c.companyTotal += cs.companyCount
		c.vaultTotal += cs.vaultCount
		c.web3Total += cs.web3Count
		c.manualTotal += cs.manualCount
	}
	return c
}
