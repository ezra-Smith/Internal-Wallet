package svc

import (
	"context"
	"fmt"
	"strings"
	"time"

	"internalwallet/proto/pb"
	"internalwallet/services/chainsync/rpc/internal/provider"
	"internalwallet/services/chainsync/rpc/internal/token"

	"github.com/ethereum/go-ethereum/common"
	"github.com/zeromicro/go-zero/core/logx"
)

// initializePreconfiguredTokens 初始化预配置的代币信息
func (tp *TokenParser) initializePreconfiguredTokens() {
	if tp.config == nil {
		return
	}

	// IMPORTANT: decimals are chain-specific in practice.
	// - Ethereum USDT/USDC: 6
	// - BSC (BNB Chain) pegged USDT/USDC: commonly 18
	// - TRON TRC20 USDT/USDC: 6
	predefinedByChain := map[string]map[string]TokenInfo{
		"ethereum": {
			"USDT": {Symbol: "USDT", Name: "Tether USD", Decimals: 6, IsStablecoin: true},
			"USDC": {Symbol: "USDC", Name: "USD Coin", Decimals: 6, IsStablecoin: true},
		},
		"bsc": {
			"USDT": {Symbol: "USDT", Name: "Tether USD", Decimals: 18, IsStablecoin: true},
			"USDC": {Symbol: "USDC", Name: "USD Coin", Decimals: 18, IsStablecoin: true},
		},
		"tron": {
			"USDT": {Symbol: "USDT", Name: "Tether USD", Decimals: 6, IsStablecoin: true},
			"USDC": {Symbol: "USDC", Name: "USD Coin", Decimals: 6, IsStablecoin: true},
		},
	}

	if tp.config.Chains.Ethereum.Enabled && len(tp.config.Chains.Ethereum.ContractAddresses) > 0 {
		predefinedTokens := predefinedByChain["ethereum"]
		for symbol, address := range tp.config.Chains.Ethereum.ContractAddresses {
			tokenInfo, exists := predefinedTokens[strings.ToUpper(symbol)]
			if !exists {
				continue
			}

			tokenInfo.Address = address
			tokenInfo.Chain = "ethereum"
			cacheKey := tp.getCacheKey("ethereum", address)
			tp.tokenCache[cacheKey] = &tokenInfo
			logx.Infof("Loaded preconfigured token: %s on ethereum at %s", symbol, address)
		}
	}

	if tp.config.Chains.BSC.Enabled && len(tp.config.Chains.BSC.ContractAddresses) > 0 {
		predefinedTokens := predefinedByChain["bsc"]
		for symbol, address := range tp.config.Chains.BSC.ContractAddresses {
			tokenInfo, exists := predefinedTokens[strings.ToUpper(symbol)]
			if !exists {
				continue
			}

			tokenInfo.Address = address
			tokenInfo.Chain = "bsc"
			cacheKey := tp.getCacheKey("bsc", address)
			tp.tokenCache[cacheKey] = &tokenInfo
			logx.Infof("Loaded preconfigured token: %s on bsc at %s", symbol, address)
		}
	}

	if tp.config.Chains.Tron.Enabled && len(tp.config.Chains.Tron.ContractAddresses) > 0 {
		predefinedTokens := predefinedByChain["tron"]
		for symbol, address := range tp.config.Chains.Tron.ContractAddresses {
			tokenInfo, exists := predefinedTokens[strings.ToUpper(symbol)]
			if !exists {
				continue
			}

			tokenInfo.Address = address
			tokenInfo.Chain = "tron"
			cacheKey := tp.getCacheKey("tron", address)
			tp.tokenCache[cacheKey] = &tokenInfo
			logx.Infof("Loaded preconfigured token: %s on tron at %s", symbol, address)
		}
	}

	logx.Infof("Initialized %d preconfigured tokens in cache", len(tp.tokenCache))
}

// getCacheKey 生成缓存键（使用原始地址格式）
func (tp *TokenParser) getCacheKey(chain, address string) string {
	chain = strings.ToLower(strings.TrimSpace(chain))
	address = strings.TrimSpace(address)
	// EVM addresses are case-insensitive; normalize to avoid cache misses due to checksum casing.
	if strings.HasPrefix(strings.ToLower(address), "0x") {
		address = strings.ToLower(address)
	}
	return fmt.Sprintf("%s:%s", chain, address)
}

// getChainName 将 BlockChainType 转换为小写链名称（用于缓存 key）
func (tp *TokenParser) getChainName(chain pb.BlockChainType) string {
	switch chain {
	case pb.BlockChainType_CHAIN_TYPE_ETHEREUM:
		return "ethereum"
	case pb.BlockChainType_CHAIN_TYPE_BSC:
		return "bsc"
	case pb.BlockChainType_CHAIN_TYPE_TRON:
		return "tron"
	default:
		return strings.ToLower(chain.String())
	}
}

// GetTokenInfo 获取代币信息（优先从缓存获取，缓存未命中则链上查询）
func (tp *TokenParser) GetTokenInfo(chain pb.BlockChainType, address string) *TokenInfo {
	chainName := tp.getChainName(chain)
	cacheKey := tp.getCacheKey(chainName, address)

	tp.cacheMutex.RLock()
	tokenInfo, exists := tp.tokenCache[cacheKey]
	tp.cacheMutex.RUnlock()
	if exists {
		return tokenInfo
	}

	tokenInfo = tp.queryTokenInfoFromChain(chain, address)
	if tokenInfo == nil {
		tokenInfo = &TokenInfo{
			Address:      address,
			Symbol:       "UNKNOWN",
			Name:         "Unknown Token",
			Decimals:     18,
			Chain:        chainName,
			IsStablecoin: false,
		}
		logx.Errorf("Failed to query token info for %s on %v, using default values", address, chain)
	} else {
		logx.Infof("Successfully queried token info: %s (%s) on %v", tokenInfo.Symbol, tokenInfo.Name, chain)
	}

	tp.cacheMutex.Lock()
	tp.tokenCache[cacheKey] = tokenInfo
	tp.cacheMutex.Unlock()

	return tokenInfo
}

// queryTokenInfoFromChain 从链上查询代币信息
func (tp *TokenParser) queryTokenInfoFromChain(chain pb.BlockChainType, address string) *TokenInfo {
	switch chain {
	case pb.BlockChainType_CHAIN_TYPE_ETHEREUM, pb.BlockChainType_CHAIN_TYPE_BSC:
		return tp.queryERC20TokenInfo(chain, address)
	case pb.BlockChainType_CHAIN_TYPE_TRON:
		return tp.queryTronTokenInfo(address)
	default:
		logx.Errorf("Unsupported chain type for token info query: %v", chain)
		return nil
	}
}

// queryERC20TokenInfo 查询ERC20代币信息
func (tp *TokenParser) queryERC20TokenInfo(chain pb.BlockChainType, contractAddress string) *TokenInfo {
	if tp.providerPool == nil {
		logx.Error("Provider pool is nil, cannot query ERC20 token info")
		return nil
	}

	var providerToUse provider.Provider
	for _, p := range tp.providerPool.GetProvidersByPriority(chain) {
		if _, ok := p.(interface {
			CallContract(ctx context.Context, contractAddress common.Address, data []byte) ([]byte, error)
		}); ok {
			providerToUse = p
			break
		}
	}
	if providerToUse == nil {
		p, err := tp.providerPool.GetProvider(chain)
		if err != nil {
			logx.Errorf("Failed to get provider for ERC20 token query: %v", err)
			return nil
		}
		providerToUse = p
	}
	if providerToUse == nil {
		logx.Errorf("No provider available for chain %v", chain)
		return nil
	}

	if !common.IsHexAddress(contractAddress) {
		logx.Errorf("Invalid contract address format: %s", contractAddress)
		return nil
	}

	contractAddr := common.HexToAddress(contractAddress)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tokenInfo := &TokenInfo{
		Address: contractAddr.String(),
		Chain:   tp.getChainName(chain),
	}

	if name, err := tp.callERC20StringFunction(ctx, providerToUse, "name", contractAddr); err != nil {
		logx.Errorf("Failed to get token name for %s: %v", contractAddress, err)
		tokenInfo.Name = "Unknown Token"
	} else {
		tokenInfo.Name = name
	}

	if symbol, err := tp.callERC20StringFunction(ctx, providerToUse, "symbol", contractAddr); err != nil {
		logx.Errorf("Failed to get token symbol for %s: %v", contractAddress, err)
		tokenInfo.Symbol = "UNKNOWN"
	} else {
		tokenInfo.Symbol = symbol
	}

	if decimals, err := tp.callERC20Uint8Function(ctx, providerToUse, "decimals", contractAddr); err != nil {
		logx.Errorf("Failed to get token decimals for %s: %v", contractAddress, err)
		tokenInfo.Decimals = 18
	} else {
		tokenInfo.Decimals = decimals
	}

	tokenInfo.IsStablecoin = tp.isStablecoin(tokenInfo.Symbol)
	return tokenInfo
}

// queryTronTokenInfo 查询TRC20代币信息
func (tp *TokenParser) queryTronTokenInfo(contractAddress string) *TokenInfo {
	if tp.providerPool == nil {
		logx.Error("Provider pool is nil, cannot query TRC20 token info")
		return nil
	}

	for _, p := range tp.providerPool.GetProvidersByPriority(pb.BlockChainType_CHAIN_TYPE_TRON) {
		getter, ok := p.(interface {
			GetTRC20TokenInfo(contractAddress string) (*token.Info, error)
		})
		if !ok {
			continue
		}

		info, err := getter.GetTRC20TokenInfo(contractAddress)
		if err != nil {
			logx.Errorf("Failed to query TRC20 token info for %s: %v", contractAddress, err)
			continue
		}
		if info == nil {
			continue
		}

		return &TokenInfo{
			Address:      info.Address,
			Symbol:       info.Symbol,
			Name:         info.Name,
			Decimals:     info.Decimals,
			Chain:        info.Chain,
			IsStablecoin: info.IsStablecoin,
		}
	}

	logx.Error("No TRON provider available for TRC20 token info query")
	return nil
}

// callERC20StringFunction 调用ERC20返回字符串的函数
func (tp *TokenParser) callERC20StringFunction(ctx context.Context, p provider.Provider, functionName string, contractAddr common.Address) (string, error) {
	data, err := tp.erc20QueryABI.Pack(functionName)
	if err != nil {
		return "", fmt.Errorf("failed to pack %s function: %v", functionName, err)
	}

	caller, ok := p.(interface {
		CallContract(ctx context.Context, contractAddress common.Address, data []byte) ([]byte, error)
	})
	if !ok {
		return "", fmt.Errorf("provider does not support contract calling")
	}

	result, err := caller.CallContract(ctx, contractAddr, data)
	if err != nil {
		return "", fmt.Errorf("failed to call %s function: %v", functionName, err)
	}

	var value string
	if err := tp.erc20QueryABI.UnpackIntoInterface(&value, functionName, result); err != nil {
		return "", fmt.Errorf("failed to unpack %s result: %v", functionName, err)
	}

	return value, nil
}

// callERC20Uint8Function 调用ERC20返回uint8的函数
func (tp *TokenParser) callERC20Uint8Function(ctx context.Context, p provider.Provider, functionName string, contractAddr common.Address) (uint8, error) {
	data, err := tp.erc20QueryABI.Pack(functionName)
	if err != nil {
		return 0, fmt.Errorf("failed to pack %s function: %v", functionName, err)
	}

	caller, ok := p.(interface {
		CallContract(ctx context.Context, contractAddress common.Address, data []byte) ([]byte, error)
	})
	if !ok {
		return 0, fmt.Errorf("provider does not support contract calling")
	}

	result, err := caller.CallContract(ctx, contractAddr, data)
	if err != nil {
		return 0, fmt.Errorf("failed to call %s function: %v", functionName, err)
	}

	var value uint8
	if err := tp.erc20QueryABI.UnpackIntoInterface(&value, functionName, result); err != nil {
		return 0, fmt.Errorf("failed to unpack %s result: %v", functionName, err)
	}

	return value, nil
}

// isStablecoin 检查是否为稳定币
func (tp *TokenParser) isStablecoin(symbol string) bool {
	symbol = strings.ToUpper(symbol)
	stablecoins := []string{"USDT", "USDC", "USDD", "BUSD", "DAI", "TUSD", "FDUSD", "PYUSD"}
	for _, stablecoin := range stablecoins {
		if symbol == stablecoin {
			return true
		}
	}
	return false
}
