package logic

import (
	"context"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/accounting/rpc/internal/model"
	"internalwallet/services/accounting/rpc/internal/svc"

	"internalwallet/pkg/accounting"

	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetLedgerTxDetailLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetLedgerTxDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetLedgerTxDetailLogic {
	return &GetLedgerTxDetailLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetLedgerTxDetailLogic) GetLedgerTxDetail(in *pb.GetLedgerTxDetailRequest) (*pb.GetLedgerTxDetailResponse, error) {
	if err := requireDB(l.svcCtx); err != nil {
		return &pb.GetLedgerTxDetailResponse{Success: false, Message: err.Error()}, nil
	}
	if l.svcCtx.LedgerRepo == nil || l.svcCtx.AccountRepo == nil || l.svcCtx.AssetRepo == nil || l.svcCtx.AccountTypeRepo == nil {
		return &pb.GetLedgerTxDetailResponse{Success: false, Message: "repository not initialized"}, nil
	}
	if in == nil || in.TxId <= 0 {
		return &pb.GetLedgerTxDetailResponse{Success: false, Message: "tx_id required"}, nil
	}

	tx, err := l.svcCtx.LedgerRepo.FindTxByID(l.ctx, in.TxId)
	if err != nil || tx == nil {
		return &pb.GetLedgerTxDetailResponse{Success: false, Message: "tx not found"}, nil
	}
	postings, err := l.svcCtx.LedgerRepo.ListPostingsByTxID(l.ctx, tx.ID)
	if err != nil {
		l.Logger.Errorf("ListPostingsByTxID failed: %v", err)
		return &pb.GetLedgerTxDetailResponse{Success: false, Message: err.Error()}, nil
	}

	accountIDs := make([]int64, 0, len(postings))
	assetCodes := make([]string, 0, len(postings))
	for _, p := range postings {
		accountIDs = append(accountIDs, p.AccountID)
		assetCodes = append(assetCodes, p.AssetCode)
	}

	accounts, err := l.svcCtx.AccountRepo.FindByIDs(l.ctx, accountIDs)
	if err != nil {
		l.Logger.Errorf("FindByIDs accounts failed: %v", err)
		return &pb.GetLedgerTxDetailResponse{Success: false, Message: err.Error()}, nil
	}
	accountByID := make(map[int64]model.AcctAccountModel, len(accounts))
	for _, a := range accounts {
		accountByID[a.ID] = a
	}

	assets, err := l.svcCtx.AssetRepo.ListByCodes(l.ctx, assetCodes)
	if err != nil {
		l.Logger.Errorf("ListByCodes assets failed: %v", err)
		return &pb.GetLedgerTxDetailResponse{Success: false, Message: err.Error()}, nil
	}
	scaleByAsset := make(map[string]int32, len(assets))
	for _, a := range assets {
		code := accounting.NormalizeAssetCode(a.Code)
		scaleByAsset[code] = a.Precision
	}

	accountTypes, err := l.svcCtx.AccountTypeRepo.List(l.ctx, false)
	if err != nil {
		l.Logger.Errorf("List account types failed: %v", err)
		return &pb.GetLedgerTxDetailResponse{Success: false, Message: err.Error()}, nil
	}
	accountTypeByCode := make(map[string]model.AcctAccountTypeModel, len(accountTypes))
	for _, t := range accountTypes {
		accountTypeByCode[accounting.NormalizeCode(t.Code)] = t
	}

	outPostings := make([]*pb.LedgerPostingItem, 0, len(postings))
	for _, p := range postings {
		assetCode := accounting.NormalizeAssetCode(p.AssetCode)
		scale := scaleByAsset[assetCode]
		acc := accountByID[p.AccountID]
		at := accountTypeByCode[accounting.NormalizeCode(acc.AccountTypeCode)]

		debitRawStr := strings.TrimSpace(p.DebitRaw)
		creditRawStr := strings.TrimSpace(p.CreditRaw)
		debitRawDec, _ := decimal.NewFromString(debitRawStr)
		creditRawDec, _ := decimal.NewFromString(creditRawStr)
		debitDecStr, _ := accounting.RawToDecimalString(debitRawDec, scale)
		creditDecStr, _ := accounting.RawToDecimalString(creditRawDec, scale)

		outPostings = append(outPostings, &pb.LedgerPostingItem{
			Seq:                    p.Seq,
			AssetCode:              assetCode,
			Scale:                  scale,
			Bucket:                 strings.TrimSpace(p.Bucket),
			AccountId:              p.AccountID,
			OwnerType:              strings.ToLower(strings.TrimSpace(acc.OwnerType)),
			OwnerId:                acc.OwnerID,
			AccountTypeCode:        accounting.NormalizeCode(acc.AccountTypeCode),
			AccountTypeName:        strings.TrimSpace(at.Name),
			AccountTypeDescription: strings.TrimSpace(at.Description),
			AccountNormalSide:      strings.ToLower(strings.TrimSpace(at.NormalSide)),
			ChainScope:             accounting.NormalizeChainCode(acc.ChainScope),
			DebitRaw:               debitRawStr,
			CreditRaw:              creditRawStr,
			DebitDecimal:           debitDecStr,
			CreditDecimal:          creditDecStr,
		})
	}

	return &pb.GetLedgerTxDetailResponse{
		Success: true,
		Message: "ok",
		Tx: &pb.LedgerTxItem{
			TxId:           tx.ID,
			CreatedAt:      formatTime(tx.CreatedAt),
			OpType:         strings.TrimSpace(tx.OpType),
			BizRef:         strings.TrimSpace(tx.BizRef),
			IdempotencyKey: strings.TrimSpace(tx.IdempotencyKey),
			RequestHash:    strings.TrimSpace(tx.RequestHash),
		},
		Postings: outPostings,
	}, nil
}
