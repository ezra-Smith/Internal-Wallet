package logic

import (
	"context"
	"sort"
	"strings"

	"internalwallet/common/constants"
	"internalwallet/common/withdrawcalc"
	"internalwallet/services/business/rpc/internal/svc"
)

// LoadWithdrawFeeRuleItems 统一加载“提现手续费规则快照项”。
//
// 规则优先级（按你的要求）：
//   - 全局优先：useGlobalWithdrawFee=true 时，只使用全局规则（链级别 > 币种级别）
//   - 如果全局没开：useGlobalWithdrawFee=false 时，只使用链规则（per-asset-chain）
//
// 注意：这里刻意不做“全局关闭时回退到全局 / 全局开启时回退到链规则”，避免口径分叉。
func LoadWithdrawFeeRuleItems(ctx context.Context, svcCtx *svc.ServiceContext, assetCode, chainCode string, useGlobalWithdrawFee bool) (string, []withdrawcalc.FeeRuleSnapshotItem) {
	assetCode = normalizeCode(assetCode)
	chainCode = normalizeCode(chainCode)

	// Helper: deterministic sort, in case upstream changes ordering.
	sortItems := func(items []withdrawcalc.FeeRuleSnapshotItem) []withdrawcalc.FeeRuleSnapshotItem {
		sort.SliceStable(items, func(i, j int) bool {
			if items[i].SortOrder != items[j].SortOrder {
				return items[i].SortOrder < items[j].SortOrder
			}
			return items[i].ID < items[j].ID
		})
		return items
	}

	// 全局开：只走全局（链 > 资产）
	if useGlobalWithdrawFee {
		if svcCtx != nil && svcCtx.CurrencyGlobalWithdrawFeeRuleRepository != nil {
			if gr, _ := svcCtx.CurrencyGlobalWithdrawFeeRuleRepository.ListByChain(ctx, chainCode); len(gr) > 0 {
				items := make([]withdrawcalc.FeeRuleSnapshotItem, 0, len(gr))
				for _, g := range gr {
					if !g.Enabled {
						continue
					}
					items = append(items, withdrawcalc.FeeRuleSnapshotItem{
						ID:        g.ID,
						RuleType:  strings.TrimSpace(g.RuleType),
						Value:     strings.TrimSpace(g.Value),
						MinFee:    g.MinFee,
						MaxFee:    g.MaxFee,
						MinAmount: g.MinAmount,
						MaxAmount: g.MaxAmount,
						SortOrder: g.SortOrder,
					})
				}
				return constants.WithdrawFeeRuleSourceGlobal, sortItems(items)
			}
			if gr, _ := svcCtx.CurrencyGlobalWithdrawFeeRuleRepository.ListByAsset(ctx, assetCode); len(gr) > 0 {
				items := make([]withdrawcalc.FeeRuleSnapshotItem, 0, len(gr))
				for _, g := range gr {
					if !g.Enabled {
						continue
					}
					items = append(items, withdrawcalc.FeeRuleSnapshotItem{
						ID:        g.ID,
						RuleType:  strings.TrimSpace(g.RuleType),
						Value:     strings.TrimSpace(g.Value),
						MinFee:    g.MinFee,
						MaxFee:    g.MaxFee,
						MinAmount: g.MinAmount,
						MaxAmount: g.MaxAmount,
						SortOrder: g.SortOrder,
					})
				}
				return constants.WithdrawFeeRuleSourceGlobal, sortItems(items)
			}
		}
		return constants.WithdrawFeeRuleSourceGlobal, nil
	}

	// 全局没开：只走链规则（per-asset-chain）
	if svcCtx != nil && svcCtx.CurrencyWithdrawFeeRuleRepository != nil {
		if rr, _ := svcCtx.CurrencyWithdrawFeeRuleRepository.ListByAssetChain(ctx, assetCode, chainCode); len(rr) > 0 {
			items := make([]withdrawcalc.FeeRuleSnapshotItem, 0, len(rr))
			for _, r := range rr {
				if !r.Enabled {
					continue
				}
				items = append(items, withdrawcalc.FeeRuleSnapshotItem{
					ID:        r.ID,
					RuleType:  strings.TrimSpace(r.RuleType),
					Value:     strings.TrimSpace(r.Value),
					MinFee:    r.MinFee,
					MaxFee:    r.MaxFee,
					MinAmount: r.MinAmount,
					MaxAmount: r.MaxAmount,
					SortOrder: r.SortOrder,
				})
			}
			return constants.WithdrawFeeRuleSourceAsset, sortItems(items)
		}
	}
	return constants.WithdrawFeeRuleSourceAsset, nil
}
