package svc

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	tronAddress "github.com/fbsobreira/gotron-sdk/pkg/address"
	"github.com/zeromicro/go-zero/core/logx"

	"internalwallet/proto/pb"
	"internalwallet/services/consolidation/rpc/internal/chainutil"
	"internalwallet/services/consolidation/rpc/internal/config"
	"internalwallet/services/consolidation/rpc/internal/repository"
)

// RunRuntimeReloader polls DB changes and refreshes the runtime snapshot.
// It returns only when ctx is canceled.
func RunRuntimeReloader(ctx context.Context, svcCtx *ServiceContext, pollInterval time.Duration) {
	if ctx == nil {
		ctx = context.Background()
	}
	if svcCtx == nil || svcCtx.CurrencyChainRepo == nil {
		logx.WithContext(ctx).Error("runtime reloader not started: svcCtx/CurrencyChainRepo not configured")
		<-ctx.Done()
		return
	}
	if pollInterval <= 0 {
		pollInterval = 2 * time.Second
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	logx.WithContext(ctx).Infow("consolidation runtime reloader started",
		logx.Field("poll_interval_ms", pollInterval.Milliseconds()),
	)

	for {
		select {
		case <-ctx.Done():
			logx.WithContext(ctx).Info("consolidation runtime reloader stopped")
			return
		case <-ticker.C:
			cfg := svcCtx.Config.Consolidation
			if !cfg.Enabled {
				continue
			}
			fp, err := svcCtx.CurrencyChainRepo.GetConsolidationFingerprint(ctx, chainutil.SupportedChains())
			if err != nil {
				logx.WithContext(ctx).Errorf("runtime fingerprint query failed: %v", err)
				svcCtx.recordRuntimeReloadError(err)
				continue
			}

			cur := svcCtx.Runtime().Fingerprint
			if fingerprintsEqual(cur, fp) {
				continue
			}

			start := time.Now()
			if err := svcCtx.reloadRuntimeFromDB(ctx, fp); err != nil {
				logx.WithContext(ctx).Errorf("runtime reload failed: %v", err)
				continue
			}
			logx.WithContext(ctx).Infow("runtime reload succeeded",
				logx.Field("duration_ms", time.Since(start).Milliseconds()),
				logx.Field("enabled_count", fp.EnabledCount),
				logx.Field("max_updated_at", fp.MaxUpdatedAt),
				logx.Field("checksum", fp.Checksum),
			)
		}
	}
}

func (s *ServiceContext) ReloadRuntimeFromDB(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if s == nil || s.CurrencyChainRepo == nil {
		return fmt.Errorf("CurrencyChainRepo not configured")
	}
	fp, err := s.CurrencyChainRepo.GetConsolidationFingerprint(ctx, chainutil.SupportedChains())
	if err != nil {
		s.recordRuntimeReloadError(err)
		return err
	}
	return s.reloadRuntimeFromDB(ctx, fp)
}

func (s *ServiceContext) reloadRuntimeFromDB(ctx context.Context, fp repository.ConsolidationFingerprint) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if s == nil {
		return fmt.Errorf("svcCtx is nil")
	}

	cfg := s.Config.Consolidation
	if !cfg.Enabled {
		return nil
	}

	s.runtimeMu.Lock()
	defer s.runtimeMu.Unlock()

	prev := s.Runtime()

	next, err := buildRuntimeConfigFromDB(ctx, s, fp)
	if err != nil {
		s.recordRuntimeReloadError(err)
		return err
	}

	// Validate token decimals/symbol via chain nodes only if token configs changed.
	if cfg.ValidateTokenInfo && tokenConfigsChanged(prev.TokenConfigs, next.TokenConfigs) {
		if s.Chain == nil {
			err := fmt.Errorf("ValidateTokenInfo enabled but chain nodes are not configured")
			s.recordRuntimeReloadError(err)
			return err
		}
		if err := validateTokenInfo(ctx, s, next); err != nil {
			s.recordRuntimeReloadError(err)
			return err
		}
	}

	now := time.Now().Local()
	next.LastReloadAt = now
	next.LastReloadOKAt = now
	next.LastReloadError = ""
	s.setRuntime(next)
	return nil
}

func (s *ServiceContext) recordRuntimeReloadError(err error) {
	if s == nil || err == nil {
		return
	}
	prev := s.Runtime()
	next := *prev // shallow copy; maps are immutable and can be shared safely
	next.LastReloadAt = time.Now().Local()
	next.LastReloadError = err.Error()
	s.setRuntime(&next)
}

func fingerprintsEqual(a, b repository.ConsolidationFingerprint) bool {
	if a.EnabledCount != b.EnabledCount {
		return false
	}
	if a.Checksum != b.Checksum {
		return false
	}
	if (a.MaxUpdatedAt == nil) != (b.MaxUpdatedAt == nil) {
		return false
	}
	if a.MaxUpdatedAt == nil && b.MaxUpdatedAt == nil {
		return true
	}
	return a.MaxUpdatedAt.Equal(*b.MaxUpdatedAt)
}

func tokenConfigsChanged(a, b map[string]map[string]config.TokenConfig) bool {
	if len(a) != len(b) {
		return true
	}
	for chain, am := range a {
		bm, ok := b[chain]
		if !ok {
			return true
		}
		if len(am) != len(bm) {
			return true
		}
		for asset, ac := range am {
			bc, ok := bm[asset]
			if !ok {
				return true
			}
			if strings.TrimSpace(ac.Contract) != strings.TrimSpace(bc.Contract) || ac.Decimals != bc.Decimals {
				return true
			}
		}
	}
	return false
}

func buildRuntimeConfigFromDB(ctx context.Context, svcCtx *ServiceContext, fp repository.ConsolidationFingerprint) (*RuntimeConfig, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if svcCtx == nil || svcCtx.CurrencyChainRepo == nil {
		return nil, fmt.Errorf("CurrencyChainRepo not configured")
	}

	enabledAssetsByChain, tokenConfigs, err := loadEnabledAssetsAndTokenConfigsFromDB(ctx, svcCtx)
	if err != nil {
		return nil, err
	}

	raw := svcCtx.Config.Consolidation
	effectiveThresholds := map[string]map[string]string{}

	// Project raw thresholds onto DB-enabled asset set.
	for chain, assets := range enabledAssetsByChain {
		for asset := range assets {
			threshold := strings.TrimSpace(raw.MinBalanceThresholds[chain][asset])
			if threshold == "" {
				return nil, fmt.Errorf("missing MinBalanceThresholds[%s][%s] for consolidation-enabled asset", chain, asset)
			}
			if _, ok := new(big.Int).SetString(threshold, 10); !ok {
				return nil, fmt.Errorf("invalid MinBalanceThresholds[%s][%s]=%q (must be base-10 integer string)", chain, asset, threshold)
			}
			if effectiveThresholds[chain] == nil {
				effectiveThresholds[chain] = map[string]string{}
			}
			effectiveThresholds[chain][asset] = threshold
		}
	}

	// Allow idle mode: no enabled assets in DB.
	if len(effectiveThresholds) == 0 {
		return &RuntimeConfig{
			EnabledAssetsByChain:          enabledAssetsByChain,
			TokenConfigs:                  tokenConfigs,
			EffectiveMinBalanceThresholds: effectiveThresholds,
			Fingerprint:                   fp,
		}, nil
	}

	// Validate per-enabled-chain required configs.
	for chain := range effectiveThresholds {
		native := strings.ToUpper(strings.TrimSpace(chainutil.NativeSymbolForChain(chain)))
		if native == "" {
			return nil, fmt.Errorf("unsupported chain in enabled set: %s", chain)
		}

		minStr := strings.TrimSpace(raw.MinConsolidationAmount[chain][native])
		if minStr == "" {
			return nil, fmt.Errorf("missing MinConsolidationAmount[%s][%s]", chain, native)
		}
		if _, ok := new(big.Int).SetString(minStr, 10); !ok {
			return nil, fmt.Errorf("invalid MinConsolidationAmount[%s][%s]=%q (must be base-10 integer string)", chain, native, minStr)
		}

		maxStr := strings.TrimSpace(raw.Risk.MaxNativeSpendPerTask[chain])
		if maxStr == "" {
			return nil, fmt.Errorf("missing Risk.MaxNativeSpendPerTask[%s]", chain)
		}
		if _, ok := new(big.Int).SetString(maxStr, 10); !ok {
			return nil, fmt.Errorf("invalid Risk.MaxNativeSpendPerTask[%s]=%q (must be base-10 integer string)", chain, maxStr)
		}

		m, ok := raw.GasSafetyMultipliers[chain]
		if !ok {
			return nil, fmt.Errorf("missing GasSafetyMultipliers[%s]", chain)
		}
		if m <= 1.0 {
			return nil, fmt.Errorf("invalid GasSafetyMultipliers[%s]=%v (must be > 1.0)", chain, m)
		}
	}

	// Destination address validation (only for target_addresses mode).
	if strings.ToLower(strings.TrimSpace(raw.ToAddressMode)) == "target_addresses" {
		for chain, assets := range effectiveThresholds {
			for asset := range assets {
				addr := strings.TrimSpace(raw.TargetAddresses[chain][asset])
				if addr == "" {
					return nil, fmt.Errorf("missing TargetAddresses[%s][%s] (required when ToAddressMode=target_addresses)", chain, asset)
				}
				if err := validateChainAddress(chain, addr); err != nil {
					return nil, fmt.Errorf("invalid TargetAddresses[%s][%s]=%q: %w", chain, asset, addr, err)
				}
			}
		}
	}

	// TRON token dependency checks (only for enabled tokens).
	tronHasToken := false
	if assets, ok := effectiveThresholds["TRON"]; ok {
		for asset := range assets {
			if strings.ToUpper(strings.TrimSpace(asset)) != "TRX" {
				tronHasToken = true
				break
			}
		}
	}
	if tronHasToken && strings.ToLower(strings.TrimSpace(raw.TronTrc20FeeMode)) == "energy_rental" && !raw.EnergyRental.Enabled {
		return nil, fmt.Errorf("TronTrc20FeeMode=energy_rental requires EnergyRental.Enabled=true when TRON tokens are enabled")
	}
	if raw.TopUp.Enabled && tronHasToken {
		bwStr := strings.TrimSpace(raw.TopUp.TronBandwidthTopUpSun)
		bw, ok := new(big.Int).SetString(bwStr, 10)
		if !ok || bw.Sign() <= 0 {
			return nil, fmt.Errorf("TopUp.TronBandwidthTopUpSun must be a positive integer (SUN) when TopUp.Enabled=true and TRON tokens are enabled: %q", raw.TopUp.TronBandwidthTopUpSun)
		}
	}

	return &RuntimeConfig{
		EnabledAssetsByChain:          enabledAssetsByChain,
		TokenConfigs:                  tokenConfigs,
		EffectiveMinBalanceThresholds: effectiveThresholds,
		Fingerprint:                   fp,
	}, nil
}

func loadEnabledAssetsAndTokenConfigsFromDB(ctx context.Context, svcCtx *ServiceContext) (map[string]map[string]struct{}, map[string]map[string]config.TokenConfig, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if svcCtx == nil || svcCtx.CurrencyChainRepo == nil {
		return nil, nil, fmt.Errorf("CurrencyChainRepo not configured")
	}

	enabledAssetsByChain := map[string]map[string]struct{}{}
	tokenConfigs := map[string]map[string]config.TokenConfig{}

	rows, err := svcCtx.CurrencyChainRepo.ListEnabledForConsolidation(ctx, chainutil.SupportedChains())
	if err != nil {
		return nil, nil, fmt.Errorf("list currency_chain_settings failed: %w", err)
	}
	if len(rows) == 0 {
		return enabledAssetsByChain, tokenConfigs, nil
	}

	for _, row := range rows {
		chain := chainutil.NormalizeChain(row.ChainCode)
		asset := chainutil.NormalizeAssetSymbol(row.AssetCode)
		if chain == "" || asset == "" {
			continue
		}

		native := chainutil.NativeSymbolForChain(chain)
		if native == "" {
			// The repo query already limits to SupportedChains(), so treat as defensive.
			continue
		}

		if enabledAssetsByChain[chain] == nil {
			enabledAssetsByChain[chain] = map[string]struct{}{}
		}
		enabledAssetsByChain[chain][asset] = struct{}{}

		// Tokens only: require contract + token_decimals.
		if asset == native {
			continue
		}

		contract := ""
		if row.ContractAddress != nil {
			contract = strings.TrimSpace(*row.ContractAddress)
		}
		if contract == "" {
			return nil, nil, fmt.Errorf("missing contract_address for %s/%s in currency_chain_settings", chain, asset)
		}
		if err := validateTokenContract(chain, contract); err != nil {
			return nil, nil, fmt.Errorf("invalid contract_address for %s/%s (%s): %w", chain, asset, contract, err)
		}
		if row.TokenDecimals == nil {
			return nil, nil, fmt.Errorf("missing token_decimals for %s/%s in currency_chain_settings", chain, asset)
		}
		if *row.TokenDecimals < 0 || *row.TokenDecimals > 30 {
			return nil, nil, fmt.Errorf("invalid token_decimals for %s/%s: %d (expected 0..30)", chain, asset, *row.TokenDecimals)
		}

		if tokenConfigs[chain] == nil {
			tokenConfigs[chain] = map[string]config.TokenConfig{}
		}
		if _, exists := tokenConfigs[chain][asset]; exists {
			return nil, nil, fmt.Errorf("duplicate token config for %s/%s in currency_chain_settings", chain, asset)
		}
		tokenConfigs[chain][asset] = config.TokenConfig{
			Contract: chainutil.NormalizeTokenContract(chain, contract),
			Decimals: uint32(*row.TokenDecimals),
		}
	}

	return enabledAssetsByChain, tokenConfigs, nil
}

func validateTokenInfo(ctx context.Context, svcCtx *ServiceContext, rt *RuntimeConfig) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if svcCtx == nil || svcCtx.Chain == nil || rt == nil {
		return fmt.Errorf("svcCtx/runtime not configured")
	}

	for chain, assets := range rt.EffectiveMinBalanceThresholds {
		chain = strings.ToUpper(strings.TrimSpace(chain))
		native := chainutil.NativeSymbolForChain(chain)
		chainEnum, err := chainutil.ChainStringToEnum(chain)
		if err != nil {
			return err
		}
		for asset, threshold := range assets {
			asset = strings.ToUpper(strings.TrimSpace(asset))
			threshold = strings.TrimSpace(threshold)
			if asset == "" || threshold == "" || asset == native {
				continue
			}
			tc, ok := rt.TokenConfigs[chain][asset]
			if !ok || strings.TrimSpace(tc.Contract) == "" {
				return fmt.Errorf("missing token config from currency_chain_settings for %s/%s", chain, asset)
			}
			resp, err := svcCtx.Chain.GetTokenInfo(ctx, &pb.GetTokenInfoReq{
				Chain:         chainEnum,
				TokenContract: strings.TrimSpace(tc.Contract),
			})
			if err != nil {
				return fmt.Errorf("GetTokenInfo failed for %s/%s (%s): %w", chain, asset, tc.Contract, err)
			}
			if resp == nil || !resp.Success || resp.TokenInfo == nil {
				msg := ""
				if resp != nil {
					msg = resp.Message
				}
				return fmt.Errorf("GetTokenInfo failed for %s/%s (%s): %s", chain, asset, tc.Contract, msg)
			}
			if resp.TokenInfo.Decimals != tc.Decimals {
				return fmt.Errorf("token decimals mismatch for %s/%s (%s): config=%d chain=%d",
					chain, asset, tc.Contract, tc.Decimals, resp.TokenInfo.Decimals)
			}
			if sym := strings.ToUpper(strings.TrimSpace(resp.TokenInfo.Symbol)); sym != "" && sym != asset {
				return fmt.Errorf("token symbol mismatch for %s/%s (%s): config_symbol=%s chain_symbol=%s",
					chain, asset, tc.Contract, asset, sym)
			}
		}
	}

	return nil
}

func validateTokenContract(chain string, contract string) error {
	contract = strings.TrimSpace(contract)
	if contract == "" {
		return fmt.Errorf("contract is empty")
	}
	switch strings.ToUpper(strings.TrimSpace(chain)) {
	case "ETH", "BSC":
		if !common.IsHexAddress(contract) {
			return fmt.Errorf("invalid EVM token contract")
		}
		return nil
	case "TRON":
		if _, err := tronAddress.Base58ToAddress(contract); err != nil {
			return fmt.Errorf("invalid TRON token contract: %v", err)
		}
		return nil
	default:
		return fmt.Errorf("unsupported chain: %s", chain)
	}
}

func validateChainAddress(chain string, addr string) error {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return fmt.Errorf("address is empty")
	}
	switch strings.ToUpper(strings.TrimSpace(chain)) {
	case "ETH", "BSC":
		if !common.IsHexAddress(addr) {
			return fmt.Errorf("invalid EVM address")
		}
		return nil
	case "TRON":
		if _, err := tronAddress.Base58ToAddress(addr); err != nil {
			return fmt.Errorf("invalid TRON base58 address: %v", err)
		}
		return nil
	default:
		return fmt.Errorf("unsupported chain: %s", chain)
	}
}
