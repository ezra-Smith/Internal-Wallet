package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"internalwallet/pkg/chainnode"
	"internalwallet/proto/pb"
	"internalwallet/services/consolidation/rpc/internal/config"
	"internalwallet/services/consolidation/rpc/internal/models"
	"internalwallet/services/consolidation/rpc/internal/repository"
	"internalwallet/services/consolidation/rpc/internal/svc"
)

type stubEvmChainClient struct {
	chainnode.Client

	estimateGasFn func(ctx context.Context, in *pb.EstimateGasReq) (*pb.EstimateGasResp, error)
	getBalanceFn  func(ctx context.Context, in *pb.GetBalanceReq) (*pb.GetBalanceResp, error)
}

func (s *stubEvmChainClient) EstimateGas(ctx context.Context, in *pb.EstimateGasReq) (*pb.EstimateGasResp, error) {
	if s.estimateGasFn == nil {
		panic("EstimateGas not implemented")
	}
	return s.estimateGasFn(ctx, in)
}

func (s *stubEvmChainClient) GetBalance(ctx context.Context, in *pb.GetBalanceReq) (*pb.GetBalanceResp, error) {
	if s.getBalanceFn == nil {
		panic("GetBalance not implemented")
	}
	return s.getBalanceFn(ctx, in)
}

func TestPrepareEvmAmounts_Token_ReserveIgnored_AllowsConsumingReserve(t *testing.T) {
	chain := &stubEvmChainClient{
		estimateGasFn: func(ctx context.Context, in *pb.EstimateGasReq) (*pb.EstimateGasResp, error) {
			return &pb.EstimateGasResp{
				Success:      true,
				Message:      "ok",
				GasLimit:     60000,
				GasPrice:     "1",
				EstimatedFee: "50",
			}, nil
		},
		getBalanceFn: func(ctx context.Context, in *pb.GetBalanceReq) (*pb.GetBalanceResp, error) {
			return &pb.GetBalanceResp{
				Success: true,
				Message: "ok",
				Balance: "100",
			}, nil
		},
	}

	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			Consolidation: config.ConsolidationConfig{
				GasSafetyMultipliers: map[string]float64{"ETH": 1.0},
				NativeReserves: map[string]map[string]string{
					"ETH": {"ETH": "80"},
				},
			},
		},
		Chain: chain,
	}
	executor := NewExecutor(svcCtx)

	contract := "0x0000000000000000000000000000000000000002"
	task := &models.ConsolidationTask{
		Chain:         "ETH",
		FromAddress:   "0x0000000000000000000000000000000000000001",
		ToAddress:     "0x0000000000000000000000000000000000000002",
		AssetSymbol:   "USDT",
		TokenContract: &contract,
	}

	res, err := executor.prepareEvmAmounts(context.Background(), task, "123")
	require.NoError(t, err)
	require.False(t, res.needGas) // reserve is ignored for tokens
	require.Equal(t, "123", res.amountToSend)
}

type stubExecTaskRepo struct {
	repository.ConsolidationTaskRepository

	markNeedGasCalled bool
	markFailedCalled  bool
	lastFailedStatus  models.ConsolidationTaskStatus
}

func (r *stubExecTaskRepo) MarkNeedGas(ctx context.Context, id int64, instanceID string, expectedVersion int64, msg string, nextAttemptAt *time.Time) (bool, error) {
	r.markNeedGasCalled = true
	return true, nil
}

func (r *stubExecTaskRepo) MarkFailed(ctx context.Context, id int64, instanceID string, expectedVersion int64, status models.ConsolidationTaskStatus, msg string, nextAttemptAt *time.Time, retryCount int32) (bool, error) {
	r.markFailedCalled = true
	r.lastFailedStatus = status
	return true, nil
}

func TestExecutor_NativeEvm_NeedGasBecomesPermanentFailed(t *testing.T) {
	chain := &stubEvmChainClient{
		estimateGasFn: func(ctx context.Context, in *pb.EstimateGasReq) (*pb.EstimateGasResp, error) {
			return &pb.EstimateGasResp{
				Success:      true,
				Message:      "ok",
				GasLimit:     21000,
				GasPrice:     "1",
				EstimatedFee: "10",
			}, nil
		},
		getBalanceFn: func(ctx context.Context, in *pb.GetBalanceReq) (*pb.GetBalanceResp, error) {
			return &pb.GetBalanceResp{
				Success: true,
				Message: "ok",
				Balance: "100",
			}, nil
		},
	}

	taskRepo := &stubExecTaskRepo{}
	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			Consolidation: config.ConsolidationConfig{
				Enabled:       true,
				ToAddressMode: "target_addresses",
				TargetAddresses: map[string]map[string]string{
					"ETH": {"ETH": "0x0000000000000000000000000000000000000002"},
				},
				NativeReserves: map[string]map[string]string{
					"ETH": {"ETH": "100"},
				},
				MinConsolidationAmount: map[string]map[string]string{
					"ETH": {"ETH": "0"},
				},
				GasSafetyMultipliers: map[string]float64{"ETH": 1.0},
				Risk: config.RiskConfig{
					MaxNativeSpendPerTask: map[string]string{"ETH": "1000"},
				},
			},
		},
		Chain:                 chain,
		ConsolidationTaskRepo: taskRepo,
		InstanceID:            "test",
	}

	executor := NewExecutor(svcCtx)
	task := &models.ConsolidationTask{
		TaskID:      "t1",
		Chain:       "ETH",
		AssetSymbol: "ETH",
		FromAddress: "0x0000000000000000000000000000000000000001",
		ToAddress:   "0x0000000000000000000000000000000000000002",
		Amount:      "100",
		Status:      models.ConsolidationTaskStatusPending,
		Version:     0,
	}
	task.ID = 1

	err := executor.execute(context.Background(), task, 0)
	require.NoError(t, err)
	require.True(t, taskRepo.markFailedCalled)
	require.False(t, taskRepo.markNeedGasCalled)
	require.Equal(t, models.ConsolidationTaskStatusPermanentFailed, taskRepo.lastFailedStatus)
}
