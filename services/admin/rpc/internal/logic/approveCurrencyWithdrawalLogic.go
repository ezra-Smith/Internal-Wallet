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
	"internalwallet/common/errcode"
	"internalwallet/common/middleware"
	"internalwallet/common/withdrawcalc"
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

type ApproveCurrencyWithdrawalLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewApproveCurrencyWithdrawalLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ApproveCurrencyWithdrawalLogic {
	return &ApproveCurrencyWithdrawalLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ApproveCurrencyWithdrawalLogic) ApproveCurrencyWithdrawal(in *pb.ApproveCurrencyWithdrawalRequest) (*pb.ApproveCurrencyWithdrawalResponse, error) {
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
	if err := verifyAdminTwoFA(l.svcCtx, l.ctx, current.ID, in.TwoFaCode); err != nil {
		return nil, err
	}

	now := time.Now()
	ip := middleware.GetClientIP(l.ctx)
	ua := middleware.GetUserAgent(l.ctx)
	note := strings.TrimSpace(in.AuditNote)

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
			return errx.New(codes.FailedPrecondition, 409, errx.CodeConflict, "NOT_PENDING", "only pending withdrawals can be approved", map[string]string{"status": m.Status})
		}
		strategy := strings.ToLower(strings.TrimSpace(m.Strategy))
		if strategy != "manual_auto" && strategy != "manual_manual" {
			return errx.New(codes.FailedPrecondition, 409, errx.CodeConflict, "NOT_MANUAL", "only manual withdrawals can be approved", map[string]string{"strategy": strings.TrimSpace(m.Strategy)})
		}

		// Enforce mandatory min_withdraw_amount config and validate amount semantics (inner deduct).
		if l.svcCtx.CurrencyChainSettingsRepo == nil {
			return errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "currency chain settings repo not configured", nil)
		}
		cs, csErr := l.svcCtx.CurrencyChainSettingsRepo.WithTx(tx).FindByAssetAndChain(l.ctx, strings.TrimSpace(m.AssetCode), strings.TrimSpace(m.ChainCode))
		if csErr != nil || cs == nil || cs.MinWithdrawAmount == nil || strings.TrimSpace(*cs.MinWithdrawAmount) == "" {
			return errx.New(codes.FailedPrecondition, 200, errx.CodeInvalidParam, "MIN_WITHDRAW_AMOUNT_NOT_CONFIGURED", "min withdraw amount not configured", map[string]string{"min_withdraw_amount": "required"})
		}
		minWithdraw, minErr := decimal.NewFromString(strings.TrimSpace(*cs.MinWithdrawAmount))
		if minErr != nil || minWithdraw.LessThanOrEqual(decimal.Zero) {
			return errx.New(codes.FailedPrecondition, 200, errx.CodeInvalidParam, "MIN_WITHDRAW_AMOUNT_NOT_CONFIGURED", "min withdraw amount not configured", map[string]string{"min_withdraw_amount": "invalid"})
		}

		amountDec, aErr := decimal.NewFromString(strings.TrimSpace(m.Amount))
		if aErr != nil {
			return errx.New(codes.InvalidArgument, 200, errx.CodeInvalidParam, "INVALID_AMOUNT", "invalid amount", map[string]string{"amount": "invalid"})
		}
		feeDec, fErr := decimal.NewFromString(strings.TrimSpace(m.Fee))
		if fErr != nil {
			feeDec = decimal.Zero
		}
		amtT := withdrawcalc.TruncateAccounting(amountDec)
		feeT := withdrawcalc.TruncateAccounting(feeDec)
		minT := withdrawcalc.TruncateAccounting(minWithdraw)
		if v := withdrawcalc.ValidateGrossAmountForInnerDeduct(amtT, feeT, minT); v != nil {
			switch v.Code {
			case "AMOUNT_MUST_EXCEED_FEE":
				return errx.New(codes.InvalidArgument, 200, errx.CodeInvalidParam, "AMOUNT_MUST_EXCEED_FEE", "amount must exceed fee", map[string]string{"amount": "must exceed fee"})
			case "AMOUNT_BELOW_MIN_PLUS_FEE":
				return errx.NewWithMetadata(codes.InvalidArgument, 200, errx.CodeInvalidParam, "AMOUNT_BELOW_MIN_PLUS_FEE", "amount below minimum withdraw amount plus fee", map[string]string{"amount": "too small"}, v.Meta)
			case "MIN_WITHDRAW_AMOUNT_NOT_CONFIGURED":
				return errx.New(codes.FailedPrecondition, 200, errx.CodeInvalidParam, "MIN_WITHDRAW_AMOUNT_NOT_CONFIGURED", "min withdraw amount not configured", map[string]string{"min_withdraw_amount": "required"})
			default:
				return errx.New(codes.InvalidArgument, 200, errx.CodeInvalidParam, v.Code, v.Message, nil)
			}
		}

		// 内扣模式：只冻结 amount（手续费从提现金额中扣除）
		// 截断到 6 位小数以匹配 Accounting 服务的精度限制
		freezeAmount := amtT.String()
		// Ensure funds are frozen for this withdrawal (idempotent).
		freezeReq := &pb.FreezeWithdrawRequest{
			IdempotencyKey:   fmt.Sprintf("withdraw:freeze:%d", m.ID),
			BizRef:           strconv.FormatInt(m.ID, 10),
			UserId:           m.UserID,
			AssetCode:        m.AssetCode,
			AmountDecimal:    freezeAmount,
			ChainCode:        strings.TrimSpace(m.ChainCode),
			ToAddress:        strings.TrimSpace(m.ToAddress),
			PrincipalDecimal: freezeAmount,
			FeeDecimal:       feeT.String(),
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

		// 检查热钱包余额和 gas 是否足够
		checkLogic := NewCheckHotWalletBalanceLogic(l.ctx, l.svcCtx)
		checkResp, checkErr := checkLogic.CheckHotWalletBalance(&pb.CheckHotWalletBalanceRequest{
			AssetCode: m.AssetCode,
			ChainCode: m.ChainCode,
			Amount:    strings.TrimSpace(m.Amount),
		})
		if checkErr != nil || checkResp == nil || !checkResp.Success || checkResp.Code != 0 {
			errMsg := "hot wallet balance check failed"
			if checkResp != nil && checkResp.Message != "" {
				errMsg = checkResp.Message
			}
			l.Errorf("热钱包余额不足: order_id=%d, asset=%s, chain=%s, amount=%s, error=%v, resp=%+v",
				m.ID, m.AssetCode, m.ChainCode, m.Amount, checkErr, checkResp)
			return errx.New(codes.FailedPrecondition, 422, errcode.CodeInsufficientBalance, "HOT_WALLET_BALANCE_INSUFFICIENT",
				errMsg, nil)
		}

		// 对于 manual_auto 策略，需要获取热钱包地址作为 from_address
		// 因为创建订单时只有 auto 策略才会设置 from_address
		fromAddress := strings.TrimSpace(m.FromAddress)
		if fromAddress == "" && strings.EqualFold(strings.TrimSpace(m.Strategy), "manual_auto") {
			hotWalletAddr, hwErr := resolveCompanyHotWalletAddress(l.ctx, l.svcCtx, strings.TrimSpace(m.ChainCode))
			if hwErr != nil {
				l.Errorf("获取热钱包地址失败: order_id=%d, chain=%s, error=%v", m.ID, m.ChainCode, hwErr)
				return errx.New(codes.FailedPrecondition, 422, errcode.CodeInternalError, "HOT_WALLET_NOT_CONFIGURED",
					"热钱包未配置，无法完成自动放币", nil)
			}
			fromAddress = hotWalletAddr
			l.Infof("已获取热钱包地址: order_id=%d, chain=%s, from_address=%s", m.ID, m.ChainCode, fromAddress)
		}

		updateFields := map[string]interface{}{
			"status":         "processing",
			"audit_admin_id": current.ID,
			"audit_note":     strPtrOrNilTrim(note),
			"audited_at":     &now,
			"updated_by":     current.ID,
		}
		// 如果获取到了热钱包地址，更新 from_address
		if fromAddress != "" && strings.TrimSpace(m.FromAddress) == "" {
			updateFields["from_address"] = fromAddress
		}

		if err := tx.WithContext(l.ctx).
			Model(&model.CurrencyWithdrawOrderModel{}).
			Where("id = ?", in.Id).
			Updates(updateFields).Error; err != nil {
			return err
		}

		// Update accounting record status to processing
		if l.svcCtx.AccountingRpc != nil {
			_, _ = l.svcCtx.AccountingRpc.UpsertUserTransactionRecordMeta(l.ctx, &pb.UpsertUserTransactionRecordMetaRequest{
				Id:     m.ID,
				TxType: 2,
				Status: "processing",
			})
		}

		events := []*model.CurrencyWithdrawOrderEventModel{
			newWithdrawOrderEvent(m.ID, constants.WithdrawEventAssetFrozen, constants.WithdrawActorSystem, 0, "", "Asset frozen for withdrawal", map[string]any{
				"tx_id":           freezeResp.TxId,
				"duplicate":       freezeResp.Duplicate,
				"asset_code":      strings.TrimSpace(m.AssetCode),
				"amount_decimal":  freezeAmount, // 内扣模式
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
				"audit_note":  note,
				"admin_id":    current.ID,
			}),
		}
		if err := l.svcCtx.CurrencyWithdrawOrderEventRepo.WithTx(tx).CreateMultiple(l.ctx, events); err != nil {
			return err
		}

		// Enqueue payout task (system or manual) after approval; worker will handle broadcast/confirm/settle.
		taskState := constants.WithdrawPayoutStatePendingBroadcast
		taskMode := constants.WithdrawPayoutModeSystem
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
			"status_to":   "processing",
			"audit_note":  note,
		})
		_ = auditRepo.CreateLog(l.ctx, &model.AdminAuditLogModel{
			AdminID:     current.ID,
			Action:      "currency.withdrawal.approve",
			TargetType:  "currency_withdraw_order",
			TargetID:    "",
			Description: "审核通过提现",
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
		l.Logger.Errorf("approve currency withdrawal failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "approve withdrawal failed", nil)
	}

	return &pb.ApproveCurrencyWithdrawalResponse{
		Success: true,
		Message: "ok",
		Data: &pb.ApproveCurrencyWithdrawalData{
			Withdrawal: toPBCurrencyWithdrawalItem(updated),
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
