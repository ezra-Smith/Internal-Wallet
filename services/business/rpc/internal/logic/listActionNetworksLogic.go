package logic

import (
	"context"
	"strings"

	"internalwallet/common/i18n"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/model"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListActionNetworksLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListActionNetworksLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListActionNetworksLogic {
	return &ListActionNetworksLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListActionNetworksLogic) ListActionNetworks(in *pb.ListActionNetworksReq) (*pb.ListActionNetworksResp, error) {
	items := []*pb.ActionNetworkItem{}
	if in == nil {
		return &pb.ListActionNetworksResp{Success: true, Items: items}, nil
	}

	assetCode := normalizeCode(in.Asset)
	action := strings.ToLower(strings.TrimSpace(in.Action))
	if action == "" {
		action = "deposit" // Default to deposit for UI flow
	}

	// 如果 asset 为空，返回所有启用的链（不进行资产过滤）
	if assetCode == "" {
		return l.listAllEnabledChains(action)
	}

	// Asset must be enabled (from Accounting - source of truth).
	if l.svcCtx.AccountingRpc != nil {
		accResp, err := l.svcCtx.AccountingRpc.GetAsset(l.ctx, &pb.GetAssetRequest{Code: assetCode})
		if err != nil || accResp == nil || !accResp.Success || accResp.Item == nil || accResp.Item.Status != 1 {
			return &pb.ListActionNetworksResp{Success: true, Items: items}, nil
		}
	}

	// Apply per-asset Web3 feature toggles (currency_settings).
	eff := defaultEffectiveCurrencySettings()
	if l.svcCtx.CurrencySettingsRepository != nil {
		if s, err := l.svcCtx.CurrencySettingsRepository.GetByAssetCode(l.ctx, assetCode); err == nil && s != nil {
			eff.Web3DepositEnabled = s.Web3DepositEnabled
			eff.Web3WithdrawEnabled = s.Web3WithdrawEnabled
			eff.UseGlobalWithdrawFee = s.UseGlobalWithdrawFee
		}
	}
	if action == "deposit" && !eff.Web3DepositEnabled {
		return &pb.ListActionNetworksResp{Success: true, Items: items}, nil
	}
	if action == "withdraw" && !eff.Web3WithdrawEnabled {
		return &pb.ListActionNetworksResp{Success: true, Items: items}, nil
	}

	// Load enabled chains dictionary (chain table with status=1).
	enabledChains := []model.ChainModel{}
	chainByCode := map[string]model.ChainModel{}
	if l.svcCtx.ChainRepository != nil {
		rows, _ := l.svcCtx.ChainRepository.ListEnabledChains(l.ctx)
		enabledChains = rows
		for _, c := range rows {
			cc := normalizeCode(c.Name)
			if cc != "" {
				chainByCode[cc] = c
			}
		}
	}

	// Load asset-chain mappings (currency_chain_settings).
	mappings := []model.CurrencyChainSettingsModel{}
	if l.svcCtx.CurrencyChainSettingsRepository != nil {
		rows, _ := l.svcCtx.CurrencyChainSettingsRepository.ListByAssetCode(l.ctx, assetCode)
		mappings = rows
	}

	// Helper: load effective fee rules for withdraw.
	// 优先级：链级别全局配置 > 币种全局配置 > 单独费率规则
	loadEffectiveFeeRules := func(chainCode string) []model.CurrencyWithdrawFeeRuleModel {
		chainCode = normalizeCode(chainCode)
		var rules []model.CurrencyWithdrawFeeRuleModel

		// 步骤1：如果开启了"使用全局"，优先查询链级别的全局配置（如 ETH、BSC、TRON 等）
		if eff.UseGlobalWithdrawFee && l.svcCtx.CurrencyGlobalWithdrawFeeRuleRepository != nil {
			if gr, _ := l.svcCtx.CurrencyGlobalWithdrawFeeRuleRepository.ListByChain(l.ctx, chainCode); len(gr) > 0 {
				for _, g := range gr {
					rules = append(rules, model.CurrencyWithdrawFeeRuleModel{
						RuleType:  g.RuleType,
						Value:     g.Value,
						MinFee:    g.MinFee,
						MaxFee:    g.MaxFee,
						MinAmount: g.MinAmount,
						MaxAmount: g.MaxAmount,
						Enabled:   g.Enabled,
						SortOrder: g.SortOrder,
					})
				}
				// 如果找到链级别全局配置，直接返回，不再查币种配置
				return rules
			}
		}

		// 步骤2：如果链级别全局配置不存在，再查询币种的全局费率规则
		if eff.UseGlobalWithdrawFee && l.svcCtx.CurrencyGlobalWithdrawFeeRuleRepository != nil {
			if gr, _ := l.svcCtx.CurrencyGlobalWithdrawFeeRuleRepository.ListByAsset(l.ctx, assetCode); len(gr) > 0 {
				for _, g := range gr {
					rules = append(rules, model.CurrencyWithdrawFeeRuleModel{
						RuleType:  g.RuleType,
						Value:     g.Value,
						MinFee:    g.MinFee,
						MaxFee:    g.MaxFee,
						MinAmount: g.MinAmount,
						MaxAmount: g.MaxAmount,
						Enabled:   g.Enabled,
						SortOrder: g.SortOrder,
					})
				}
				return rules
			}
		}

		// 步骤3：如果全局配置都不存在，查询单独的费率规则配置（per-asset-chain）作为回退
		if l.svcCtx.CurrencyWithdrawFeeRuleRepository != nil {
			if rr, _ := l.svcCtx.CurrencyWithdrawFeeRuleRepository.ListByAssetChain(l.ctx, assetCode, chainCode); len(rr) > 0 {
				rules = rr
			}
		}

		return rules
	}

	// Helper: get chain-specific ETA (with i18n support).
	getChainEta := func(chainName string) string {
		switch strings.ToUpper(chainName) {
		case "TRON":
			return i18n.T(l.ctx, "CHAIN_ETA_1_MINUTE", nil)
		case "BSC":
			return i18n.T(l.ctx, "CHAIN_ETA_1_MINUTE", nil)
		case "ETH":
			return i18n.T(l.ctx, "CHAIN_ETA_1_MINUTE", nil)
		case "BTC":
			return i18n.T(l.ctx, "CHAIN_ETA_30_MINUTES", nil)
		default:
			return i18n.T(l.ctx, "CHAIN_ETA_UNKNOWN", nil)
		}
	}

	// Helper: build network item.
	buildItem := func(chain model.ChainModel, minAmountText, feeText string) *pb.ActionNetworkItem {
		chainName := chain.Network
		if chainName == "" {
			chainName = chain.Name
		}
		iconUrl := strings.TrimSpace(chain.IconUrl)
		return &pb.ActionNetworkItem{
			Chain:         chain.Name,
			ChainName:     chainName,
			IconUrl:       iconUrl,
			Eta:           getChainEta(chain.Name),
			MinAmountText: minAmountText,
			FeeText:       feeText,
		}
	}

	// Build items based on currency_chain_settings mappings.
	if len(mappings) > 0 {
		for _, m := range mappings {
			// Check mapping status: deleted_at IS NULL AND status=1
			if m.IsDeleted() || m.Status != 1 {
				continue
			}
			cc := normalizeCode(m.ChainCode)
			// Check chain exists and is enabled (chain.status=1)
			ch, ok := chainByCode[cc]
			if !ok {
				continue
			}
			// Check per-action enablement in mapping
			if action == "deposit" && !m.DepositEnabled {
				continue
			}
			if action == "withdraw" && !m.WithdrawEnabled {
				continue
			}

			// Build min amount text and fee text
			var minAmountValue string
			feeText := ""

			if action == "withdraw" {
				// 先查询规则，然后使用规则计算 minAmount 和 feeText，避免重复查询
				rules := loadEffectiveFeeRules(cc)
				minAmountValue = GetEffectiveWithdrawMinAmountWithRules(rules, m.MinWithdrawAmount)
				feeText = feeTextForRules(assetCode, rules)
				if feeText == "" {
					feeText = "0 " + assetCode
				}
			} else if action == "deposit" {
				minAmountValue = GetEffectiveDepositMinAmount(m.MinDepositAmount, assetCode)
			} else {
				minAmountValue = "0.01"
			}
			minAmount := minAmountValue + " " + assetCode

			items = append(items, buildItem(ch, minAmount, feeText))
		}
	} else {
		// Backward compatible: no mapping configured -> return all enabled chains.
		for _, c := range enabledChains {
			// 使用公共方法获取有效最小金额（没有 currency_chain_settings 配置时，传 nil）
			var minAmountValue string
			feeText := ""

			if action == "withdraw" {
				// 先查询规则，然后使用规则计算 minAmount 和 feeText，避免重复查询
				rules := loadEffectiveFeeRules(c.Name)
				minAmountValue = GetEffectiveWithdrawMinAmountWithRules(rules, nil)
				feeText = feeTextForRules(assetCode, rules)
				if feeText == "" {
					feeText = "0 " + assetCode
				}
			} else if action == "deposit" {
				minAmountValue = GetEffectiveDepositMinAmount(nil, assetCode)
			} else {
				minAmountValue = "0.01"
			}
			minAmount := minAmountValue + " " + assetCode

			items = append(items, buildItem(c, minAmount, feeText))
		}
	}

	return &pb.ListActionNetworksResp{Success: true, Items: items}, nil
}

// listAllEnabledChains 返回所有启用的链（当 asset 参数为空时调用）
func (l *ListActionNetworksLogic) listAllEnabledChains(action string) (*pb.ListActionNetworksResp, error) {
	items := []*pb.ActionNetworkItem{}

	// Load all enabled chains (chain table with status=1)
	if l.svcCtx.ChainRepository == nil {
		return &pb.ListActionNetworksResp{Success: true, Items: items}, nil
	}

	enabledChains, err := l.svcCtx.ChainRepository.ListEnabledChains(l.ctx)
	if err != nil || len(enabledChains) == 0 {
		return &pb.ListActionNetworksResp{Success: true, Items: items}, nil
	}

	// Helper: get chain-specific ETA (with i18n support).
	getChainEta := func(chainName string) string {
		switch strings.ToUpper(chainName) {
		case "TRON":
			return i18n.T(l.ctx, "CHAIN_ETA_1_MINUTE", nil)
		case "BSC":
			return i18n.T(l.ctx, "CHAIN_ETA_1_MINUTE", nil)
		case "ETH":
			return i18n.T(l.ctx, "CHAIN_ETA_1_MINUTE", nil)
		case "BTC":
			return i18n.T(l.ctx, "CHAIN_ETA_30_MINUTES", nil)
		default:
			return i18n.T(l.ctx, "CHAIN_ETA_UNKNOWN", nil)
		}
	}

	// Build items for all enabled chains (without asset-specific filtering)
	for _, chain := range enabledChains {
		chainName := chain.Network
		if chainName == "" {
			chainName = chain.Name
		}
		iconUrl := strings.TrimSpace(chain.IconUrl)

		item := &pb.ActionNetworkItem{
			Chain:     chain.Name,
			ChainName: chainName,
			IconUrl:   iconUrl,
			Eta:       getChainEta(chain.Name),
			// 当没有指定资产时，不显示最小金额和手续费信息
			MinAmountText: "",
			FeeText:       "",
		}

		items = append(items, item)
	}

	return &pb.ListActionNetworksResp{Success: true, Items: items}, nil
}
