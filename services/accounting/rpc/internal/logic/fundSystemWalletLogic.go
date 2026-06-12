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

type FundSystemWalletLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewFundSystemWalletLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FundSystemWalletLogic {
	return &FundSystemWalletLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *FundSystemWalletLogic) FundSystemWallet(in *pb.FundSystemWalletRequest) (*pb.LedgerTxResponse, error) {
	if err := requireDB(l.svcCtx); err != nil {
		return &pb.LedgerTxResponse{Success: false, Message: err.Error()}, nil
	}
	if l.svcCtx.AssetRepo == nil || l.svcCtx.AccountRepo == nil {
		return &pb.LedgerTxResponse{Success: false, Message: "repository not initialized"}, nil
	}
	if in == nil || strings.TrimSpace(in.IdempotencyKey) == "" ||
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
	if err := e.EnsureSystemAccounts(l.ctx, chainCode); err != nil {
		l.Logger.Errorf("EnsureSystemAccounts failed: %v", err)
		return &pb.LedgerTxResponse{Success: false, Message: err.Error()}, nil
	}

	hotAcc, err := l.svcCtx.AccountRepo.FindByOwnerType(l.ctx, engine.OwnerTypeSystem, 0, engine.AccountTypeSysWalletHot, chainCode)
	if err != nil || hotAcc == nil {
		return &pb.LedgerTxResponse{Success: false, Message: "system hot wallet not found"}, nil
	}
	capitalAcc, err := l.svcCtx.AccountRepo.FindByOwnerType(l.ctx, engine.OwnerTypeSystem, 0, engine.AccountTypeSysCapital, "")
	if err != nil || capitalAcc == nil {
		return &pb.LedgerTxResponse{Success: false, Message: "system capital account not found"}, nil
	}

	res, err := e.PostTx(l.ctx, engine.PostTxRequest{
		IdempotencyKey: in.IdempotencyKey,
		OpType:         "FundSystemWallet",
		BizRef:         normalizeBizRef(in.BizRef),
		Postings: []engine.Posting{
			{AssetCode: assetCode, AccountID: hotAcc.ID, Bucket: engine.BalanceBucketAvailable, DebitRaw: amountRaw, CreditRaw: decimal.Zero},
			{AssetCode: assetCode, AccountID: capitalAcc.ID, Bucket: engine.BalanceBucketAvailable, DebitRaw: decimal.Zero, CreditRaw: amountRaw},
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
