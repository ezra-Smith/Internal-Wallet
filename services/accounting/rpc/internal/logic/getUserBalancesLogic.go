package logic

import (
	"context"
	"sort"

	"internalwallet/pkg/accounting"
	"internalwallet/proto/pb"
	"internalwallet/services/accounting/rpc/internal/engine"
	"internalwallet/services/accounting/rpc/internal/svc"

	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetUserBalancesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetUserBalancesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUserBalancesLogic {
	return &GetUserBalancesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ==================== Read APIs ====================
func (l *GetUserBalancesLogic) GetUserBalances(in *pb.GetUserBalancesRequest) (*pb.GetUserBalancesResponse, error) {
	if err := requireDB(l.svcCtx); err != nil {
		return &pb.GetUserBalancesResponse{}, nil
	}
	if l.svcCtx.BalanceRepo == nil || l.svcCtx.AssetRepo == nil {
		return &pb.GetUserBalancesResponse{}, nil
	}
	if in == nil || in.UserId <= 0 {
		return &pb.GetUserBalancesResponse{}, nil
	}

	// Best-effort ensure setup so new assets get zero rows on demand.
	_ = engine.New(l.svcCtx.DB).EnsureUserAccountingSetup(l.ctx, in.UserId)

	rows, err := l.svcCtx.BalanceRepo.ListUserLiabilityBalances(l.ctx, in.UserId)
	if err != nil {
		l.Logger.Errorf("GetUserBalances list balances failed: %v", err)
		return &pb.GetUserBalancesResponse{}, nil
	}

	type agg struct {
		availableRaw string
		lockedRaw    string
	}
	byAsset := map[string]*agg{}
	for _, r := range rows {
		a := accounting.NormalizeAssetCode(r.AssetCode)
		if a == "" {
			continue
		}
		item := byAsset[a]
		if item == nil {
			item = &agg{availableRaw: "0", lockedRaw: "0"}
			byAsset[a] = item
		}
		switch r.Bucket {
		case engine.BalanceBucketAvailable:
			item.availableRaw = r.BalanceRaw
		case engine.BalanceBucketLocked:
			item.lockedRaw = r.BalanceRaw
		}
	}

	assetCodes := make([]string, 0, len(byAsset))
	for a := range byAsset {
		assetCodes = append(assetCodes, a)
	}
	sort.Strings(assetCodes)

	out := make([]*pb.UserBalanceItem, 0, len(assetCodes))
	for _, assetCode := range assetCodes {
		asset, err := l.svcCtx.AssetRepo.FindByCode(l.ctx, assetCode)
		var scale int32
		if err == nil && asset != nil {
			scale = asset.Precision
		}
		availableRawDec, _ := decimal.NewFromString(byAsset[assetCode].availableRaw)
		lockedRawDec, _ := decimal.NewFromString(byAsset[assetCode].lockedRaw)

		availableDecStr, _ := accounting.RawToDecimalString(availableRawDec, scale)
		lockedDecStr, _ := accounting.RawToDecimalString(lockedRawDec, scale)

		// 对可用余额和锁定余额都保留6位小数并四舍五入（固定显示6位）
		if availableDec, err := decimal.NewFromString(availableDecStr); err == nil {
			availableDecStr = availableDec.Round(6).StringFixed(6)
		}
		if lockedDec, err := decimal.NewFromString(lockedDecStr); err == nil {
			lockedDecStr = lockedDec.Round(6).StringFixed(6)
		}

		out = append(out, &pb.UserBalanceItem{
			AssetCode:    assetCode,
			Scale:        scale,
			Available:    availableDecStr,
			Locked:       lockedDecStr,
			AvailableRaw: byAsset[assetCode].availableRaw,
			LockedRaw:    byAsset[assetCode].lockedRaw,
		})
	}

	return &pb.GetUserBalancesResponse{Items: out}, nil
}
