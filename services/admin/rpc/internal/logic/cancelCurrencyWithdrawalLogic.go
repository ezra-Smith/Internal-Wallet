package logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"internalwallet/common/constants"
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

type CancelCurrencyWithdrawalLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCancelCurrencyWithdrawalLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CancelCurrencyWithdrawalLogic {
	return &CancelCurrencyWithdrawalLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CancelCurrencyWithdrawalLogic) CancelCurrencyWithdrawal(in *pb.CancelCurrencyWithdrawalRequest) (*pb.CancelCurrencyWithdrawalResponse, error) {
	if in == nil || in.Id <= 0 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid id", map[string]string{"id": "required"})
	}
	if l.svcCtx.DB == nil || l.svcCtx.CurrencyWithdrawOrderRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}
	if l.svcCtx.CurrencyWithdrawOrderEventRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "withdraw events repo not configured", nil)
	}
	if l.svcCtx.AccountingRpc == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "ACCOUNTING_NOT_CONFIGURED", "accounting rpc not configured", nil)
	}
	current, ok := admininterceptor.GetCurrentAdmin(l.ctx)
	if !ok || current == nil {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}

	now := time.Now()
	ip := middleware.GetClientIP(l.ctx)
	ua := middleware.GetUserAgent(l.ctx)
	reason := strings.TrimSpace(in.Reason)

	var updated *model.CurrencyWithdrawOrderModel
	if err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		var m model.CurrencyWithdrawOrderModel
		if err := tx.WithContext(l.ctx).
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", in.Id).
			First(&m).Error; err != nil {
			return err
		}
		if m.Status != "pending" {
			return errx.New(codes.FailedPrecondition, 409, errx.CodeConflict, "NOT_PENDING", "only pending withdrawals can be cancelled", map[string]string{"status": m.Status})
		}

		events := make([]*model.CurrencyWithdrawOrderEventModel, 0, 2)

		// Check if asset was frozen before attempting to unfreeze
		eventRepo := l.svcCtx.CurrencyWithdrawOrderEventRepo.WithTx(tx)
		frozenEvents, err := eventRepo.FindByOrderAndEventType(l.ctx, m.ID, constants.WithdrawEventAssetFrozen)
		if err != nil {
			l.Logger.Errorf("failed to check frozen events: %v", err)
			// Continue anyway, attempt unfreeze
		}

		// Only unfreeze if asset was frozen
		if len(frozenEvents) > 0 {
			// 内扣模式：冻结的是 amount（不含 fee），所以解冻也是 amount
			// 截断到 6 位小数以匹配 Accounting 服务的精度限制
			unfreezeAmount := strings.TrimSpace(m.Amount)
			if amountDec, parseErr := decimal.NewFromString(unfreezeAmount); parseErr == nil {
				unfreezeAmount = amountDec.Truncate(6).String()
			}
			unfreezeReq := &pb.UnfreezeWithdrawRequest{
				IdempotencyKey: fmt.Sprintf("withdraw:unfreeze:%d", m.ID),
				BizRef:         strconv.FormatInt(m.ID, 10),
				UserId:         m.UserID,
				AssetCode:      m.AssetCode,
				AmountDecimal:  unfreezeAmount,
				FinalStatus:    "cancelled",
				Reason:         reason,
				TxHash: func() string {
					if m.TxHash == nil {
						return ""
					}
					return strings.TrimSpace(*m.TxHash)
				}(),
			}
			unfreezeResp, unfreezeErr := callAccountingTxWithRetry(l.ctx, 3, func(ctx context.Context) (*pb.LedgerTxResponse, error) {
				return l.svcCtx.AccountingRpc.UnfreezeWithdraw(ctx, unfreezeReq)
			})
			if err := errFromAccountingTx("UNFREEZE_WITHDRAW", unfreezeResp, unfreezeErr); err != nil {
				return err
			}

			events = append(events, newWithdrawOrderEvent(m.ID, constants.WithdrawEventAssetUnfrozen, constants.WithdrawActorSystem, 0, "", "Asset unfrozen", map[string]any{
				"tx_id":           unfreezeResp.TxId,
				"duplicate":       unfreezeResp.Duplicate,
				"asset_code":      strings.TrimSpace(m.AssetCode),
				"amount_decimal":  unfreezeAmount, // 内扣模式
				"idempotency_key": unfreezeReq.IdempotencyKey,
				"biz_ref":         strings.TrimSpace(unfreezeReq.BizRef),
			}))
		} else {
			l.Logger.Infof("No frozen event found for withdrawal %d, skipping unfreeze", m.ID)
			// Still update accounting record status even if no frozen event
			if l.svcCtx.AccountingRpc != nil {
				_, _ = l.svcCtx.AccountingRpc.UpsertUserTransactionRecordMeta(l.ctx, &pb.UpsertUserTransactionRecordMetaRequest{
					Id:     m.ID,
					TxType: 2,
					Status: "cancelled",
				})
			}
		}

		// Always record cancellation event
		events = append(events, newWithdrawOrderEvent(m.ID, constants.WithdrawEventOrderCancelled, constants.WithdrawActorAdmin, current.ID, ip, "Withdrawal cancelled", map[string]any{
			"id":          m.ID,
			"user_id":     m.UserID,
			"asset_code":  strings.TrimSpace(m.AssetCode),
			"chain_code":  strings.TrimSpace(m.ChainCode),
			"amount":      strings.TrimSpace(m.Amount),
			"fee":         strings.TrimSpace(m.Fee),
			"strategy":    strings.TrimSpace(m.Strategy),
			"status_from": strings.TrimSpace(m.Status),
			"status_to":   "cancelled",
			"reason":      reason,
			"admin_id":    current.ID,
		}))

		if err := eventRepo.CreateMultiple(l.ctx, events); err != nil {
			return err
		}

		if err := tx.WithContext(l.ctx).
			Model(&model.CurrencyWithdrawOrderModel{}).
			Where("id = ?", in.Id).
			Updates(map[string]interface{}{
				"status": "cancelled",
				"error_message": func() interface{} {
					if reason == "" {
						return nil
					}
					return reason
				}(),
				"updated_at": now,
				"updated_by": current.ID,
			}).Error; err != nil {
			return err
		}

		var out model.CurrencyWithdrawOrderModel
		if err := tx.WithContext(l.ctx).
			Where("id = ?", in.Id).
			First(&out).Error; err != nil {
			return err
		}
		updated = &out

		auditRepo := repository.NewAdminAuditLogRepository(tx)
		detailsBytes, _ := json.Marshal(map[string]interface{}{
			"id":          in.Id,
			"user_id":     m.UserID,
			"asset_code":  m.AssetCode,
			"chain_code":  m.ChainCode,
			"amount":      m.Amount,
			"strategy":    strings.TrimSpace(m.Strategy),
			"status_from": m.Status,
			"status_to":   "cancelled",
			"reason":      reason,
		})
		_ = auditRepo.CreateLog(l.ctx, &model.AdminAuditLogModel{
			AdminID:     current.ID,
			Action:      "currency.withdrawal.cancel",
			TargetType:  "currency_withdraw_order",
			TargetID:    strconv.FormatInt(in.Id, 10),
			Description: "取消提现: " + strconv.FormatInt(in.Id, 10),
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
			return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "NOT_FOUND", "withdrawal not found", nil)
		}
		l.Logger.Errorf("cancel currency withdrawal failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "cancel withdrawal failed", nil)
	}

	return &pb.CancelCurrencyWithdrawalResponse{
		Success: true,
		Message: "ok",
		Data: &pb.CancelCurrencyWithdrawalData{
			Withdrawal: toPBCurrencyWithdrawalItem(updated),
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
