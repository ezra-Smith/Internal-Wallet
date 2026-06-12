package logic

import (
	"context"
	"strings"

	"internalwallet/pkg/accounting"
	"internalwallet/proto/pb"
	"internalwallet/services/accounting/rpc/internal/engine"
	"internalwallet/services/accounting/rpc/internal/model"
	"internalwallet/services/accounting/rpc/internal/svc"

	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/logx"
)

type ConfirmDepositLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewConfirmDepositLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ConfirmDepositLogic {
	return &ConfirmDepositLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ConfirmDepositLogic) ConfirmDeposit(in *pb.ConfirmDepositRequest) (*pb.LedgerTxResponse, error) {
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
		strings.TrimSpace(in.AmountDecimal) == "" {
		return &pb.LedgerTxResponse{Success: false, Message: "invalid params"}, nil
	}

	assetCode := accounting.NormalizeAssetCode(in.AssetCode)
	chainCode := accounting.NormalizeChainCode(in.ChainCode)
	asset, err := l.svcCtx.AssetRepo.FindByCode(l.ctx, assetCode)
	if err != nil || asset == nil || asset.Status != 1 {
		return &pb.LedgerTxResponse{Success: false, Message: "asset not available"}, nil
	}
	amountRaw, err := accounting.ParseDecimalToRawExact(in.AmountDecimal, asset.Precision, false)
	if err != nil {
		return &pb.LedgerTxResponse{Success: false, Message: err.Error()}, nil
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

	res, err := e.PostTx(l.ctx, engine.PostTxRequest{
		IdempotencyKey: in.IdempotencyKey,
		OpType:         "ConfirmDeposit",
		BizRef:         normalizeBizRef(in.BizRef),
		Postings: []engine.Posting{
			{AssetCode: assetCode, AccountID: hotAcc.ID, Bucket: engine.BalanceBucketAvailable, DebitRaw: amountRaw, CreditRaw: decimal.Zero},
			{AssetCode: assetCode, AccountID: userAcc.ID, Bucket: engine.BalanceBucketAvailable, DebitRaw: decimal.Zero, CreditRaw: amountRaw},
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

	// Write user-facing transaction record (deposit) after ledger success.
	rec := &model.AcctUserTransactionRecordModel{
		UserID:           in.UserId,
		TxType:           1,
		AssetCode:        assetCode,
		ChainCode:        chainCode,
		AmountDecimal:    strings.TrimSpace(in.AmountDecimal),
		FeeDecimal:       "0",
		Status:           "completed",
		Memo:             strings.TrimSpace(in.Memo),
		FromAddress:      strings.TrimSpace(in.FromAddress),
		ToAddress:        strings.TrimSpace(in.ToAddress),
		TxHash:           strings.TrimSpace(in.TxHash),
		ConfirmLedgerTxID: func() *int64 {
			v := res.TxID
			return &v
		}(),
		BizRef:         normalizeBizRef(in.BizRef),
		IdempotencyKey: strings.TrimSpace(in.IdempotencyKey),
	}
	rec.ID = res.TxID
	if err := l.svcCtx.UserTxRepo.Upsert(l.ctx, rec); err != nil {
		// Best-effort: ledger is already committed; do not fail the financial write.
		l.Logger.Errorf("Upsert user deposit record failed: %v", err)
	}

	return &pb.LedgerTxResponse{Success: true, Message: "ok", TxId: res.TxID, Duplicate: res.Duplicate}, nil
}
