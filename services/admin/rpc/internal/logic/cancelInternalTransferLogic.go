package logic

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
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

	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type CancelInternalTransferLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCancelInternalTransferLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CancelInternalTransferLogic {
	return &CancelInternalTransferLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CancelInternalTransferLogic) CancelInternalTransfer(in *pb.CancelInternalTransferRequest) (*pb.CancelInternalTransferResponse, error) {
	if in == nil || in.Id <= 0 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid id", map[string]string{"id": "required"})
	}
	if l.svcCtx.DB == nil || l.svcCtx.CurrencyTransferOrderRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}
	current, ok := admininterceptor.GetCurrentAdmin(l.ctx)
	if !ok || current == nil {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}

	now := time.Now()
	ip := middleware.GetClientIP(l.ctx)
	ua := middleware.GetUserAgent(l.ctx)
	reason := strings.TrimSpace(in.Reason)

	var updated *model.CurrencyTransferOrderModel
	if err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		var m model.CurrencyTransferOrderModel
		if err := tx.WithContext(l.ctx).
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", in.Id).
			First(&m).Error; err != nil {
			return err
		}
		// Can cancel pending or processing transfers
		if m.Status != model.TransferStatusPending && m.Status != model.TransferStatusProcessing {
			return errx.New(codes.FailedPrecondition, 409, errx.CodeConflict, "INVALID_STATUS", "only pending or processing transfers can be cancelled", map[string]string{"status": m.Status})
		}

		// 如果有冻结资金，先解冻
		if m.FreezeLedgerTxID != nil && *m.FreezeLedgerTxID > 0 {
			l.Logger.Infof("Unfreezing funds for transfer %d (freeze_tx=%d)", in.Id, *m.FreezeLedgerTxID)
			unfreezeKey := "transfer_unfreeze:" + strconv.FormatInt(m.ID, 10)
			unfreezeBizRef := "internal_transfer_cancel:" + strconv.FormatInt(m.ID, 10)

			// 截断金额精度到6位小数，避免 Accounting 服务精度校验失败
			amountDecimal, _ := decimal.NewFromString(m.Amount)
			amountTruncated := amountDecimal.Truncate(6).String()

			if l.svcCtx.AccountingRpc != nil {
				unfreezeResp, unfreezeErr := l.svcCtx.AccountingRpc.UnfreezeUserAssets(l.ctx, &pb.UnfreezeUserAssetsRequest{
					IdempotencyKey: unfreezeKey,
					BizRef:         unfreezeBizRef,
					UserId:         m.FromUserID,
					AssetCode:      m.AssetCode,
					AmountDecimal:  amountTruncated,
				})
				if unfreezeErr != nil {
					l.Logger.Errorf("Failed to unfreeze user assets: %v", unfreezeErr)
					return unfreezeErr
				}
				if unfreezeResp == nil || !unfreezeResp.Success {
					errMsg := "unfreeze failed"
					if unfreezeResp != nil && unfreezeResp.Message != "" {
						errMsg = unfreezeResp.Message
					}
					l.Logger.Errorf("Unfreeze failed: %s", errMsg)
					return errors.New(errMsg)
				}
				l.Logger.Infof("Funds unfrozen successfully for transfer %d", in.Id)
			}
		}

		// Update order status to cancelled
		errMsg := "Cancelled: " + reason
		if err := tx.WithContext(l.ctx).
			Model(&model.CurrencyTransferOrderModel{}).
			Where("id = ?", in.Id).
			Updates(map[string]interface{}{
				"status":         model.TransferStatusCancelled,
				"error_message":  &errMsg,
				"audit_admin_id": current.ID,
				"audit_note":     strPtrOrNilTrim(reason),
				"audited_at":     &now,
				"updated_by":     current.ID,
			}).Error; err != nil {
			return err
		}

		var out model.CurrencyTransferOrderModel
		if err := tx.WithContext(l.ctx).
			Where("id = ?", in.Id).
			First(&out).Error; err != nil {
			return err
		}
		updated = &out

		// Update user transaction record status (转出方记录)
		if l.svcCtx.UserTransactionRecordRepo != nil {
			fromIdempotencyKey := "transfer:" + strconv.FormatInt(m.ID, 10) + ":from"
			if _, err := l.svcCtx.UserTransactionRecordRepo.UpdateStatusByIdempotencyKey(l.ctx, fromIdempotencyKey, "cancelled", nil); err != nil {
				l.Logger.Errorf("Failed to update user transaction record status: %v", err)
				// Log error but don't fail the transaction (best-effort)
			}
		}

		// Create audit log
		auditRepo := repository.NewAdminAuditLogRepository(tx)
		detailsBytes, _ := json.Marshal(map[string]interface{}{
			"id":           in.Id,
			"from_user_id": m.FromUserID,
			"to_user_id":   m.ToUserID,
			"asset_code":   m.AssetCode,
			"amount":       m.Amount,
			"strategy":     strings.TrimSpace(m.Strategy),
			"status_from":  m.Status,
			"status_to":    model.TransferStatusCancelled,
			"reason":       reason,
		})
		_ = auditRepo.CreateLog(l.ctx, &model.AdminAuditLogModel{
			AdminID:     current.ID,
			Action:      "currency.transfer.cancel",
			TargetType:  "currency_transfer_order",
			TargetID:    strconv.FormatInt(in.Id, 10),
			Description: "取消内部转账",
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
			return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "NOT_FOUND", "transfer not found", nil)
		}
		l.Logger.Errorf("cancel internal transfer failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "cancel transfer failed", nil)
	}

	return &pb.CancelInternalTransferResponse{
		Success: true,
		Message: "ok",
		Data: &pb.CancelInternalTransferData{
			Transfer: toPBInternalTransferItem(updated),
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
