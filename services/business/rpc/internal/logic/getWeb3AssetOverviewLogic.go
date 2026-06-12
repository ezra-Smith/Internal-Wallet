package logic

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"sync"
	"time"

	"internalwallet/common/mq"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/model"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type GetWeb3AssetOverviewLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetWeb3AssetOverviewLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetWeb3AssetOverviewLogic {
	return &GetWeb3AssetOverviewLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetWeb3AssetOverviewLogic) GetWeb3AssetOverview(in *pb.GetWeb3AssetOverviewReq) (*pb.GetWeb3AssetOverviewResp, error) {
	startTime := time.Now()

	// 验证 address
	address := strings.TrimSpace(in.Address)
	if address == "" {
		return nil, errx.Web3AddressRequired()
	}

	// 获取可选的 network 参数
	network := strings.TrimSpace(in.Network)

	// 兜底登记（监控地址）：
	// - 该接口是“公开查询”，但客户端通常会携带鉴权；为了避免“链上转入但永远不落库”的情况，
	//   这里将 (network,address) 作为一个 manual monitor 写入 `address_monitors`，并发布 Kafka 事件给 chainsync
	//   实现近实时生效（定时全量刷新兜底）。
	// - 为降低被滥用的风险：仅在 network 可解析为 ETH/BSC/TRON 时启用兜底；network 为空时仅按地址格式推断。
	l.ensureAddressMonitorFallback(address, network)

	// 公开查询：直接根据地址查询资产余额（不需要身份验证）
	// 从 web3_user_address_balances 表查询该地址的余额记录
	// network 参数为空则查询所有网络，否则只查询指定网络
	if l.svcCtx.Web3UserAddressBalanceRepository == nil {
		l.Errorf("Web3UserAddressBalanceRepository is nil, service not properly initialized")
		return nil, errx.ServiceNotAvailable("web3 balance service")
	}
	dbStart := time.Now()
	dbCtx, dbCancel := context.WithTimeout(context.Background(), 5*time.Second)
	balances, err := l.svcCtx.Web3UserAddressBalanceRepository.FindByAddress(dbCtx, address, network)
	dbCancel()

	if network != "" {
		l.Infof("[性能] 数据库查询余额耗时: %v, address=%s, network=%s", time.Since(dbStart), address, network)
	} else {
		l.Infof("[性能] 数据库查询余额耗时: %v, address=%s", time.Since(dbStart), address)
	}

	if err != nil {
		l.Errorf("查询地址余额失败: address=%s, network=%s, error=%v", address, network, err)
		return nil, errx.Web3AddressQueryFailed()
	}

	// 如果没有余额记录，尝试从链上查询
	if len(balances) == 0 {
		if network != "" {
			l.Infof("地址在数据库中没有余额记录，将从链上查询: address=%s, network=%s, 已耗时=%v", address, network, time.Since(startTime))
		} else {
			l.Infof("地址在数据库中没有余额记录，将从链上查询: address=%s, 已耗时=%v", address, time.Since(startTime))
		}
		return l.queryFromChain(address, network)
	}

	// 检查地址是否被列入黑名单（通过查询地址表）
	addrCheckCtx, addrCheckCancel := context.WithTimeout(context.Background(), 3*time.Second)
	addrRecord, err := l.svcCtx.Web3UserAddressRepository.FindByAddress(addrCheckCtx, address)
	addrCheckCancel()
	if err == nil && addrRecord.IsBlacklisted {
		return nil, errx.Web3AddressBlacklisted()
	}

	// 如果显式指定了 network，则倾向返回“更实时”的链上余额：
	// - DB 中余额记录可能是占位/未同步（last_synced_at 为空）
	// - 或者同步时间过旧（例如刚收到链上转账）
	// 这能避免“USDT 链上有余额，但接口长期显示 0”被旧数据卡住。
	if network != "" {
		// NOTE: TRON token balances may be unavailable on some nodes (no constant contract support).
		// Keep a longer freshness window to avoid repeatedly overwriting correct DB balances with 0 placeholders.
		balanceFreshWindow := 30 * time.Second
		if strings.ToUpper(strings.TrimSpace(network)) == "TRON" {
			balanceFreshWindow = 10 * time.Minute
		}
		now := time.Now()
		normalized := strings.ToUpper(strings.TrimSpace(network))

		// Refresh strategy:
		// - If ANY balance row for this network was synced recently, trust DB and skip refresh.
		// - Only refresh when ALL rows are stale (or have never been synced).
		hasAnyRow := false
		hasFreshRow := false
		for _, b := range balances {
			// 只针对当前 network 的记录判断新鲜度
			bChain := strings.ToUpper(strings.TrimSpace(b.ChainCode))
			if bChain == "" {
				bChain = strings.ToUpper(strings.TrimSpace(b.Network))
			}
			if bChain != normalized {
				continue
			}
			hasAnyRow = true

			if b.LastSyncedAt != nil && now.Sub(*b.LastSyncedAt) <= balanceFreshWindow {
				hasFreshRow = true
				break
			}
		}

		if hasAnyRow && !hasFreshRow {
			l.Infof("余额记录不新鲜，触发链上刷新: address=%s, network=%s", address, normalized)
			chainResp, chainErr := l.queryFromChain(address, normalized)

			// 链上查询成功判断标准：
			// 1. 无错误
			// 2. 有响应
			// 3. 链上资产数 >= 数据库资产数（避免因账户未激活导致链上只返回 TRX 占位符而丢失 USDT 等代币）
			dbAssetCount := len(balances)
			chainAssetCount := int32(0)
			if chainResp != nil {
				chainAssetCount = chainResp.TokenCount
			}

			if chainErr == nil && chainResp != nil && chainAssetCount >= int32(dbAssetCount) {
				return chainResp, nil
			}

			// 链上查询失败或返回的资产数少于数据库，回退使用数据库余额
			l.Infof("链上刷新不完整，回退使用数据库余额: address=%s, network=%s, chainErr=%v, dbAssets=%d, chainAssets=%d",
				address, normalized, chainErr, dbAssetCount, chainAssetCount)
		}
	}

	// 构建资产列表
	var assetItems []*pb.Web3AssetItem
	// BalanceUSDRaw 现在是 string (DECIMAL(65,0))，使用 decimal 进行累加
	totalBalanceUSDRaw := decimal.Zero

	// 创建 asset_code 到 index 的映射，用于批量获取资产信息
	assetCodeSet := make(map[string]bool)
	for _, balance := range balances {
		assetCodeSet[balance.AssetCode] = true
	}

	// 过滤掉 web3_asset_display_enabled = 0 的资产
	// 批量查询该地址涉及的链上的 web3_asset_display_enabled 配置
	chainCodesSet := make(map[string]bool)
	for _, b := range balances {
		chainCode := strings.ToUpper(strings.TrimSpace(b.ChainCode))
		if chainCode == "" {
			chainCode = strings.ToUpper(strings.TrimSpace(b.Network))
		}
		if chainCode != "" {
			chainCodesSet[chainCode] = true
		}
	}

	// 从 currency_chain_settings 查询 web3_asset_display_enabled 配置
	web3DisplayMap := make(map[string]bool) // key: "ASSET_CODE:CHAIN_CODE", value: web3_asset_display_enabled
	if l.svcCtx.CurrencyChainSettingsRepository != nil {
		for chainCode := range chainCodesSet {
			settingsCtx, settingsCancel := context.WithTimeout(context.Background(), 3*time.Second)
			settings, _ := l.svcCtx.CurrencyChainSettingsRepository.ListWeb3SupportedAssetsByChain(settingsCtx, chainCode)
			settingsCancel()
			for _, s := range settings {
				key := fmt.Sprintf("%s:%s", strings.ToUpper(s.AssetCode), strings.ToUpper(s.ChainCode))
				web3DisplayMap[key] = true
			}
		}
	}

	// 过滤掉未启用 web3 display 的资产
	filteredBalances := make([]*model.Web3UserAddressBalanceModel, 0, len(balances))
	for _, b := range balances {
		chainCode := strings.ToUpper(strings.TrimSpace(b.ChainCode))
		if chainCode == "" {
			chainCode = strings.ToUpper(strings.TrimSpace(b.Network))
		}
		key := fmt.Sprintf("%s:%s", strings.ToUpper(b.AssetCode), chainCode)
		if web3DisplayMap[key] {
			filteredBalances = append(filteredBalances, b)
		}
	}
	balances = filteredBalances

	// 从 accounting 服务批量获取资产信息（Logo 等）- 使用并发查询优化性能
	assetInfoMap := make(map[string]*pb.AcctAsset)
	if l.svcCtx.AccountingRpc != nil {
		var assetInfoMutex sync.Mutex
		var assetWg sync.WaitGroup

		for assetCode := range assetCodeSet {
			assetWg.Add(1)
			go func(code string) {
				defer assetWg.Done()

				// 先尝试从 Redis 缓存获取（使用独立 context）
				if l.svcCtx.RedisClient != nil {
					cacheKey := fmt.Sprintf("asset:info:%s", code)
					redisCtx, redisCancel := context.WithTimeout(context.Background(), 2*time.Second)
					cached, err := l.svcCtx.RedisClient.Get(redisCtx, cacheKey).Result()
					redisCancel()
					if err == nil && cached != "" {
						// 简单缓存：只缓存 IconUrl
						assetInfoMutex.Lock()
						assetInfoMap[code] = &pb.AcctAsset{
							Code:    code,
							IconUrl: cached,
						}
						assetInfoMutex.Unlock()
						return
					}
				}

				// 缓存未命中，从 AccountingRpc 查询（使用独立 context）
				rpcCtx, rpcCancel := context.WithTimeout(context.Background(), 5*time.Second)
				resp, err := l.svcCtx.AccountingRpc.GetAsset(rpcCtx, &pb.GetAssetRequest{Code: code})
				rpcCancel()
				if err == nil && resp.Success && resp.Item != nil {
					assetInfoMutex.Lock()
					assetInfoMap[code] = resp.Item
					assetInfoMutex.Unlock()

					// 写入 Redis 缓存（1小时）
					if l.svcCtx.RedisClient != nil && resp.Item.IconUrl != "" {
						cacheKey := fmt.Sprintf("asset:info:%s", code)
						cacheCtx, cacheCancel := context.WithTimeout(context.Background(), 1*time.Second)
						l.svcCtx.RedisClient.Set(cacheCtx, cacheKey, resp.Item.IconUrl, 1*time.Hour)
						cacheCancel()
					}
				}
			}(assetCode)
		}

		assetWg.Wait()
	}

	for _, balance := range balances {
		// 累加总估值（BalanceUSDRaw 是 string，需要转换为 decimal）
		if balanceUSDRaw, err := decimal.NewFromString(balance.BalanceUSDRaw); err == nil {
			totalBalanceUSDRaw = totalBalanceUSDRaw.Add(balanceUSDRaw)
		}

		// 从 accounting 获取 Logo
		iconUrl := ""
		if assetInfo, ok := assetInfoMap[balance.AssetCode]; ok {
			iconUrl = assetInfo.IconUrl
		}

		// 从 market 服务获取价格和小时K线数据（使用独立 context）
		priceUsd := ""
		priceChange24h := ""
		sparklineSVG := ""

		if l.svcCtx.RedisClient != nil {
			// 使用独立的 context 避免超时
			priceCtx, priceCancel := context.WithTimeout(context.Background(), 2*time.Second)
			// 获取当前价格（USD价格，而不是USDT价格）
			if price, ok := GetAssetPriceUSD(priceCtx, l.svcCtx.RedisClient, balance.AssetCode); ok {
				priceUsd = price.String()
			}

			// 获取过去24小时的小时K线数据并生成SVG
			hourlyPoints, err := GetHourlySparkline(priceCtx, l.svcCtx.RedisClient, balance.AssetCode, 24)
			priceCancel()
			if err == nil && len(hourlyPoints) > 0 {
				svg, changePercent := GenerateSVGSparkline(hourlyPoints, 60, 20)
				sparklineSVG = svg
				priceChange24h = changePercent
			}
		}

		// 精度分离：decimals 用于展示（asset.Precision=6），contract_decimals 用于交易（token_decimals=18）
		isTron := strings.ToUpper(balance.Network) == "TRON"
		isNative := balance.TokenAddress == nil || strings.TrimSpace(*balance.TokenAddress) == ""

		// 展示精度：优先使用 asset.Precision（如 USDT=6）
		var displayDecimals int32 = int32(balance.TokenDecimals)
		if isNative {
			if assetInfo, ok := assetInfoMap[balance.AssetCode]; ok && assetInfo.Precision > 0 {
				displayDecimals = assetInfo.Precision
			} else if isTron {
				displayDecimals = 6
			} else if displayDecimals <= 0 {
				displayDecimals = 18
			}
		} else {
			// Token：优先用 asset.Precision
			if assetInfo, ok := assetInfoMap[balance.AssetCode]; ok && assetInfo.Precision > 0 {
				displayDecimals = assetInfo.Precision
			} else if displayDecimals <= 0 {
				displayDecimals = 18
			}
		}

		// 链上合约精度：用于客户端构造交易
		// ⚠️ 不能直接使用 balance.TokenDecimals，可能不准确
		// 应该从 currency_chain_settings 查询或使用默认值
		contractDecimals := int32(getDefaultDecimals(balance.ChainCode, isNative))

		// 尝试从 currency_chain_settings 获取准确的 token_decimals
		if l.svcCtx.CurrencyChainSettingsRepository != nil {
			settingCtx, settingCancel := context.WithTimeout(context.Background(), 2*time.Second)
			setting, err := l.svcCtx.CurrencyChainSettingsRepository.FindByAssetChain(settingCtx, balance.AssetCode, balance.ChainCode)
			settingCancel()
			if err == nil && setting != nil && setting.TokenDecimals != nil && *setting.TokenDecimals > 0 {
				contractDecimals = *setting.TokenDecimals
			}
		}

		// 获取原始余额
		balanceRawOriginal := balance.BalanceRaw
		if balanceRawOriginal == "" || balanceRawOriginal == "0" {
			balanceRawOriginal = balance.Balance
		}

		// 确定 DB 存储的精度
		dbDecimals := balance.TokenDecimals
		if dbDecimals <= 0 {
			dbDecimals = 18
		}

		// Balance：展示精度下的 raw（与 decimals 一致）
		balanceRaw := balanceRawOriginal
		if int32(dbDecimals) != displayDecimals && balanceRawOriginal != "" && balanceRawOriginal != "0" {
			if converted := convertBalanceToPrecision(balanceRawOriginal, dbDecimals, int(displayDecimals)); converted != "" {
				balanceRaw = converted
			}
		}

		// Amount：链上精度的 raw（客户端直接用于构造交易）
		amount := balanceRawOriginal
		if int32(dbDecimals) != contractDecimals && balanceRawOriginal != "" && balanceRawOriginal != "0" {
			if converted := convertBalanceToPrecision(balanceRawOriginal, dbDecimals, int(contractDecimals)); converted != "" {
				amount = converted
			}
		}

		// Network 字段统一使用链码（大写），避免出现 Tron/TRON 等大小写不一致导致的重复展示
		normalizedNetwork := strings.ToUpper(strings.TrimSpace(balance.ChainCode))
		if normalizedNetwork == "" {
			normalizedNetwork = strings.ToUpper(strings.TrimSpace(balance.Network))
		}

		assetItem := &pb.Web3AssetItem{
			Asset:            balance.AssetCode,
			Network:          normalizedNetwork,
			ChainId:          balance.ChainID,
			Address:          balance.WalletAddress,
			Balance:          balanceRaw, // 展示精度的 raw，与 decimals 一致
			BalanceUsd:       balance.BalanceUSD,
			IconUrl:          iconUrl,
			PriceChange_24H:  priceChange24h,
			PriceUsd:         priceUsd,
			Sparkline:        []string{sparklineSVG},
			IsPrimary:        false,
			Decimals:         displayDecimals,  // 展示精度（asset.Precision，如 USDT=6）
			ContractDecimals: contractDecimals, // 链上精度（用于交易，如 BSC USDT=18）
			Amount:           amount,           // 使用 contract_decimals 计算
		}

		assetItems = append(assetItems, assetItem)
	}

	// 补充显示支持的币种列表（即使余额为0也要显示）
	// 根据地址类型或 network 参数确定要查询的链
	chainsToSupplement := l.determineChainsForAddress(address, network)
	existingAssetMap := make(map[string]bool) // 记录已存在的资产（asset_code + chain_code）
	for _, item := range assetItems {
		key := fmt.Sprintf("%s:%s", item.Asset, strings.ToUpper(strings.TrimSpace(item.Network)))
		existingAssetMap[key] = true
	}

	// 如果用户显式指定了 network，但数据库只命中了部分资产（典型：只有主币/只有USD估值>0的资产），
	// 继续用“0余额占位项”会把真实链上余额展示成 0。此时直接触发一次链上刷新，返回真实余额并回写数据库。
	if network != "" && l.svcCtx.CurrencyChainSettingsRepository != nil && l.svcCtx.ChainRpc != nil {
		needRefreshFromChain := false
		for _, chainInfo := range chainsToSupplement {
			supplementCtx, supplementCancel := context.WithTimeout(context.Background(), 3*time.Second)
			settings, err := l.svcCtx.CurrencyChainSettingsRepository.ListWeb3SupportedAssetsByChain(supplementCtx, chainInfo.chainCode)
			supplementCancel()
			if err != nil || len(settings) == 0 {
				continue
			}

			for _, setting := range settings {
				key := fmt.Sprintf("%s:%s", setting.AssetCode, chainInfo.chainCode)
				if !existingAssetMap[key] {
					needRefreshFromChain = true
					break
				}
			}
			if needRefreshFromChain {
				break
			}
		}

		if needRefreshFromChain {
			l.Infof("数据库余额不完整，触发链上刷新: address=%s, network=%s", address, network)
			return l.queryFromChain(address, network)
		}
	}

	// 为每个链查询支持的币种，补充余额为0的资产
	for _, chainInfo := range chainsToSupplement {
		if l.svcCtx.CurrencyChainSettingsRepository == nil {
			continue
		}

		// 查询该链支持的 Web3 资产
		supplementCtx, supplementCancel := context.WithTimeout(context.Background(), 3*time.Second)
		settings, err := l.svcCtx.CurrencyChainSettingsRepository.ListWeb3SupportedAssetsByChain(supplementCtx, chainInfo.chainCode)
		supplementCancel()
		if err != nil {
			l.Debugf("查询支持的币种失败: chain=%s error=%v", chainInfo.chainCode, err)
			continue
		}

		// 为每个支持的币种创建资产项（如果不存在）
		for _, setting := range settings {
			key := fmt.Sprintf("%s:%s", setting.AssetCode, chainInfo.chainCode)
			if existingAssetMap[key] {
				// 已存在，跳过
				continue
			}

			// 创建余额为0的资产项
			zeroBalanceItem := l.createZeroBalanceAssetItem(address, chainInfo, setting)
			if zeroBalanceItem != nil {
				assetItems = append(assetItems, zeroBalanceItem)
				existingAssetMap[key] = true
			}
		}
	}

	// 计算总估值（分转美元）
	// 将 decimal 转换为 int64（如果值超过 int64 范围，会溢出）
	totalBalanceUSDRawInt64 := int64(0)
	if totalBalanceUSDRaw.IsPositive() {
		// 尝试转换为 int64，如果失败则使用 0
		if val, err := strconv.ParseInt(totalBalanceUSDRaw.String(), 10, 64); err == nil {
			totalBalanceUSDRawInt64 = val
		}
	}
	totalBalanceUSD := formatUSDFromRaw(totalBalanceUSDRawInt64)

	l.Infof("成功获取 Web3 地址资产: address=%s, token_count=%d, total_usd=%s",
		address, len(assetItems), totalBalanceUSD)

	return &pb.GetWeb3AssetOverviewResp{
		Success:         true,
		Message:         "ok",
		Address:         address,
		TotalBalanceUsd: totalBalanceUSD,
		Items:           assetItems,
		TokenCount:      int32(len(assetItems)),
	}, nil
}

func (l *GetWeb3AssetOverviewLogic) ensureAddressMonitorFallback(address, network string) {
	if l == nil || l.svcCtx == nil || l.svcCtx.DB == nil {
		return
	}
	address = strings.TrimSpace(address)
	network = strings.TrimSpace(network)
	if address == "" {
		return
	}

	chainCode, chainStr := l.resolveChainForMonitor(address, network)
	if chainCode == "" || chainStr == "" {
		return
	}

	normalizedAddr := l.normalizeMonitoredAddressForChain(chainCode, address)
	if normalizedAddr == "" {
		return
	}

	monitorID := fmt.Sprintf("manual_%s_%s", chainStr, normalizedAddr)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	db := l.svcCtx.DB.WithContext(ctx)

	var existing model.AddressMonitorModel
	err := db.Unscoped().Where("monitor_id = ?", monitorID).First(&existing).Error
	createdOrUpdated := false
	switch {
	case err == nil:
		// Skip if already in desired state (avoid noisy updates on hot endpoints).
		needUpdate := false
		if existing.Active != true {
			needUpdate = true
		}
		if strings.TrimSpace(existing.Chain) != chainStr {
			needUpdate = true
		}
		if strings.TrimSpace(existing.Address) != normalizedAddr {
			needUpdate = true
		}
		if strings.TrimSpace(existing.MonitorType) != "all" {
			needUpdate = true
		}
		if strings.TrimSpace(existing.Priority) != "normal" {
			needUpdate = true
		}
		if strings.TrimSpace(existing.Tag) != "web3_overview_fallback" {
			needUpdate = true
		}
		if existing.DeletedAt.Valid {
			needUpdate = true
		}

		if needUpdate {
			updates := map[string]interface{}{
				"chain":        chainStr,
				"address":      normalizedAddr,
				"monitor_type": "all",
				"priority":     "normal",
				"tag":          "web3_overview_fallback",
				"active":       true,
				"deleted_at":   gorm.Expr("NULL"),
				"updated_at":   time.Now().Local(),
			}
			// Best-effort only; never fail the main request.
			if updateErr := db.Unscoped().Model(&model.AddressMonitorModel{}).Where("monitor_id = ?", monitorID).Updates(updates).Error; updateErr != nil {
				l.Errorf("兜底登记 address_monitors 更新失败: monitor_id=%s chain=%s addr=%s err=%v", monitorID, chainStr, normalizedAddr, updateErr)
				return
			}
			createdOrUpdated = true
			l.Infof("兜底登记 address_monitors 已更新: monitor_id=%s chain=%s addr=%s", monitorID, chainStr, normalizedAddr)
		}
	case errors.Is(err, gorm.ErrRecordNotFound):
		m := &model.AddressMonitorModel{
			MonitorID:   monitorID,
			Chain:       chainStr,
			Address:     normalizedAddr,
			MonitorType: "all",
			Priority:    "normal",
			Tag:         "web3_overview_fallback",
			Active:      true,
			Metadata: model.AddressMonitorMetadata{
				"reason": "business.web3_asset_overview.fallback",
			},
		}
		// Best-effort only; if duplicate (race), it's fine.
		if createErr := db.Create(m).Error; createErr != nil {
			l.Errorf("兜底登记 address_monitors 新增失败: monitor_id=%s chain=%s addr=%s err=%v", monitorID, chainStr, normalizedAddr, createErr)
			return
		}
		createdOrUpdated = true
		l.Infof("兜底登记 address_monitors 已新增: monitor_id=%s chain=%s addr=%s", monitorID, chainStr, normalizedAddr)
	default:
		l.Errorf("兜底登记 address_monitors 查询失败: monitor_id=%s err=%v", monitorID, err)
		return
	}

	// Publish near-real-time event (chainsync consumes it).
	// Only publish when we actually created/updated, to avoid spamming Kafka on hot endpoints.
	if createdOrUpdated {
		l.svcCtx.PublishAddressMonitorEvent(l.ctx, mq.AddressMonitorEvent{
			Action:  mq.AddressMonitorActionUpsert,
			Source:  mq.AddressMonitorSourceManual,
			Chain:   chainCode,      // canonical: ETH/BSC/TRON
			Address: normalizedAddr, // keep normalized for deterministic matching
			Reason:  "business.web3_asset_overview.fallback",
			Metadata: map[string]string{
				"monitor_id": monitorID,
			},
		})
	}
}

func (l *GetWeb3AssetOverviewLogic) resolveChainForMonitor(address, network string) (chainCodeUpper string, chainStrForDB string) {
	// Prefer explicit network when provided.
	n := strings.ToUpper(strings.TrimSpace(network))
	switch n {
	case "ETH", "ETHEREUM":
		return "ETH", "ethereum"
	case "BSC", "BNB", "BNB SMART CHAIN":
		return "BSC", "bsc"
	case "TRON", "TRX":
		return "TRON", "tron"
	case "":
		// infer by address format
	default:
		// unsupported network label
	}

	addr := strings.TrimSpace(address)
	if strings.HasPrefix(strings.ToLower(addr), "0x") && len(addr) == 42 {
		return "ETH", "ethereum"
	}
	if strings.HasPrefix(addr, "T") && len(addr) >= 34 && len(addr) <= 36 {
		return "TRON", "tron"
	}
	return "", ""
}

func (l *GetWeb3AssetOverviewLogic) normalizeMonitoredAddressForChain(chainCodeUpper string, address string) string {
	addr := strings.TrimSpace(address)
	if addr == "" {
		return ""
	}
	switch strings.ToUpper(strings.TrimSpace(chainCodeUpper)) {
	case "ETH", "BSC":
		if strings.HasPrefix(strings.ToLower(addr), "0x") {
			return strings.ToLower(addr)
		}
		// Non-0x: keep original
		return addr
	default:
		// TRON/base58 is case-sensitive; keep original
		return addr
	}
}

// queryFromChain 从链上实时查询地址资产（地址不在数据库中时）
// 使用并发查询优化性能，避免超时
// network 参数为可选，如果指定则只查询该网络的资产
func (l *GetWeb3AssetOverviewLogic) queryFromChain(address string, network string) (*pb.GetWeb3AssetOverviewResp, error) {
	chainQueryStart := time.Now()
	defer func() {
		if network != "" {
			l.Infof("[性能] 链上查询总耗时: %v, address=%s, network=%s", time.Since(chainQueryStart), address, network)
		} else {
			l.Infof("[性能] 链上查询总耗时: %v, address=%s", time.Since(chainQueryStart), address)
		}
	}()

	if l.svcCtx.ChainRpc == nil {
		l.Infof("ChainRpc not configured, cannot query from chain: address=%s", address)
		// 返回成功但无余额，避免接口报错
		return &pb.GetWeb3AssetOverviewResp{
			Success:         true,
			Message:         "",
			Address:         address,
			TotalBalanceUsd: "0",
			Items:           []*pb.Web3AssetItem{},
			TokenCount:      0,
		}, nil
	}

	// 判断地址类型并尝试查询常见链
	address = strings.TrimSpace(address)
	network = strings.TrimSpace(strings.ToUpper(network)) // 标准化 network 参数

	var chainsToQuery []struct {
		chainType pb.ChainRpcType
		chainCode string
		assetCode string
		chainID   int64
	}

	// EVM 地址（0x开头，42字符）
	if strings.HasPrefix(address, "0x") && len(address) == 42 {
		allEvmChains := []struct {
			chainType pb.ChainRpcType
			chainCode string
			assetCode string
			chainID   int64
		}{
			{pb.ChainRpcType_CHAIN_TYPE_ETHEREUM, "ETH", "ETH", 1},
			{pb.ChainRpcType_CHAIN_TYPE_BSC, "BSC", "BNB", 56},
		}

		// 如果指定了 network，只查询该网络
		if network != "" {
			for _, chain := range allEvmChains {
				if chain.chainCode == network || chain.assetCode == network {
					chainsToQuery = append(chainsToQuery, chain)
					break
				}
			}
			// 如果指定的 network 不在支持列表中，返回空结果
			if len(chainsToQuery) == 0 {
				l.Infof("指定的 network 不支持该地址类型: network=%s, address=%s", network, address)
				return &pb.GetWeb3AssetOverviewResp{
					Success:         true,
					Message:         "",
					Address:         address,
					TotalBalanceUsd: "0",
					Items:           []*pb.Web3AssetItem{},
					TokenCount:      0,
				}, nil
			}
		} else {
			// 未指定 network，查询所有 EVM 链
			chainsToQuery = allEvmChains
		}
	} else if strings.HasPrefix(address, "T") && len(address) >= 34 && len(address) <= 36 {
		// TRON 地址（T开头，34-36字符）
		tronChain := struct {
			chainType pb.ChainRpcType
			chainCode string
			assetCode string
			chainID   int64
		}{pb.ChainRpcType_CHAIN_TYPE_TRON, "TRON", "TRX", 728126428}

		// 如果指定了 network，检查是否匹配 TRON
		if network != "" {
			if network == "TRON" || network == "TRX" {
				chainsToQuery = append(chainsToQuery, tronChain)
			} else {
				l.Infof("指定的 network 不匹配 TRON 地址: network=%s, address=%s", network, address)
				return &pb.GetWeb3AssetOverviewResp{
					Success:         true,
					Message:         "",
					Address:         address,
					TotalBalanceUsd: "0",
					Items:           []*pb.Web3AssetItem{},
					TokenCount:      0,
				}, nil
			}
		} else {
			// 未指定 network，查询 TRON
			chainsToQuery = append(chainsToQuery, tronChain)
		}
	} else {
		// 未知地址格式，返回错误
		return &pb.GetWeb3AssetOverviewResp{
			Success:         true,
			Message:         "",
			Address:         address,
			TotalBalanceUsd: "0",
			Items:           []*pb.Web3AssetItem{},
			TokenCount:      0,
		}, nil
	}

	// 从数据库查询每个链上启用的 Web3 资产（包括主币和代币）
	// 使用 currency_chain_settings 表作为数据源
	// 使用并发查询提升性能，避免超时
	var assetItems []*pb.Web3AssetItem
	// queriedItems only includes assets successfully fetched from ChainRPC.
	// We must NOT persist placeholder "0 balance" items derived from config, otherwise it will overwrite real balances.
	var queriedItems []*pb.Web3AssetItem
	var assetItemsMutex sync.Mutex
	totalBalanceUSDRaw := decimal.Zero

	// 用于等待所有 goroutine 完成
	var wg sync.WaitGroup
	// 限制并发数为 5，避免过多并发请求导致超时
	semaphore := make(chan struct{}, 5)

	// 创建总体超时控制（25秒，留出5秒缓冲给 API Gateway 的30秒超时）
	overallCtx, overallCancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer overallCancel()

	for _, chainInfo := range chainsToQuery {
		// 检查总体超时
		select {
		case <-overallCtx.Done():
			l.Errorf("总体查询超时，停止查询: address=%s, elapsed=%v", address, time.Since(chainQueryStart))
			goto queryComplete
		default:
		}

		// 从数据库查询该链上所有启用的 Web3 资产
		var settings []model.CurrencyChainSettingsModel
		if l.svcCtx.CurrencyChainSettingsRepository != nil {
			// 使用独立的 context 进行数据库查询，避免继承超时
			dbCtx, dbCancel := context.WithTimeout(context.Background(), 3*time.Second)
			settings, _ = l.svcCtx.CurrencyChainSettingsRepository.ListWeb3SupportedAssetsByChain(dbCtx, chainInfo.chainCode)
			dbCancel()
		}

		// 如果没有从数据库查询到配置，使用默认主币
		if len(settings) == 0 {
			// 检查总体超时
			select {
			case <-overallCtx.Done():
				continue
			default:
			}

			// 查询主币余额（作为后备方案）
			// 创建独立的超时 context（10秒），避免使用继承的 context 超时
			chainRpcCtx, chainRpcCancel := context.WithTimeout(context.Background(), 10*time.Second)
			nativeResp, err := l.svcCtx.ChainRpc.GetBalance(chainRpcCtx, &pb.GetBalanceReq{
				Chain:   chainInfo.chainType,
				Address: address,
			})
			chainRpcCancel()
			if err == nil && nativeResp != nil && nativeResp.Success {
				balanceStr := strings.TrimSpace(nativeResp.Balance)
				if balanceStr == "" {
					balanceStr = "0"
				}

				balanceDecimal, err := decimal.NewFromString(balanceStr)
				if err == nil && balanceDecimal.IsPositive() {
					usdCents := int64(0)
					if nativeResp.BalanceUsd > 0 {
						usdCents = int64(nativeResp.BalanceUsd * 100)
					}
					if usdCents < 0 {
						usdCents = 0
					}
					totalBalanceUSDRaw = totalBalanceUSDRaw.Add(decimal.NewFromInt(usdCents))
					balanceUSD := formatUSDFromRaw(usdCents)

					// 使用缓存获取资产图标（使用独立 context）
					iconUrl := l.getAssetIconUrlWithCache(context.Background(), chainInfo.assetCode)

					// 从 market 服务获取价格和小时K线数据（使用独立 context）
					priceUsd := ""
					priceChange24h := ""
					sparklineSVG := ""

					if l.svcCtx.RedisClient != nil {
						// 使用独立的 context 避免超时
						redisCtx, redisCancel := context.WithTimeout(context.Background(), 2*time.Second)
						// 获取当前价格（USD价格，而不是USDT价格）
						if price, ok := GetAssetPriceUSD(redisCtx, l.svcCtx.RedisClient, chainInfo.assetCode); ok {
							priceUsd = price.String()
						}

						// 获取过去24小时的小时K线数据并生成SVG
						hourlyPoints, err := GetHourlySparkline(redisCtx, l.svcCtx.RedisClient, chainInfo.assetCode, 24)
						if err == nil && len(hourlyPoints) > 0 {
							svg, changePercent := GenerateSVGSparkline(hourlyPoints, 60, 20)
							sparklineSVG = svg
							priceChange24h = changePercent
						}
						redisCancel()
					}

					// 获取主币精度（链上精度）
					chainDecimals := getDefaultDecimals(chainInfo.chainCode, true)

					// 展示精度：优先使用 asset.Precision
					displayDecimals := chainDecimals
					if l.svcCtx.AccountingRpc != nil {
						assetCtx, assetCancel := context.WithTimeout(context.Background(), 2*time.Second)
						assetResp, err := l.svcCtx.AccountingRpc.GetAsset(assetCtx, &pb.GetAssetRequest{Code: chainInfo.assetCode})
						assetCancel()
						if err == nil && assetResp != nil && assetResp.Success && assetResp.Item != nil && assetResp.Item.Precision > 0 {
							displayDecimals = int(assetResp.Item.Precision)
							// 将链上 raw 转为展示精度 raw
							if displayDecimals != chainDecimals && balanceStr != "" && balanceStr != "0" {
								if converted := convertBalanceToPrecision(balanceStr, chainDecimals, displayDecimals); converted != "" {
									balanceStr = converted
								}
							}
						}
					}

					// 将余额转换为链上精度的 raw（客户端直接用于构造交易）
					nativeAmount := balanceStr
					// 如果当前 balanceStr 不是链上精度，需要转换
					if displayDecimals != chainDecimals && balanceStr != "" && balanceStr != "0" {
						if converted := convertBalanceToPrecision(balanceStr, displayDecimals, chainDecimals); converted != "" {
							nativeAmount = converted
						}
					}

					assetItems = append(assetItems, &pb.Web3AssetItem{
						Asset:            chainInfo.assetCode,
						Network:          chainInfo.chainCode,
						ChainId:          chainInfo.chainID,
						Address:          address,
						Balance:          balanceStr,
						BalanceUsd:       balanceUSD,
						IconUrl:          iconUrl,
						PriceChange_24H:  priceChange24h,
						PriceUsd:         priceUsd,
						Sparkline:        []string{sparklineSVG}, // SVG作为第一个元素
						IsPrimary:        false,
						Decimals:         int32(displayDecimals), // 展示精度（asset.Precision）
						ContractDecimals: int32(chainDecimals),   // 链上精度（用于交易）
						Amount:           nativeAmount,           // 使用 contract_decimals 计算
					})
				}
			}
			continue
		}

		// 遍历数据库中的资产配置，使用并发查询余额（避免串行超时）
		// 限制查询数量，优先查询主币和常见代币（避免查询过多导致超时）
		maxAssetsToQuery := 20 // 最多查询20个资产
		assetsToQuery := settings
		if len(settings) > maxAssetsToQuery {
			// 优先查询主币和常见代币（USDT, USDC）
			priorityAssets := []string{"USDT", "USDC"}
			prioritySettings := []model.CurrencyChainSettingsModel{}
			otherSettings := []model.CurrencyChainSettingsModel{}

			for _, s := range settings {
				isPriority := false
				for _, priority := range priorityAssets {
					if s.AssetCode == priority {
						isPriority = true
						break
					}
				}
				if isPriority {
					prioritySettings = append(prioritySettings, s)
				} else {
					otherSettings = append(otherSettings, s)
				}
			}

			// 合并：优先资产 + 其他资产（最多20个）
			remainingSlots := maxAssetsToQuery - len(prioritySettings)
			if remainingSlots < 0 {
				remainingSlots = 0
			}
			if remainingSlots > len(otherSettings) {
				remainingSlots = len(otherSettings)
			}
			assetsToQuery = append(prioritySettings, otherSettings[:remainingSlots]...)
			l.Infof("资产数量过多，限制查询数量: total=%d, querying=%d, address=%s", len(settings), len(assetsToQuery), address)
		}

		for _, setting := range assetsToQuery {
			// 检查总体超时
			select {
			case <-overallCtx.Done():
				l.Errorf("总体查询超时，跳过剩余资产: address=%s", address)
				goto queryComplete
			default:
			}

			wg.Add(1)
			go func(s model.CurrencyChainSettingsModel, chain struct {
				chainType pb.ChainRpcType
				chainCode string
				assetCode string
				chainID   int64
			}) {
				defer wg.Done()

				// 限流：获取信号量
				select {
				case semaphore <- struct{}{}:
					defer func() { <-semaphore }()
				case <-overallCtx.Done():
					return // 总体超时，直接返回
				}

				queryStart := time.Now()
				var balanceStr string
				var usdCents int64
				var assetCode string
				var finalDecimals int         // 最终用于返回的展示精度
				var finalContractDecimals int // 最终用于返回的链上精度

				// 判断是主币还是代币（contract_address 为空表示主币）
				isNative := s.ContractAddress == nil || strings.TrimSpace(*s.ContractAddress) == ""
				// 检查总体超时
				select {
				case <-overallCtx.Done():
					return
				default:
				}

				if isNative {
					// 主币：使用 GetBalance
					// 创建独立的超时 context（10秒），避免使用继承的 context 超时
					chainRpcCtx, chainRpcCancel := context.WithTimeout(context.Background(), 10*time.Second)
					resp, err := l.svcCtx.ChainRpc.GetBalance(chainRpcCtx, &pb.GetBalanceReq{
						Chain:   chain.chainType,
						Address: address,
					})
					chainRpcCancel()
					l.Infof("[性能] GetBalance耗时: %v, chain=%s, asset=%s", time.Since(queryStart), chain.chainCode, s.AssetCode)
					if err != nil {
						l.Debugf("查询主币余额失败: chain=%s asset=%s address=%s error=%v", chain.chainCode, s.AssetCode, address, err)
						return
					}
					if resp == nil || !resp.Success {
						return
					}

					balanceStr = strings.TrimSpace(resp.Balance)
					if balanceStr == "" {
						balanceStr = "0"
					}
					if resp.BalanceUsd > 0 {
						usdCents = int64(resp.BalanceUsd * 100)
					}
					assetCode = s.AssetCode
					// 主币链上精度
					chainDecimals := getDefaultDecimals(chain.chainCode, true)
					finalContractDecimals = chainDecimals

					// 展示精度：优先使用 asset.Precision
					finalDecimals = chainDecimals
					if l.svcCtx.AccountingRpc != nil {
						assetCtx, assetCancel := context.WithTimeout(context.Background(), 2*time.Second)
						assetResp, err := l.svcCtx.AccountingRpc.GetAsset(assetCtx, &pb.GetAssetRequest{Code: assetCode})
						assetCancel()
						if err == nil && assetResp != nil && assetResp.Success && assetResp.Item != nil && assetResp.Item.Precision > 0 {
							displayDecimals := int(assetResp.Item.Precision)
							// 将链上 raw 转为展示精度 raw
							if displayDecimals != chainDecimals && balanceStr != "" && balanceStr != "0" {
								if converted := convertBalanceToPrecision(balanceStr, chainDecimals, displayDecimals); converted != "" {
									balanceStr = converted
								}
							}
							finalDecimals = displayDecimals
						}
					}
				} else {
					// 代币：使用 GetTokenBalance
					contractAddr := strings.TrimSpace(*s.ContractAddress)
					// 创建独立的超时 context（10秒），避免使用继承的 context 超时
					chainRpcCtx, chainRpcCancel := context.WithTimeout(context.Background(), 10*time.Second)
					resp, err := l.svcCtx.ChainRpc.GetTokenBalance(chainRpcCtx, &pb.GetTokenBalanceReq{
						Chain:         chain.chainType,
						Address:       address,
						TokenContract: contractAddr,
					})
					chainRpcCancel()
					l.Infof("[性能] GetTokenBalance耗时: %v, chain=%s, asset=%s", time.Since(queryStart), chain.chainCode, s.AssetCode)
					if err != nil {
						l.Debugf("查询代币余额失败: chain=%s asset=%s contract=%s address=%s error=%v", chain.chainCode, s.AssetCode, contractAddr, address, err)
						return
					}
					if resp == nil || !resp.Success {
						return
					}

					balanceStr = strings.TrimSpace(resp.Balance)
					if balanceStr == "" {
						balanceStr = "0"
					}
					if resp.BalanceUsd > 0 {
						usdCents = int64(resp.BalanceUsd * 100)
					}
					// 优先使用 ChainRPC 返回的代币符号，否则使用数据库中的 asset_code
					assetCode = strings.ToUpper(strings.TrimSpace(resp.TokenSymbol))
					if assetCode == "" || assetCode == "UNKNOWN" {
						assetCode = s.AssetCode
					}

					// 链上精度（用于 contract_decimals）
					// ⚠️ 不能直接使用 resp.TokenDecimals（ChainRPC 可能返回错误值）
					// 应该从 currency_chain_settings 查询准确值
					chainDecimals := getDefaultDecimals(chain.chainCode, false)

					// 从 currency_chain_settings 获取准确的 token_decimals
					if l.svcCtx.CurrencyChainSettingsRepository != nil {
						settingCtx, settingCancel := context.WithTimeout(context.Background(), 2*time.Second)
						setting, err := l.svcCtx.CurrencyChainSettingsRepository.FindByAssetChain(settingCtx, s.AssetCode, chain.chainCode)
						settingCancel()
						if err == nil && setting != nil && setting.TokenDecimals != nil && *setting.TokenDecimals > 0 {
							chainDecimals = int(*setting.TokenDecimals)
						}
					}

					// 展示精度：优先用 asset.Precision
					displayDecimals := chainDecimals
					if l.svcCtx.AccountingRpc != nil {
						assetCtx, assetCancel := context.WithTimeout(context.Background(), 2*time.Second)
						assetResp, err := l.svcCtx.AccountingRpc.GetAsset(assetCtx, &pb.GetAssetRequest{Code: s.AssetCode})
						assetCancel()
						if err == nil && assetResp != nil && assetResp.Success && assetResp.Item != nil && assetResp.Item.Precision > 0 {
							displayDecimals = int(assetResp.Item.Precision)
							// 将链上 raw 转为展示精度 raw
							if displayDecimals != chainDecimals && balanceStr != "" && balanceStr != "0" {
								if converted := convertBalanceToPrecision(balanceStr, chainDecimals, displayDecimals); converted != "" {
									balanceStr = converted
								}
							}
						}
					}
					finalDecimals = displayDecimals
					finalContractDecimals = chainDecimals
				}

				// 解析余额（即使为0也要显示）
				balanceDecimal, err := decimal.NewFromString(balanceStr)
				if err != nil {
					// 余额格式错误，跳过
					return
				}
				// 注意：即使余额为0，也要显示资产项（显示支持的币种列表）

				// 使用缓存获取资产图标（使用独立 context）
				iconUrl := l.getAssetIconUrlWithCache(context.Background(), assetCode)

				// 从 market 服务获取价格和小时K线数据（使用独立 context）
				priceUsd := ""
				priceChange24h := ""
				sparklineSVG := ""
				var priceDecimal decimal.Decimal

				if l.svcCtx.RedisClient != nil {
					// 使用独立的 context 避免超时
					redisCtx, redisCancel := context.WithTimeout(context.Background(), 2*time.Second)
					// 获取当前价格（USD价格，而不是USDT价格）
					if price, ok := GetAssetPriceUSD(redisCtx, l.svcCtx.RedisClient, assetCode); ok {
						priceUsd = price.String()
						priceDecimal = price
					}

					// 获取过去24小时的小时K线数据并生成SVG
					hourlyPoints, err := GetHourlySparkline(redisCtx, l.svcCtx.RedisClient, assetCode, 24)
					if err == nil && len(hourlyPoints) > 0 {
						svg, changePercent := GenerateSVGSparkline(hourlyPoints, 60, 20)
						sparklineSVG = svg
						priceChange24h = changePercent
					}
					redisCancel()
				}

				// 计算 USD 价值：优先使用 ChainRPC 返回的 BalanceUsd，否则使用 balance * price_usd
				if usdCents <= 0 && !priceDecimal.IsZero() {
					// 需要自己计算：balance * price_usd
					// 使用已确定的精度（finalDecimals 已经优先使用了后台配置的精度）
					decimals := finalDecimals

					// 将最小单位余额转换为实际余额（除以 10^decimals）
					decimalsDivisor := decimal.NewFromInt(1).Shift(int32(decimals))
					actualBalance := balanceDecimal.Div(decimalsDivisor)

					// 计算 USD 价值：actualBalance * price_usd
					balanceUsdValue := actualBalance.Mul(priceDecimal)
					// 转换为分（cents）
					usdCents = balanceUsdValue.Mul(decimal.NewFromInt(100)).IntPart()
					if usdCents < 0 {
						usdCents = 0
					}
				} else if usdCents < 0 {
					// 既没有 ChainRPC 的 BalanceUsd，也没有价格，设置为 0
					usdCents = 0
				}

				balanceUSD := formatUSDFromRaw(usdCents)

				// 将余额转换为链上精度的 raw（客户端直接用于构造交易）
				tokenAmount := balanceStr
				// 如果当前 balanceStr 不是链上精度，需要转换
				if finalDecimals != finalContractDecimals && balanceStr != "" && balanceStr != "0" {
					if converted := convertBalanceToPrecision(balanceStr, finalDecimals, finalContractDecimals); converted != "" {
						tokenAmount = converted
					}
				}

				assetItem := &pb.Web3AssetItem{
					Asset:            assetCode,
					Network:          chain.chainCode,
					ChainId:          chain.chainID,
					Address:          address,
					Balance:          balanceStr,
					BalanceUsd:       balanceUSD,
					IconUrl:          iconUrl,
					PriceChange_24H:  priceChange24h,
					PriceUsd:         priceUsd,
					Sparkline:        []string{sparklineSVG}, // SVG作为第一个元素
					IsPrimary:        false,
					Decimals:         int32(finalDecimals),         // 展示精度（asset.Precision，如 USDT=6）
					ContractDecimals: int32(finalContractDecimals), // 链上精度（用于交易，如 BSC USDT=18）
					Amount:           tokenAmount,                  // 使用 contract_decimals 计算
				}

				// 线程安全地添加结果
				assetItemsMutex.Lock()
				assetItems = append(assetItems, assetItem)
				queriedItems = append(queriedItems, assetItem)
				totalBalanceUSDRaw = totalBalanceUSDRaw.Add(decimal.NewFromInt(usdCents))
				assetItemsMutex.Unlock()
			}(setting, chainInfo)
		}
	}

queryComplete:
	// 等待所有并发查询完成，但设置超时
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// 所有查询完成
		l.Infof("[性能] 所有链上资产查询完成，共查询 %d 个资产", len(assetItems))
	case <-overallCtx.Done():
		// 总体超时，返回已有结果
		l.Errorf("[性能] 查询超时，返回已有结果: address=%s, found_assets=%d, elapsed=%v",
			address, len(assetItems), time.Since(chainQueryStart))
	}

	// 补充显示支持的币种列表（即使余额为0也要显示）
	// 记录已存在的资产（asset_code + chain_code）
	existingAssetMap := make(map[string]bool)
	for _, item := range assetItems {
		key := fmt.Sprintf("%s:%s", item.Asset, strings.ToUpper(strings.TrimSpace(item.Network)))
		existingAssetMap[key] = true
	}

	// 为每个链查询支持的币种，补充余额为0的资产
	for _, chainInfo := range chainsToQuery {
		if l.svcCtx.CurrencyChainSettingsRepository == nil {
			continue
		}

		// 查询该链支持的 Web3 资产
		supplementCtx, supplementCancel := context.WithTimeout(context.Background(), 3*time.Second)
		settings, err := l.svcCtx.CurrencyChainSettingsRepository.ListWeb3SupportedAssetsByChain(supplementCtx, chainInfo.chainCode)
		supplementCancel()
		if err != nil {
			l.Debugf("查询支持的币种失败: chain=%s error=%v", chainInfo.chainCode, err)
			continue
		}

		// 为每个支持的币种创建资产项（如果不存在）
		for _, setting := range settings {
			key := fmt.Sprintf("%s:%s", setting.AssetCode, chainInfo.chainCode)
			if existingAssetMap[key] {
				// 已存在，跳过
				continue
			}

			// 创建余额为0的资产项
			zeroBalanceItem := l.createZeroBalanceAssetItem(address, chainInfo, setting)
			if zeroBalanceItem != nil {
				assetItems = append(assetItems, zeroBalanceItem)
				existingAssetMap[key] = true
			}
		}
	}

	// 计算总估值
	totalBalanceUSDRawInt64 := int64(0)
	if totalBalanceUSDRaw.IsPositive() {
		if val, err := strconv.ParseInt(totalBalanceUSDRaw.String(), 10, 64); err == nil {
			totalBalanceUSDRawInt64 = val
		}
	}
	totalBalanceUSD := formatUSDFromRaw(totalBalanceUSDRawInt64)

	if len(assetItems) == 0 {
		// 地址合法但暂无支持的币种配置，正常返回 200
		return &pb.GetWeb3AssetOverviewResp{
			Success:         true,
			Message:         "ok",
			Address:         address,
			TotalBalanceUsd: "0",
			Items:           []*pb.Web3AssetItem{},
			TokenCount:      0,
		}, nil
	}

	l.Infof("成功从链上查询地址资产: address=%s, token_count=%d, total_usd=%s",
		address, len(assetItems), totalBalanceUSD)

	// 异步保存链上查询到的余额数据到数据库（下次查询直接从数据库返回）
	if len(assetItems) > 0 && l.svcCtx.Web3UserAddressBalanceRepository != nil {
		// Persist only successfully queried ChainRPC items (exclude config-generated placeholders).
		if len(queriedItems) > 0 {
			go l.saveChainBalancesToDB(address, queriedItems)
		}
	}

	return &pb.GetWeb3AssetOverviewResp{
		Success:         true,
		Message:         "ok",
		Address:         address,
		TotalBalanceUsd: totalBalanceUSD,
		Items:           assetItems,
		TokenCount:      int32(len(assetItems)),
	}, nil
}

// saveChainBalancesToDB 异步保存链上查询到的余额数据到数据库
// 这样下次查询可以直接从数据库返回，不用再等链上 RPC
func (l *GetWeb3AssetOverviewLogic) saveChainBalancesToDB(address string, items []*pb.Web3AssetItem) {
	// 使用独立的 context，避免被请求 context 取消
	saveCtx, saveCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer saveCancel()

	savedCount := 0
	for _, item := range items {
		// 解析 balance_usd 为 cents
		balanceUsdCents := int64(0)
		if item.BalanceUsd != "" && item.BalanceUsd != "0" {
			if usdDecimal, err := decimal.NewFromString(item.BalanceUsd); err == nil {
				balanceUsdCents = usdDecimal.Mul(decimal.NewFromInt(100)).IntPart()
			}
		}

		// 解析 balance_raw
		balanceRaw := "0"
		if item.Balance != "" {
			balanceRaw = item.Balance
		}

		// 判断是否为原生代币
		tokenType := "native"
		var tokenAddress *string
		// 从 item.Decimals 获取精度（这是从 ChainRPC 获取的准确精度）
		tokenDecimals := int(item.Decimals)
		if tokenDecimals <= 0 {
			// 如果精度无效，使用默认值
			tokenDecimals = 18
		}

		// 根据 asset 和 network 判断是否为原生代币
		nativeAssets := map[string]string{
			"ETH": "ETH", "BSC": "BNB", "TRON": "TRX",
			"POLYGON": "MATIC", "ARBITRUM": "ETH", "OPTIMISM": "ETH",
		}
		isNative := nativeAssets[item.Network] == item.Asset

		if !isNative {
			tokenType = "erc20" // 或 trc20，根据链判断
			if item.Network == "TRON" {
				tokenType = "trc20"
			}
			// 需要从 currency_chain_settings 获取合约地址
			if l.svcCtx.CurrencyChainSettingsRepository != nil {
				settingCtx, settingCancel := context.WithTimeout(context.Background(), 2*time.Second)
				setting, err := l.svcCtx.CurrencyChainSettingsRepository.FindByAssetChain(settingCtx, item.Asset, item.Network)
				settingCancel()
				if err == nil && setting.ContractAddress != nil {
					tokenAddress = setting.ContractAddress
				}
			}
		}

		now := time.Now()

		// 构建余额记录
		balanceModel := &model.Web3UserAddressBalanceModel{
			AssetCode:        item.Asset,
			ChainCode:        item.Network,
			WalletAddress:    address,
			Network:          item.Network,
			ChainID:          item.ChainId,
			Balance:          item.Balance,
			BalanceRaw:       balanceRaw,
			BalanceUSD:       item.BalanceUsd,
			BalanceUSDRaw:    fmt.Sprintf("%d", balanceUsdCents),
			TokenAddress:     tokenAddress,
			TokenDecimals:    tokenDecimals,
			TokenType:        tokenType,
			IsVerified:       0,
			IsSpam:           0,
			BalanceUpdatedAt: &now,
			LastSyncedAt:     &now,
			SyncStatus:       "synced",
		}

		// Upsert 到数据库
		if err := l.svcCtx.Web3UserAddressBalanceRepository.Upsert(saveCtx, balanceModel); err != nil {
			l.Errorf("保存链上余额到数据库失败: address=%s, asset=%s, chain=%s, error=%v",
				address, item.Asset, item.Network, err)
		} else {
			savedCount++
		}
	}

	l.Infof("链上余额数据已保存到数据库: address=%s, saved=%d/%d", address, savedCount, len(items))
}

// formatUSDFromRaw 将 balance_usd_raw（分）转换为 USD 字符串
// balance_usd_raw 使用分作为单位（1美元 = 100分）
func formatUSDFromRaw(raw int64) string {
	if raw == 0 {
		return "0"
	}
	// 转换为美元
	dollars := float64(raw) / 100.0
	// 格式化为字符串，保留两位小数
	formatted := strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", dollars), "0"), ".")
	return formatted
}

// getAssetIconUrlWithCache 从 accounting 服务获取资产图标 URL（带 Redis 缓存）
// 使用独立的 context 避免超时
func (l *GetWeb3AssetOverviewLogic) getAssetIconUrlWithCache(ctx context.Context, assetCode string) string {
	if l.svcCtx.AccountingRpc == nil {
		return ""
	}

	// 先尝试从 Redis 缓存获取
	if l.svcCtx.RedisClient != nil {
		cacheKey := fmt.Sprintf("asset:info:%s", assetCode)
		cached, err := l.svcCtx.RedisClient.Get(ctx, cacheKey).Result()
		if err == nil && cached != "" {
			return cached
		}
	}

	// 缓存未命中，从 AccountingRpc 查询（使用独立超时 context）
	assetCtx, assetCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer assetCancel()

	assetResp, err := l.svcCtx.AccountingRpc.GetAsset(assetCtx, &pb.GetAssetRequest{Code: assetCode})
	if err == nil && assetResp != nil && assetResp.Success && assetResp.Item != nil {
		iconUrl := assetResp.Item.IconUrl

		// 写入 Redis 缓存（1小时）
		if l.svcCtx.RedisClient != nil && iconUrl != "" {
			cacheKey := fmt.Sprintf("asset:info:%s", assetCode)
			// 使用独立的 context 进行缓存写入
			cacheCtx, cacheCancel := context.WithTimeout(context.Background(), 1*time.Second)
			l.svcCtx.RedisClient.Set(cacheCtx, cacheKey, iconUrl, 1*time.Hour)
			cacheCancel()
		}

		return iconUrl
	}

	return ""
}

// networkToChainRpcType 将 network 字符串转换为 ChainRpcType
func (l *GetWeb3AssetOverviewLogic) networkToChainRpcType(network string) pb.ChainRpcType {
	switch strings.ToUpper(network) {
	case "ETH", "ETHEREUM":
		return pb.ChainRpcType_CHAIN_TYPE_ETHEREUM
	case "BSC":
		return pb.ChainRpcType_CHAIN_TYPE_BSC
	case "TRON", "TRX":
		return pb.ChainRpcType_CHAIN_TYPE_TRON
	default:
		return pb.ChainRpcType_CHAIN_TYPE_UNSPECIFIED
	}
}

// convertBalanceToPrecision 将余额从 fromDecimals 精度转换为 toDecimals 精度下的 raw 字符串（整除截断）。
// 用于统一返回「资产精度」下的 balance，例如链上 18 位 raw 转为资产 6 位 raw。
func convertBalanceToPrecision(balanceRaw string, fromDecimals, toDecimals int) string {
	balanceRaw = strings.TrimSpace(balanceRaw)
	if balanceRaw == "" || fromDecimals < 0 || toDecimals < 0 {
		return ""
	}
	bi, ok := new(big.Int).SetString(balanceRaw, 10)
	if !ok || bi.Sign() < 0 {
		return ""
	}
	fromDiv := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(fromDecimals)), nil)
	toMul := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(toDecimals)), nil)
	bi.Mul(bi, toMul)
	bi.Div(bi, fromDiv)
	return bi.String()
}

// getTokenDecimalsFromCache 从 Redis 缓存获取代币精度
// 返回 0 表示缓存未命中
func (l *GetWeb3AssetOverviewLogic) getTokenDecimalsFromCache(network, contractAddress string) int32 {
	if l.svcCtx.RedisClient == nil || contractAddress == "" {
		return 0
	}

	cacheKey := fmt.Sprintf("token:decimals:%s:%s", strings.ToUpper(network), strings.ToLower(contractAddress))
	cacheCtx, cacheCancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cacheCancel()

	cached, err := l.svcCtx.RedisClient.Get(cacheCtx, cacheKey).Result()
	if err != nil || cached == "" {
		return 0
	}

	decimals, err := strconv.ParseInt(cached, 10, 32)
	if err != nil || decimals <= 0 {
		return 0
	}

	return int32(decimals)
}

// setTokenDecimalsToCache 将代币精度写入 Redis 缓存
// 缓存 24 小时（代币精度不会变化）
func (l *GetWeb3AssetOverviewLogic) setTokenDecimalsToCache(network, contractAddress string, decimals int32) {
	if l.svcCtx.RedisClient == nil || contractAddress == "" || decimals <= 0 {
		return
	}

	cacheKey := fmt.Sprintf("token:decimals:%s:%s", strings.ToUpper(network), strings.ToLower(contractAddress))
	cacheCtx, cacheCancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cacheCancel()

	// 缓存 24 小时，代币精度不会变化
	err := l.svcCtx.RedisClient.Set(cacheCtx, cacheKey, fmt.Sprintf("%d", decimals), 24*time.Hour).Err()
	if err != nil {
		l.Debugf("写入代币精度缓存失败: network=%s contract=%s decimals=%d error=%v",
			network, contractAddress, decimals, err)
	}
}

// updateTokenDecimalsInDB 异步更新数据库中的代币精度
func (l *GetWeb3AssetOverviewLogic) updateTokenDecimalsInDB(assetCode, chainCode, walletAddress string, decimals int) {
	if assetCode == "" || chainCode == "" || walletAddress == "" || decimals <= 0 {
		return
	}

	updateCtx, updateCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer updateCancel()

	db := l.svcCtx.Web3UserAddressBalanceRepository.GetDB()
	if db == nil {
		return
	}

	err := db.WithContext(updateCtx).
		Model(&model.Web3UserAddressBalanceModel{}).
		Where("asset_code = ? AND chain_code = ? AND wallet_address = ? AND deleted_at IS NULL",
			assetCode, chainCode, walletAddress).
		Update("token_decimals", decimals).Error

	if err != nil {
		l.Errorf("更新代币精度失败: asset=%s chain=%s address=%s decimals=%d error=%v",
			assetCode, chainCode, walletAddress, decimals, err)
	} else {
		l.Infof("已更新代币精度: asset=%s chain=%s address=%s decimals=%d",
			assetCode, chainCode, walletAddress, decimals)
	}
}

// determineChainsForAddress 根据地址类型或 network 参数确定要查询的链
func (l *GetWeb3AssetOverviewLogic) determineChainsForAddress(address, network string) []struct {
	chainType pb.ChainRpcType
	chainCode string
	assetCode string
	chainID   int64
} {
	address = strings.TrimSpace(address)
	network = strings.TrimSpace(strings.ToUpper(network))

	var chains []struct {
		chainType pb.ChainRpcType
		chainCode string
		assetCode string
		chainID   int64
	}

	// EVM 地址（0x开头，42字符）
	if strings.HasPrefix(address, "0x") && len(address) == 42 {
		allEvmChains := []struct {
			chainType pb.ChainRpcType
			chainCode string
			assetCode string
			chainID   int64
		}{
			{pb.ChainRpcType_CHAIN_TYPE_ETHEREUM, "ETH", "ETH", 1},
			{pb.ChainRpcType_CHAIN_TYPE_BSC, "BSC", "BNB", 56},
		}

		if network != "" {
			// 如果指定了 network，只查询该网络
			for _, chain := range allEvmChains {
				if chain.chainCode == network || chain.assetCode == network {
					chains = append(chains, chain)
					break
				}
			}
		} else {
			// 未指定 network，查询所有 EVM 链
			chains = allEvmChains
		}
	} else if strings.HasPrefix(address, "T") && len(address) >= 34 && len(address) <= 36 {
		// TRON 地址（T开头，34-36字符）
		tronChain := struct {
			chainType pb.ChainRpcType
			chainCode string
			assetCode string
			chainID   int64
		}{pb.ChainRpcType_CHAIN_TYPE_TRON, "TRON", "TRX", 728126428}

		if network != "" {
			// 如果指定了 network，检查是否匹配 TRON
			if network == "TRON" || network == "TRX" {
				chains = append(chains, tronChain)
			}
		} else {
			// 未指定 network，查询 TRON
			chains = append(chains, tronChain)
		}
	}

	return chains
}

// createZeroBalanceAssetItem 创建余额为0的资产项
func (l *GetWeb3AssetOverviewLogic) createZeroBalanceAssetItem(
	address string,
	chainInfo struct {
		chainType pb.ChainRpcType
		chainCode string
		assetCode string
		chainID   int64
	},
	setting model.CurrencyChainSettingsModel,
) *pb.Web3AssetItem {
	// 从 accounting 获取资产信息（图标、展示精度）
	iconUrl := ""
	var displayDecimals int32
	if l.svcCtx.AccountingRpc != nil {
		assetCtx, assetCancel := context.WithTimeout(context.Background(), 2*time.Second)
		assetResp, err := l.svcCtx.AccountingRpc.GetAsset(assetCtx, &pb.GetAssetRequest{Code: setting.AssetCode})
		assetCancel()
		if err == nil && assetResp != nil && assetResp.Success && assetResp.Item != nil {
			iconUrl = assetResp.Item.IconUrl
			if assetResp.Item.Precision > 0 {
				displayDecimals = assetResp.Item.Precision
			}
		}
	}

	// 链上合约精度（用于交易）
	// ⚠️ 必须从 currency_chain_settings.token_decimals 获取准确值
	isNative := setting.ContractAddress == nil || strings.TrimSpace(*setting.ContractAddress) == ""
	var contractDecimals int32

	// 优先使用 setting.TokenDecimals（来自 currency_chain_settings）
	if setting.TokenDecimals != nil && *setting.TokenDecimals > 0 {
		contractDecimals = *setting.TokenDecimals
	} else {
		// 回退到默认值
		contractDecimals = int32(getDefaultDecimals(chainInfo.chainCode, isNative))
	}

	// 若展示精度未配置，使用链上精度
	if displayDecimals == 0 {
		displayDecimals = contractDecimals
	}

	// 从 market 服务获取价格（使用独立 context）
	priceUsd := ""
	priceChange24h := ""
	sparklineSVG := ""

	if l.svcCtx.RedisClient != nil {
		priceCtx, priceCancel := context.WithTimeout(context.Background(), 2*time.Second)
		// 获取当前价格（USD价格，而不是USDT价格）
		if price, ok := GetAssetPriceUSD(priceCtx, l.svcCtx.RedisClient, setting.AssetCode); ok {
			priceUsd = price.String()
		}

		// 获取过去24小时的小时K线数据并生成SVG
		hourlyPoints, err := GetHourlySparkline(priceCtx, l.svcCtx.RedisClient, setting.AssetCode, 24)
		priceCancel()
		if err == nil && len(hourlyPoints) > 0 {
			svg, changePercent := GenerateSVGSparkline(hourlyPoints, 60, 20)
			sparklineSVG = svg
			priceChange24h = changePercent
		}
	}

	return &pb.Web3AssetItem{
		Asset:            setting.AssetCode,
		Network:          chainInfo.chainCode,
		ChainId:          chainInfo.chainID,
		Address:          address,
		Balance:          "0",
		BalanceUsd:       "0",
		IconUrl:          iconUrl,
		PriceChange_24H:  priceChange24h,
		PriceUsd:         priceUsd,
		Sparkline:        []string{sparklineSVG},
		IsPrimary:        false,
		Decimals:         displayDecimals,  // 展示精度（asset.Precision，如 USDT=6）
		ContractDecimals: contractDecimals, // 链上精度（用于交易，如 BSC USDT=18）
		Amount:           "0",              // 余额为0
	}
}
