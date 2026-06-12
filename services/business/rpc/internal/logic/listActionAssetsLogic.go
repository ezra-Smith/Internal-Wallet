package logic

import (
	"context"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/model"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListActionAssetsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListActionAssetsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListActionAssetsLogic {
	return &ListActionAssetsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListActionAssetsLogic) ListActionAssets(in *pb.ListActionAssetsReq) (*pb.ListActionAssetsResp, error) {
	items := []*pb.ActionAssetItem{}
	hot := []*pb.ActionAssetItem{}
	action := "deposit" // Default to deposit
	q := ""
	keyword := ""
	if in != nil {
		q = strings.ToUpper(strings.TrimSpace(in.Query))
		keyword = strings.TrimSpace(in.Query)
		if strings.TrimSpace(in.Action) != "" {
			action = strings.ToLower(strings.TrimSpace(in.Action))
		}
	}

	// Step 1: List enabled assets from Accounting (source of truth for asset master data).
	rows := make([]*pb.AcctAsset, 0)
	if l.svcCtx.AccountingRpc != nil {
		page := int32(1)
		pageSize := int32(100)
		for {
			resp, err := l.svcCtx.AccountingRpc.ListAssets(l.ctx, &pb.ListAssetsRequest{
				Page:     page,
				PageSize: pageSize,
				Keyword:  keyword,
				Status:   1, // Only enabled assets
			})
			if err != nil || resp == nil || !resp.Success {
				break
			}
			if len(resp.Items) == 0 {
				break
			}
			rows = append(rows, resp.Items...)
			if int64(page)*int64(pageSize) >= resp.Total {
				break
			}
			page++
		}
	}

	// Step 2: Batch load currency_settings (asset-level feature toggles).
	settingsByCode := map[string]*model.CurrencySettingsModel{}
	if l.svcCtx.CurrencySettingsRepository != nil && len(rows) > 0 {
		codes := make([]string, 0, len(rows))
		for _, a := range rows {
			if a == nil || strings.TrimSpace(a.Code) == "" {
				continue
			}
			codes = append(codes, strings.TrimSpace(a.Code))
		}
		if settings, err := l.svcCtx.CurrencySettingsRepository.ListByAssetCodes(l.ctx, codes); err == nil {
			for i := range settings {
				s := settings[i]
				cc := strings.ToUpper(strings.TrimSpace(s.AssetCode))
				if cc != "" {
					settingsByCode[cc] = &s
				}
			}
		}
	}

	// Step 3: Check which assets have at least one enabled chain mapping for the action.
	// This ensures we only show assets that can actually be deposited/withdrawn.
	assetsWithEnabledChain := map[string]bool{}
	if l.svcCtx.CurrencyChainSettingsRepository != nil {
		// Get all enabled chain mappings
		if allMappings, err := l.svcCtx.CurrencyChainSettingsRepository.ListAllEnabled(l.ctx); err == nil {
			for _, m := range allMappings {
				if m.IsDeleted() || m.Status != 1 {
					continue
				}
				assetCode := strings.ToUpper(strings.TrimSpace(m.AssetCode))
				if assetCode == "" {
					continue
				}
				// Check action-specific enablement
				if action == "deposit" && m.DepositEnabled {
					assetsWithEnabledChain[assetCode] = true
				} else if action == "withdraw" && m.WithdrawEnabled {
					assetsWithEnabledChain[assetCode] = true
				}
			}
		}
	}

	// Step 4: Build response items.
	for _, a := range rows {
		if a == nil || strings.TrimSpace(a.Code) == "" {
			continue
		}
		code := strings.ToUpper(strings.TrimSpace(a.Code))

		// Filter by search query
		if q != "" && !strings.Contains(code, q) && !strings.Contains(strings.ToUpper(a.Name), q) {
			continue
		}

		// Check asset-level feature toggle (currency_settings.web3_deposit_enabled / web3_withdraw_enabled)
		if action == "deposit" || action == "withdraw" {
			s := settingsByCode[code]
			// Default: enabled if no settings configured
			enabled := true
			if s != nil {
				if action == "deposit" {
					enabled = s.Web3DepositEnabled
				} else {
					enabled = s.Web3WithdrawEnabled
				}
			}
			if !enabled {
				continue
			}
		}

		// Check if asset has at least one enabled chain mapping (if mappings are configured)
		// Skip this check if no mappings exist at all (backward compatible)
		if len(assetsWithEnabledChain) > 0 && !assetsWithEnabledChain[code] {
			continue
		}

		name := strings.TrimSpace(a.Name)
		if name == "" {
			name = code
		}

		// Use is_hot from asset master data (Accounting service)
		// Fallback to hardcoded defaults if not set
		isHot := a.IsHot
		if !isHot {
			// Backward compatible: default hot assets
			isHot = strings.EqualFold(code, "USDT") || strings.EqualFold(code, "BTC") || strings.EqualFold(code, "ETH")
		}

		item := &pb.ActionAssetItem{
			Asset:     code,
			AssetName: name,
			IconUrl:   strings.TrimSpace(a.IconUrl),
			IsHot:     isHot,
		}
		items = append(items, item)
		if item.IsHot {
			hot = append(hot, item)
		}
	}
	return &pb.ListActionAssetsResp{Success: true, HotItems: hot, Items: items}, nil
}
