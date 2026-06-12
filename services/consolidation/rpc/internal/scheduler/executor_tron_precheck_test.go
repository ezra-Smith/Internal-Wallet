package scheduler

import (
	"context"
	"testing"

	"internalwallet/proto/pb"
	"internalwallet/services/consolidation/rpc/internal/config"
	"internalwallet/services/consolidation/rpc/internal/models"
	"internalwallet/services/consolidation/rpc/internal/svc"

	"github.com/stretchr/testify/require"
)

type stubChainClient struct {
	estimateTronFeeFn      func(ctx context.Context, in *pb.EstimateTronFeeReq) (*pb.EstimateTronFeeResp, error)
	getTronResourcesFn     func(ctx context.Context, in *pb.GetTronAccountResourcesReq) (*pb.GetTronAccountResourcesResp, error)
	getTronFeeRatesFn      func(ctx context.Context) (bandwidthFeeSunPerUnit int64, energyFeeSunPerUnit int64, err error)
	buildTronTransactionFn func(ctx context.Context, in *pb.ChainRpcBuildTronTransactionReq) (*pb.ChainRpcBuildTronTransactionResp, error)
}

func (s *stubChainClient) GetBalance(ctx context.Context, in *pb.GetBalanceReq) (*pb.GetBalanceResp, error) {
	panic("not implemented")
}

func (s *stubChainClient) GetTokenBalance(ctx context.Context, in *pb.GetTokenBalanceReq) (*pb.GetTokenBalanceResp, error) {
	panic("not implemented")
}

func (s *stubChainClient) GetTokenInfo(ctx context.Context, in *pb.GetTokenInfoReq) (*pb.GetTokenInfoResp, error) {
	panic("not implemented")
}

func (s *stubChainClient) SuggestGasPrice(ctx context.Context, chain pb.ChainRpcType) (string, error) {
	panic("not implemented")
}

func (s *stubChainClient) EstimateGas(ctx context.Context, in *pb.EstimateGasReq) (*pb.EstimateGasResp, error) {
	panic("not implemented")
}

func (s *stubChainClient) BuildTransaction(ctx context.Context, in *pb.BuildTransactionReq) (*pb.BuildTransactionResp, error) {
	panic("not implemented")
}

func (s *stubChainClient) BroadcastTransaction(ctx context.Context, in *pb.BroadcastTransactionReq) (*pb.BroadcastTransactionResp, error) {
	panic("not implemented")
}

func (s *stubChainClient) GetTransactionReceipt(ctx context.Context, in *pb.GetTransactionReceiptReq) (*pb.GetTransactionReceiptResp, error) {
	panic("not implemented")
}

func (s *stubChainClient) GetBlockHeight(ctx context.Context, in *pb.GetBlockHeightReq) (*pb.GetBlockHeightResp, error) {
	panic("not implemented")
}

func (s *stubChainClient) EstimateTronFee(ctx context.Context, in *pb.EstimateTronFeeReq) (*pb.EstimateTronFeeResp, error) {
	if s.estimateTronFeeFn == nil {
		panic("EstimateTronFee not implemented")
	}
	return s.estimateTronFeeFn(ctx, in)
}

func (s *stubChainClient) GetTronFeeRates(ctx context.Context) (bandwidthFeeSunPerUnit int64, energyFeeSunPerUnit int64, err error) {
	if s.getTronFeeRatesFn == nil {
		panic("GetTronFeeRates not implemented")
	}
	return s.getTronFeeRatesFn(ctx)
}

func (s *stubChainClient) GetTronAccountResources(ctx context.Context, in *pb.GetTronAccountResourcesReq) (*pb.GetTronAccountResourcesResp, error) {
	if s.getTronResourcesFn == nil {
		panic("GetTronAccountResources not implemented")
	}
	return s.getTronResourcesFn(ctx, in)
}

func (s *stubChainClient) BuildTronTransaction(ctx context.Context, in *pb.ChainRpcBuildTronTransactionReq) (*pb.ChainRpcBuildTronTransactionResp, error) {
	if s.buildTronTransactionFn == nil {
		panic("BuildTronTransaction not implemented")
	}
	return s.buildTronTransactionFn(ctx, in)
}

func TestPrepareTronPrecheck_Token_EnergyRentalMode_ReturnsNeedEnergy(t *testing.T) {
	chain := &stubChainClient{
		estimateTronFeeFn: func(ctx context.Context, in *pb.EstimateTronFeeReq) (*pb.EstimateTronFeeResp, error) {
			return &pb.EstimateTronFeeResp{
				Success:            true,
				Message:            "ok",
				BandwidthRequired:  345,
				EnergyRequired:     31000,
				EstimatedFeeSun:    "1000000",
				ToAddressActivated: true,
				ActivationFeeSun:   "0",
			}, nil
		},
		getTronResourcesFn: func(ctx context.Context, in *pb.GetTronAccountResourcesReq) (*pb.GetTronAccountResourcesResp, error) {
			return &pb.GetTronAccountResourcesResp{
				Success:            true,
				Message:            "ok",
				BandwidthAvailable: 0,
				EnergyAvailable:    0,
				TrxBalanceSun:      "2000000",
			}, nil
		},
		getTronFeeRatesFn: func(ctx context.Context) (int64, int64, error) {
			// Bandwidth fee: 1 SUN per unit.
			return 1, 0, nil
		},
	}

	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			Consolidation: config.ConsolidationConfig{
				GasSafetyMultipliers: map[string]float64{"TRON": 1.2},
				TronTrc20FeeMode:     "energy_rental",
				EnergyRental: config.EnergyRentalConfig{
					Enabled: true,
				},
			},
		},
		Chain: chain,
	}
	executor := NewExecutor(svcCtx)

	contract := "TContract"
	task := &models.ConsolidationTask{
		Chain:         "TRON",
		FromAddress:   "TFrom",
		ToAddress:     "TTo",
		AssetSymbol:   "USDT",
		TokenContract: &contract,
	}

	res, err := executor.prepareTronPrecheck(context.Background(), task, "100")
	require.NoError(t, err)
	require.True(t, res.needEnergy)
	require.False(t, res.needGas)
	require.Nil(t, res.deferUntil)
}

func TestPrepareTronPrecheck_Token_TrxFeeMode_SufficientTrx_Proceeds(t *testing.T) {
	chain := &stubChainClient{
		estimateTronFeeFn: func(ctx context.Context, in *pb.EstimateTronFeeReq) (*pb.EstimateTronFeeResp, error) {
			return &pb.EstimateTronFeeResp{
				Success:            true,
				Message:            "ok",
				BandwidthRequired:  345,
				EnergyRequired:     31000,
				EstimatedFeeSun:    "3000000",
				ToAddressActivated: true,
				ActivationFeeSun:   "0",
			}, nil
		},
		getTronResourcesFn: func(ctx context.Context, in *pb.GetTronAccountResourcesReq) (*pb.GetTronAccountResourcesResp, error) {
			return &pb.GetTronAccountResourcesResp{
				Success:            true,
				Message:            "ok",
				BandwidthAvailable: 0,
				EnergyAvailable:    0,
				TrxBalanceSun:      "10000000",
			}, nil
		},
		// If called, test should fail (trx_fee mode must not rent energy).
		getTronFeeRatesFn: nil,
	}

	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			Consolidation: config.ConsolidationConfig{
				GasSafetyMultipliers: map[string]float64{"TRON": 1.2},
				TronTrc20FeeMode:     "trx_fee",
				EnergyRental: config.EnergyRentalConfig{
					Enabled: true,
				},
			},
		},
		Chain: chain,
	}
	executor := NewExecutor(svcCtx)

	contract := "TContract"
	task := &models.ConsolidationTask{
		Chain:         "TRON",
		FromAddress:   "TFrom",
		ToAddress:     "TTo",
		AssetSymbol:   "USDT",
		TokenContract: &contract,
	}

	res, err := executor.prepareTronPrecheck(context.Background(), task, "100")
	require.NoError(t, err)
	require.False(t, res.needEnergy)
	require.False(t, res.needGas)
	require.NotNil(t, res.estimatedFee)
	require.Equal(t, "3600000", *res.estimatedFee) // 3_000_000 * 1.2 => 3_600_000 (ceil)
	require.Equal(t, "100", res.amountToSend)
}

func TestPrepareTronPrecheck_Token_TrxFeeMode_InsufficientTrx_ReturnsNeedGas(t *testing.T) {
	chain := &stubChainClient{
		estimateTronFeeFn: func(ctx context.Context, in *pb.EstimateTronFeeReq) (*pb.EstimateTronFeeResp, error) {
			return &pb.EstimateTronFeeResp{
				Success:            true,
				Message:            "ok",
				BandwidthRequired:  345,
				EnergyRequired:     31000,
				EstimatedFeeSun:    "3000000",
				ToAddressActivated: true,
				ActivationFeeSun:   "0",
			}, nil
		},
		getTronResourcesFn: func(ctx context.Context, in *pb.GetTronAccountResourcesReq) (*pb.GetTronAccountResourcesResp, error) {
			return &pb.GetTronAccountResourcesResp{
				Success:            true,
				Message:            "ok",
				BandwidthAvailable: 0,
				EnergyAvailable:    0,
				TrxBalanceSun:      "1000000",
			}, nil
		},
		// If called, test should fail (trx_fee mode must not rent energy).
		getTronFeeRatesFn: nil,
	}

	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			Consolidation: config.ConsolidationConfig{
				GasSafetyMultipliers: map[string]float64{"TRON": 1.2},
				TronTrc20FeeMode:     "trx_fee",
			},
		},
		Chain: chain,
	}
	executor := NewExecutor(svcCtx)

	contract := "TContract"
	task := &models.ConsolidationTask{
		Chain:         "TRON",
		FromAddress:   "TFrom",
		ToAddress:     "TTo",
		AssetSymbol:   "USDT",
		TokenContract: &contract,
	}

	res, err := executor.prepareTronPrecheck(context.Background(), task, "100")
	require.NoError(t, err)
	require.False(t, res.needEnergy)
	require.True(t, res.needGas)
	require.Contains(t, res.needGasReason, "insufficient TRX")
	require.Contains(t, res.needGasReason, "for fee")
}

func TestPrepareTronPrecheck_Token_TrxFeeMode_ReserveIgnored_AllowsConsumingReserve(t *testing.T) {
	chain := &stubChainClient{
		estimateTronFeeFn: func(ctx context.Context, in *pb.EstimateTronFeeReq) (*pb.EstimateTronFeeResp, error) {
			return &pb.EstimateTronFeeResp{
				Success:            true,
				Message:            "ok",
				BandwidthRequired:  0,
				EnergyRequired:     0,
				EstimatedFeeSun:    "1000000",
				ToAddressActivated: true,
				ActivationFeeSun:   "0",
			}, nil
		},
		getTronResourcesFn: func(ctx context.Context, in *pb.GetTronAccountResourcesReq) (*pb.GetTronAccountResourcesResp, error) {
			return &pb.GetTronAccountResourcesResp{
				Success:            true,
				Message:            "ok",
				BandwidthAvailable: 0,
				EnergyAvailable:    0,
				TrxBalanceSun:      "1500000",
			}, nil
		},
		// trx_fee mode must not rent energy.
		getTronFeeRatesFn: nil,
	}

	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			Consolidation: config.ConsolidationConfig{
				GasSafetyMultipliers: map[string]float64{"TRON": 1.0},
				TronTrc20FeeMode:     "trx_fee",
				NativeReserves: map[string]map[string]string{
					"TRON": {"TRX": "2000000"},
				},
			},
		},
		Chain: chain,
	}
	executor := NewExecutor(svcCtx)

	contract := "TContract"
	task := &models.ConsolidationTask{
		Chain:         "TRON",
		FromAddress:   "TFrom",
		ToAddress:     "TTo",
		AssetSymbol:   "USDT",
		TokenContract: &contract,
	}

	res, err := executor.prepareTronPrecheck(context.Background(), task, "100")
	require.NoError(t, err)
	require.False(t, res.needEnergy)
	require.False(t, res.needGas) // reserve is ignored for tokens
	require.NotNil(t, res.estimatedFee)
	require.Equal(t, "1000000", *res.estimatedFee)
	require.Equal(t, "100", res.amountToSend)
}

func TestPrepareTronPrecheck_Token_EnergyRentalMode_BandwidthInsufficient_ReturnsNeedBandwidth(t *testing.T) {
	chain := &stubChainClient{
		estimateTronFeeFn: func(ctx context.Context, in *pb.EstimateTronFeeReq) (*pb.EstimateTronFeeResp, error) {
			return &pb.EstimateTronFeeResp{
				Success:            true,
				Message:            "ok",
				BandwidthRequired:  345,
				EnergyRequired:     31000,
				EstimatedFeeSun:    "1000000",
				ToAddressActivated: true,
				ActivationFeeSun:   "0",
			}, nil
		},
		getTronResourcesFn: func(ctx context.Context, in *pb.GetTronAccountResourcesReq) (*pb.GetTronAccountResourcesResp, error) {
			return &pb.GetTronAccountResourcesResp{
				Success:            true,
				Message:            "ok",
				BandwidthAvailable: 0,
				EnergyAvailable:    0,
				TrxBalanceSun:      "500000", // 0.5 TRX
			}, nil
		},
		// No longer used by precheck, but keep stubbed for safety.
		getTronFeeRatesFn: func(ctx context.Context) (int64, int64, error) { return 1, 0, nil },
	}

	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			Consolidation: config.ConsolidationConfig{
				GasSafetyMultipliers: map[string]float64{"TRON": 1.0},
				TronTrc20FeeMode:     "energy_rental",
				EnergyRental: config.EnergyRentalConfig{
					Enabled: true,
				},
				TopUp: config.TopUpConfig{
					TronBandwidthTopUpSun: "1000000", // 1 TRX
				},
			},
		},
		Chain: chain,
	}
	executor := NewExecutor(svcCtx)

	contract := "TContract"
	task := &models.ConsolidationTask{
		Chain:         "TRON",
		FromAddress:   "TFrom",
		ToAddress:     "TTo",
		AssetSymbol:   "USDT",
		TokenContract: &contract,
	}

	res, err := executor.prepareTronPrecheck(context.Background(), task, "100")
	require.NoError(t, err)
	require.True(t, res.needBandwidth)
	require.False(t, res.needEnergy)
	require.False(t, res.needGas)
	require.Equal(t, uint64(345), res.bandwidthNeed)
	require.Equal(t, uint64(0), res.bandwidthHave)
	require.Equal(t, "1000000", res.bandwidthTopUp)
	require.Equal(t, "1000000", res.requiredNative)
	require.Equal(t, "500000", res.shortfall) // 1_000_000 - 500_000
}

func TestPrepareTronPrecheck_Token_TrxFeeMode_BandwidthInsufficient_AddsFixedCushionAfterBuffer(t *testing.T) {
	chain := &stubChainClient{
		estimateTronFeeFn: func(ctx context.Context, in *pb.EstimateTronFeeReq) (*pb.EstimateTronFeeResp, error) {
			return &pb.EstimateTronFeeResp{
				Success:            true,
				Message:            "ok",
				BandwidthRequired:  345,
				EnergyRequired:     31000,
				EstimatedFeeSun:    "3000000",
				ToAddressActivated: true,
				ActivationFeeSun:   "0",
			}, nil
		},
		getTronResourcesFn: func(ctx context.Context, in *pb.GetTronAccountResourcesReq) (*pb.GetTronAccountResourcesResp, error) {
			return &pb.GetTronAccountResourcesResp{
				Success:            true,
				Message:            "ok",
				BandwidthAvailable: 0,
				EnergyAvailable:    0,
				TrxBalanceSun:      "4000000", // 4 TRX
			}, nil
		},
		// trx_fee mode must not depend on fee-rate RPCs.
		getTronFeeRatesFn: nil,
	}

	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			Consolidation: config.ConsolidationConfig{
				GasSafetyMultipliers: map[string]float64{"TRON": 1.2},
				TronTrc20FeeMode:     "trx_fee",
				TopUp: config.TopUpConfig{
					TronBandwidthTopUpSun: "1000000", // 1 TRX cushion
				},
			},
		},
		Chain: chain,
	}
	executor := NewExecutor(svcCtx)

	contract := "TContract"
	task := &models.ConsolidationTask{
		Chain:         "TRON",
		FromAddress:   "TFrom",
		ToAddress:     "TTo",
		AssetSymbol:   "USDT",
		TokenContract: &contract,
	}

	res, err := executor.prepareTronPrecheck(context.Background(), task, "100")
	require.NoError(t, err)
	require.False(t, res.needBandwidth)
	require.False(t, res.needEnergy)
	require.True(t, res.needGas)
	require.NotNil(t, res.estimatedFee)
	require.Equal(t, "3600000", *res.estimatedFee) // 3_000_000 * 1.2 => 3_600_000 (ceil), THEN +1_000_000 for required
	require.Equal(t, "4600000", res.requiredNative)
	require.Equal(t, "600000", res.shortfall) // 4_600_000 - 4_000_000
	require.Contains(t, res.needGasReason, "bandwidth")
}
