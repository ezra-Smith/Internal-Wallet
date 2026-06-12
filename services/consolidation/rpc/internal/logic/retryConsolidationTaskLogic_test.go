package logic

import (
	"context"
	"testing"
	"time"

	"internalwallet/proto/pb"
	"internalwallet/services/consolidation/rpc/internal/config"
	"internalwallet/services/consolidation/rpc/internal/models"
	"internalwallet/services/consolidation/rpc/internal/repository"
	"internalwallet/services/consolidation/rpc/internal/svc"

	"github.com/stretchr/testify/require"
)

type stubRetryTaskRepo struct {
	repository.ConsolidationTaskRepository

	task             *models.ConsolidationTask
	forceCalled      bool
	forceRetryCount  int32
	forceAttemptTime *time.Time
}

type stubRetryTopUpRepo struct {
	repository.TopUpRecordRepository

	called bool
	taskID string
}

func (r *stubRetryTopUpRepo) SoftDeleteByTaskID(ctx context.Context, taskID string) (int64, error) {
	r.called = true
	r.taskID = taskID
	return 3, nil
}

func (r *stubRetryTaskRepo) GetByTaskID(ctx context.Context, taskID string) (*models.ConsolidationTask, error) {
	return r.task, nil
}

func (r *stubRetryTaskRepo) ForceRetryPending(ctx context.Context, id int64, msg string, nextAttemptAt *time.Time, retryCount int32) (bool, error) {
	r.forceCalled = true
	r.forceRetryCount = retryCount
	r.forceAttemptTime = nextAttemptAt
	return true, nil
}

func TestRetryConsolidationTask_ResetsRetryCountEvenWhenExceeded(t *testing.T) {
	task := &models.ConsolidationTask{
		TaskID:      "t1",
		Chain:       "TRON",
		AssetSymbol: "USDT",
		FromAddress: "TFrom",
		ToAddress:   "TTo",
		Amount:      "1",
		Status:      models.ConsolidationTaskStatusPermanentFailed,
		RetryCount:  999, // simulate already exceeded MaxRetries
		Version:     1,
	}

	repo := &stubRetryTaskRepo{task: task}
	topRepo := &stubRetryTopUpRepo{}
	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			Consolidation: config.ConsolidationConfig{
				MaxRetries: 3,
			},
		},
		ConsolidationTaskRepo: repo,
		TopUpRecordRepo:       topRepo,
		// ConsolidationLogRepo intentionally nil: WriteTaskLog should be a no-op in tests.
	}

	l := NewRetryConsolidationTaskLogic(context.Background(), svcCtx)
	resp, err := l.RetryConsolidationTask(&pb.RetryConsolidationTaskRequest{TaskId: task.TaskID})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.True(t, resp.Success)

	require.True(t, repo.forceCalled)
	require.Equal(t, int32(0), repo.forceRetryCount)
	require.NotNil(t, repo.forceAttemptTime)

	require.True(t, topRepo.called)
	require.Equal(t, task.TaskID, topRepo.taskID)
}

func TestRetryConsolidationTask_RejectsInProgress(t *testing.T) {
	task := &models.ConsolidationTask{
		TaskID:     "t2",
		Chain:      "ETH",
		Amount:     "1",
		Status:     models.ConsolidationTaskStatusInProgress,
		RetryCount: 1,
	}
	repo := &stubRetryTaskRepo{task: task}
	svcCtx := &svc.ServiceContext{ConsolidationTaskRepo: repo}

	l := NewRetryConsolidationTaskLogic(context.Background(), svcCtx)
	resp, err := l.RetryConsolidationTask(&pb.RetryConsolidationTaskRequest{TaskId: task.TaskID})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.False(t, resp.Success)
	require.Contains(t, resp.Message, "not retryable")
	require.False(t, repo.forceCalled)
}

