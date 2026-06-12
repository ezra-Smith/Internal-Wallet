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

type AdminAdjustLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAdminAdjustLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminAdjustLogic {
	return &AdminAdjustLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AdminAdjustLogic) AdminAdjust(in *pb.AdminAdjustRequest) (*pb.LedgerTxResponse, error) {
	if err := requireDB(l.svcCtx); err != nil {
		return &pb.LedgerTxResponse{Success: false, Message: err.Error()}, nil
	}
	if l.svcCtx.AssetRepo == nil || l.svcCtx.AccountRepo == nil {
		return &pb.LedgerTxResponse{Success: false, Message: "repository not initialized"}, nil
	}
	if in == nil || strings.TrimSpace(in.IdempotencyKey) == "" || strings.TrimSpace(in.TargetAccountTypeCode) == "" ||
		strings.TrimSpace(in.AssetCode) == "" || strings.TrimSpace(in.AmountDecimal) == "" {
		return &pb.LedgerTxResponse{Success: false, Message: "invalid params"}, nil
	}

	ownerType, err := ownerTypeToString(in.TargetOwnerType)
	if err != nil {
		return &pb.LedgerTxResponse{Success: false, Message: err.Error()}, nil
	}
	if ownerType == engine.OwnerTypeUser {
		if in.TargetOwnerId <= 0 {
			return &pb.LedgerTxResponse{Success: false, Message: "invalid target_owner_id"}, nil
		}
	} else {
		// system
		if in.TargetOwnerId != 0 {
			return &pb.LedgerTxResponse{Success: false, Message: "system owner_id must be 0"}, nil
		}
	}

	targetTypeCode := strings.ToUpper(strings.TrimSpace(in.TargetAccountTypeCode))
	chainCode := accounting.NormalizeChainCode(in.ChainCode)
	chainScope := ""
	if targetTypeCode == engine.AccountTypeSysWalletHot {
		if chainCode == "" {
			return &pb.LedgerTxResponse{Success: false, Message: "chain_code required for SYS_WALLET_HOT"}, nil
		}
		chainScope = chainCode
	}

	bucket := in.Bucket
	if bucket == pb.BalanceBucket_BUCKET_UNSPECIFIED {
		bucket = pb.BalanceBucket_BUCKET_AVAILABLE
	}
	bucketStr, err := bucketToString(bucket)
	if err != nil {
		return &pb.LedgerTxResponse{Success: false, Message: err.Error()}, nil
	}
	if ownerType == engine.OwnerTypeSystem && bucketStr != engine.BalanceBucketAvailable {
		return &pb.LedgerTxResponse{Success: false, Message: "system bucket must be available"}, nil
	}

	if in.Direction == pb.AdjustDirection_ADJUST_DIRECTION_UNSPECIFIED {
		return &pb.LedgerTxResponse{Success: false, Message: "direction required"}, nil
	}
	if targetTypeCode == engine.AccountTypeSysAdjustExp || targetTypeCode == engine.AccountTypeSysAdjustInc {
		return &pb.LedgerTxResponse{Success: false, Message: "cannot adjust adjustment accounts directly"}, nil
	}

	assetCode := accounting.NormalizeAssetCode(in.AssetCode)
	asset, err := l.svcCtx.AssetRepo.FindByCode(l.ctx, assetCode)
	if err != nil || asset == nil {
		return &pb.LedgerTxResponse{Success: false, Message: "asset not found"}, nil
	}
	amountRaw, err := accounting.ParseDecimalToRawExact(in.AmountDecimal, asset.Precision, false)
	if err != nil {
		return &pb.LedgerTxResponse{Success: false, Message: err.Error()}, nil
	}

	e := engine.New(l.svcCtx.DB)
	// Ensure system contra accounts, and optionally hot wallet.
	if err := e.EnsureSystemAccounts(l.ctx, chainScope); err != nil {
		l.Logger.Errorf("EnsureSystemAccounts failed: %v", err)
		return &pb.LedgerTxResponse{Success: false, Message: err.Error()}, nil
	}
	if ownerType == engine.OwnerTypeUser {
		if err := e.EnsureUserAccountingSetup(l.ctx, in.TargetOwnerId); err != nil {
			l.Logger.Errorf("EnsureUserAccountingSetup failed: %v", err)
			return &pb.LedgerTxResponse{Success: false, Message: err.Error()}, nil
		}
	}

	targetAcc, err := l.svcCtx.AccountRepo.FindByOwnerType(l.ctx, ownerType, in.TargetOwnerId, targetTypeCode, chainScope)
	if err != nil || targetAcc == nil {
		return &pb.LedgerTxResponse{Success: false, Message: "target account not found"}, nil
	}
	adjExpAcc, err := l.svcCtx.AccountRepo.FindByOwnerType(l.ctx, engine.OwnerTypeSystem, 0, engine.AccountTypeSysAdjustExp, "")
	if err != nil || adjExpAcc == nil {
		return &pb.LedgerTxResponse{Success: false, Message: "adjust expense account not found"}, nil
	}
	adjIncAcc, err := l.svcCtx.AccountRepo.FindByOwnerType(l.ctx, engine.OwnerTypeSystem, 0, engine.AccountTypeSysAdjustInc, "")
	if err != nil || adjIncAcc == nil {
		return &pb.LedgerTxResponse{Success: false, Message: "adjust income account not found"}, nil
	}

	// Determine target normal_side.
	type sideRow struct {
		NormalSide string `gorm:"column:normal_side"`
	}
	var srow sideRow
	if err := l.svcCtx.DB.WithContext(l.ctx).
		Table("acct_accounts a").
		Select("t.normal_side").
		Joins("JOIN acct_account_types t ON t.code = a.account_type_code").
		Where("a.id = ? AND a.deleted_at IS NULL AND t.deleted_at IS NULL", targetAcc.ID).
		Scan(&srow).Error; err != nil {
		return &pb.LedgerTxResponse{Success: false, Message: "failed to load account type"}, nil
	}
	targetSide := strings.ToLower(strings.TrimSpace(srow.NormalSide))
	if targetSide != "debit" && targetSide != "credit" {
		return &pb.LedgerTxResponse{Success: false, Message: "invalid account normal_side"}, nil
	}

	var postings []engine.Posting
	switch in.Direction {
	case pb.AdjustDirection_ADJUST_INCREASE:
		if targetSide == "debit" {
			postings = []engine.Posting{
				{AssetCode: assetCode, AccountID: targetAcc.ID, Bucket: bucketStr, DebitRaw: amountRaw, CreditRaw: decimal.Zero},
				{AssetCode: assetCode, AccountID: adjIncAcc.ID, Bucket: engine.BalanceBucketAvailable, DebitRaw: decimal.Zero, CreditRaw: amountRaw},
			}
		} else {
			postings = []engine.Posting{
				{AssetCode: assetCode, AccountID: targetAcc.ID, Bucket: bucketStr, DebitRaw: decimal.Zero, CreditRaw: amountRaw},
				{AssetCode: assetCode, AccountID: adjExpAcc.ID, Bucket: engine.BalanceBucketAvailable, DebitRaw: amountRaw, CreditRaw: decimal.Zero},
			}
		}
	case pb.AdjustDirection_ADJUST_DECREASE:
		if targetSide == "debit" {
			postings = []engine.Posting{
				{AssetCode: assetCode, AccountID: targetAcc.ID, Bucket: bucketStr, DebitRaw: decimal.Zero, CreditRaw: amountRaw},
				{AssetCode: assetCode, AccountID: adjExpAcc.ID, Bucket: engine.BalanceBucketAvailable, DebitRaw: amountRaw, CreditRaw: decimal.Zero},
			}
		} else {
			postings = []engine.Posting{
				{AssetCode: assetCode, AccountID: targetAcc.ID, Bucket: bucketStr, DebitRaw: amountRaw, CreditRaw: decimal.Zero},
				{AssetCode: assetCode, AccountID: adjIncAcc.ID, Bucket: engine.BalanceBucketAvailable, DebitRaw: decimal.Zero, CreditRaw: amountRaw},
			}
		}
	default:
		return &pb.LedgerTxResponse{Success: false, Message: "invalid direction"}, nil
	}

	res, err := e.PostTx(l.ctx, engine.PostTxRequest{
		IdempotencyKey: in.IdempotencyKey,
		OpType:         "AdminAdjust",
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

	// Best-effort: write user-facing transaction record for transfer batch credits.
	//
	// NOTE: `acct_user_transaction_records` is a read model currently used by Business APIs.
	// Transfer batches are internal-ledger-only (no on-chain tx_hash); we store them as tx_type=deposit.
	if l.svcCtx.UserTxRepo != nil &&
		ownerType == engine.OwnerTypeUser &&
		targetTypeCode == engine.AccountTypeUserLiability &&
		bucketStr == engine.BalanceBucketAvailable &&
		in.Direction == pb.AdjustDirection_ADJUST_INCREASE {
		chainCodeForRecord := accounting.NormalizeChainCode(in.ChainCode)
		// Transfer batches usually pass empty chain_code; keep a stable label for UI filtering.
		if chainCodeForRecord == "" {
			if biz := strings.ToLower(strings.TrimSpace(in.BizRef)); strings.HasPrefix(biz, "transfer_batch:") {
				chainCodeForRecord = "INTERNAL"
			}
		}

		rec := &model.AcctUserTransactionRecordModel{
			UserID:        in.TargetOwnerId,
			TxType:        1, // deposit
			AssetCode:     assetCode,
			ChainCode:     chainCodeForRecord,
			AmountDecimal: strings.TrimSpace(in.AmountDecimal),
			FeeDecimal:    "0",
			Status:        "completed",
			Memo:          strings.TrimSpace(in.Note),
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
			l.Logger.Errorf("Upsert user transfer batch record failed: %v", err)
		}
	}

	return &pb.LedgerTxResponse{Success: true, Message: "ok", TxId: res.TxID, Duplicate: res.Duplicate}, nil
}
