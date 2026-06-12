package logic

import (
	"context"
	"strconv"
	"strings"

	"internalwallet/common/middleware"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetAssetDetailLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetAssetDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetAssetDetailLogic {
	return &GetAssetDetailLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetAssetDetailLogic) GetAssetDetail(in *pb.GetAssetDetailReq) (*pb.GetAssetDetailResp, error) {
	if in == nil || in.Asset == "" {
		return nil, errx.InvalidParam("invalid params")
	}
	uidStr := middleware.GetUserID(l.ctx)
	if uidStr == "" {
		return nil, errx.Unauthorized("unauthorized")
	}
	uid, _ := strconv.ParseInt(uidStr, 10, 64)
	assetCode := normalizeCode(in.Asset)

	var items []*pb.TransactionRecordItem
	total := int64(0)
	if l.svcCtx.AccountingRpc != nil {
		accResp, err := l.svcCtx.AccountingRpc.ListUserTransactionRecords(l.ctx, &pb.ListUserTransactionRecordsRequest{
			Page:      in.Page,
			PageSize:  in.PageSize,
			UserId:    uid,
			AssetCode: assetCode,
		})
		if err == nil && accResp != nil && accResp.Success {
			total = accResp.Total
			for _, r := range accResp.GetItems() {
				if r == nil {
					continue
				}
				items = append(items, &pb.TransactionRecordItem{
					Id:        strconv.FormatInt(r.Id, 10),
					Type:      pb.TransactionType(r.TxType),
					Asset:     strings.TrimSpace(r.AssetCode),
					Amount:    strings.TrimSpace(r.AmountDecimal),
					Timestamp: r.Timestamp,
					Status:    strings.TrimSpace(r.Status),
					Memo:      strings.TrimSpace(r.Memo),
				})
			}
		}
	}

	assetName := assetCode
	iconURL := ""
	totalBalanceStr := "0"
	availableBalanceStr := "0"
	frozenBalanceStr := "0"
	if uid > 0 && l.svcCtx.AccountingRpc != nil {
		// Fill asset meta best-effort.
		if aResp, err := l.svcCtx.AccountingRpc.GetAsset(l.ctx, &pb.GetAssetRequest{Code: assetCode}); err == nil && aResp != nil && aResp.Success && aResp.Item != nil {
			if v := strings.TrimSpace(aResp.Item.Name); v != "" {
				assetName = v
			}
			iconURL = strings.TrimSpace(aResp.Item.IconUrl)
		}

		// Balances are sourced from Accounting (single source of truth).
		accResp, err := l.svcCtx.AccountingRpc.GetUserBalances(l.ctx, &pb.GetUserBalancesRequest{UserId: uid})
		if err != nil {
			l.Logger.Errorf("GetUserBalances failed: %v", err)
		} else if accResp != nil {
			for _, it := range accResp.GetItems() {
				if it == nil {
					continue
				}
				if normalizeCode(it.AssetCode) != assetCode {
					continue
				}
				availableBalanceStr = strings.TrimSpace(it.Available)
				frozenBalanceStr = strings.TrimSpace(it.Locked)
				availDec, err1 := decimal.NewFromString(availableBalanceStr)
				if err1 != nil {
					availDec = decimal.Zero
				}
				frozenDec, err2 := decimal.NewFromString(frozenBalanceStr)
				if err2 != nil {
					frozenDec = decimal.Zero
				}
				totalBalanceStr = availDec.Add(frozenDec).String()
				break
			}
		}
	}

	fiatCurrency := "USD"
	fiatSymbol := "$"
	return &pb.GetAssetDetailResp{
		Success:            true,
		Asset:              assetCode,
		AssetName:          assetName,
		IconUrl:            iconURL,
		TotalBalance:       totalBalanceStr,
		AvailableBalance:   availableBalanceStr,
		FrozenBalance:      frozenBalanceStr,
		TotalValuationUsdt: "0",
		TotalValuationFiat: "0",
		FiatCurrency:       fiatCurrency,
		FiatSymbol:         fiatSymbol,
		PriceUsdt:          "0",
		TodayChangePercent: "0",
		Total:              int32(total),
		Items:              items,
	}, nil
}
