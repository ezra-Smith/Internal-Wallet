package scheduler

import (
	"context"
	"testing"
	"time"

	"internalwallet/pkg/chainnode"
	"internalwallet/proto/pb"
	"internalwallet/services/consolidation/rpc/internal/config"
	"internalwallet/services/consolidation/rpc/internal/models"
	"internalwallet/services/consolidation/rpc/internal/repository"
	"internalwallet/services/consolidation/rpc/internal/svc"

	"github.com/stretchr/testify/require"
)

type stubStatusTaskRepo struct {
	repository.ConsolidationTaskRepository

	markRetryPendingCalled bool
	markFailedCalled       bool

	lastRetryCount int32
	lastMsg        string
}

func (r *stubStatusTaskRepo) MarkRetryPending(ctx context.Context, id int64, instanceID string, expectedVersion int64, msg string, nextAttemptAt *time.Time, retryCount int32) (bool, error) {
	r.markRetryPendingCalled = true
	r.lastRetryCount = retryCount
	r.lastMsg = msg
	return true, nil
}

func (r *stubStatusTaskRepo) MarkFailed(ctx context.Context, id int64, instanceID string, expectedVersion int64, status models.ConsolidationTaskStatus, msg string, nextAttemptAt *time.Time, retryCount int32) (bool, error) {
	r.markFailedCalled = true
	r.lastRetryCount = retryCount
	r.lastMsg = msg
	return true, nil
}

type stubStatusChain struct {
	chainnode.Client

	getReceiptFn func(ctx context.Context, in *pb.GetTransactionReceiptReq) (*pb.GetTransactionReceiptResp, error)
}

func (s *stubStatusChain) GetTransactionReceipt(ctx context.Context, in *pb.GetTransactionReceiptReq) (*pb.GetTransactionReceiptResp, error) {
	if s.getReceiptFn == nil {
		panic("GetTransactionReceipt not implemented")
	}
	return s.getReceiptFn(ctx, in)
}

func TestStatusTracker_TxFailed_SchedulesRetryPending(t *testing.T) {
	now := time.Now().Local()
	txHash := "0xabc"
	startedAt := now.Add(-10 * time.Second)

	task := &models.ConsolidationTask{
		TaskID:      "t1",
		Chain:       "ETH",
		FromAddress: "0xFrom",
		ToAddress:   "0xTo",
		Amount:      "1",
		Status:      models.ConsolidationTaskStatusInProgress,
		TxHash:      &txHash,
		StartedAt:   &startedAt,
		RetryCount:  0,
		Version:     0,
	}

	taskRepo := &stubStatusTaskRepo{}
	chain := &stubStatusChain{
		getReceiptFn: func(ctx context.Context, in *pb.GetTransactionReceiptReq) (*pb.GetTransactionReceiptResp, error) {
			return &pb.GetTransactionReceiptResp{
				Success: true,
				Receipt: &pb.TransactionReceipt{
					Status:      pb.TxStatus_TX_STATUS_FAILED,
					BlockNumber: "1",
				},
			}, nil
		},
	}

	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			Consolidation: config.ConsolidationConfig{
				Enabled:             true,
				MaxRetries:          3,
				RetryBackoffSeconds: []int64{1},
			},
		},
		Chain:                 chain,
		ConsolidationTaskRepo: taskRepo,
		InstanceID:            "test",
	}

	tracker := NewStatusTracker(svcCtx)
	tracker.processOne(context.Background(), task, now, 0)

	require.True(t, taskRepo.markRetryPendingCalled)
	require.False(t, taskRepo.markFailedCalled)
	require.Equal(t, int32(1), taskRepo.lastRetryCount)
	require.Contains(t, taskRepo.lastMsg, "tx status=TX_STATUS_FAILED")
}

func TestStatusTracker_NoReceiptBeyondMaxNoReceiptDuration_SchedulesRetry(t *testing.T) {
	now := time.Now().Local()
	txHash := "0xabc"
	startedAt := now.Add(-5 * time.Second)

	task := &models.ConsolidationTask{
		TaskID:      "t2",
		Chain:       "ETH",
		FromAddress: "0xFrom",
		ToAddress:   "0xTo",
		Amount:      "1",
		Status:      models.ConsolidationTaskStatusInProgress,
		TxHash:      &txHash,
		StartedAt:   &startedAt,
		RetryCount:  0,
		Version:     0,
	}

	taskRepo := &stubStatusTaskRepo{}
	chain := &stubStatusChain{
		getReceiptFn: func(ctx context.Context, in *pb.GetTransactionReceiptReq) (*pb.GetTransactionReceiptResp, error) {
			return &pb.GetTransactionReceiptResp{Success: false, Message: "node error"}, nil
		},
	}

	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			Consolidation: config.ConsolidationConfig{
				Enabled:              true,
				MaxRetries:           3,
				RetryBackoffSeconds:  []int64{1},
				MaxNoReceiptDuration: 1,
			},
		},
		Chain:                 chain,
		ConsolidationTaskRepo: taskRepo,
		InstanceID:            "test",
	}

	tracker := NewStatusTracker(svcCtx)
	tracker.processOne(context.Background(), task, now, 0)

	require.True(t, taskRepo.markRetryPendingCalled)
	require.Equal(t, int32(1), taskRepo.lastRetryCount)
	require.Contains(t, taskRepo.lastMsg, "no receipt beyond")
}
