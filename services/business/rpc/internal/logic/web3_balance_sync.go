package logic

import (
	"context"
	"math"
	"math/big"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"

	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/model"
	"internalwallet/services/business/rpc/internal/svc"
)

// syncWeb3AddressAllAssets 从链上同步 Web3 地址的所有资产余额（主币+代币）到 web3_user_address_balances 表
// 查询 currency_chain_settings 表获取该链上所有启用的资产，并发查询余额后写入数据库
func syncWeb3AddressAllAssets(ctx context.Context, svcCtx *svc.ServiceContext, addr *model.Web3UserAddressModel) {
	if ctx == nil || svcCtx == nil || addr == nil {
		return
	}
	logger := logx.WithContext(ctx)

	if svcCtx.ChainRpc == nil || svcCtx.Web3UserAddressBalanceRepository == nil {
		logger.Debug("ChainRpc or Web3UserAddressBalanceRepository not configured, skip asset sync")
		return
	}

	// 获取链信息
	chainType, chainCode, _, _ := mapChainForNativeBalance(addr.ChainID)
	if chainType == pb.ChainRpcType_CHAIN_TYPE_UNSPECIFIED || chainCode == "" {
		logger.Debugf("Unsupported chainId for asset sync: chain_id=%d", addr.ChainID)
		return
	}

	// 从数据库查询该链上所有启用的 Web3 资产（包括主币和代币）
	var settings []model.CurrencyChainSettingsModel
	if svcCtx.CurrencyChainSettingsRepository != nil {
		var err error
		settings, err = svcCtx.CurrencyChainSettingsRepository.ListWeb3SupportedAssetsByChain(ctx, chainCode)
		if err != nil {
			logger.Errorf("查询链资产配置失败: chain=%s error=%v", chainCode, err)
			return
		}
	}

	if len(settings) == 0 {
		logger.Infof("链上没有配置资产，仅同步主币: chain=%s", chainCode)
		// 退化为只同步主币
		syncWeb3AddressNativeBalance(ctx, svcCtx, addr)
		return
	}

	logger.Infof("开始同步地址资产: address=%s chain=%s asset_count=%d", addr.Address, chainCode, len(settings))

	// 使用并发查询，限制并发数为 5（避免打爆 ChainRpc）
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 5)
	successCount := 0
	failedCount := 0
	var mu sync.Mutex

	for _, setting := range settings {
		wg.Add(1)
		go func(s model.CurrencyChainSettingsModel) {
			defer wg.Done()

			// 限流
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// 判断是主币还是代币
			isNative := s.ContractAddress == nil || strings.TrimSpace(*s.ContractAddress) == ""

			// 获取 decimals（默认值为 18）
			decimals := getDefaultDecimals(chainCode, isNative)

			var success bool
			if isNative {
				success = syncAssetBalance(ctx, svcCtx, addr, chainType, chainCode, s.AssetCode, nil, decimals)
			} else {
				contractAddr := strings.TrimSpace(*s.ContractAddress)
				success = syncAssetBalance(ctx, svcCtx, addr, chainType, chainCode, s.AssetCode, &contractAddr, decimals)
			}

			mu.Lock()
			if success {
				successCount++
			} else {
				failedCount++
			}
			mu.Unlock()
		}(setting)
	}

	wg.Wait()
	logger.Infof("地址资产同步完成: address=%s chain=%s success=%d failed=%d",
		addr.Address, chainCode, successCount, failedCount)
}

// syncAssetBalance 同步单个资产余额（可以是主币或代币）
func syncAssetBalance(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	addr *model.Web3UserAddressModel,
	chainType pb.ChainRpcType,
	chainCode string,
	assetCode string,
	contractAddress *string, // nil 表示主币
	decimals int,
) bool {
	logger := logx.WithContext(ctx)

	var balanceStr string       // 原始余额（Wei/最小单位）
	var balanceFormatted string // 格式化余额（人类可读）
	var usdCents int64

	// 查询余额
	if contractAddress == nil {
		// 主币：使用 GetBalance
		resp, err := svcCtx.ChainRpc.GetBalance(ctx, &pb.GetBalanceReq{
			Chain:   chainType,
			Address: addr.Address,
		})
		if err != nil {
			logger.Debugf("查询主币余额失败: chain=%s asset=%s address=%s error=%v",
				chainCode, assetCode, addr.Address, err)
			return false
		}
		if resp == nil || !resp.Success {
			return false
		}

		balanceStr = strings.TrimSpace(resp.Balance)
		if balanceStr == "" {
			balanceStr = "0"
		}
		// 使用格式化余额（人类可读）
		balanceFormatted = strings.TrimSpace(resp.BalanceFormatted)
		if balanceFormatted == "" {
			balanceFormatted = "0"
		}
		if resp.BalanceUsd > 0 {
			usdCents = int64(resp.BalanceUsd * 100)
		}
	} else {
		// 代币：使用 GetTokenBalance
		resp, err := svcCtx.ChainRpc.GetTokenBalance(ctx, &pb.GetTokenBalanceReq{
			Chain:         chainType,
			Address:       addr.Address,
			TokenContract: *contractAddress,
		})
		if err != nil {
			logger.Debugf("查询代币余额失败: chain=%s asset=%s contract=%s address=%s error=%v",
				chainCode, assetCode, *contractAddress, addr.Address, err)
			return false
		}
		if resp == nil || !resp.Success {
			return false
		}

		balanceStr = strings.TrimSpace(resp.Balance)
		if balanceStr == "" {
			balanceStr = "0"
		}
		// 使用格式化余额（人类可读）
		balanceFormatted = strings.TrimSpace(resp.BalanceFormatted)
		if balanceFormatted == "" {
			balanceFormatted = "0"
		}
		if resp.BalanceUsd > 0 {
			usdCents = int64(resp.BalanceUsd * 100)
		}

		// 优先使用 ChainRPC 返回的代币符号
		if resp.TokenSymbol != "" {
			assetCode = strings.ToUpper(strings.TrimSpace(resp.TokenSymbol))
		}

		// 优先使用 ChainRPC 返回的代币精度（如果有效）
		if resp.TokenDecimals > 0 {
			decimals = int(resp.TokenDecimals)
			logger.Debugf("使用 ChainRPC 返回的代币精度: asset=%s contract=%s decimals=%d",
				assetCode, *contractAddress, decimals)
		}
	}

	// 检查余额是否大于 0
	// 余额为 0 时：
	// - 若数据库已有记录：更新为 0，避免旧值残留导致展示错误
	// - 若数据库无记录：跳过写入（节省空间）
	balanceDecimal, err := decimal.NewFromString(balanceStr)
	if err != nil {
		logger.Debugf("余额解析失败: address=%s asset=%s balance_raw=%s error=%v", addr.Address, assetCode, balanceStr, err)
		return false
	}
	if balanceDecimal.IsZero() {
		db := svcCtx.Web3UserAddressBalanceRepository.GetDB()
		if db != nil {
			var existing model.Web3UserAddressBalanceModel
			findErr := db.WithContext(ctx).
				Where("wallet_address = ? AND asset_code = ? AND chain_code = ? AND deleted_at IS NULL",
					addr.Address, assetCode, chainCode).
				First(&existing).Error
			switch findErr {
			case nil:
				updateErr := db.WithContext(ctx).
					Model(&model.Web3UserAddressBalanceModel{}).
					Where("id = ?", existing.ID).
					Updates(map[string]interface{}{
						"balance":            "0",
						"balance_raw":        "0",
						"balance_usd":        "0",
						"balance_usd_raw":    "0",
						"balance_updated_at": gorm.Expr("NOW()"),
						"last_synced_at":     gorm.Expr("NOW()"),
						"sync_status":        "synced",
						"sync_error":         nil,
					}).Error
				if updateErr != nil {
					logger.Errorf("更新余额为0失败: id=%d address=%s asset=%s error=%v", existing.ID, addr.Address, assetCode, updateErr)
					return false
				}
				logger.Infof("余额为0，已更新为0: address=%s asset=%s", addr.Address, assetCode)
				return true
			case gorm.ErrRecordNotFound:
				// no-op
			default:
				logger.Errorf("查询余额记录失败(余额为0): address=%s asset=%s error=%v", addr.Address, assetCode, findErr)
				return false
			}
		}
		logger.Debugf("余额为0，跳过写入: address=%s asset=%s", addr.Address, assetCode)
		return true // 不算失败
	}

	// 解析 balanceStr 为 int64
	var balanceRawInt64 int64
	if bi, ok := new(big.Int).SetString(balanceStr, 10); ok {
		balanceRawInt64 = bi.Int64()
	}

	// 如果 ChainRpc 没有返回 USD 估值，则自己计算
	if usdCents <= 0 && svcCtx.RedisClient != nil {
		// 从 Redis 获取当前价格（USD价格，而不是USDT价格）
		if price, ok := GetAssetPriceUSD(ctx, svcCtx.RedisClient, assetCode); ok && price.IsPositive() {
			var actualBalance decimal.Decimal
			// 优先使用 ChainRPC 返回的格式化余额
			if balanceFormatted != "" && balanceFormatted != "0" {
				if balanceFormattedDecimal, err := decimal.NewFromString(balanceFormatted); err == nil {
					actualBalance = balanceFormattedDecimal
				}
			}
			// 如果格式化余额不可用，从原始余额和精度计算
			if actualBalance.IsZero() && balanceDecimal.IsPositive() {
				decimalsDivisor := decimal.NewFromInt(1).Shift(int32(decimals))
				actualBalance = balanceDecimal.Div(decimalsDivisor)
			}
			// 计算 USD 价值：actualBalance * price_usd
			if actualBalance.IsPositive() {
				usdValue := actualBalance.Mul(price)
				// 转换为分（cents）
				usdCents = usdValue.Mul(decimal.NewFromInt(100)).IntPart()
				logger.Debugf("计算USD估值: asset=%s balance=%s price=%s usd_cents=%d",
					assetCode, actualBalance.String(), price.String(), usdCents)
			}
		}
	}

	if usdCents < 0 {
		usdCents = 0
	}
	balanceUSD := formatUSDFromCents(usdCents)
	balanceUSDRawStr := strconv.FormatInt(usdCents, 10)

	// 查询数据库是否已有记录
	db := svcCtx.Web3UserAddressBalanceRepository.GetDB()
	var existing model.Web3UserAddressBalanceModel
	err = db.WithContext(ctx).
		Where("wallet_address = ? AND asset_code = ? AND chain_code = ? AND deleted_at IS NULL",
			addr.Address, assetCode, chainCode).
		First(&existing).Error

	now := time.Now()
	tokenType := "native"
	if contractAddress != nil {
		tokenType = "ERC20" // 根据链类型可以细化（ERC20/TRC20/BEP20）
	}

	switch err {
	case nil:
		// 更新现有记录
		updateErr := svcCtx.Web3UserAddressBalanceRepository.UpdateBalance(
			ctx,
			existing.ID,
			balanceFormatted, // 使用格式化余额（人类可读）
			balanceRawInt64,
			balanceUSD,
			usdCents,
		)
		if updateErr != nil {
			logger.Errorf("更新余额失败: id=%d address=%s asset=%s error=%v",
				existing.ID, addr.Address, assetCode, updateErr)
			return false
		}

		// 如果精度发生变化，单独更新精度字段（特别是代币，可能从默认值 18 更新为实际值 6）
		if contractAddress != nil && existing.TokenDecimals != decimals {
			db := svcCtx.Web3UserAddressBalanceRepository.GetDB()
			if db != nil {
				updateDecimalsErr := db.WithContext(ctx).
					Model(&model.Web3UserAddressBalanceModel{}).
					Where("id = ?", existing.ID).
					Update("token_decimals", decimals).Error
				if updateDecimalsErr != nil {
					logger.Infof("更新代币精度失败: id=%d address=%s asset=%s old_decimals=%d new_decimals=%d error=%v",
						existing.ID, addr.Address, assetCode, existing.TokenDecimals, decimals, updateDecimalsErr)
				} else {
					logger.Infof("更新代币精度成功: address=%s asset=%s old_decimals=%d new_decimals=%d",
						addr.Address, assetCode, existing.TokenDecimals, decimals)
				}
			}
		}

		logger.Infof("更新余额成功: address=%s asset=%s balance=%s (raw=%s) usd=%s decimals=%d",
			addr.Address, assetCode, balanceFormatted, balanceStr, balanceUSD, decimals)
		return true

	case gorm.ErrRecordNotFound:
		// 创建新记录（不绑定到特定用户/设备）
		newBalance := &model.Web3UserAddressBalanceModel{
			AssetCode:        assetCode,
			ChainCode:        chainCode,
			WalletAddress:    addr.Address,
			Network:          chainCode, // 使用 chainCode（ETH/BSC/TRON）而不是 addr.Network，确保与前端查询一致
			ChainID:          addr.ChainID,
			Balance:          balanceFormatted, // 使用格式化余额（人类可读）
			BalanceRaw:       balanceStr,       // 原始余额（Wei/最小单位）
			BalanceUSD:       balanceUSD,
			BalanceUSDRaw:    balanceUSDRawStr,
			TokenAddress:     contractAddress,
			TokenDecimals:    decimals,
			TokenType:        tokenType,
			IsVerified:       0,
			IsSpam:           0,
			BalanceUpdatedAt: &now,
			LastSyncedAt:     &now,
			SyncStatus:       "synced",
			SyncError:        nil,
		}

		if createErr := svcCtx.Web3UserAddressBalanceRepository.Create(ctx, newBalance); createErr != nil {
			logger.Errorf("创建余额记录失败: address=%s asset=%s error=%v",
				addr.Address, assetCode, createErr)
			return false
		}
		logger.Infof("创建余额成功: address=%s asset=%s balance=%s (raw=%s) usd=%s",
			addr.Address, assetCode, balanceFormatted, balanceStr, balanceUSD)
		return true

	default:
		logger.Errorf("查询余额记录失败: address=%s asset=%s error=%v",
			addr.Address, assetCode, err)
		return false
	}
}

// syncWeb3AddressNativeBalance 从链上同步 Web3 地址的主币余额到 web3_user_address_balances 表
// 仅处理主币（ETH/BNB/TRX），Token 余额由交易确认消费逻辑增量维护。
// 注意：建议使用 syncWeb3AddressAllAssets 替代此函数，以同步所有资产
func syncWeb3AddressNativeBalance(ctx context.Context, svcCtx *svc.ServiceContext, addr *model.Web3UserAddressModel) {
	if ctx == nil || svcCtx == nil || addr == nil {
		return
	}
	logger := logx.WithContext(ctx)

	if svcCtx.ChainRpc == nil || svcCtx.Web3UserAddressBalanceRepository == nil {
		logger.Debug("ChainRpc or Web3UserAddressBalanceRepository not configured, skip native balance sync")
		return
	}

	chainType, chainCode, assetCode, tokenDecimals := mapChainForNativeBalance(addr.ChainID)
	if chainType == pb.ChainRpcType_CHAIN_TYPE_UNSPECIFIED || chainCode == "" || assetCode == "" {
		logger.Debugf("Unsupported chainId for native balance sync: chain_id=%d", addr.ChainID)
		return
	}

	resp, err := svcCtx.ChainRpc.GetBalance(ctx, &pb.GetBalanceReq{
		Chain:   chainType,
		Address: strings.TrimSpace(addr.Address),
	})
	if err != nil {
		logger.Errorf("Failed to get native balance from ChainRpc: chain=%v address=%s error=%v", chainType, addr.Address, err)
		return
	}
	if resp == nil || !resp.Success {
		msg := ""
		if resp != nil {
			msg = resp.Message
		}
		logger.Errorf("ChainRpc.GetBalance failed: chain=%v address=%s msg=%s", chainType, addr.Address, strings.TrimSpace(msg))
		return
	}

	balanceStr := strings.TrimSpace(resp.Balance)
	if balanceStr == "" {
		balanceStr = "0"
	}

	// 解析原始余额为 int64（可能溢出，溢出时退化为 0）
	var balanceRawInt64 int64
	if bi, ok := new(big.Int).SetString(balanceStr, 10); ok {
		// 尝试转换为 int64；如果溢出则置 0（仅影响聚合排序，不影响原始字符串）
		balanceRawInt64 = bi.Int64()
	}

	// 计算 USD 估值（以"分"为单位）
	usdCents := int64(math.Round(resp.BalanceUsd * 100))
	if usdCents < 0 {
		usdCents = 0
	}

	// 如果 ChainRpc 没有返回 USD 估值，则自己计算
	if usdCents <= 0 && svcCtx.RedisClient != nil {
		// 从 Redis 获取当前价格（USD价格，而不是USDT价格）
		if price, ok := GetAssetPriceUSD(ctx, svcCtx.RedisClient, assetCode); ok && price.IsPositive() {
			// 将最小单位余额转换为实际余额（除以 10^decimals）
			balanceDecimal, err := decimal.NewFromString(balanceStr)
			if err == nil && balanceDecimal.IsPositive() {
				decimalsDivisor := decimal.NewFromInt(1).Shift(int32(tokenDecimals))
				actualBalance := balanceDecimal.Div(decimalsDivisor)
				// 计算 USD 价值：actualBalance * price_usd
				usdValue := actualBalance.Mul(price)
				// 转换为分（cents）
				usdCents = usdValue.Mul(decimal.NewFromInt(100)).IntPart()
				logger.Debugf("计算USD估值: asset=%s balance=%s price=%s usd_cents=%d",
					assetCode, balanceStr, price.String(), usdCents)
			}
		}
	}

	if usdCents < 0 {
		usdCents = 0
	}
	balanceUSD := formatUSDFromCents(usdCents)
	balanceUSDRawStr := strconv.FormatInt(usdCents, 10)

	db := svcCtx.Web3UserAddressBalanceRepository.GetDB()
	if db == nil {
		logger.Debug("Web3UserAddressBalanceRepository DB is nil, skip native balance sync")
		return
	}

	now := time.Now()

	// 查询是否已有该地址+资产+链的余额记录
	var existing model.Web3UserAddressBalanceModel
	err = db.WithContext(ctx).
		Where("wallet_address = ? AND asset_code = ? AND chain_code = ? AND deleted_at IS NULL",
			addr.Address, assetCode, chainCode).
		First(&existing).Error

	switch err {
	case nil:
		// 更新现有记录
		updateErr := svcCtx.Web3UserAddressBalanceRepository.UpdateBalance(
			ctx,
			existing.ID,
			balanceStr,
			balanceRawInt64,
			balanceUSD,
			usdCents,
		)
		if updateErr != nil {
			logger.Errorf("Failed to update native balance record: id=%d address=%s asset=%s chain=%s error=%v",
				existing.ID, addr.Address, assetCode, chainCode, updateErr)
		}
	case gorm.ErrRecordNotFound:
		// 创建新记录（不绑定到特定用户/设备）
		newBalance := &model.Web3UserAddressBalanceModel{
			AssetCode:        assetCode,
			ChainCode:        chainCode,
			WalletAddress:    addr.Address,
			Network:          chainCode, // 使用 chainCode（ETH/BSC/TRON）而不是 addr.Network，确保与前端查询一致
			ChainID:          addr.ChainID,
			Balance:          balanceStr,
			BalanceRaw:       balanceStr,
			BalanceUSD:       balanceUSD,
			BalanceUSDRaw:    balanceUSDRawStr,
			TokenAddress:     nil,
			TokenDecimals:    tokenDecimals,
			TokenType:        "native",
			IsVerified:       0,
			IsSpam:           0,
			BalanceUpdatedAt: &now,
			LastSyncedAt:     &now,
			SyncStatus:       "synced",
			SyncError:        nil,
		}

		if createErr := svcCtx.Web3UserAddressBalanceRepository.Create(ctx, newBalance); createErr != nil {
			logger.Errorf("Failed to create native balance record: address=%s asset=%s chain=%s error=%v",
				addr.Address, assetCode, chainCode, createErr)
		}
	default:
		logger.Errorf("Failed to query native balance record: address=%s asset=%s chain=%s error=%v",
			addr.Address, assetCode, chainCode, err)
	}
}

// mapChainForNativeBalance 将 chainId 映射到 ChainRpcType / chain_code / asset_code / decimals
func mapChainForNativeBalance(chainId int64) (pb.ChainRpcType, string, string, int) {
	switch chainId {
	case 1, 11155111:
		// Ethereum 主网 / Sepolia
		return pb.ChainRpcType_CHAIN_TYPE_ETHEREUM, "ETH", "ETH", 18
	case 56, 97:
		// BSC 主网 / 测试网
		return pb.ChainRpcType_CHAIN_TYPE_BSC, "BSC", "BNB", 18
	case 728126428:
		// TRON 主网（代码库统一使用此值）
		return pb.ChainRpcType_CHAIN_TYPE_TRON, "TRON", "TRX", 6
	default:
		return pb.ChainRpcType_CHAIN_TYPE_UNSPECIFIED, "", "", 0
	}
}

// formatUSDFromCents 将"分"转换为 USD 字符串（与 getWeb3AssetOverviewLogic 中格式保持一致）
func formatUSDFromCents(cents int64) string {
	if cents == 0 {
		return "0"
	}
	dollars := float64(cents) / 100.0
	s := strconv.FormatFloat(dollars, 'f', 2, 64)
	// 去掉多余的 0 和小数点
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	return s
}

// getDefaultDecimals 根据链和资产类型获取默认的小数位数
func getDefaultDecimals(chainCode string, isNative bool) int {
	if isNative {
		// 主币的小数位数
		switch strings.ToUpper(chainCode) {
		case "ETH", "BSC":
			return 18
		case "TRON":
			return 6
		default:
			return 18
		}
	}
	// 代币默认使用 18 位小数（大部分 ERC20/BEP20 代币）
	// USDT/USDC 等稳定币是 6 位，但这里用默认值，实际精度从链上获取
	return 18
}
