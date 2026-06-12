package logic

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"sync"

	"internalwallet/common/middleware"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetAssetOverviewLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetAssetOverviewLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetAssetOverviewLogic {
	return &GetAssetOverviewLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetAssetOverviewLogic) GetAssetOverview(in *pb.GetAssetOverviewReq) (*pb.GetAssetOverviewResp, error) {
	uidStr := middleware.GetUserID(l.ctx)
	uid, _ := strconv.ParseInt(uidStr, 10, 64)

	// Get user profile for username
	var username string
	if uid > 0 && l.svcCtx.UserAccountRepository != nil {
		if profile, err := l.svcCtx.UserAccountRepository.GetByID(l.ctx, uid); err == nil && profile != nil {
			username = strings.TrimSpace(profile.Nickname)
			if username == "" {
				// Fallback to email prefix or phone
				if profile.Email != "" {
					parts := strings.Split(profile.Email, "@")
					if len(parts) > 0 {
						username = parts[0]
					}
				} else if profile.Phone != "" {
					// Mask phone for privacy
					if len(profile.Phone) > 4 {
						username = "****" + profile.Phone[len(profile.Phone)-4:]
					} else {
						username = profile.Phone
					}
				}
			}
		}
	}

	// Step 1: Get user balances from Accounting (source of truth)
	type balanceInfo struct {
		available decimal.Decimal
		locked    decimal.Decimal
		total     decimal.Decimal
		scale     int32
	}
	balances := make(map[string]balanceInfo)

	if uid > 0 && l.svcCtx.AccountingRpc != nil {
		accResp, err := l.svcCtx.AccountingRpc.GetUserBalances(l.ctx, &pb.GetUserBalancesRequest{UserId: uid})
		if err != nil {
			l.Logger.Errorf("GetUserBalances failed: %v", err)
		} else if accResp != nil {
			for _, it := range accResp.GetItems() {
				if it == nil {
					continue
				}
				assetCode := normalizeCode(it.AssetCode)
				if assetCode == "" {
					continue
				}
				availDec, _ := decimal.NewFromString(strings.TrimSpace(it.Available))
				lockedDec, _ := decimal.NewFromString(strings.TrimSpace(it.Locked))
				balances[assetCode] = balanceInfo{
					available: availDec,
					locked:    lockedDec,
					total:     availDec.Add(lockedDec),
					scale:     it.Scale,
				}
			}
		}
	}

	// Step 2: Get asset metadata (icons) from Accounting - always fetch available assets
	assetMeta := make(map[string]*pb.AcctAsset)
	if l.svcCtx.AccountingRpc != nil {
		listResp, err := l.svcCtx.AccountingRpc.ListAssets(l.ctx, &pb.ListAssetsRequest{
			Page:     1,
			PageSize: 100,
			Status:   1, // enabled only
		})
		if err != nil {
			l.Logger.Errorf("ListAssets failed: %v", err)
		} else if listResp != nil {
			for _, asset := range listResp.GetItems() {
				if asset == nil {
					continue
				}
				code := normalizeCode(asset.Code)
				if code != "" {
					assetMeta[code] = asset
				}
			}
		}
	}

	// Collect asset codes: use all available assets (from assetMeta), not just user balances
	// This ensures users see all assets even with 0 balance
	assetCodes := make([]string, 0, len(assetMeta))
	for code := range assetMeta {
		assetCodes = append(assetCodes, code)
	}
	sort.Strings(assetCodes)

	// If no assets available in system, return empty response
	if len(assetCodes) == 0 {
		return &pb.GetAssetOverviewResp{
			Success:          true,
			TotalBalance:     "0",
			AvailableBalance: "0",
			FrozenBalance:    "0",
			PriceChange_24H:  "0",
			Username:         username,
			Uid:              uidStr,
			TokenVolume:      "0",
			TokenBalance:     "0",
			Items:            []*pb.AssetItem{},
		}, nil
	}

	// Step 3: Batch get market prices from Redis
	prices := BatchGetAssetPrices(l.ctx, l.svcCtx.RedisClient, assetCodes)

	// Step 4: Concurrently get sparkline data for 24h price change
	type sparklineResult struct {
		assetCode string
		sparkline []string
		change24h string
		oldPrice  decimal.Decimal
		newPrice  decimal.Decimal
	}
	sparklineCh := make(chan sparklineResult, len(assetCodes))
	var wg sync.WaitGroup
	for _, code := range assetCodes {
		wg.Add(1)
		go func(assetCode string) {
			defer wg.Done()
			sparkline, err := GetSparklineLast(l.ctx, l.svcCtx.RedisClient, assetCode, 24)
			if err != nil {
				l.Logger.Debugf("GetSparkline failed for %s: %v", assetCode, err)
				sparklineCh <- sparklineResult{assetCode: assetCode}
				return
			}
			change24h, _ := Calculate24hPriceChange(sparkline)

			// Parse old and new prices for weighted average calculation
			var oldPrice, newPrice decimal.Decimal
			if len(sparkline) >= 2 {
				oldPrice, _ = decimal.NewFromString(sparkline[0])
				newPrice, _ = decimal.NewFromString(sparkline[len(sparkline)-1])
			}

			sparklineCh <- sparklineResult{
				assetCode: assetCode,
				sparkline: sparkline,
				change24h: change24h,
				oldPrice:  oldPrice,
				newPrice:  newPrice,
			}
		}(code)
	}
	go func() {
		wg.Wait()
		close(sparklineCh)
	}()

	sparklines := make(map[string]sparklineResult, len(assetCodes))
	for sr := range sparklineCh {
		sparklines[sr.assetCode] = sr
	}

	// Step 5: Build response items and calculate totals
	items := make([]*pb.AssetItem, 0, len(assetCodes))
	totalBalanceUSDT := decimal.Zero
	totalAvailableUSDT := decimal.Zero
	totalFrozenUSDT := decimal.Zero
	tokenCount := 0

	// For weighted 24h change calculation
	totalOldValueUSDT := decimal.Zero
	totalNewValueUSDT := decimal.Zero

	for _, assetCode := range assetCodes {
		// Get balance for this asset, default to zero if not found
		bal, hasBalance := balances[assetCode]
		if !hasBalance {
			bal = balanceInfo{
				available: decimal.Zero,
				locked:    decimal.Zero,
				total:     decimal.Zero,
				scale:     0,
			}
		}

		// Count tokens with non-zero balance
		if !bal.total.IsZero() {
			tokenCount++
		}

		item := &pb.AssetItem{
			Asset:            assetCode,
			TotalBalance:     bal.total.String(),
			AvailableBalance: bal.available.String(),
			FrozenBalance:    bal.locked.String(),
			PriceChange_24H:  "0",
			PriceUsdt:        "0",
			BalanceUsdt:      "0",
		}

		// Fill icon URL and decimals from asset metadata
		if meta, ok := assetMeta[assetCode]; ok {
			item.IconUrl = strings.TrimSpace(meta.IconUrl)
			if meta.Precision > 0 {
				item.Decimals = meta.Precision
			}
		}
		
		// Fallback: use scale from balance info if metadata precision not available
		if item.Decimals == 0 && bal.scale > 0 {
			item.Decimals = bal.scale
		}

		// Fill price and calculate USDT valuation
		if price, ok := prices[assetCode]; ok {
			item.PriceUsdt = price.StringFixed(8)

			// Calculate balance in USDT
			balanceUSDT := bal.total.Mul(price)
			// 使用更高精度显示小额资产估值（避免 0.004 四舍五入后变成 0）
			if balanceUSDT.LessThan(decimal.NewFromFloat(0.01)) {
				item.BalanceUsdt = balanceUSDT.Truncate(6).String() // 小额保留 6 位
			} else {
				item.BalanceUsdt = balanceUSDT.Truncate(2).StringFixed(2) // 大额固定 2 位小数
			}

			// Accumulate totals
			totalBalanceUSDT = totalBalanceUSDT.Add(balanceUSDT)
			totalAvailableUSDT = totalAvailableUSDT.Add(bal.available.Mul(price))
			totalFrozenUSDT = totalFrozenUSDT.Add(bal.locked.Mul(price))

			// For weighted 24h change
			if sr, ok := sparklines[assetCode]; ok && sr.newPrice.GreaterThan(decimal.Zero) {
				totalNewValueUSDT = totalNewValueUSDT.Add(bal.total.Mul(sr.newPrice))
				if sr.oldPrice.GreaterThan(decimal.Zero) {
					totalOldValueUSDT = totalOldValueUSDT.Add(bal.total.Mul(sr.oldPrice))
				} else {
					totalOldValueUSDT = totalOldValueUSDT.Add(bal.total.Mul(sr.newPrice))
				}
			} else {
				// Use current price for both if no sparkline
				totalNewValueUSDT = totalNewValueUSDT.Add(balanceUSDT)
				totalOldValueUSDT = totalOldValueUSDT.Add(balanceUSDT)
			}
		}

		// Fill 24h price change (sparkline not returned to reduce payload)
		if sr, ok := sparklines[assetCode]; ok {
			if sr.change24h != "" {
				item.PriceChange_24H = sr.change24h
			}
		}

		items = append(items, item)
	}

	// Calculate overall portfolio 24h change
	overallChange24h := "0"
	if totalOldValueUSDT.GreaterThan(decimal.Zero) {
		change := totalNewValueUSDT.Sub(totalOldValueUSDT).Div(totalOldValueUSDT).Mul(decimal.NewFromInt(100))
		overallChange24h = change.Round(2).String()
	}

	return &pb.GetAssetOverviewResp{
		Success:          true,
		TotalBalance:     totalBalanceUSDT.Round(2).String(),
		AvailableBalance: totalAvailableUSDT.Round(2).String(),
		FrozenBalance:    totalFrozenUSDT.Round(2).String(),
		PriceChange_24H:  overallChange24h,
		Username:         username,
		Uid:              uidStr,
		TokenVolume:      strconv.Itoa(tokenCount),
		TokenBalance:     totalBalanceUSDT.Round(2).String(),
		Items:            items,
	}, nil
}
