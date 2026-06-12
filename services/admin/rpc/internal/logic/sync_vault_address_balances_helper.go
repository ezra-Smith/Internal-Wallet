package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/logx"
)

// BinanceTickerKey is the Redis hash key for Binance market tickers
const BinanceTickerKey = "binance:tickers"

// BinanceTickerValue represents a ticker value from Binance Redis cache
type BinanceTickerValue struct {
	Price string `json:"price"`
	Ts    int64  `json:"ts"`
}

// SyncVaultAddressBalancesHelper 从链上同步单个 Vault 地址的所有资产余额
// 参考 getWeb3AssetOverviewLogic.go 的实现
func SyncVaultAddressBalancesHelper(ctx context.Context, svcCtx *svc.ServiceContext, logger logx.Logger, addr *model.VaultAddressModel) error {
	if addr == nil || svcCtx == nil {
		return fmt.Errorf("invalid parameters")
	}

	// 在同步前确保所有热钱包地址都在 vault_addresses 中（批量检查，只执行一次）
	// 这个调用是幂等的，不会重复添加
	logger.Info("检查并同步公司热钱包到 vault_addresses...")
	if err := EnsureCompanyHotWalletsInVault(ctx, svcCtx, logger); err != nil {
		logger.Errorf("同步热钱包到 vault_addresses 失败: %v (继续执行余额同步)", err)
		// 不阻塞余额同步流程，仅记录错误
	}

	// 1. 查询网络信息
	network, err := svcCtx.VaultNetworkRepo.FindByID(ctx, addr.NetworkID)
	if err != nil || network == nil {
		return fmt.Errorf("network not found: %w", err)
	}

	// 2. 检查是否配置了 ChainRpc
	if svcCtx.ChainRpc == nil {
		return fmt.Errorf("chainRpc not configured")
	}

	// 3. 映射链类型
	chainType := mapChainType(network.ChainType)
	if chainType == pb.ChainRpcType_CHAIN_TYPE_UNSPECIFIED {
		return fmt.Errorf("unsupported chain type: %s", network.ChainType)
	}

	chainCode := chainTypeToChainCode(network.ChainType)
	if chainCode == "" {
		return fmt.Errorf("unsupported chain code for chain_type: %s", network.ChainType)
	}

	// 4. 查询该链支持的币种配置 (currency_chain_settings)
	var settings []*model.CurrencyChainSettingsModel
	if err := svcCtx.DB.WithContext(ctx).
		Where("chain_code = ? AND status = 1", chainCode).
		Find(&settings).Error; err != nil {
		return fmt.Errorf("failed to query currency chain settings: %w", err)
	}

	if len(settings) == 0 {
		logger.Infof("No currency chain settings found for chain_code=%s, skipping balance sync", chainCode)
		return nil
	}

	// 5. 查询 Accounting 服务获取资产精度和状态
	precisionByAsset := make(map[string]int32)
	statusByAsset := make(map[string]int32)

	assetCodes := make([]string, 0, len(settings))
	seenAsset := make(map[string]struct{})
	for _, s := range settings {
		if s == nil {
			continue
		}
		code := strings.ToUpper(strings.TrimSpace(s.AssetCode))
		if code == "" {
			continue
		}
		if _, ok := seenAsset[code]; ok {
			continue
		}
		seenAsset[code] = struct{}{}
		assetCodes = append(assetCodes, code)
	}

	if len(assetCodes) > 0 && svcCtx.AccountingRpc != nil {
		for _, code := range assetCodes {
			accResp, err := svcCtx.AccountingRpc.GetAsset(ctx, &pb.GetAssetRequest{Code: code})
			if err != nil {
				logger.Errorf("Failed to query asset info from accounting: code=%s err=%v", code, err)
				continue
			}
			if accResp == nil || !accResp.Success || accResp.Item == nil {
				continue
			}
			precisionByAsset[code] = accResp.Item.Precision
			statusByAsset[code] = accResp.Item.Status
		}
	}

	// 6. 构建 token 列表 (合约地址)
	type tokenMeta struct {
		AssetCode       string
		ContractAddress string
		Precision       int32
	}
	contractAddrs := make([]string, 0, len(settings))
	byContract := make(map[string]tokenMeta, len(settings))
	for _, s := range settings {
		if s == nil {
			continue
		}
		assetCode := strings.ToUpper(strings.TrimSpace(s.AssetCode))
		if assetCode == "" || statusByAsset[assetCode] != 1 {
			continue
		}
		contract := ""
		if s.ContractAddress != nil {
			contract = strings.TrimSpace(*s.ContractAddress)
		}
		if contract == "" {
			// 原生币，后续单独处理
			continue
		}

		// 优先使用 currency_chain_settings.token_decimals（链上的实际精度）
		// 如果没有设置，则根据链和资产类型推断，最后才使用 Accounting 服务的精度
		precision := int32(18) // 默认精度
		if s.TokenDecimals != nil && *s.TokenDecimals > 0 && *s.TokenDecimals <= 30 {
			precision = *s.TokenDecimals
		} else {
			// 根据链和资产类型推断精度
			if chainCode == "BSC" {
				if assetCode == "USDC" || assetCode == "USDT" {
					precision = 18 // BSC 链上的 USDC/USDT 是 18 位精度
				} else if assetCode == "BNB" {
					precision = 18
				}
			} else if chainCode == "ETH" || chainCode == "Ethereum" {
				if assetCode == "USDC" || assetCode == "USDT" {
					precision = 6 // Ethereum 链上的 USDC/USDT 是 6 位精度
				} else if assetCode == "ETH" {
					precision = 18
				}
			} else if chainCode == "TRON" {
				if assetCode == "USDC" || assetCode == "USDT" {
					precision = 6 // TRON 链上的 USDC/USDT 是 6 位精度
				} else if assetCode == "TRX" {
					precision = 6
				}
			} else {
				// 如果无法推断，使用 Accounting 服务的精度（作为后备）
				if p, ok := precisionByAsset[assetCode]; ok && p > 0 {
					precision = p
				}
			}
		}

		contractAddrs = append(contractAddrs, contract)
		byContract[strings.ToLower(contract)] = tokenMeta{
			AssetCode:       assetCode,
			ContractAddress: contract,
			Precision:       precision,
		}
		logger.Infof("Token meta: asset=%s chain=%s contract=%s precision=%d (from token_decimals=%v)",
			assetCode, chainCode, contract, precision, s.TokenDecimals)
	}

	// 7. 确定原生币
	nativeAssetCode := ""
	expectedNative := nativeAssetCodeForChainCode(chainCode)
	for _, s := range settings {
		if s == nil {
			continue
		}
		assetCode := strings.ToUpper(strings.TrimSpace(s.AssetCode))
		if assetCode == "" || statusByAsset[assetCode] != 1 {
			continue
		}
		contract := ""
		if s.ContractAddress != nil {
			contract = strings.TrimSpace(*s.ContractAddress)
		}
		if contract != "" {
			continue
		}
		if expectedNative != "" && assetCode != expectedNative {
			continue
		}
		nativeAssetCode = assetCode
		break
	}
	if nativeAssetCode == "" && expectedNative != "" && statusByAsset[expectedNative] == 1 {
		nativeAssetCode = expectedNative
	}

	// 8. 调用 ChainSync RPC 获取链上余额
	chainSyncType := mapChainTypeForChainSync(network.ChainType)
	resp, err := svcCtx.ChainSyncClient.GetAddressBalance(ctx, &pb.GetAddressBalanceReq{
		Chain:   chainSyncType,
		Address: addr.Address,
		Tokens:  contractAddrs,
	})
	if err != nil {
		return fmt.Errorf("chainSync.GetAddressBalance failed: %w", err)
	}
	if resp == nil || !resp.Success {
		return fmt.Errorf("chainSync response not success")
	}

	// 9. 解析链上余额并写入数据库
	now := time.Now()

	// 获取资产价格（从 Redis）
	assetPrices := getAssetPricesFromRedis(ctx, svcCtx.RedisClient, assetCodes, logger)

	sumTokenUSDCents := int64(0)

	// 9.1 处理 Token 余额
	// 记录链上实际返回的 token 地址，用于后续清理不存在的 token 余额
	syncedTokenContracts := make(map[string]bool)

	logger.Infof("ChainSync returned %d token balances, expecting %d tokens from config", len(resp.TokenBalances), len(byContract))
	for _, tb := range resp.TokenBalances {
		if tb == nil {
			continue
		}
		contractLower := strings.ToLower(strings.TrimSpace(tb.TokenAddress))
		meta, ok := byContract[contractLower]
		if !ok {
			logger.Infof("Token contract not found in config: contract=%s", contractLower)
			continue
		}

		// 记录这个合约地址已经被同步
		syncedTokenContracts[strings.ToLower(meta.ContractAddress)] = true
		logger.Infof("Processing token balance from ChainSync: asset=%s contract=%s balance=%s formatted=%s",
			meta.AssetCode, meta.ContractAddress, tb.Balance, tb.FormattedBalance)

		// 优先使用 FormattedBalance，如果为空则从 Balance（原始值）计算
		formatted := strings.TrimSpace(tb.FormattedBalance)
		if formatted == "" {
			// Balance 是原始整数字符串，需要根据精度转换为格式化字符串
			rawBalance := strings.TrimSpace(tb.Balance)
			if rawBalance == "" || rawBalance == "0" {
				// 链上余额为0，也要更新数据库（设置为0），避免保留旧的余额数据
				balanceModel := &model.VaultAddressBalanceModel{
					AddressID:       addr.ID,
					NetworkID:       addr.NetworkID,
					Currency:        meta.AssetCode,
					ContractAddress: meta.ContractAddress,
					Balance:         "0",
					BalanceRaw:      0,
					BalanceUSD:      "0",
					BalanceUSDRaw:   0,
					LastSyncedAt:    &now,
					CreatedAt:       &now,
					UpdatedAt:       &now,
				}
				if err := svcCtx.VaultAddressBalanceRepo.Upsert(ctx, balanceModel); err != nil {
					logger.Errorf("Failed to upsert zero balance: address_id=%d asset=%s err=%v", addr.ID, meta.AssetCode, err)
				}
				continue
			}
			// 将原始余额转换为格式化余额（除以 10^precision）
			formatted = formatRawBalanceWithPrecision(rawBalance, meta.Precision)
			logger.Infof("Formatted token balance from raw: asset=%s raw=%s formatted=%s precision=%d",
				meta.AssetCode, rawBalance, formatted, meta.Precision)
		}
		if formatted == "" || formatted == "0" {
			// 链上余额为0，也要更新数据库（设置为0），避免保留旧的余额数据
			balanceModel := &model.VaultAddressBalanceModel{
				AddressID:       addr.ID,
				NetworkID:       addr.NetworkID,
				Currency:        meta.AssetCode,
				ContractAddress: meta.ContractAddress,
				Balance:         "0",
				BalanceRaw:      0,
				BalanceUSD:      "0",
				BalanceUSDRaw:   0,
				LastSyncedAt:    &now,
				CreatedAt:       &now,
				UpdatedAt:       &now,
			}
			if err := svcCtx.VaultAddressBalanceRepo.Upsert(ctx, balanceModel); err != nil {
				logger.Errorf("Failed to upsert zero balance: address_id=%d asset=%s err=%v", addr.ID, meta.AssetCode, err)
			}
			continue
		}
		amtRes, err := parseAmountToRaw(formatted, meta.Precision, true)
		if err != nil {
			logger.Errorf("Failed to parse token balance: asset=%s formatted=%s err=%v", meta.AssetCode, formatted, err)
			continue
		}

		// 再次检查：如果解析后的原始余额为0，也要更新数据库为0
		if amtRes.Raw == 0 {
			logger.Infof("Parsed balance is 0, updating database to 0: asset=%s contract=%s", meta.AssetCode, meta.ContractAddress)
			balanceModel := &model.VaultAddressBalanceModel{
				AddressID:       addr.ID,
				NetworkID:       addr.NetworkID,
				Currency:        meta.AssetCode,
				ContractAddress: meta.ContractAddress,
				Balance:         "0",
				BalanceRaw:      0,
				BalanceUSD:      "0",
				BalanceUSDRaw:   0,
				LastSyncedAt:    &now,
				CreatedAt:       &now,
				UpdatedAt:       &now,
			}
			if err := svcCtx.VaultAddressBalanceRepo.Upsert(ctx, balanceModel); err != nil {
				logger.Errorf("Failed to upsert zero balance (parsed): address_id=%d asset=%s err=%v", addr.ID, meta.AssetCode, err)
			}
			continue
		}

		// 计算 USD 价值
		balanceDec, _ := decimal.NewFromString(formatted)
		price := assetPrices[meta.AssetCode]
		usdValue := balanceDec.Mul(price)
		usdCents := usdValue.Mul(decimal.NewFromInt(100)).Round(0).IntPart()
		if usdCents < 0 {
			usdCents = 0
		}
		sumTokenUSDCents += usdCents

		logger.Infof("Upserting token balance: asset=%s contract=%s balance=%s balance_raw=%d",
			meta.AssetCode, meta.ContractAddress, amtRes.AmountStr, amtRes.Raw)
		balanceModel := &model.VaultAddressBalanceModel{
			AddressID:       addr.ID,
			NetworkID:       addr.NetworkID,
			Currency:        meta.AssetCode,
			ContractAddress: meta.ContractAddress,
			Balance:         amtRes.AmountStr,
			BalanceRaw:      amtRes.Raw,
			BalanceUSD:      centsToUSDString(usdCents),
			BalanceUSDRaw:   usdCents,
			LastSyncedAt:    &now,
			CreatedAt:       &now,
			UpdatedAt:       &now,
		}

		if err := svcCtx.VaultAddressBalanceRepo.Upsert(ctx, balanceModel); err != nil {
			logger.Errorf("Failed to upsert vault address balance: address_id=%d asset=%s err=%v", addr.ID, meta.AssetCode, err)
		} else {
			logger.Infof("Successfully upserted token balance: asset=%s contract=%s balance=%s",
				meta.AssetCode, meta.ContractAddress, amtRes.AmountStr)
		}
	}

	// 9.1.1 清理链上不存在的 token 余额（设置为0）
	// 对于配置中应该存在的 token，但链上没有返回的，也要更新为0
	logger.Infof("Checking for missing tokens: syncedTokenContracts has %d entries, byContract has %d entries",
		len(syncedTokenContracts), len(byContract))
	for _, meta := range byContract {
		contractLower := strings.ToLower(meta.ContractAddress)
		if !syncedTokenContracts[contractLower] {
			// 这个 token 在配置中存在，但链上没有返回，说明余额为0
			logger.Infof("Token not found on chain, setting balance to 0: asset=%s contract=%s address_id=%d",
				meta.AssetCode, meta.ContractAddress, addr.ID)
			balanceModel := &model.VaultAddressBalanceModel{
				AddressID:       addr.ID,
				NetworkID:       addr.NetworkID,
				Currency:        meta.AssetCode,
				ContractAddress: meta.ContractAddress,
				Balance:         "0",
				BalanceRaw:      0,
				BalanceUSD:      "0",
				BalanceUSDRaw:   0,
				LastSyncedAt:    &now,
				CreatedAt:       &now,
				UpdatedAt:       &now,
			}
			if err := svcCtx.VaultAddressBalanceRepo.Upsert(ctx, balanceModel); err != nil {
				logger.Errorf("Failed to upsert zero balance for missing token: address_id=%d asset=%s err=%v", addr.ID, meta.AssetCode, err)
			}
		}
	}

	// 9.2 处理原生币余额
	totalUSDCents := sumTokenUSDCents
	if nativeAssetCode != "" {
		formatted := strings.TrimSpace(resp.FormattedNativeBalance)
		if formatted == "" {
			formatted = "0"
		}

		// 确定原生币精度：根据链类型推断，而不是使用 Accounting 服务的精度
		nativePrecision := int32(18) // 默认 18 位
		if chainCode == "TRON" {
			nativePrecision = 6 // TRON 是 6 位
		} else if chainCode == "BSC" {
			if nativeAssetCode == "BNB" {
				nativePrecision = 18
			}
		} else if chainCode == "ETH" || chainCode == "Ethereum" {
			if nativeAssetCode == "ETH" {
				nativePrecision = 18
			}
		}

		amtRes, err := parseAmountToRaw(formatted, nativePrecision, true)
		if err == nil {
			// 计算 USD 价值
			balanceDec, _ := decimal.NewFromString(formatted)
			price := assetPrices[nativeAssetCode]
			usdValue := balanceDec.Mul(price)
			nativeUsdCents := usdValue.Mul(decimal.NewFromInt(100)).Round(0).IntPart()
			if nativeUsdCents < 0 {
				nativeUsdCents = 0
			}
			totalUSDCents += nativeUsdCents

			balanceModel := &model.VaultAddressBalanceModel{
				AddressID:       addr.ID,
				NetworkID:       addr.NetworkID,
				Currency:        nativeAssetCode,
				ContractAddress: "",
				Balance:         amtRes.AmountStr,
				BalanceRaw:      amtRes.Raw,
				BalanceUSD:      centsToUSDString(nativeUsdCents),
				BalanceUSDRaw:   nativeUsdCents,
				LastSyncedAt:    &now,
				CreatedAt:       &now,
				UpdatedAt:       &now,
			}

			if err := svcCtx.VaultAddressBalanceRepo.Upsert(ctx, balanceModel); err != nil {
				logger.Errorf("Failed to upsert vault address balance (native): address_id=%d asset=%s err=%v", addr.ID, nativeAssetCode, err)
			}
		} else {
			logger.Errorf("Failed to parse native balance: asset=%s formatted=%s err=%v", nativeAssetCode, formatted, err)
		}
	}

	logger.Infof("Successfully synced vault address balances: address_id=%d address=%s total_usd_cents=%d", addr.ID, maskAddress(addr.Address), totalUSDCents)

	// 10. 汇总更新 vault_balances 表（将 vault_address_balances 按 network + currency + contract_address 聚合）
	if err := syncVaultBalancesFromAddresses(ctx, svcCtx, logger, network.ID); err != nil {
		logger.Errorf("Failed to sync vault balances from addresses: network_id=%d err=%v", network.ID, err)
		// 不阻塞主流程，单个地址同步成功即可
	}

	return nil
}

// syncVaultBalancesFromAddresses 从 vault_address_balances 汇总更新 vault_balances 表
// 按 network_id + currency + contract_address 聚合所有导入地址的余额
func syncVaultBalancesFromAddresses(ctx context.Context, svcCtx *svc.ServiceContext, logger logx.Logger, networkID int64) error {
	if svcCtx == nil || svcCtx.DB == nil || svcCtx.VaultBalanceRepo == nil {
		return fmt.Errorf("invalid service context")
	}

	type aggregatedBalance struct {
		Currency        string
		ContractAddress string
		BalanceRaw      int64
		BalanceUSDRaw   int64
	}

	var results []aggregatedBalance
	err := svcCtx.DB.WithContext(ctx).
		Model(&model.VaultAddressBalanceModel{}).
		Select("currency, contract_address, SUM(balance_raw) as balance_raw, SUM(balance_usd_raw) as balance_usd_raw").
		Where("network_id = ? AND deleted_at IS NULL", networkID).
		Group("currency, contract_address").
		Having("SUM(balance_raw) > 0 OR SUM(balance_usd_raw) > 0").
		Scan(&results).Error

	if err != nil {
		return fmt.Errorf("failed to aggregate vault address balances: %w", err)
	}

	// 获取资产精度信息（用于格式化余额）
	precisionByAsset := make(map[string]int32)
	assetCodes := make([]string, 0, len(results))
	seenAsset := make(map[string]struct{})
	for _, r := range results {
		code := strings.ToUpper(strings.TrimSpace(r.Currency))
		if code == "" {
			continue
		}
		if _, ok := seenAsset[code]; ok {
			continue
		}
		seenAsset[code] = struct{}{}
		assetCodes = append(assetCodes, code)
	}

	if len(assetCodes) > 0 && svcCtx.AccountingRpc != nil {
		for _, code := range assetCodes {
			accResp, err := svcCtx.AccountingRpc.GetAsset(ctx, &pb.GetAssetRequest{Code: code})
			if err == nil && accResp != nil && accResp.Success && accResp.Item != nil {
				precisionByAsset[code] = accResp.Item.Precision
			} else {
				precisionByAsset[code] = 18 // 默认精度
			}
		}
	}

	now := time.Now()
	for _, r := range results {
		currency := strings.TrimSpace(r.Currency)
		contractAddress := strings.TrimSpace(r.ContractAddress)
		precision := precisionByAsset[currency]
		if precision == 0 {
			precision = 18 // 默认精度
		}

		// 格式化余额字符串
		balanceStr := rawToFixedAmountString(r.BalanceRaw, precision)
		balanceUSDStr := centsToUSDString(r.BalanceUSDRaw)

		balanceModel := &model.VaultBalanceModel{
			NetworkID:       networkID,
			Currency:        currency,
			ContractAddress: contractAddress,
			Balance:         balanceStr,
			BalanceRaw:      r.BalanceRaw,
			BalanceUSD:      balanceUSDStr,
			BalanceUSDRaw:   r.BalanceUSDRaw,
			LastUpdatedAt:   &now,
			CreatedAt:       &now,
			UpdatedAt:       &now,
		}

		if err := svcCtx.VaultBalanceRepo.Upsert(ctx, balanceModel); err != nil {
			logger.Errorf("Failed to upsert vault balance: network_id=%d currency=%s contract=%s err=%v",
				networkID, currency, contractAddress, err)
			continue
		}
	}

	logger.Infof("Successfully synced vault balances from addresses: network_id=%d aggregated_count=%d", networkID, len(results))
	return nil
}

// mapChainTypeForChainSync 映射 chain_type 到 ChainSync 的 BlockChainType 枚举
func mapChainTypeForChainSync(chainType string) pb.BlockChainType {
	switch strings.ToLower(strings.TrimSpace(chainType)) {
	case "ethereum":
		return pb.BlockChainType_CHAIN_TYPE_ETHEREUM
	case "tron":
		return pb.BlockChainType_CHAIN_TYPE_TRON
	case "bsc":
		return pb.BlockChainType_CHAIN_TYPE_BSC
	default:
		return pb.BlockChainType_CHAIN_TYPE_UNSPECIFIED
	}
}

// getAssetPricesFromRedis 从 Redis 获取资产价格（USDT 计价）
func getAssetPricesFromRedis(ctx context.Context, rdb *redis.Client, assetCodes []string, logger logx.Logger) map[string]decimal.Decimal {
	result := make(map[string]decimal.Decimal, len(assetCodes))
	if rdb == nil || len(assetCodes) == 0 {
		return result
	}

	// 构建交易对符号列表
	symbols := make([]string, 0, len(assetCodes))
	symbolToCode := make(map[string]string, len(assetCodes))
	for _, code := range assetCodes {
		code = strings.ToUpper(strings.TrimSpace(code))
		if code == "" {
			continue
		}
		if code == "USDT" {
			result[code] = decimal.NewFromInt(1)
			continue
		}
		symbol := code + "USDT"
		symbols = append(symbols, symbol)
		symbolToCode[symbol] = code
	}

	if len(symbols) == 0 {
		return result
	}

	// 从 Redis 批量获取价格（binance:tickers）
	vals, err := rdb.HMGet(ctx, BinanceTickerKey, symbols...).Result()
	if err != nil {
		logger.Errorf("Failed to get prices from Redis: %v", err)
		return result
	}

	for i, val := range vals {
		if val == nil {
			continue
		}
		valStr, ok := val.(string)
		if !ok {
			continue
		}

		var tv BinanceTickerValue
		if err := json.Unmarshal([]byte(valStr), &tv); err != nil {
			continue
		}

		price, err := decimal.NewFromString(tv.Price)
		if err != nil {
			continue
		}

		// 从交易对符号中提取资产代码
		symbol := symbols[i]
		if assetCode, ok := symbolToCode[symbol]; ok {
			result[assetCode] = price
		}
	}

	logger.Infof("Got %d asset prices from Redis: %v", len(result), result)
	return result
}
