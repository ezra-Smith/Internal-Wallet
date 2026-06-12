package logic

import (
	"context"
	"encoding/json"
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

type CancelDepositLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCancelDepositLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CancelDepositLogic {
	return &CancelDepositLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CancelDepositLogic) CancelDeposit(in *pb.CancelDepositRequest) (*pb.CancelDepositResponse, error) {
	if in == nil || in.Id <= 0 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid id", map[string]string{"id": "required"})
	}
	reason := strings.TrimSpace(in.Reason)
	if reason == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "reason required", map[string]string{"reason": "required"})
	}
	if l.svcCtx.DB == nil || l.svcCtx.WalletDepositRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}
	current, ok := admininterceptor.GetCurrentAdmin(l.ctx)
	if !ok || current == nil {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}

	now := time.Now()
	ip := middleware.GetClientIP(l.ctx)
	ua := middleware.GetUserAgent(l.ctx)

	var updated *model.WalletDepositModel
	if err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		var m model.WalletDepositModel
		if err := tx.WithContext(l.ctx).
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND deleted_at IS NULL", in.Id).
			First(&m).Error; err != nil {
			return err
		}
		if m.Status != "pending" {
			return errx.New(codes.FailedPrecondition, 409, errx.CodeConflict, "NOT_PENDING", "only pending deposits can be cancelled", map[string]string{"status": m.Status})
		}

		if err := tx.WithContext(l.ctx).
			Model(&model.WalletDepositModel{}).
			Where("id = ? AND deleted_at IS NULL", in.Id).
			Updates(map[string]interface{}{
				"status":     "failed",
				"admin_id":   current.ID,
				"admin_note": reason,
				"updated_at": &now,
			}).Error; err != nil {
			return err
		}

		var out model.WalletDepositModel
		if err := tx.WithContext(l.ctx).
			Where("id = ? AND deleted_at IS NULL", in.Id).
			First(&out).Error; err != nil {
			return err
		}
		updated = &out

		auditRepo := repository.NewAdminAuditLogRepository(tx)
		detailsBytes, _ := json.Marshal(map[string]interface{}{
			"id":               in.Id,
			"transaction_hash": maskAddress(m.TransactionHash),
			"reason":           reason,
		})
		_ = auditRepo.CreateLog(l.ctx, &model.AdminAuditLogModel{
			AdminID:     current.ID,
			Action:      "wallet.deposit.cancel",
			TargetType:  "wallet_deposit",
			TargetID:    maskAddress(m.TransactionHash),
			Description: "取消充值记录: " + maskAddress(m.TransactionHash),
			Details:     detailsBytes,
			IP:          ip,
			UserAgent:   ua,
		})
		return nil
	}); err != nil {
		if _, ok := status.FromError(err); ok {
			return nil, err
		}
		if strings.Contains(strings.ToLower(err.Error()), "record not found") || strings.Contains(strings.ToLower(err.Error()), "not found") {
			return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "NOT_FOUND", "deposit not found", nil)
		}
		l.Logger.Errorf("cancel deposit failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "cancel deposit failed", nil)
	}

	return &pb.CancelDepositResponse{
		Success: true,
		Message: "ok",
		Data: &pb.CancelDepositData{
			Deposit: toPBDepositItem(updated),
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
