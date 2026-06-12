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

type TransferLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewTransferLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TransferLogic {
	return &TransferLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *TransferLogic) Transfer(in *pb.TransferRequest) (*pb.LedgerTxResponse, error) {
	if err := requireDB(l.svcCtx); err != nil {
		return &pb.LedgerTxResponse{Success: false, Message: err.Error()}, nil
	}
	if l.svcCtx.AssetRepo == nil || l.svcCtx.AccountRepo == nil {
		return &pb.LedgerTxResponse{Success: false, Message: "repository not initialized"}, nil
	}
	if in == nil || strings.TrimSpace(in.IdempotencyKey) == "" || in.FromUserId <= 0 || in.ToUserId <= 0 ||
		strings.TrimSpace(in.AssetCode) == "" || strings.TrimSpace(in.AmountDecimal) == "" {
		return &pb.LedgerTxResponse{Success: false, Message: "invalid params"}, nil
	}

	fromBucket := in.FromBucket
	toBucket := in.ToBucket
	if fromBucket == pb.BalanceBucket_BUCKET_UNSPECIFIED {
		fromBucket = pb.BalanceBucket_BUCKET_AVAILABLE
	}
	if toBucket == pb.BalanceBucket_BUCKET_UNSPECIFIED {
		toBucket = pb.BalanceBucket_BUCKET_AVAILABLE
	}
	fromBucketStr, err := bucketToString(fromBucket)
	if err != nil {
		return &pb.LedgerTxResponse{Success: false, Message: err.Error()}, nil
	}
	toBucketStr, err := bucketToString(toBucket)
	if err != nil {
		return &pb.LedgerTxResponse{Success: false, Message: err.Error()}, nil
	}
	if in.FromUserId == in.ToUserId && fromBucketStr == toBucketStr {
		return &pb.LedgerTxResponse{Success: false, Message: "no-op transfer"}, nil
	}

	assetCode := accounting.NormalizeAssetCode(in.AssetCode)
	asset, err := l.svcCtx.AssetRepo.FindByCode(l.ctx, assetCode)
	if err != nil || asset == nil || asset.Status != 1 {
		return &pb.LedgerTxResponse{Success: false, Message: "asset not available"}, nil
	}

	var amountRaw decimal.Decimal

	// If skip_precision_check is true, use relaxed parsing (truncate excess precision)
	// This is for admin approval with historical data that may exceed asset precision
	if in.SkipPrecisionCheck {
		amountRaw, err = parseDecimalToRawRelaxed(in.AmountDecimal, asset.Precision, false)
		if err != nil {
			return &pb.LedgerTxResponse{Success: false, Message: err.Error()}, nil
		}
	} else {
		// Strict precision check (existing logic)
		amountRaw, err = accounting.ParseDecimalToRawExact(in.AmountDecimal, asset.Precision, false)
		if err != nil {
			return &pb.LedgerTxResponse{Success: false, Message: err.Error()}, nil
		}
	}

	e := engine.New(l.svcCtx.DB)
	if err := e.EnsureUserAccountingSetup(l.ctx, in.FromUserId); err != nil {
		l.Logger.Errorf("EnsureUserAccountingSetup(from) failed: %v", err)
		return &pb.LedgerTxResponse{Success: false, Message: err.Error()}, nil
	}
	if in.ToUserId != in.FromUserId {
		if err := e.EnsureUserAccountingSetup(l.ctx, in.ToUserId); err != nil {
			l.Logger.Errorf("EnsureUserAccountingSetup(to) failed: %v", err)
			return &pb.LedgerTxResponse{Success: false, Message: err.Error()}, nil
		}
	}

	fromAcc, err := l.svcCtx.AccountRepo.FindByOwnerType(l.ctx, engine.OwnerTypeUser, in.FromUserId, engine.AccountTypeUserLiability, "")
	if err != nil || fromAcc == nil {
		return &pb.LedgerTxResponse{Success: false, Message: "from account not found"}, nil
	}
	toAcc, err := l.svcCtx.AccountRepo.FindByOwnerType(l.ctx, engine.OwnerTypeUser, in.ToUserId, engine.AccountTypeUserLiability, "")
	if err != nil || toAcc == nil {
		return &pb.LedgerTxResponse{Success: false, Message: "to account not found"}, nil
	}

	res, err := e.PostTx(l.ctx, engine.PostTxRequest{
		IdempotencyKey: in.IdempotencyKey,
		OpType:         "Transfer",
		BizRef:         normalizeBizRef(in.BizRef),
		Postings: []engine.Posting{
			{AssetCode: assetCode, AccountID: fromAcc.ID, Bucket: fromBucketStr, DebitRaw: amountRaw, CreditRaw: decimal.Zero},
			{AssetCode: assetCode, AccountID: toAcc.ID, Bucket: toBucketStr, DebitRaw: decimal.Zero, CreditRaw: amountRaw},
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
