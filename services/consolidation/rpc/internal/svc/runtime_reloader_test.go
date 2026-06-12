package svc

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"internalwallet/services/consolidation/rpc/internal/config"
	"internalwallet/services/consolidation/rpc/internal/repository"
)

type stubCurrencyChainRepo struct {
	mu sync.Mutex

	rows        []repository.CurrencyChainSettingsRow
	listErr     error
	fingerprint repository.ConsolidationFingerprint
	fpErr       error

	listCalled chan struct{}
}

func (s *stubCurrencyChainRepo) ListEnabledForConsolidation(ctx context.Context, chainCodes []string) ([]repository.CurrencyChainSettingsRow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listCalled != nil {
		select {
		case <-s.listCalled:
		default:
			close(s.listCalled)
		}
	}
	if s.listErr != nil {
		return nil, s.listErr
	}
	out := make([]repository.CurrencyChainSettingsRow, len(s.rows))
	copy(out, s.rows)
	return out, nil
}

func (s *stubCurrencyChainRepo) GetConsolidationFingerprint(ctx context.Context, chainCodes []string) (repository.ConsolidationFingerprint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fpErr != nil {
		return repository.ConsolidationFingerprint{}, s.fpErr
	}
	return s.fingerprint, nil
}

func TestLoadEnabledAssetsAndTokenConfigsFromDB_NoRows_OK(t *testing.T) {
	svcCtx := &ServiceContext{
		Config: config.Config{
			Consolidation: config.ConsolidationConfig{Enabled: true},
		},
		CurrencyChainRepo: &stubCurrencyChainRepo{rows: nil},
	}
	svcCtx.runtimeCfg.Store(NewEmptyRuntimeConfig())

	enabled, tokenCfg, err := loadEnabledAssetsAndTokenConfigsFromDB(context.Background(), svcCtx)
	require.NoError(t, err)
	require.Empty(t, enabled)
	require.Empty(t, tokenCfg)
}

func TestLoadEnabledAssetsAndTokenConfigsFromDB_MissingTokenDecimals_FailFast(t *testing.T) {
	rows := []repository.CurrencyChainSettingsRow{
		{ChainCode: "TRON", AssetCode: "TRX"},
		{ChainCode: "TRON", AssetCode: "USDT", ContractAddress: ptr("TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t"), TokenDecimals: nil},
	}
	svcCtx := &ServiceContext{
		Config: config.Config{
			Consolidation: config.ConsolidationConfig{Enabled: true},
		},
		CurrencyChainRepo: &stubCurrencyChainRepo{rows: rows},
	}
	svcCtx.runtimeCfg.Store(NewEmptyRuntimeConfig())

	_, _, err := loadEnabledAssetsAndTokenConfigsFromDB(context.Background(), svcCtx)
	require.Error(t, err)
}

func TestLoadEnabledAssetsAndTokenConfigsFromDB_InvalidDecimals_FailFast(t *testing.T) {
	dec := int32(31)
	rows := []repository.CurrencyChainSettingsRow{
		{ChainCode: "TRON", AssetCode: "TRX"},
		{ChainCode: "TRON", AssetCode: "USDT", ContractAddress: ptr("TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t"), TokenDecimals: &dec},
	}
	svcCtx := &ServiceContext{
		Config: config.Config{
			Consolidation: config.ConsolidationConfig{Enabled: true},
		},
		CurrencyChainRepo: &stubCurrencyChainRepo{rows: rows},
	}
	svcCtx.runtimeCfg.Store(NewEmptyRuntimeConfig())

	_, _, err := loadEnabledAssetsAndTokenConfigsFromDB(context.Background(), svcCtx)
	require.Error(t, err)
}

func TestLoadEnabledAssetsAndTokenConfigsFromDB_InvalidContract_FailFast(t *testing.T) {
	dec := int32(6)
	rows := []repository.CurrencyChainSettingsRow{
		{ChainCode: "ETH", AssetCode: "ETH"},
		{ChainCode: "ETH", AssetCode: "USDT", ContractAddress: ptr("0x123"), TokenDecimals: &dec},
	}
	svcCtx := &ServiceContext{
		Config: config.Config{
			Consolidation: config.ConsolidationConfig{Enabled: true},
		},
		CurrencyChainRepo: &stubCurrencyChainRepo{rows: rows},
	}
	svcCtx.runtimeCfg.Store(NewEmptyRuntimeConfig())

	_, _, err := loadEnabledAssetsAndTokenConfigsFromDB(context.Background(), svcCtx)
	require.Error(t, err)
}

func TestLoadEnabledAssetsAndTokenConfigsFromDB_Success(t *testing.T) {
	dec6 := int32(6)
	rows := []repository.CurrencyChainSettingsRow{
		{ChainCode: "tron", AssetCode: "trx"},
		{ChainCode: "TRON", AssetCode: "USDT", ContractAddress: ptr("TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t"), TokenDecimals: &dec6},
		{ChainCode: "eth", AssetCode: "eth"},
		{ChainCode: "ETH", AssetCode: "USDT", ContractAddress: ptr("0x000000000000000000000000000000000000dEaD"), TokenDecimals: &dec6},
	}
	svcCtx := &ServiceContext{
		Config: config.Config{
			Consolidation: config.ConsolidationConfig{Enabled: true},
		},
		CurrencyChainRepo: &stubCurrencyChainRepo{rows: rows},
	}
	svcCtx.runtimeCfg.Store(NewEmptyRuntimeConfig())

	enabled, tokenCfg, err := loadEnabledAssetsAndTokenConfigsFromDB(context.Background(), svcCtx)
	require.NoError(t, err)

	require.Contains(t, enabled["TRON"], "TRX")
	require.Contains(t, enabled["TRON"], "USDT")
	require.Contains(t, enabled["ETH"], "ETH")
	require.Contains(t, enabled["ETH"], "USDT")

	// Token configs only.
	tcTron, ok := tokenCfg["TRON"]["USDT"]
	require.True(t, ok)
	require.Equal(t, uint32(6), tcTron.Decimals)
	require.Equal(t, "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t", tcTron.Contract)

	tcEth, ok := tokenCfg["ETH"]["USDT"]
	require.True(t, ok)
	require.Equal(t, uint32(6), tcEth.Decimals)
	require.Equal(t, "0x000000000000000000000000000000000000dead", tcEth.Contract)
	_, hasNative := tokenCfg["ETH"]["ETH"]
	require.False(t, hasNative)
}

func TestReloadRuntimeFromDB_MissingThreshold_KeepOldConfig(t *testing.T) {
	now := time.Now().Local()

	repo := &stubCurrencyChainRepo{
		fingerprint: repository.ConsolidationFingerprint{EnabledCount: 1, MaxUpdatedAt: &now},
		rows: []repository.CurrencyChainSettingsRow{
			{ChainCode: "TRON", AssetCode: "TRX"},
		},
	}
	svcCtx := &ServiceContext{
		Config: config.Config{
			Consolidation: config.ConsolidationConfig{
				Enabled:           true,
				ValidateTokenInfo: false,
				ToAddressMode:     "system_hot_wallet",
				MinBalanceThresholds: map[string]map[string]string{
					"TRON": {"TRX": "100"},
				},
				MinConsolidationAmount: map[string]map[string]string{
					"TRON": {"TRX": "1"},
				},
				Risk: config.RiskConfig{
					MaxNativeSpendPerTask: map[string]string{"TRON": "1"},
				},
				GasSafetyMultipliers: map[string]float64{"TRON": 2.0},
			},
		},
		CurrencyChainRepo: repo,
	}
	svcCtx.runtimeCfg.Store(NewEmptyRuntimeConfig())

	require.NoError(t, svcCtx.ReloadRuntimeFromDB(context.Background()))
	require.Equal(t, "100", svcCtx.Runtime().EffectiveMinBalanceThresholds["TRON"]["TRX"])

	// Enable USDT in DB but missing threshold in raw config => reload must fail and keep old snapshot.
	dec6 := int32(6)
	repo.mu.Lock()
	repo.fingerprint = repository.ConsolidationFingerprint{EnabledCount: 2, MaxUpdatedAt: &now}
	repo.rows = []repository.CurrencyChainSettingsRow{
		{ChainCode: "TRON", AssetCode: "TRX"},
		{ChainCode: "TRON", AssetCode: "USDT", ContractAddress: ptr("TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t"), TokenDecimals: &dec6},
	}
	repo.mu.Unlock()

	err := svcCtx.ReloadRuntimeFromDB(context.Background())
	require.Error(t, err)
	require.Equal(t, "100", svcCtx.Runtime().EffectiveMinBalanceThresholds["TRON"]["TRX"])
	require.Empty(t, svcCtx.Runtime().EffectiveMinBalanceThresholds["TRON"]["USDT"])
	require.NotEmpty(t, svcCtx.Runtime().LastReloadError)
}

func TestReloadRuntimeFromDB_HotEnableNewAsset_OK(t *testing.T) {
	now := time.Now().Local()

	repo := &stubCurrencyChainRepo{
		fingerprint: repository.ConsolidationFingerprint{EnabledCount: 1, MaxUpdatedAt: &now},
		rows: []repository.CurrencyChainSettingsRow{
			{ChainCode: "TRON", AssetCode: "TRX"},
		},
	}
	svcCtx := &ServiceContext{
		Config: config.Config{
			Consolidation: config.ConsolidationConfig{
				Enabled:           true,
				ValidateTokenInfo: false,
				ToAddressMode:     "system_hot_wallet",
				MinBalanceThresholds: map[string]map[string]string{
					"TRON": {"TRX": "100", "USDT": "200"},
				},
				MinConsolidationAmount: map[string]map[string]string{
					"TRON": {"TRX": "1"},
				},
				Risk: config.RiskConfig{
					MaxNativeSpendPerTask: map[string]string{"TRON": "1"},
				},
				GasSafetyMultipliers: map[string]float64{"TRON": 2.0},
				TronTrc20FeeMode:     "energy_rental",
				EnergyRental: config.EnergyRentalConfig{
					Enabled: true,
				},
				TopUp: config.TopUpConfig{
					Enabled:                 true,
					TronBandwidthTopUpSun:   "1",
					MaxTopUpAttemptsPerTask: 1,
					TopUpInterval:           1,
					VerifyInterval:          1,
					TaskRetryDelaySeconds:   1,
					HotWalletAddressType:    "hot_primary",
				},
			},
		},
		CurrencyChainRepo: repo,
	}
	svcCtx.runtimeCfg.Store(NewEmptyRuntimeConfig())

	require.NoError(t, svcCtx.ReloadRuntimeFromDB(context.Background()))
	require.Equal(t, "100", svcCtx.Runtime().EffectiveMinBalanceThresholds["TRON"]["TRX"])

	// Enable USDT in DB => reload should succeed and expose token config.
	dec6 := int32(6)
	repo.mu.Lock()
	repo.fingerprint = repository.ConsolidationFingerprint{EnabledCount: 2, MaxUpdatedAt: &now}
	repo.rows = []repository.CurrencyChainSettingsRow{
		{ChainCode: "TRON", AssetCode: "TRX"},
		{ChainCode: "TRON", AssetCode: "USDT", ContractAddress: ptr("TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t"), TokenDecimals: &dec6},
	}
	repo.mu.Unlock()

	require.NoError(t, svcCtx.ReloadRuntimeFromDB(context.Background()))
	require.Equal(t, "200", svcCtx.Runtime().EffectiveMinBalanceThresholds["TRON"]["USDT"])
	require.Equal(t, uint32(6), svcCtx.Runtime().TokenConfigs["TRON"]["USDT"].Decimals)
}

func TestRunRuntimeReloader_FingerprintUnchanged_SkipReload(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	repo := &stubCurrencyChainRepo{
		fingerprint: repository.ConsolidationFingerprint{EnabledCount: 0, MaxUpdatedAt: nil},
		listCalled:  make(chan struct{}),
	}
	svcCtx := &ServiceContext{
		Config: config.Config{
			Consolidation: config.ConsolidationConfig{Enabled: true},
		},
		CurrencyChainRepo: repo,
	}
	svcCtx.runtimeCfg.Store(NewEmptyRuntimeConfig())

	go RunRuntimeReloader(ctx, svcCtx, 10*time.Millisecond)

	select {
	case <-repo.listCalled:
		t.Fatalf("expected no reload when fingerprint unchanged")
	case <-time.After(80 * time.Millisecond):
		// ok
	}
}

func TestRunRuntimeReloader_FingerprintChanged_TriggersReload(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	now := time.Now().Local()
	listCalled := make(chan struct{})
	repo := &stubCurrencyChainRepo{
		fingerprint: repository.ConsolidationFingerprint{EnabledCount: 0, MaxUpdatedAt: nil},
		listCalled:  listCalled,
	}
	svcCtx := &ServiceContext{
		Config: config.Config{
			Consolidation: config.ConsolidationConfig{
				Enabled:           true,
				ValidateTokenInfo: false,
				ToAddressMode:     "system_hot_wallet",
				MinBalanceThresholds: map[string]map[string]string{
					"TRON": {"TRX": "100"},
				},
				MinConsolidationAmount: map[string]map[string]string{
					"TRON": {"TRX": "1"},
				},
				Risk: config.RiskConfig{
					MaxNativeSpendPerTask: map[string]string{"TRON": "1"},
				},
				GasSafetyMultipliers: map[string]float64{"TRON": 2.0},
			},
		},
		CurrencyChainRepo: repo,
	}
	svcCtx.runtimeCfg.Store(NewEmptyRuntimeConfig())

	go RunRuntimeReloader(ctx, svcCtx, 10*time.Millisecond)

	// Flip fingerprint and provide enabled rows.
	repo.mu.Lock()
	repo.fingerprint = repository.ConsolidationFingerprint{EnabledCount: 1, MaxUpdatedAt: &now}
	repo.rows = []repository.CurrencyChainSettingsRow{
		{ChainCode: "TRON", AssetCode: "TRX"},
	}
	repo.mu.Unlock()

	select {
	case <-listCalled:
		// ok
	case <-time.After(2 * time.Second):
		t.Fatalf("expected reload to be triggered")
	}

	deadline := time.Now().Add(800 * time.Millisecond)
	for time.Now().Before(deadline) {
		if svcCtx.Runtime().EffectiveMinBalanceThresholds["TRON"]["TRX"] == "100" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	require.Equal(t, "100", svcCtx.Runtime().EffectiveMinBalanceThresholds["TRON"]["TRX"])
}

func TestRunRuntimeReloader_SameTimestampButChecksumChanged_TriggersReload(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	now := time.Now().Local()
	listCalled := make(chan struct{})
	repo := &stubCurrencyChainRepo{
		fingerprint: repository.ConsolidationFingerprint{EnabledCount: 1, MaxUpdatedAt: &now, Checksum: 1},
		rows: []repository.CurrencyChainSettingsRow{
			{ChainCode: "TRON", AssetCode: "TRX"},
		},
		listCalled: listCalled,
	}
	svcCtx := &ServiceContext{
		Config: config.Config{
			Consolidation: config.ConsolidationConfig{
				Enabled:           true,
				ValidateTokenInfo: false,
				ToAddressMode:     "system_hot_wallet",
				MinBalanceThresholds: map[string]map[string]string{
					"TRON": {"TRX": "100"},
				},
				MinConsolidationAmount: map[string]map[string]string{
					"TRON": {"TRX": "1"},
				},
				Risk: config.RiskConfig{
					MaxNativeSpendPerTask: map[string]string{"TRON": "1"},
				},
				GasSafetyMultipliers: map[string]float64{"TRON": 2.0},
			},
		},
		CurrencyChainRepo: repo,
	}
	rt := NewEmptyRuntimeConfig()
	rt.Fingerprint = repo.fingerprint
	svcCtx.runtimeCfg.Store(rt)

	go RunRuntimeReloader(ctx, svcCtx, 10*time.Millisecond)

	select {
	case <-listCalled:
		t.Fatalf("expected no reload when fingerprint unchanged")
	case <-time.After(80 * time.Millisecond):
		// ok
	}

	repo.mu.Lock()
	repo.fingerprint = repository.ConsolidationFingerprint{EnabledCount: 1, MaxUpdatedAt: &now, Checksum: 2}
	repo.mu.Unlock()

	select {
	case <-listCalled:
		// ok
	case <-time.After(2 * time.Second):
		t.Fatalf("expected reload to be triggered")
	}
}

func ptr(s string) *string { return &s }
