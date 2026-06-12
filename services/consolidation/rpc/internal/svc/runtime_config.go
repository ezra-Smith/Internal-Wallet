package svc

import (
	"time"

	"internalwallet/services/consolidation/rpc/internal/config"
	"internalwallet/services/consolidation/rpc/internal/repository"
)

// RuntimeConfig is an immutable snapshot of consolidation settings derived from:
// - DB table currency_chain_settings (enabled asset list + token metadata)
// - raw YAML config (thresholds and other static configs)
//
// It must be replaced atomically as a whole. Never mutate maps stored in this struct after publication.
type RuntimeConfig struct {
	EnabledAssetsByChain          map[string]map[string]struct{}           // chain -> set(asset_symbol)
	TokenConfigs                  map[string]map[string]config.TokenConfig // chain -> asset_symbol -> token config
	EffectiveMinBalanceThresholds map[string]map[string]string             // chain -> asset_symbol -> threshold (smallest unit, base-10)

	Fingerprint repository.ConsolidationFingerprint

	LastReloadAt    time.Time
	LastReloadOKAt  time.Time
	LastReloadError string
}

func NewEmptyRuntimeConfig() *RuntimeConfig {
	return &RuntimeConfig{
		EnabledAssetsByChain:          map[string]map[string]struct{}{},
		TokenConfigs:                  map[string]map[string]config.TokenConfig{},
		EffectiveMinBalanceThresholds: map[string]map[string]string{},
	}
}

func (s *ServiceContext) Runtime() *RuntimeConfig {
	if s == nil {
		return NewEmptyRuntimeConfig()
	}
	v := any(nil)
	func() {
		defer func() {
			if r := recover(); r != nil {
				// atomic.Value panics on Load() before the first Store().
				// Be defensive for tests or manual ServiceContext construction.
				s.runtimeCfg.Store(NewEmptyRuntimeConfig())
				v = s.runtimeCfg.Load()
			}
		}()
		v = s.runtimeCfg.Load()
	}()
	if cfg, ok := v.(*RuntimeConfig); ok && cfg != nil {
		return cfg
	}
	return NewEmptyRuntimeConfig()
}

func (s *ServiceContext) setRuntime(cfg *RuntimeConfig) {
	if s == nil || cfg == nil {
		return
	}
	s.runtimeCfg.Store(cfg)
}
