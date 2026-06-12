package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"internalwallet/services/consolidation/rpc/internal/config"
	"internalwallet/services/consolidation/rpc/internal/models"
	"internalwallet/services/consolidation/rpc/internal/repository"
	"internalwallet/services/consolidation/rpc/internal/svc"
)

type stubTopUpRejectTaskRepo struct {
	repository.ConsolidationTaskRepository

	markFailedCalled bool
	lastStatus       models.ConsolidationTaskStatus
}

func (r *stubTopUpRejectTaskRepo) MarkFailed(ctx context.Context, id int64, instanceID string, expectedVersion int64, status models.ConsolidationTaskStatus, msg string, nextAttemptAt *time.Time, retryCount int32) (bool, error) {
	r.markFailedCalled = true
	r.lastStatus = status
	return true, nil
}

func TestTopUpWorker_NativeTask_AlwaysPermanentFailed(t *testing.T) {
	taskRepo := &stubTopUpRejectTaskRepo{}
	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			Consolidation: config.ConsolidationConfig{
				TopUp: config.TopUpConfig{Enabled: true},
			},
		},
		ConsolidationTaskRepo: taskRepo,
		InstanceID:            "test",
	}

	worker := NewTopUpWorker(svcCtx)
	task := &models.ConsolidationTask{
		TaskID:      "t1",
		Chain:       "TRON",
		AssetSymbol: "TRX",
		FromAddress: "TFrom",
		ToAddress:   "TTo",
		Status:      models.ConsolidationTaskStatusNeedGas,
		Version:     0,
	}
	task.ID = 1

	err := worker.handleTask(context.Background(), task, 0)
	require.NoError(t, err)
	require.True(t, taskRepo.markFailedCalled)
	require.Equal(t, models.ConsolidationTaskStatusPermanentFailed, taskRepo.lastStatus)
}
