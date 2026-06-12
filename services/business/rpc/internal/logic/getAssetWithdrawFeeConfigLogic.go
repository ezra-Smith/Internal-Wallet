package logic

import (
	"context"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/model"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetAssetWithdrawFeeConfigLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetAssetWithdrawFeeConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetAssetWithdrawFeeConfigLogic {
	return &GetAssetWithdrawFeeConfigLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetAssetWithdrawFeeConfig 获取币种提现手续费规则配置
// 返回指定资产在各个链上的提现手续费规则配置（最小值、最大值）
func (l *GetAssetWithdrawFeeConfigLogic) GetAssetWithdrawFeeConfig(in *pb.GetAssetWithdrawFeeConfigReq) (*pb.GetAssetWithdrawFeeConfigResp, error) {
	items := []*pb.AssetWithdrawFeeConfigItem{}

	// 参数校验：asset 为必填
	if in == nil || strings.TrimSpace(in.Asset) == "" {
		return &pb.GetAssetWithdrawFeeConfigResp{
			Success: false,
			Message: "asset is required",
			Items:   items,
		}, nil
	}

	assetCode := normalizeCode(in.Asset)
	chainFilter := normalizeCode(in.Chain) // 可选的链过滤参数

	// 1. 验证资产是否存在且已启用
	if l.svcCtx.AccountingRpc != nil {
		accResp, err := l.svcCtx.AccountingRpc.GetAsset(l.ctx, &pb.GetAssetRequest{Code: assetCode})
		if err != nil || accResp == nil || !accResp.Success || accResp.Item == nil || accResp.Item.Status != 1 {
			return &pb.GetAssetWithdrawFeeConfigResp{
				Success: false,
				Message: "asset not found or disabled",
				Items:   items,
			}, nil
		}
	}

	// 2. 获取币种设置（用于判断是否使用全局费率）
	eff := defaultEffectiveCurrencySettings()
	if l.svcCtx.CurrencySettingsRepository != nil {
		if s, err := l.svcCtx.CurrencySettingsRepository.GetByAssetCode(l.ctx, assetCode); err == nil && s != nil {
			eff.Web3WithdrawEnabled = s.Web3WithdrawEnabled
			eff.UseGlobalWithdrawFee = s.UseGlobalWithdrawFee
		}
	}

	// 如果该币种未启用提现功能，返回空列表
	if !eff.Web3WithdrawEnabled {
		return &pb.GetAssetWithdrawFeeConfigResp{
			Success: true,
			Message: "withdraw not enabled for this asset",
			Items:   items,
		}, nil
	}

	// 3. 加载启用的链列表
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

	// 4. 加载资产-链映射配置（currency_chain_settings）
	mappings := []model.CurrencyChainSettingsModel{}
	if l.svcCtx.CurrencyChainSettingsRepository != nil {
		rows, _ := l.svcCtx.CurrencyChainSettingsRepository.ListByAssetCode(l.ctx, assetCode)
		mappings = rows
	}

	// 5. 辅助函数：加载生效的费率规则
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

	// 6. 辅助函数：处理单个链的配置
	// minWithdrawAmountFromSettings: currency_chain_settings 表中的 min_withdraw_amount
	processChain := func(chain model.ChainModel, minWithdrawAmountFromSettings *string) {
		chainCode := normalizeCode(chain.Name)

		// 如果指定了链过滤，只处理匹配的链
		if chainFilter != "" && chainCode != chainFilter {
			return
		}

		// 加载该链的费率规则
		rules := loadEffectiveFeeRules(chainCode)

		// 从规则中提取最大值（最小值使用公共方法计算）
		_, maxAmount, _ := getWithdrawAmountRangeFromRules(rules)

		chainName := chain.Network
		if chainName == "" {
			chainName = chain.Name
		}

		// 使用公共方法计算有效的最小提现金额（综合考虑费率规则和 currency_chain_settings）
		// 传入已查询的规则，避免重复查询数据库
		finalMinAmount := GetEffectiveWithdrawMinAmountWithRules(rules, minWithdrawAmountFromSettings)

		item := &pb.AssetWithdrawFeeConfigItem{
			Chain:             chain.Name,
			ChainName:         chainName,
			MinWithdrawAmount: finalMinAmount,
			MaxWithdrawAmount: "",
		}

		if maxAmount != nil {
			item.MaxWithdrawAmount = *maxAmount
		}

		items = append(items, item)
	}

	// 7. 如果有 currency_chain_settings 映射配置，优先使用
	if len(mappings) > 0 {
		for _, m := range mappings {
			// 检查映射状态：deleted_at IS NULL AND status=1
			if m.IsDeleted() || m.Status != 1 {
				continue
			}

			// 检查是否启用提现
			if !m.WithdrawEnabled {
				continue
			}

			cc := normalizeCode(m.ChainCode)
			// 检查链是否存在且已启用
			ch, ok := chainByCode[cc]
			if !ok {
				continue
			}

			// 传递 currency_chain_settings 中的 MinWithdrawAmount
			processChain(ch, m.MinWithdrawAmount)
		}
	} else {
		// 向后兼容：如果没有配置映射，返回所有启用的链
		for _, c := range enabledChains {
			// 没有配置映射时，MinWithdrawAmount 传 nil
			processChain(c, nil)
		}
	}

	return &pb.GetAssetWithdrawFeeConfigResp{
		Success: true,
		Message: "success",
		Items:   items,
	}, nil
}
