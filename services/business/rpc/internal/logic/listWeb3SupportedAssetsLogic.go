package logic

import (
	"context"
	"strconv"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListWeb3SupportedAssetsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListWeb3SupportedAssetsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListWeb3SupportedAssetsLogic {
	return &ListWeb3SupportedAssetsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListWeb3SupportedAssets 获取 Web3 支持的资产列表（根据链查询）
func (l *ListWeb3SupportedAssetsLogic) ListWeb3SupportedAssets(in *pb.ListWeb3SupportedAssetsReq) (*pb.ListWeb3SupportedAssetsResp, error) {
	// 验证 chain_code
	chainCode := strings.ToUpper(strings.TrimSpace(in.ChainCode))
	if chainCode == "" {
		return nil, errx.Web3ChainCodeRequired()
	}

	// 查询该链支持的资产列表（过滤 web3_asset_display_enabled=1）
	settings, err := l.svcCtx.CurrencyChainSettingsRepository.ListWeb3SupportedAssetsByChain(l.ctx, chainCode)
	if err != nil {
		l.Errorf("查询 Web3 支持的资产失败: chain_code=%s, error=%v", chainCode, err)
		return nil, errx.DBError()
	}

	// 批量获取资产信息（icon_url 等）
	assetCodeSet := make(map[string]bool)
	for _, setting := range settings {
		assetCodeSet[setting.AssetCode] = true
	}

	assetInfoMap := make(map[string]*pb.AcctAsset)
	if l.svcCtx.AccountingRpc != nil {
		for assetCode := range assetCodeSet {
			if item := GetAssetInfoWithCache(l.ctx, l.svcCtx, assetCode); item != nil {
				assetInfoMap[assetCode] = item
			}
		}
	}

	// 获取链信息
	chainName := l.getChainName(chainCode)
	chainID := l.getChainID(chainCode)

	// 精度过滤条件（如果指定了 precision > 0，则只返回匹配的资产）
	precisionFilter := in.Precision
	if precisionFilter < 0 {
		precisionFilter = 0 // 负数视为不过滤
	}

	// 构建返回列表
	items := make([]*pb.Web3SupportedAssetItem, 0, len(settings))
	for _, setting := range settings {
		// 从 accounting 获取资产信息（名称、图标、展示精度）
		var assetName, iconURL string
		var displayPrecision int32
		if assetInfo, ok := assetInfoMap[setting.AssetCode]; ok {
			assetName = assetInfo.Name
			iconURL = assetInfo.IconUrl
			if assetInfo.Precision > 0 {
				displayPrecision = assetInfo.Precision
			}
		} else {
			assetName = setting.AssetCode
		}

		// 链上合约精度（用于客户端构造交易）
		contractDecimals := int32(18) // 默认 EVM 18 位
		if setting.TokenDecimals != nil && *setting.TokenDecimals > 0 {
			contractDecimals = *setting.TokenDecimals
		} else {
			// 如果 token_decimals 未配置，使用链默认值
			contractDecimals = l.getDefaultPrecision(chainCode)
		}

		// 展示精度：优先使用 asset.Precision，否则使用链上精度
		if displayPrecision == 0 {
			displayPrecision = contractDecimals
		}

		// 精度过滤：如果指定了精度过滤条件，只返回匹配的资产（基于展示精度）
		if precisionFilter > 0 && displayPrecision != precisionFilter {
			continue
		}

		// 合约地址
		contractAddress := ""
		if setting.ContractAddress != nil {
			contractAddress = *setting.ContractAddress
		}

		// 最小金额
		minDepositAmount := ""
		if setting.MinDepositAmount != nil {
			minDepositAmount = *setting.MinDepositAmount
		}
		minWithdrawAmount := ""
		if setting.MinWithdrawAmount != nil {
			minWithdrawAmount = *setting.MinWithdrawAmount
		}

		item := &pb.Web3SupportedAssetItem{
			AssetCode:         setting.AssetCode,
			AssetName:         assetName,
			ChainCode:         setting.ChainCode,
			ChainName:         chainName,
			ChainId:           chainID,
			ContractAddress:   contractAddress,
			IconUrl:           iconURL,
			DepositEnabled:    setting.DepositEnabled,
			WithdrawEnabled:   setting.WithdrawEnabled,
			MinDepositAmount:  minDepositAmount,
			MinWithdrawAmount: minWithdrawAmount,
			Precision:         displayPrecision, // 展示精度（asset.Precision，如 USDT=6）
			ContractDecimals:  contractDecimals, // 链上精度（用于交易，如 BSC USDT=18）
		}

		items = append(items, item)
	}

	l.Infof("查询 Web3 支持的资产成功: chain_code=%s, count=%d", chainCode, len(items))

	return &pb.ListWeb3SupportedAssetsResp{
		Success:   true,
		Message:   "ok",
		ChainCode: chainCode,
		Items:     items,
	}, nil
}

// getChainName 获取链名称（可以从配置或数据库读取）
func (l *ListWeb3SupportedAssetsLogic) getChainName(chainCode string) string {
	// 这里可以从 ChainRepository 查询，暂时简单映射
	chainNames := map[string]string{
		"ETH":  "Ethereum",
		"BSC":  "BNB",
		"TRON": "Tron",
	}

	if name, ok := chainNames[chainCode]; ok {
		return name
	}

	return chainCode
}

// getChainID 获取链ID（从数据库查询）
func (l *ListWeb3SupportedAssetsLogic) getChainID(chainCode string) string {
	if l.svcCtx.ChainRepository == nil {
		return ""
	}
	chain, err := l.svcCtx.ChainRepository.FindByName(l.ctx, chainCode)
	if err != nil || chain == nil {
		return ""
	}
	return strconv.Itoa(chain.ChainID)
}

// getDefaultPrecision 根据链代码获取默认精度（当无法从 Accounting 获取时使用）
func (l *ListWeb3SupportedAssetsLogic) getDefaultPrecision(chainCode string) int32 {
	chainCodeUpper := strings.ToUpper(chainCode)
	switch chainCodeUpper {
	case "TRON", "TRX":
		return 6 // TRON 主币 6 位小数
	case "ETH", "ETHEREUM", "BSC", "BNB", "POLYGON":
		return 18 // EVM 链通常 18 位小数
	case "BTC", "BITCOIN":
		return 8 // Bitcoin 8 位小数
	default:
		return 18 // 默认 18 位
	}
}
