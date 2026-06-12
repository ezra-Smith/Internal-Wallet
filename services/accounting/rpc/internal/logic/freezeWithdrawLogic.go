package logic

import (
	"context"
	"strconv"
	"strings"

	"internalwallet/pkg/accounting"
	"internalwallet/proto/pb"
	"internalwallet/services/accounting/rpc/internal/engine"
	"internalwallet/services/accounting/rpc/internal/model"
	"internalwallet/services/accounting/rpc/internal/svc"

	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/logx"
)

type FreezeWithdrawLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewFreezeWithdrawLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FreezeWithdrawLogic {
	return &FreezeWithdrawLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ==================== Fixed double-entry templates ====================
func (l *FreezeWithdrawLogic) FreezeWithdraw(in *pb.FreezeWithdrawRequest) (*pb.LedgerTxResponse, error) {
	if err := requireDB(l.svcCtx); err != nil {
		return &pb.LedgerTxResponse{Success: false, Message: err.Error()}, nil
	}
	if l.svcCtx.AssetRepo == nil || l.svcCtx.AccountRepo == nil {
		return &pb.LedgerTxResponse{Success: false, Message: "repository not initialized"}, nil
	}
	if l.svcCtx.UserTxRepo == nil {
		return &pb.LedgerTxResponse{Success: false, Message: "repository not initialized"}, nil
	}
	if in == nil || strings.TrimSpace(in.IdempotencyKey) == "" || in.UserId <= 0 || strings.TrimSpace(in.AssetCode) == "" || strings.TrimSpace(in.AmountDecimal) == "" {
		return &pb.LedgerTxResponse{Success: false, Message: "invalid params"}, nil
	}

	assetCode := accounting.NormalizeAssetCode(in.AssetCode)
	asset, err := l.svcCtx.AssetRepo.FindByCode(l.ctx, assetCode)
	if err != nil || asset == nil || asset.Status != 1 {
		return &pb.LedgerTxResponse{Success: false, Message: "asset not available"}, nil
	}
	raw, err := accounting.ParseDecimalToRawExact(in.AmountDecimal, asset.Precision, false)
	if err != nil {
		return &pb.LedgerTxResponse{Success: false, Message: err.Error()}, nil
	}

	e := engine.New(l.svcCtx.DB)
	if err := e.EnsureUserAccountingSetup(l.ctx, in.UserId); err != nil {
		l.Logger.Errorf("EnsureUserAccountingSetup failed: %v", err)
		return &pb.LedgerTxResponse{Success: false, Message: err.Error()}, nil
	}
	userAcc, err := l.svcCtx.AccountRepo.FindByOwnerType(l.ctx, engine.OwnerTypeUser, in.UserId, engine.AccountTypeUserLiability, "")
	if err != nil || userAcc == nil {
		return &pb.LedgerTxResponse{Success: false, Message: "user account not found"}, nil
	}

	res, err := e.PostTx(l.ctx, engine.PostTxRequest{
		IdempotencyKey: in.IdempotencyKey,
		OpType:         "FreezeWithdraw",
		BizRef:         normalizeBizRef(in.BizRef),
		Postings: []engine.Posting{
			{AssetCode: assetCode, AccountID: userAcc.ID, Bucket: engine.BalanceBucketAvailable, DebitRaw: raw, CreditRaw: decimal.Zero},
			{AssetCode: assetCode, AccountID: userAcc.ID, Bucket: engine.BalanceBucketLocked, DebitRaw: decimal.Zero, CreditRaw: raw},
		},
	})
	if err != nil {
		msg := err.Error()
		switch err {
		case engine.ErrInsufficientFunds:
			msg = "insufficient funds"
		case engine.ErrIdempotencyConflict:
			msg = "idempotency conflict"
		}
		return &pb.LedgerTxResponse{Success: false, Message: msg}, nil
	}

	// Best-effort upsert user-facing withdrawal record.
	withdrawID, _ := strconv.ParseInt(strings.TrimSpace(in.BizRef), 10, 64)
	if withdrawID > 0 {
		chainCode := accounting.NormalizeChainCode(in.ChainCode)
		principal := strings.TrimSpace(in.PrincipalDecimal)
		if principal == "" {
			principal = strings.TrimSpace(in.AmountDecimal)
		}
		fee := strings.TrimSpace(in.FeeDecimal)
		if fee == "" {
			fee = "0"
		}
		// 内扣模式：实际转账金额 = principal - fee
		// AmountDecimal 应该存储实际转账金额，而不是用户输入的金额
		actualAmount := principal
		if fee != "0" {
			principalDec, err1 := decimal.NewFromString(principal)
			feeDec, err2 := decimal.NewFromString(fee)
			if err1 == nil && err2 == nil && principalDec.GreaterThan(feeDec) {
				actualAmountDec := principalDec.Sub(feeDec)
				actualAmount = actualAmountDec.String()
			}
		}
		rec := &model.AcctUserTransactionRecordModel{
			UserID:        in.UserId,
			TxType:        2,
			AssetCode:     assetCode,
			ChainCode:     chainCode,
			AmountDecimal: actualAmount, // 实际转账金额（principal - fee）
			FeeDecimal:    fee,
			Status:        "pending",
			Memo:          strings.TrimSpace(in.Memo),
			ToAddress:     strings.TrimSpace(in.ToAddress),
			FreezeLedgerTxID: func() *int64 {
				v := res.TxID
				return &v
			}(),
			BizRef:         normalizeBizRef(in.BizRef),
			IdempotencyKey: strings.TrimSpace(in.IdempotencyKey),
		}
		rec.ID = withdrawID
		if err := l.svcCtx.UserTxRepo.Upsert(l.ctx, rec); err != nil {
			l.Logger.Errorf("Upsert user withdraw record failed: %v", err)
		}
	}

	return &pb.LedgerTxResponse{Success: true, Message: "ok", TxId: res.TxID, Duplicate: res.Duplicate}, nil
}
