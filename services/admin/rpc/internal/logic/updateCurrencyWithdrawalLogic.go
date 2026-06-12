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

type UpdateCurrencyWithdrawalLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateCurrencyWithdrawalLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateCurrencyWithdrawalLogic {
	return &UpdateCurrencyWithdrawalLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func canTransitionCurrencyWithdrawal(from, to string) bool {
	from = strings.TrimSpace(from)
	to = strings.TrimSpace(to)
	if from == to {
		return true
	}
	switch from {
	case "pending":
		return to == "processing" || to == "failed" || to == "cancelled"
	case "processing":
		return to == "failed" || to == "completed"
	default:
		return false
	}
}

func (l *UpdateCurrencyWithdrawalLogic) UpdateCurrencyWithdrawal(in *pb.UpdateCurrencyWithdrawalRequest) (*pb.UpdateCurrencyWithdrawalResponse, error) {
	if in == nil || in.Id <= 0 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid id", map[string]string{"id": "required"})
	}
	if l.svcCtx.DB == nil || l.svcCtx.CurrencyWithdrawOrderRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}
	if l.svcCtx.CurrencyWithdrawOrderEventRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "withdraw events repo not configured", nil)
	}
	if l.svcCtx.CurrencyWithdrawPayoutTaskRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "withdraw payout task repo not configured", nil)
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

	var metaReq *pb.UpsertUserTransactionRecordMetaRequest
	var updated *model.CurrencyWithdrawOrderModel
	if err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		var m model.CurrencyWithdrawOrderModel
		if err := tx.WithContext(l.ctx).
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", in.Id).
			First(&m).Error; err != nil {
			return err
		}

		if m.Status == "completed" || m.Status == "failed" || m.Status == "cancelled" || m.Status == "rejected" {
			return errx.New(codes.FailedPrecondition, 409, errx.CodeConflict, "IMMUTABLE", "withdrawal is immutable", map[string]string{"status": m.Status})
		}

		fields := map[string]interface{}{
			"updated_at": now,
			"updated_by": current.ID,
		}
		changes := map[string]interface{}{}
		events := make([]*model.CurrencyWithdrawOrderEventModel, 0, 8)

		statusFrom := strings.TrimSpace(m.Status)
		statusTo := statusFrom
		txHashNew := ""

		if in.Status != nil {
			v := strings.TrimSpace(in.Status.Value)
			if !validateWithdrawalStatus(v) {
				return errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_STATUS", "invalid status", map[string]string{"status": "invalid"})
			}
			if v == "completed" {
				// Allow manual completion only for manual_manual strategy from processing status with tx_hash
				strategy := strings.ToLower(strings.TrimSpace(m.Strategy))
				if strategy != "manual_manual" {
					return errx.New(codes.FailedPrecondition, 409, errx.CodeConflict, "STRATEGY_NOT_ALLOWED", "only manual_manual withdrawals can be manually completed", map[string]string{"strategy": strings.TrimSpace(m.Strategy)})
				}
				if m.Status != "processing" {
					return errx.New(codes.FailedPrecondition, 409, errx.CodeConflict, "INVALID_STATUS", "can only complete withdrawals in processing status", map[string]string{"status": m.Status})
				}
				if in.TxHash == nil || strings.TrimSpace(in.TxHash.Value) == "" {
					return errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "TX_HASH_REQUIRED", "tx_hash is required when manually completing withdrawal", map[string]string{"tx_hash": "required"})
				}
			}
			if !canTransitionCurrencyWithdrawal(m.Status, v) {
				return errx.New(codes.FailedPrecondition, 409, errx.CodeConflict, "INVALID_TRANSITION", "invalid status transition", map[string]string{"from": m.Status, "to": v})
			}
			statusTo = v
			fields["status"] = v
			changes["status"] = v
		}

		if in.TxHash != nil {
			strategy := strings.ToLower(strings.TrimSpace(m.Strategy))
			if strategy != "manual_manual" {
				return errx.New(codes.FailedPrecondition, 409, errx.CodeConflict, "TX_HASH_NOT_ALLOWED", "tx_hash can only be set for manual_manual withdrawals", map[string]string{"strategy": strings.TrimSpace(m.Strategy)})
			}
			if strings.TrimSpace(m.Status) != "processing" {
				return errx.New(codes.FailedPrecondition, 409, errx.CodeConflict, "NOT_PROCESSING", "tx_hash can only be set when withdrawal is processing", map[string]string{"status": strings.TrimSpace(m.Status)})
			}
			v := strings.TrimSpace(in.TxHash.Value)
			txHashNew = v
			if v == "" {
				fields["tx_hash"] = nil
				changes["tx_hash"] = ""
			} else {
				if l.svcCtx.ChainRpc == nil {
					return errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "chain rpc not configured", nil)
				}
				st, stErr := l.svcCtx.ChainRpc.GetTransactionStatus(l.ctx, &pb.GetTransactionStatusReq{
					Chain:  mapChainType(m.ChainCode),
					TxHash: v,
				})
				if stErr != nil || st == nil || !st.Success {
					return errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_TX_HASH", "invalid tx_hash", map[string]string{"tx_hash": "not found"})
				}
				fields["tx_hash"] = v
				changes["tx_hash"] = maskAddress(v)
				fields["transfer_admin_id"] = current.ID
				fields["error_message"] = nil
			}
		}

		if txHashNew != "" {
			events = append(events, newWithdrawOrderEvent(m.ID, constants.WithdrawEventTransferInitiated, constants.WithdrawActorAdmin, current.ID, ip, "Transfer initiated", map[string]any{
				"id":       m.ID,
				"tx_hash":  txHashNew,
				"strategy": strings.TrimSpace(m.Strategy),
				"status":   strings.TrimSpace(m.Status),
				"admin_id": current.ID,
			}))
		}

		if in.TransferNote != nil {
			fields["transfer_note"] = strPtrOrNilTrim(in.TransferNote.Value)
			changes["transfer_note"] = strings.TrimSpace(in.TransferNote.Value)
		}

		if len(changes) == 0 {
			return errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "no fields to update", map[string]string{"fields": "empty"})
		}

		if statusFrom != statusTo {
			bizRef := strconv.FormatInt(m.ID, 10)
			// 内扣模式：只冻结/解冻 amount（手续费从提现金额中扣除）
			// 截断到 6 位小数以匹配 Accounting 服务的精度限制
			freezeUnfreezeAmount := strings.TrimSpace(m.Amount)
			if amountDec, parseErr := decimal.NewFromString(freezeUnfreezeAmount); parseErr == nil {
				freezeUnfreezeAmount = amountDec.Truncate(6).String()
			}
			switch {
			case statusFrom == "pending" && statusTo == "processing":
				freezeReq := &pb.FreezeWithdrawRequest{
					IdempotencyKey:   fmt.Sprintf("withdraw:freeze:%d", m.ID),
					BizRef:           bizRef,
					UserId:           m.UserID,
					AssetCode:        m.AssetCode,
					AmountDecimal:    freezeUnfreezeAmount, // 内扣模式：只冻结 amount
					ChainCode:        strings.TrimSpace(m.ChainCode),
					ToAddress:        strings.TrimSpace(m.ToAddress),
					PrincipalDecimal: strings.TrimSpace(m.Amount),
					FeeDecimal:       strings.TrimSpace(m.Fee),
					Memo: func() string {
						if m.MemoTag == nil {
							return ""
						}
						return strings.TrimSpace(*m.MemoTag)
					}(),
				}
				freezeResp, freezeErr := callAccountingTxWithRetry(l.ctx, 3, func(ctx context.Context) (*pb.LedgerTxResponse, error) {
					return l.svcCtx.AccountingRpc.FreezeWithdraw(ctx, freezeReq)
				})
				if err := errFromAccountingTx("FREEZE_WITHDRAW", freezeResp, freezeErr); err != nil {
					return err
				}
				events = append(events,
					newWithdrawOrderEvent(m.ID, constants.WithdrawEventAssetFrozen, constants.WithdrawActorSystem, 0, "", "Asset frozen for withdrawal", map[string]any{
						"tx_id":           freezeResp.TxId,
						"duplicate":       freezeResp.Duplicate,
						"asset_code":      strings.TrimSpace(m.AssetCode),
						"amount_decimal":  freezeUnfreezeAmount, // 内扣模式
						"idempotency_key": freezeReq.IdempotencyKey,
						"biz_ref":         strings.TrimSpace(freezeReq.BizRef),
					}),
					newWithdrawOrderEvent(m.ID, constants.WithdrawEventAuditApproved, constants.WithdrawActorAdmin, current.ID, ip, "Withdrawal approved", map[string]any{
						"id":          m.ID,
						"user_id":     m.UserID,
						"asset_code":  strings.TrimSpace(m.AssetCode),
						"chain_code":  strings.TrimSpace(m.ChainCode),
						"amount":      strings.TrimSpace(m.Amount),
						"fee":         strings.TrimSpace(m.Fee),
						"strategy":    strings.TrimSpace(m.Strategy),
						"status_from": strings.TrimSpace(m.Status),
						"status_to":   "processing",
						"via":         "UpdateCurrencyWithdrawal",
						"admin_id":    current.ID,
					}),
				)

				taskMode := constants.WithdrawPayoutModeSystem
				taskState := constants.WithdrawPayoutStatePendingBroadcast
				nextRetry := now
				if strings.EqualFold(strings.TrimSpace(m.Strategy), "manual_manual") {
					taskMode = constants.WithdrawPayoutModeManual
					taskState = constants.WithdrawPayoutStateAwaitingManualTxHash
					nextRetry = now.Add(1 * time.Hour)
				}
				if err := l.svcCtx.CurrencyWithdrawPayoutTaskRepo.WithTx(tx).CreateIfNotExists(l.ctx, &model.CurrencyWithdrawPayoutTaskModel{
					WithdrawOrderID:      m.ID,
					Mode:                 taskMode,
					State:                taskState,
					MaxBroadcastAttempts: 3,
					NextRetryTime:        nextRetry,
				}); err != nil {
					return err
				}
			case statusFrom == "pending" && (statusTo == "failed" || statusTo == "cancelled" || statusTo == "rejected"):
				// Check if asset was frozen before attempting to unfreeze
				eventRepo := l.svcCtx.CurrencyWithdrawOrderEventRepo.WithTx(tx)
				frozenEvents, checkErr := eventRepo.FindByOrderAndEventType(l.ctx, m.ID, constants.WithdrawEventAssetFrozen)
				if checkErr != nil {
					l.Logger.Errorf("failed to check frozen events: %v", checkErr)
					// Continue anyway, attempt unfreeze
				}

				// Only unfreeze if asset was frozen
				if len(frozenEvents) > 0 {
					unfreezeReq := &pb.UnfreezeWithdrawRequest{
						IdempotencyKey: fmt.Sprintf("withdraw:unfreeze:%d", m.ID),
						BizRef:         bizRef,
						UserId:         m.UserID,
						AssetCode:      m.AssetCode,
						AmountDecimal:  freezeUnfreezeAmount, // 内扣模式：解冻 amount
						FinalStatus:    statusTo,
						TxHash: func() string {
							if txHashNew != "" {
								return txHashNew
							}
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
						"amount_decimal":  freezeUnfreezeAmount, // 内扣模式
						"idempotency_key": unfreezeReq.IdempotencyKey,
						"biz_ref":         strings.TrimSpace(unfreezeReq.BizRef),
					}))
				} else {
					l.Logger.Infof("No frozen event found for withdrawal %d, skipping unfreeze", m.ID)
				}
				if statusTo == "rejected" {
					events = append(events, newWithdrawOrderEvent(m.ID, constants.WithdrawEventAuditRejected, constants.WithdrawActorAdmin, current.ID, ip, "Withdrawal rejected", map[string]any{
						"id":          m.ID,
						"user_id":     m.UserID,
						"asset_code":  strings.TrimSpace(m.AssetCode),
						"chain_code":  strings.TrimSpace(m.ChainCode),
						"amount":      strings.TrimSpace(m.Amount),
						"fee":         strings.TrimSpace(m.Fee),
						"strategy":    strings.TrimSpace(m.Strategy),
						"status_from": strings.TrimSpace(m.Status),
						"status_to":   "rejected",
						"via":         "UpdateCurrencyWithdrawal",
						"admin_id":    current.ID,
					}))
				} else if statusTo == "cancelled" {
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
						"via":         "UpdateCurrencyWithdrawal",
						"admin_id":    current.ID,
					}))
				} else if statusTo == "failed" {
					// 系统/链上失败（非管理员拒绝）
					events = append(events, newWithdrawOrderEvent(m.ID, constants.WithdrawEventTransferFailed, constants.WithdrawActorAdmin, current.ID, ip, "Withdrawal failed", map[string]any{
						"id":          m.ID,
						"user_id":     m.UserID,
						"asset_code":  strings.TrimSpace(m.AssetCode),
						"chain_code":  strings.TrimSpace(m.ChainCode),
						"amount":      strings.TrimSpace(m.Amount),
						"fee":         strings.TrimSpace(m.Fee),
						"strategy":    strings.TrimSpace(m.Strategy),
						"status_from": strings.TrimSpace(m.Status),
						"status_to":   "failed",
						"via":         "UpdateCurrencyWithdrawal",
						"admin_id":    current.ID,
					}))
				}
				_ = l.svcCtx.CurrencyWithdrawPayoutTaskRepo.WithTx(tx).UpdateFields(l.ctx, m.ID, map[string]interface{}{
					"state":           constants.WithdrawPayoutStateFailed,
					"next_retry_time": now.Add(24 * time.Hour),
				})
			case statusFrom == "processing" && statusTo == "failed":
				txHashForEvent := txHashNew
				if txHashForEvent == "" && m.TxHash != nil {
					txHashForEvent = strings.TrimSpace(*m.TxHash)
				}

				// Check if asset was frozen before attempting to unfreeze
				eventRepo := l.svcCtx.CurrencyWithdrawOrderEventRepo.WithTx(tx)
				frozenEvents, checkErr := eventRepo.FindByOrderAndEventType(l.ctx, m.ID, constants.WithdrawEventAssetFrozen)
				if checkErr != nil {
					l.Logger.Errorf("failed to check frozen events: %v", checkErr)
					// Continue anyway, attempt unfreeze
				}

				// Record transfer failed event first
				events = append(events, newWithdrawOrderEvent(m.ID, constants.WithdrawEventTransferFailed, constants.WithdrawActorAdmin, current.ID, ip, "Transfer failed", map[string]any{
					"id":       m.ID,
					"tx_hash":  txHashForEvent,
					"strategy": strings.TrimSpace(m.Strategy),
					"status":   "failed",
					"admin_id": current.ID,
				}))

				// Only unfreeze if asset was frozen
				if len(frozenEvents) > 0 {
					unfreezeReq := &pb.UnfreezeWithdrawRequest{
						IdempotencyKey: fmt.Sprintf("withdraw:unfreeze:%d", m.ID),
						BizRef:         bizRef,
						UserId:         m.UserID,
						AssetCode:      m.AssetCode,
						AmountDecimal:  freezeUnfreezeAmount, // 内扣模式：解冻 amount
						FinalStatus:    "failed",
						TxHash: func() string {
							if txHashNew != "" {
								return txHashNew
							}
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
						"amount_decimal":  freezeUnfreezeAmount, // 内扣模式
						"idempotency_key": unfreezeReq.IdempotencyKey,
						"biz_ref":         strings.TrimSpace(unfreezeReq.BizRef),
					}))
				} else {
					l.Logger.Infof("No frozen event found for withdrawal %d, skipping unfreeze", m.ID)
				}
				_ = l.svcCtx.CurrencyWithdrawPayoutTaskRepo.WithTx(tx).UpdateFields(l.ctx, m.ID, map[string]interface{}{
					"state":           constants.WithdrawPayoutStateFailed,
					"next_retry_time": now.Add(24 * time.Hour),
				})
			case statusFrom == "processing" && statusTo == "completed":
				// Manual completion: settle the withdrawal (deduct assets from frozen)
				if l.svcCtx.AccountingRpc == nil {
					return errx.New(codes.Internal, 500, errx.CodeInternalError, "ACCOUNTING_NOT_CONFIGURED", "accounting rpc not configured", nil)
				}

				// 计算实际到账金额（内扣模式：手续费从提现金额中扣除）
				amountDec, _ := decimal.NewFromString(strings.TrimSpace(m.Amount))
				feeDec, _ := decimal.NewFromString(strings.TrimSpace(m.Fee))
				actualAmountDec := amountDec.Sub(feeDec).Truncate(6) // 实际到账 = amount - fee
				feeDecTrunc := feeDec.Truncate(6)

				settleReq := &pb.SettleWithdrawRequest{
					IdempotencyKey:     fmt.Sprintf("withdraw:settle:%d", m.ID),
					BizRef:             bizRef,
					UserId:             m.UserID,
					AssetCode:          m.AssetCode,
					ChainCode:          strings.TrimSpace(m.ChainCode),
					AmountDecimal:      actualAmountDec.String(), // 内扣模式：实际到账金额
					FeeDecimal:         feeDecTrunc.String(),
					TxHash:             txHashNew,
					FromAddress:        strings.TrimSpace(m.FromAddress),
					ToAddress:          strings.TrimSpace(m.ToAddress),
					SkipPrecisionCheck: true, // Skip precision check for admin manual completion
				}
				settleResp, settleErr := callAccountingTxWithRetry(l.ctx, 3, func(ctx context.Context) (*pb.LedgerTxResponse, error) {
					return l.svcCtx.AccountingRpc.SettleWithdraw(ctx, settleReq)
				})
				if err := errFromAccountingTx("SETTLE_WITHDRAW", settleResp, settleErr); err != nil {
					return err
				}

				// Record completion events
				events = append(events,
					newWithdrawOrderEvent(m.ID, constants.WithdrawEventTransferCompleted, constants.WithdrawActorAdmin, current.ID, ip, "Transfer completed (manual)", map[string]any{
						"id":       m.ID,
						"tx_hash":  txHashNew,
						"strategy": strings.TrimSpace(m.Strategy),
						"admin_id": current.ID,
					}),
					newWithdrawOrderEvent(m.ID, constants.WithdrawEventAssetDeducted, constants.WithdrawActorSystem, 0, "", "Asset deducted", map[string]any{
						"tx_id":           settleResp.TxId,
						"duplicate":       settleResp.Duplicate,
						"asset_code":      strings.TrimSpace(m.AssetCode),
						"amount_decimal":  actualAmountDec.String(),
						"idempotency_key": settleReq.IdempotencyKey,
						"biz_ref":         strings.TrimSpace(settleReq.BizRef),
					}),
					newWithdrawOrderEvent(m.ID, constants.WithdrawEventFeeCollected, constants.WithdrawActorSystem, 0, "", "Fee collected", map[string]any{
						"tx_id":           settleResp.TxId,
						"duplicate":       settleResp.Duplicate,
						"asset_code":      strings.TrimSpace(m.AssetCode),
						"fee_decimal":     feeDecTrunc.String(),
						"biz_ref":         strings.TrimSpace(settleReq.BizRef),
						"idempotency_key": settleReq.IdempotencyKey,
					}),
				)

				// Update additional fields for completed status
				fields["transferred_at"] = &now
				fields["transfer_admin_id"] = current.ID
				fields["error_message"] = nil

				// Update payout task to done
				_ = l.svcCtx.CurrencyWithdrawPayoutTaskRepo.WithTx(tx).UpdateFields(l.ctx, m.ID, map[string]interface{}{
					"state":           constants.WithdrawPayoutStateDone,
					"last_error":      nil,
					"next_retry_time": now.Add(24 * time.Hour),
				})
			}
		}

		if err := tx.WithContext(l.ctx).
			Model(&model.CurrencyWithdrawOrderModel{}).
			Where("id = ?", in.Id).
			Updates(fields).Error; err != nil {
			return err
		}

		// If admin provided a manual tx_hash (manual_manual), move payout task into confirming state.
		if in.TxHash != nil {
			if txHashNew == "" {
				_ = l.svcCtx.CurrencyWithdrawPayoutTaskRepo.WithTx(tx).UpdateFields(l.ctx, m.ID, map[string]interface{}{
					"mode":            constants.WithdrawPayoutModeManual,
					"state":           constants.WithdrawPayoutStateAwaitingManualTxHash,
					"tx_hash":         nil,
					"next_retry_time": now.Add(1 * time.Hour),
				})
			} else {
				if err := l.svcCtx.CurrencyWithdrawPayoutTaskRepo.WithTx(tx).CreateIfNotExists(l.ctx, &model.CurrencyWithdrawPayoutTaskModel{
					WithdrawOrderID:      m.ID,
					Mode:                 constants.WithdrawPayoutModeManual,
					State:                constants.WithdrawPayoutStateConfirming,
					TxHash:               &txHashNew,
					MaxBroadcastAttempts: 0,
					NextRetryTime:        now,
				}); err != nil {
					return err
				}
				if err := l.svcCtx.CurrencyWithdrawPayoutTaskRepo.WithTx(tx).UpdateFields(l.ctx, m.ID, map[string]interface{}{
					"mode":            constants.WithdrawPayoutModeManual,
					"state":           constants.WithdrawPayoutStateConfirming,
					"tx_hash":         txHashNew,
					"last_error":      nil,
					"next_retry_time": now,
				}); err != nil {
					return err
				}
			}
		}

		if in.Status != nil || in.TxHash != nil {
			statusForMeta := ""
			if in.Status != nil {
				statusForMeta = mapCurrencyWithdrawOrderStatusToTxStatus(strings.TrimSpace(in.Status.Value))
			}
			txHashForMeta := ""
			if in.TxHash != nil {
				txHashForMeta = strings.TrimSpace(in.TxHash.Value)
			}
			if statusForMeta != "" || txHashForMeta != "" {
				req := &pb.UpsertUserTransactionRecordMetaRequest{
					Id:     in.Id,
					TxType: 2,
				}
				if statusForMeta != "" {
					req.Status = statusForMeta
				}
				if txHashForMeta != "" {
					req.TxHash = txHashForMeta
				}
				if v := strings.TrimSpace(m.ToAddress); v != "" {
					req.ToAddress = v
				}
				metaReq = req
			}
		}

		// If no status transition events were created but fields were updated, record an order.updated event
		if len(events) == 0 && len(changes) > 0 {
			events = append(events, newWithdrawOrderEvent(m.ID, constants.WithdrawEventOrderUpdated, constants.WithdrawActorAdmin, current.ID, ip, "Withdrawal order updated", changes))
		}

		if len(events) > 0 {
			if err := l.svcCtx.CurrencyWithdrawOrderEventRepo.WithTx(tx).CreateMultiple(l.ctx, events); err != nil {
				return err
			}
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
			"id":      in.Id,
			"changes": changes,
		})
		_ = auditRepo.CreateLog(l.ctx, &model.AdminAuditLogModel{
			AdminID:     current.ID,
			Action:      "currency.withdrawal.update",
			TargetType:  "currency_withdraw_order",
			TargetID:    "",
			Description: "更新提现记录",
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
		l.Logger.Errorf("update currency withdrawal failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "update withdrawal failed", nil)
	}

	if metaReq != nil && l.svcCtx.AccountingRpc != nil {
		metaResp, metaErr := l.svcCtx.AccountingRpc.UpsertUserTransactionRecordMeta(l.ctx, metaReq)
		if metaErr != nil || metaResp == nil || !metaResp.Success {
			l.Logger.Errorf("UpsertUserTransactionRecordMeta failed: err=%v resp=%v", metaErr, metaResp)
		}
	}

	return &pb.UpdateCurrencyWithdrawalResponse{
		Success: true,
		Message: "ok",
		Data: &pb.UpdateCurrencyWithdrawalData{
			Withdrawal: toPBCurrencyWithdrawalItem(updated),
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
