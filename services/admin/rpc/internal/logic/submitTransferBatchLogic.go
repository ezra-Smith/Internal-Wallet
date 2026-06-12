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

type SubmitTransferBatchLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSubmitTransferBatchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SubmitTransferBatchLogic {
	return &SubmitTransferBatchLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SubmitTransferBatchLogic) SubmitTransferBatch(in *pb.SubmitTransferBatchRequest) (*pb.SubmitTransferBatchResponse, error) {
	if in == nil || strings.TrimSpace(in.BatchId) == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "batch_id required", map[string]string{"batch_id": "required"})
	}
	if l.svcCtx.DB == nil || l.svcCtx.TransferBatchRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}
	current, ok := admininterceptor.GetCurrentAdmin(l.ctx)
	if !ok || current == nil {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
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
		if m.Status != "draft" {
			return errx.New(codes.FailedPrecondition, 409, errx.CodeConflict, "BATCH_NOT_DRAFT", "batch is not draft", map[string]string{"status": m.Status})
		}

		if err := tx.WithContext(l.ctx).
			Model(&model.TransferBatchModel{}).
			Where("batch_id = ?", strings.TrimSpace(in.BatchId)).
			Updates(map[string]interface{}{
				"status":       "pending",
				"submitted_at": &now,
			}).Error; err != nil {
			return err
		}

		detailsBytes, _ := json.Marshal(map[string]interface{}{
			"batch_id":    in.BatchId,
			"note":        note,
			"status_from": "draft",
			"status_to":   "pending",
		})
		auditRepo := repository.NewAdminAuditLogRepository(tx)
		_ = auditRepo.CreateLog(l.ctx, &model.AdminAuditLogModel{
			AdminID:     current.ID,
			Action:      "transfer.batch.submit",
			TargetType:  "transfer_batch",
			TargetID:    in.BatchId,
			Description: "提交转账批次审批",
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
		l.Logger.Errorf("submit transfer batch failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	return &pb.SubmitTransferBatchResponse{
		Success: true,
		Message: resp.Msg(l.ctx, "BATCH_SUBMITTED"),
		Data: &pb.SubmitTransferBatchData{
			BatchId:     strings.TrimSpace(in.BatchId),
			Status:      "pending",
			SubmittedAt: formatTime(now),
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
