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

type SettleWithdrawLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSettleWithdrawLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SettleWithdrawLogic {
	return &SettleWithdrawLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SettleWithdrawLogic) SettleWithdraw(in *pb.SettleWithdrawRequest) (*pb.LedgerTxResponse, error) {
	if err := requireDB(l.svcCtx); err != nil {
		return &pb.LedgerTxResponse{Success: false, Message: err.Error()}, nil
	}
	if l.svcCtx.AssetRepo == nil || l.svcCtx.AccountRepo == nil {
		return &pb.LedgerTxResponse{Success: false, Message: "repository not initialized"}, nil
	}
	if l.svcCtx.UserTxRepo == nil {
		return &pb.LedgerTxResponse{Success: false, Message: "repository not initialized"}, nil
	}
	if in == nil || strings.TrimSpace(in.IdempotencyKey) == "" || in.UserId <= 0 ||
		strings.TrimSpace(in.AssetCode) == "" || strings.TrimSpace(in.ChainCode) == "" ||
		strings.TrimSpace(in.AmountDecimal) == "" || strings.TrimSpace(in.FeeDecimal) == "" {
		return &pb.LedgerTxResponse{Success: false, Message: "invalid params"}, nil
	}

	assetCode := accounting.NormalizeAssetCode(in.AssetCode)
	chainCode := accounting.NormalizeChainCode(in.ChainCode)
	asset, err := l.svcCtx.AssetRepo.FindByCode(l.ctx, assetCode)
	if err != nil || asset == nil || asset.Status != 1 {
		return &pb.LedgerTxResponse{Success: false, Message: "asset not available"}, nil
	}

	var amountRaw, feeRaw decimal.Decimal

	// If skip_precision_check is true, use relaxed parsing (truncate excess precision)
	// This is for admin manual completion with historical data that may exceed asset precision
	if in.SkipPrecisionCheck {
		amountRaw, err = parseDecimalToRawRelaxed(in.AmountDecimal, asset.Precision, false)
		if err != nil {
			return &pb.LedgerTxResponse{Success: false, Message: err.Error()}, nil
		}
		feeRaw, err = parseDecimalToRawRelaxed(in.FeeDecimal, asset.Precision, true)
		if err != nil {
			return &pb.LedgerTxResponse{Success: false, Message: err.Error()}, nil
		}
	} else {
		// Strict precision check (existing logic)
		amountRaw, err = accounting.ParseDecimalToRawExact(in.AmountDecimal, asset.Precision, false)
		if err != nil {
			return &pb.LedgerTxResponse{Success: false, Message: err.Error()}, nil
		}
		feeRaw, err = accounting.ParseDecimalToRawExact(in.FeeDecimal, asset.Precision, true)
		if err != nil {
			return &pb.LedgerTxResponse{Success: false, Message: err.Error()}, nil
		}
	}

	totalRaw := amountRaw.Add(feeRaw)
	if err := accounting.ValidateDecimal65Int(totalRaw); err != nil || totalRaw.LessThanOrEqual(decimal.Zero) {
		return &pb.LedgerTxResponse{Success: false, Message: "invalid amount"}, nil
	}

	e := engine.New(l.svcCtx.DB)
	if err := e.EnsureUserAccountingSetup(l.ctx, in.UserId); err != nil {
		l.Logger.Errorf("EnsureUserAccountingSetup failed: %v", err)
		return &pb.LedgerTxResponse{Success: false, Message: err.Error()}, nil
	}
	if err := e.EnsureSystemAccounts(l.ctx, chainCode); err != nil {
		l.Logger.Errorf("EnsureSystemAccounts failed: %v", err)
		return &pb.LedgerTxResponse{Success: false, Message: err.Error()}, nil
	}

	userAcc, err := l.svcCtx.AccountRepo.FindByOwnerType(l.ctx, engine.OwnerTypeUser, in.UserId, engine.AccountTypeUserLiability, "")
	if err != nil || userAcc == nil {
		return &pb.LedgerTxResponse{Success: false, Message: "user account not found"}, nil
	}
	hotAcc, err := l.svcCtx.AccountRepo.FindByOwnerType(l.ctx, engine.OwnerTypeSystem, 0, engine.AccountTypeSysWalletHot, chainCode)
	if err != nil || hotAcc == nil {
		return &pb.LedgerTxResponse{Success: false, Message: "system hot wallet not found"}, nil
	}
	feeAcc, err := l.svcCtx.AccountRepo.FindByOwnerType(l.ctx, engine.OwnerTypeSystem, 0, engine.AccountTypeSysFeeIncome, "")
	if err != nil || feeAcc == nil {
		return &pb.LedgerTxResponse{Success: false, Message: "fee income account not found"}, nil
	}

	postings := []engine.Posting{
		{AssetCode: assetCode, AccountID: userAcc.ID, Bucket: engine.BalanceBucketLocked, DebitRaw: totalRaw, CreditRaw: decimal.Zero},
		{AssetCode: assetCode, AccountID: hotAcc.ID, Bucket: engine.BalanceBucketAvailable, DebitRaw: decimal.Zero, CreditRaw: amountRaw},
	}
	if feeRaw.GreaterThan(decimal.Zero) {
		postings = append(postings, engine.Posting{AssetCode: assetCode, AccountID: feeAcc.ID, Bucket: engine.BalanceBucketAvailable, DebitRaw: decimal.Zero, CreditRaw: feeRaw})
	}

	res, err := e.PostTx(l.ctx, engine.PostTxRequest{
		IdempotencyKey: in.IdempotencyKey,
		OpType:         "SettleWithdraw",
		BizRef:         normalizeBizRef(in.BizRef),
		Postings:       postings,
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

	// Best-effort update user-facing withdrawal record.
	withdrawID, _ := strconv.ParseInt(strings.TrimSpace(in.BizRef), 10, 64)
	if withdrawID > 0 {
		fee := strings.TrimSpace(in.FeeDecimal)
		if fee == "" {
			fee = "0"
		}
		rec := &model.AcctUserTransactionRecordModel{
			UserID:        in.UserId,
			TxType:        2,
			AssetCode:     assetCode,
			ChainCode:     chainCode,
			AmountDecimal: strings.TrimSpace(in.AmountDecimal),
			FeeDecimal:    fee,
			Status:        "success",
			Memo:          strings.TrimSpace(in.Memo),
			FromAddress:   strings.TrimSpace(in.FromAddress),
			ToAddress:     strings.TrimSpace(in.ToAddress),
			TxHash:        strings.TrimSpace(in.TxHash),
			SettleLedgerTxID: func() *int64 {
				v := res.TxID
				return &v
			}(),
			BizRef:         normalizeBizRef(in.BizRef),
			IdempotencyKey: strings.TrimSpace(in.IdempotencyKey),
		}
		rec.ID = withdrawID
		if err := l.svcCtx.UserTxRepo.Upsert(l.ctx, rec); err != nil {
			l.Logger.Errorf("Upsert user withdraw settle record failed: %v", err)
		}
	}

	return &pb.LedgerTxResponse{Success: true, Message: "ok", TxId: res.TxID, Duplicate: res.Duplicate}, nil
}
