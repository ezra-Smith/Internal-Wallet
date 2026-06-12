package svc

import (
	"internalwallet/proto/pb"
	"internalwallet/services/chainsync/rpc/internal/config"
)

func (am *AddressMonitor) getChainScanConfig(chainType pb.BlockChainType) config.ChainScanConfig {
	if am.config == nil {
		return config.ChainScanConfig{
			BatchSize:       100,
			CheckInterval:   5,
			ProgressLogStep: 100,
		}
	}

	switch chainType {
	case pb.BlockChainType_CHAIN_TYPE_ETHEREUM:
		cfg := am.config.Chains.Ethereum.ScanConfig
		if cfg.CheckInterval == 0 {
			cfg.CheckInterval = 5
		}
		if cfg.BatchSize == 0 {
			cfg.BatchSize = 100
		}
		return cfg
	case pb.BlockChainType_CHAIN_TYPE_BSC:
		cfg := am.config.Chains.BSC.ScanConfig
		if cfg.CheckInterval == 0 {
			cfg.CheckInterval = 5
		}
		if cfg.BatchSize == 0 {
			cfg.BatchSize = 100
		}
		return cfg
	case pb.BlockChainType_CHAIN_TYPE_TRON:
		cfg := am.config.Chains.Tron.ScanConfig
		if cfg.CheckInterval == 0 {
			cfg.CheckInterval = 5
		}
		if cfg.BatchSize == 0 {
			cfg.BatchSize = 100
		}
		return cfg
	default:
		return config.ChainScanConfig{
			BatchSize:       100,
			CheckInterval:   5,
			ProgressLogStep: 100,
		}
	}
}

