package logic

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"internalwallet/common/middleware"
	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	admininterceptor "internalwallet/services/admin/rpc/internal/interceptor"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/repository"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ExecuteTransferBatchLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewExecuteTransferBatchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ExecuteTransferBatchLogic {
	return &ExecuteTransferBatchLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ExecuteTransferBatchLogic) ExecuteTransferBatch(in *pb.ExecuteTransferBatchRequest) (*pb.ExecuteTransferBatchResponse, error) {
	if in == nil || strings.TrimSpace(in.BatchId) == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "batch_id required", map[string]string{"batch_id": "required"})
	}
	// Prevent entering a stuck state if the background worker is not running.
	if !l.svcCtx.Config.TransferBatchWorker.Enabled {
		return nil, errx.New(codes.FailedPrecondition, 409, errx.CodeConflict, "TRANSFER_BATCH_WORKER_DISABLED", "transfer batch worker is disabled", map[string]string{"worker": "disabled"})
	}
	if l.svcCtx.DB == nil || l.svcCtx.TransferBatchRepo == nil || l.svcCtx.TransferBatchItemRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}
	if l.svcCtx.AccountingRpc == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "ACCOUNTING_NOT_CONFIGURED", "accounting rpc not configured", nil)
	}
	current, ok := admininterceptor.GetCurrentAdmin(l.ctx)
	if !ok || current == nil {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}
	if err := verifyAdminTwoFA(l.svcCtx, l.ctx, current.ID, in.TwoFaCode); err != nil {
		return nil, err
	}

	now := time.Now()
	ip := middleware.GetClientIP(l.ctx)
	ua := middleware.GetUserAgent(l.ctx)
	note := strings.TrimSpace(in.Note)

	if err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		var m model.TransferBatchModel
		if err := tx.WithContext(l.ctx).
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("batch_id = ?", strings.TrimSpace(in.BatchId)).
			First(&m).Error; err != nil {
			return err
		}
		if m.Status != "approved" {
			return errx.New(codes.FailedPrecondition, 409, errx.CodeConflict, "BATCH_NOT_APPROVED", "batch is not approved", map[string]string{"status": m.Status})
		}
		// Transfer batches are internal-ledger only. Prevent putting non-internal batches into a stuck "processing" state.
		if !strings.EqualFold(strings.TrimSpace(m.Network), "internal") {
			return errx.New(codes.FailedPrecondition, 409, errx.CodeConflict, "BATCH_NOT_INTERNAL", "batch is not internal", map[string]string{"network": strings.TrimSpace(m.Network)})
		}

		if err := tx.WithContext(l.ctx).
			Model(&model.TransferBatchModel{}).
			Where("batch_id = ?", strings.TrimSpace(in.BatchId)).
			Updates(map[string]interface{}{
				"status":       "processing",
				"processed_at": &now,
			}).Error; err != nil {
			return err
		}

		detailsBytes, _ := json.Marshal(map[string]interface{}{
			"batch_id":    in.BatchId,
			"note":        note,
			"status_from": "approved",
			"status_to":   "processing",
		})
		auditRepo := repository.NewAdminAuditLogRepository(tx)
		_ = auditRepo.CreateLog(l.ctx, &model.AdminAuditLogModel{
			AdminID:     current.ID,
			Action:      "transfer.batch.execute",
			TargetType:  "transfer_batch",
			TargetID:    in.BatchId,
			Description: "执行转账批次",
			Details:     detailsBytes,
			IP:          ip,
			UserAgent:   ua,
		})
		return nil
	}); err != nil {
		if _, ok := status.FromError(err); ok {
			return nil, err
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "BATCH_NOT_FOUND", "batch not found", nil)
		}
		l.Logger.Errorf("execute transfer batch failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	return &pb.ExecuteTransferBatchResponse{
		Success: true,
		Message: resp.Msg(l.ctx, "BATCH_EXEC_STARTED"),
		Data: &pb.ExecuteTransferBatchData{
			BatchId:   strings.TrimSpace(in.BatchId),
			Status:    "processing",
			StartedAt: formatTime(now),
			StartedBy: current.Username,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
