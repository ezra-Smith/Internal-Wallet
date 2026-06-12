package logic

import (
	"context"
	"strings"

	"internalwallet/pkg/accounting"
	"internalwallet/proto/pb"
	"internalwallet/services/accounting/rpc/internal/engine"
	"internalwallet/services/accounting/rpc/internal/svc"

	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/logx"
)

type UnfreezeUserAssetsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUnfreezeUserAssetsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UnfreezeUserAssetsLogic {
	return &UnfreezeUserAssetsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// UnfreezeUserAssets unfreezes user assets by moving funds from locked to available bucket.
// This is typically used when admin unfreezes a user account.
func (l *UnfreezeUserAssetsLogic) UnfreezeUserAssets(in *pb.UnfreezeUserAssetsRequest) (*pb.LedgerTxResponse, error) {
	if err := requireDB(l.svcCtx); err != nil {
		return &pb.LedgerTxResponse{Success: false, Message: err.Error()}, nil
	}
	if l.svcCtx.AssetRepo == nil || l.svcCtx.AccountRepo == nil {
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

	// Transfer from locked to available bucket (unfreeze assets).
	res, err := e.PostTx(l.ctx, engine.PostTxRequest{
		IdempotencyKey: in.IdempotencyKey,
		OpType:         "UnfreezeUserAssets",
		BizRef:         normalizeBizRef(in.BizRef),
		Postings: []engine.Posting{
			{AssetCode: assetCode, AccountID: userAcc.ID, Bucket: engine.BalanceBucketLocked, DebitRaw: raw, CreditRaw: decimal.Zero},
			{AssetCode: assetCode, AccountID: userAcc.ID, Bucket: engine.BalanceBucketAvailable, DebitRaw: decimal.Zero, CreditRaw: raw},
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

	return &pb.LedgerTxResponse{Success: true, Message: "ok", TxId: res.TxID, Duplicate: res.Duplicate}, nil
}
