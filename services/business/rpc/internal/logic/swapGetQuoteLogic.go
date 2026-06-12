package logic

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type SwapGetQuoteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSwapGetQuoteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SwapGetQuoteLogic {
	return &SwapGetQuoteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ==================== Web3 Swap (proxy to swap microservice) ====================
func (l *SwapGetQuoteLogic) SwapGetQuote(in *pb.BusinessSwapGetQuoteRequest) (*pb.BusinessSwapGetQuoteResponse, error) {
	if in == nil {
		return nil, errx.InvalidParam("invalid params")
	}
	if l.svcCtx.SwapRpc == nil {
		return nil, errx.SwapServiceNotAvailable()
	}
	if in.ChainId <= 0 {
		return nil, errx.InvalidChainId()
	}
	fromTokenAddr := strings.TrimSpace(in.FromTokenAddress)
	toTokenAddr := strings.TrimSpace(in.ToTokenAddress)
	if fromTokenAddr == "" || toTokenAddr == "" {
		return nil, errx.InvalidTokenAddress()
	}
	if strings.TrimSpace(in.Amount) == "" {
		return nil, errx.InvalidAmount()
	}

	// 1. 获取代币配置，进行金额校验
	l.Infof("[SwapGetQuote] Starting amount validation for token %s on chain %d", fromTokenAddr, in.ChainId)
	tokenConfig, err := l.getTokenConfig(in.ChainId, fromTokenAddr)
	if err != nil {
		l.Errorf("[SwapGetQuote] failed to get token config for %s: %v, skipping amount validation", fromTokenAddr, err)
		// 如果获取配置失败，继续执行（不阻塞报价请求）
	} else if tokenConfig == nil {
		l.Infof("[SwapGetQuote] token %s not found in swap configs, skipping amount validation", fromTokenAddr)
	} else {
		l.Infof("[SwapGetQuote] token config found: decimals=%d, min=$%s, max=$%s, price=$%s",
			tokenConfig.Decimals, tokenConfig.MinSwapAmountUSD.String(),
			tokenConfig.MaxSwapAmountUSD.String(), tokenConfig.PriceUSD.String())
		// 校验金额是否在允许范围内
		if err := l.validateSwapAmount(in.Amount, tokenConfig); err != nil {
			l.Infof("[SwapGetQuote] amount validation failed: %v", err)
			return nil, err
		}
		l.Infof("[SwapGetQuote] amount validation passed")
	}

	// 2. 调用 swap 服务获取报价
	resp, err := l.svcCtx.SwapRpc.GetQuote(l.ctx, &pb.GetQuoteRequest{
		ChainId:          in.ChainId,
		FromTokenAddress: in.FromTokenAddress,
		ToTokenAddress:   in.ToTokenAddress,
		Amount:           in.Amount,
		IncludeProtocols: in.IncludeProtocols,
	})
	if err != nil {
		return nil, l.mapSwapError(err)
	}
	if resp == nil {
		l.Errorf("swap get quote returned nil response")
		return nil, errx.SwapServiceError()
	}
	if !resp.Success {
		l.Errorf("swap get quote failed: %s", resp.Message)
		return nil, l.mapUpstreamErrorMessage(resp.Message)
	}
	if resp.Data == nil {
		l.Errorf("swap get quote returned nil data")
		return nil, errx.InvalidParam("quote data is empty")
	}

	// 映射报价数据
	bizData := mapSwapQuoteToBiz(resp.Data)
	if bizData == nil {
		l.Errorf("failed to map swap quote to business data")
		return nil, errx.Internal("failed to process quote data")
	}

	// 计算矿工费百分比（使用 defer recover 防止 panic）
	var gasFeePercentage string
	func() {
		defer func() {
			if r := recover(); r != nil {
				l.Errorf("panic in calculateGasFeePercentage: %v", r)
				gasFeePercentage = "0"
			}
		}()
		gasFeePercentage = l.calculateGasFeePercentage(
			in.ChainId,
			resp.Data,
		)
	}()

	// 设置矿工费百分比
	bizData.GasFeePercentage = gasFeePercentage

	return &pb.BusinessSwapGetQuoteResponse{
		Success: true,
		Message: "ok",
		Data:    bizData,
	}, nil
}

// calculateGasFeePercentage 计算矿工费百分比
// 矿工费百分比 = (矿工费 USD / 交易金额 USD) * 100
func (l *SwapGetQuoteLogic) calculateGasFeePercentage(chainId int64, quoteData *pb.QuoteData) string {
	if quoteData == nil {
		l.Debugf("quoteData is nil, returning 0")
		return "0"
	}

	// 1. 调用 ChainRPC 的 EstimateGas 获取矿工费的 USD 价值
	gasFeeUSD := l.getGasFeeUSD(chainId, quoteData)
	if gasFeeUSD == nil || gasFeeUSD.LessThanOrEqual(decimal.Zero) {
		l.Debugf("gasFeeUSD is nil or <= 0, returning 0")
		return "0"
	}

	// 2. 获取代币价格，计算交易金额的 USD 价值
	if quoteData.FromToken == nil {
		l.Debugf("FromToken is nil, returning 0")
		return "0"
	}
	fromTokenPrice := l.getTokenPriceUSD(quoteData.FromToken)
	if fromTokenPrice == nil || fromTokenPrice.LessThanOrEqual(decimal.Zero) {
		l.Debugf("fromTokenPrice is nil or <= 0, returning 0")
		return "0"
	}

	// 3. 计算交易金额的 USD 价值
	// fromAmount 是 wei，需要除以 10^decimals
	if strings.TrimSpace(quoteData.FromAmount) == "" {
		l.Debugf("FromAmount is empty, returning 0")
		return "0"
	}
	fromAmount, err := decimal.NewFromString(quoteData.FromAmount)
	if err != nil {
		l.Debugf("Failed to parse from_amount: %v", err)
		return "0"
	}
	if fromAmount.LessThanOrEqual(decimal.Zero) {
		l.Debugf("fromAmount <= 0, returning 0")
		return "0"
	}

	decimals := quoteData.FromToken.GetDecimals()
	if decimals == 0 {
		decimals = 18 // 默认 18 位小数
	}

	// 转换为代币数量（除以 10^decimals）
	divisor := decimal.NewFromInt(1)
	for i := uint32(0); i < decimals; i++ {
		divisor = divisor.Mul(decimal.NewFromInt(10))
	}
	if divisor.LessThanOrEqual(decimal.Zero) {
		l.Debugf("divisor <= 0, returning 0")
		return "0"
	}
	tokenAmount := fromAmount.Div(divisor)

	// 计算交易金额的 USD 价值
	txAmountUSD := tokenAmount.Mul(*fromTokenPrice)

	// 4. 计算百分比 = (矿工费 USD / 交易金额 USD) * 100
	if txAmountUSD.LessThanOrEqual(decimal.Zero) {
		l.Debugf("txAmountUSD <= 0, returning 0")
		return "0"
	}

	percentage := gasFeeUSD.Div(txAmountUSD).Mul(decimal.NewFromInt(100))
	if percentage.LessThan(decimal.Zero) {
		return "0"
	}
	return percentage.StringFixed(6) // 保留 6 位小数
}

// getGasFeeUSD 获取矿工费的 USD 价值
// 使用 quoteData.EstimatedGas（Gas Limit）和从 ChainRPC 获取的 Gas Price 来计算
func (l *SwapGetQuoteLogic) getGasFeeUSD(chainId int64, quoteData *pb.QuoteData) *decimal.Decimal {
	if l.svcCtx.ChainRpc == nil || quoteData == nil {
		return nil
	}

	// 将 chainId 转换为 ChainRpcType
	chainType := l.chainIdToChainRpcType(chainId)
	if chainType == pb.ChainRpcType_CHAIN_TYPE_UNSPECIFIED {
		return nil
	}

	// 如果 EstimatedGas 为 0，说明报价中没有提供 Gas 信息
	if quoteData.EstimatedGas == 0 {
		return nil
	}

	// 方案1: 调用 GetGasPrice 获取 Gas Price，然后计算矿工费 USD
	// 这是更准确的方法，因为我们可以使用实际的 Gas Limit 和 Gas Price
	// 注意：GetGasPrice 可能还未实现，如果失败则使用备用方案
	gasPriceResp, err := l.svcCtx.ChainRpc.GetGasPrice(l.ctx, &pb.GetGasPriceReq{
		Chain: chainType,
		Speed: pb.GasSpeed_GAS_SPEED_STANDARD, // 使用标准速度
	})
	if err != nil || gasPriceResp == nil || !gasPriceResp.Success || len(gasPriceResp.Prices) == 0 {
		l.Debugf("Failed to get gas price or prices empty, using EstimateGas fallback: %v", err)
		// 如果获取 Gas Price 失败，尝试使用 EstimateGas（需要一个有效的地址）
		return l.getGasFeeUSDViaEstimateGas(chainId, quoteData)
	}

	// 获取标准速度的 Gas Price
	var gasPriceWei *decimal.Decimal
	for _, priceInfo := range gasPriceResp.Prices {
		if priceInfo.Speed == pb.GasSpeed_GAS_SPEED_STANDARD {
			price, err := decimal.NewFromString(priceInfo.GasPrice)
			if err == nil {
				gasPriceWei = &price
				break
			}
		}
	}
	if gasPriceWei == nil {
		// 如果没有找到标准速度，使用第一个可用的价格
		for _, priceInfo := range gasPriceResp.Prices {
			price, err := decimal.NewFromString(priceInfo.GasPrice)
			if err == nil {
				gasPriceWei = &price
				break
			}
		}
	}
	if gasPriceWei == nil {
		l.Debugf("No valid gas price found")
		return l.getGasFeeUSDViaEstimateGas(chainId, quoteData)
	}

	// 计算矿工费 = Gas Limit * Gas Price
	gasLimit := decimal.NewFromInt(int64(quoteData.EstimatedGas))
	gasFeeWei := gasLimit.Mul(*gasPriceWei)

	// 转换为 USD：需要获取主币价格
	gasFeeUSD := l.convertGasFeeToUSD(chainType, gasFeeWei)
	return gasFeeUSD
}

// getGasFeeUSDViaEstimateGas 通过调用 EstimateGas 获取矿工费的 USD 价值
// 这是一个备用方案，当 GetGasPrice 不可用时使用
func (l *SwapGetQuoteLogic) getGasFeeUSDViaEstimateGas(chainId int64, quoteData *pb.QuoteData) *decimal.Decimal {
	if l.svcCtx.ChainRpc == nil || quoteData == nil {
		return nil
	}

	// 将 chainId 转换为 ChainRpcType
	chainType := l.chainIdToChainRpcType(chainId)
	if chainType == pb.ChainRpcType_CHAIN_TYPE_UNSPECIFIED {
		return nil
	}

	// 调用 EstimateGas 需要一个有效的 FromAddress
	// 我们可以使用一个虚拟地址（例如 0x1111111111111111111111111111111111111111）
	// 但对于实际的 swap 交易估算，我们需要 DEX Router 地址
	// 这里我们使用一个简单的估算：调用一个普通的 ERC20 转账来估算基础 Gas

	// 如果 ToToken 地址有效，我们可以用它来估算（虽然不是完全准确）
	toAddr := quoteData.ToToken.GetAddress()
	if toAddr == "" {
		return nil
	}

	gasResp, err := l.svcCtx.ChainRpc.EstimateGas(l.ctx, &pb.EstimateGasReq{
		Chain:       chainType,
		FromAddress: "0x1111111111111111111111111111111111111111", // 使用一个有效的非零地址
		ToAddress:   toAddr,
		Value:       "0",
		Data:        "",
	})
	if err != nil || gasResp == nil || !gasResp.Success {
		l.Debugf("Failed to estimate gas: %v", err)
		return nil
	}

	// 解析矿工费的 USD 价值
	gasFeeUSD, err := decimal.NewFromString(gasResp.EstimatedFeeUsd)
	if err != nil {
		l.Debugf("Failed to parse estimated_fee_usd: %v", err)
		return nil
	}

	return &gasFeeUSD
}

// convertGasFeeToUSD 将矿工费（wei）转换为 USD
func (l *SwapGetQuoteLogic) convertGasFeeToUSD(chainType pb.ChainRpcType, gasFeeWei decimal.Decimal) *decimal.Decimal {
	// 获取主币符号和单位
	var symbol string
	var decimals int64
	switch chainType {
	case pb.ChainRpcType_CHAIN_TYPE_ETHEREUM:
		symbol = "ETHUSDT"
		decimals = 18
	case pb.ChainRpcType_CHAIN_TYPE_BSC:
		symbol = "BNBUSDT"
		decimals = 18
	case pb.ChainRpcType_CHAIN_TYPE_TRON:
		symbol = "TRXUSDT"
		decimals = 6 // TRX 使用 sun，1 TRX = 1,000,000 sun
	default:
		return nil
	}

	// 从 Redis 获取主币价格
	priceUSD := l.getPriceFromRedis(symbol)
	if priceUSD <= 0 {
		// 如果无法从 Redis 获取价格，使用默认价格
		var defaultPrice float64
		switch chainType {
		case pb.ChainRpcType_CHAIN_TYPE_ETHEREUM:
			defaultPrice = 3000.0
		case pb.ChainRpcType_CHAIN_TYPE_BSC:
			defaultPrice = 600.0
		case pb.ChainRpcType_CHAIN_TYPE_TRON:
			defaultPrice = 0.1
		default:
			return nil
		}
		l.Debugf("Using default price for %s: %.2f USD (Redis unavailable)", symbol, defaultPrice)
		priceUSD = defaultPrice
	}

	// 转换为主币数量（除以 10^decimals）
	divisor := decimal.NewFromInt(1)
	for i := int64(0); i < decimals; i++ {
		divisor = divisor.Mul(decimal.NewFromInt(10))
	}
	coinAmount := gasFeeWei.Div(divisor)

	// 计算 USD 价值
	priceDecimal := decimal.NewFromFloat(priceUSD)
	usdValue := coinAmount.Mul(priceDecimal)

	return &usdValue
}

// getTokenPriceUSD 从 Redis 获取代币价格（USD）
func (l *SwapGetQuoteLogic) getTokenPriceUSD(token *pb.SwapTokenInfo) *decimal.Decimal {
	if token == nil || l.svcCtx.RedisClient == nil {
		return nil
	}

	// 获取代币 symbol，构造交易对（例如 ETHUSDT）
	symbol := strings.ToUpper(strings.TrimSpace(token.Symbol))
	if symbol == "" {
		return nil
	}

	// 如果是主币（ETH, BNB, TRX），直接添加 USDT
	// 否则使用 token 的 symbol + USDT
	tickerSymbol := symbol + "USDT"

	// 从 Redis 获取价格
	price := l.getPriceFromRedis(tickerSymbol)
	if price <= 0 {
		return nil
	}

	result := decimal.NewFromFloat(price)
	return &result
}

// getPriceFromRedis 从 Redis 获取交易对价格
// 数据格式来自 market 服务: {"price":"94250.00","ts":1735297200000}
func (l *SwapGetQuoteLogic) getPriceFromRedis(symbol string) float64 {
	if l.svcCtx.RedisClient == nil {
		return 0
	}

	tickerHashKey := "binance:tickers"
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// 从 Redis Hash 获取价格数据
	priceData, err := l.svcCtx.RedisClient.HGet(ctx, tickerHashKey, symbol).Result()
	if err == redis.Nil || err != nil {
		l.Debugf("Failed to get price from Redis for %s: %v", symbol, err)
		return 0
	}

	// 解析 JSON 数据
	var tickerData struct {
		Price string `json:"price"`
		Ts    int64  `json:"ts"`
	}
	if err := json.Unmarshal([]byte(priceData), &tickerData); err != nil {
		l.Debugf("Failed to parse price data for %s: %v", symbol, err)
		return 0
	}

	// 转换为 float64
	price, err := strconv.ParseFloat(tickerData.Price, 64)
	if err != nil {
		l.Debugf("Failed to parse price string for %s: %v", symbol, err)
		return 0
	}

	return price
}

// chainIdToChainRpcType 将 chainId 转换为 ChainRpcType
func (l *SwapGetQuoteLogic) chainIdToChainRpcType(chainId int64) pb.ChainRpcType {
	switch chainId {
	case 1: // Ethereum Mainnet
		return pb.ChainRpcType_CHAIN_TYPE_ETHEREUM
	case 56: // BSC Mainnet
		return pb.ChainRpcType_CHAIN_TYPE_BSC
	case 11155111: // Sepolia Testnet (Ethereum)
		return pb.ChainRpcType_CHAIN_TYPE_ETHEREUM
	case 97: // BSC Testnet
		return pb.ChainRpcType_CHAIN_TYPE_BSC
	default:
		// 默认使用 Ethereum
		return pb.ChainRpcType_CHAIN_TYPE_ETHEREUM
	}
}

// ==================== 金额校验相关方法 ====================

// tokenConfig 代币配置（用于金额校验）
type tokenConfig struct {
	Decimals        uint32
	MinSwapAmountUSD decimal.Decimal
	MaxSwapAmountUSD decimal.Decimal
	PriceUSD        decimal.Decimal
}

// getTokenConfig 获取代币配置
func (l *SwapGetQuoteLogic) getTokenConfig(chainId int64, tokenAddress string) (*tokenConfig, error) {
	// 调用 swap 服务获取支持的代币列表
	resp, err := l.svcCtx.SwapRpc.GetSupportedTokens(l.ctx, &pb.GetSupportedTokensRequest{
		ChainId: chainId,
	})
	if err != nil {
		return nil, err
	}
	if resp == nil || !resp.Success {
		return nil, nil
	}

	// 查找目标代币
	tokenAddress = strings.ToLower(strings.TrimSpace(tokenAddress))
	for _, token := range resp.Tokens {
		if strings.ToLower(strings.TrimSpace(token.Address)) == tokenAddress {
			cfg := &tokenConfig{
				Decimals: token.Decimals,
			}
			if cfg.Decimals == 0 {
				cfg.Decimals = 18 // 默认 18 位小数
			}

			// 解析最小/最大金额
			if token.MinSwapAmountUsd != "" {
				if min, err := decimal.NewFromString(token.MinSwapAmountUsd); err == nil {
					cfg.MinSwapAmountUSD = min
				}
			}
			if token.MaxSwapAmountUsd != "" {
				if max, err := decimal.NewFromString(token.MaxSwapAmountUsd); err == nil {
					cfg.MaxSwapAmountUSD = max
				}
			}

			// 获取代币价格
			cfg.PriceUSD = l.getTokenPriceBySymbol(token.Symbol)

			return cfg, nil
		}
	}

	return nil, nil // 代币不在支持列表中
}

// getTokenPriceBySymbol 通过 symbol 获取代币价格
func (l *SwapGetQuoteLogic) getTokenPriceBySymbol(symbol string) decimal.Decimal {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if symbol == "" {
		return decimal.Zero
	}

	// 稳定币价格固定为 1 USD
	if symbol == "USDT" || symbol == "USDC" || symbol == "DAI" || symbol == "BUSD" {
		return decimal.NewFromInt(1)
	}

	// 从 Redis 获取价格
	tickerSymbol := symbol + "USDT"
	price := l.getPriceFromRedis(tickerSymbol)
	if price <= 0 {
		return decimal.Zero
	}
	return decimal.NewFromFloat(price)
}

// validateSwapAmount 校验兑换金额是否在允许范围内
func (l *SwapGetQuoteLogic) validateSwapAmount(amountStr string, cfg *tokenConfig) error {
	if cfg == nil {
		return nil
	}

	// 解析金额（原始单位，如 wei）
	amount, err := decimal.NewFromString(amountStr)
	if err != nil || amount.LessThanOrEqual(decimal.Zero) {
		return errx.InvalidAmount()
	}

	// 转换为代币数量（除以 10^decimals）
	divisor := decimal.NewFromInt(10).Pow(decimal.NewFromInt(int64(cfg.Decimals)))
	tokenAmount := amount.Div(divisor)

	// 计算 USD 价值
	if cfg.PriceUSD.LessThanOrEqual(decimal.Zero) {
		// 如果无法获取价格，跳过校验
		l.Infof("token price unavailable, skipping amount validation")
		return nil
	}
	amountUSD := tokenAmount.Mul(cfg.PriceUSD)

	l.Infof("swap amount validation: tokenAmount=%s, priceUSD=%s, amountUSD=%s, min=%s, max=%s",
		tokenAmount.String(), cfg.PriceUSD.String(), amountUSD.String(),
		cfg.MinSwapAmountUSD.String(), cfg.MaxSwapAmountUSD.String())

	// 校验最小金额
	if cfg.MinSwapAmountUSD.GreaterThan(decimal.Zero) && amountUSD.LessThan(cfg.MinSwapAmountUSD) {
		return errx.SwapAmountTooSmall(cfg.MinSwapAmountUSD.StringFixed(2))
	}

	// 校验最大金额
	if cfg.MaxSwapAmountUSD.GreaterThan(decimal.Zero) && amountUSD.GreaterThan(cfg.MaxSwapAmountUSD) {
		return errx.SwapAmountTooLarge(cfg.MaxSwapAmountUSD.StringFixed(2))
	}

	return nil
}

// mapSwapError 映射 swap 服务的 gRPC 错误为用户友好的错误
func (l *SwapGetQuoteLogic) mapSwapError(err error) error {
	if err == nil {
		return nil
	}

	st, ok := status.FromError(err)
	if !ok {
		l.Errorf("swap get quote failed: %v", err)
		return errx.SwapServiceError()
	}

	msg := st.Message()
	l.Errorf("swap get quote failed: %v (code: %v)", err, st.Code())

	switch st.Code() {
	case codes.ResourceExhausted:
		return errx.SwapRateLimited()
	case codes.Unavailable:
		return errx.SwapServiceNotAvailable()
	case codes.InvalidArgument:
		// 检查是否是上游错误
		return l.mapUpstreamErrorMessage(msg)
	default:
		return errx.SwapServiceError()
	}
}

// mapUpstreamErrorMessage 将上游错误消息映射为用户友好的错误
func (l *SwapGetQuoteLogic) mapUpstreamErrorMessage(msg string) error {
	msgLower := strings.ToLower(msg)

	// 流动性不足
	if strings.Contains(msgLower, "insufficient liquidity") ||
		strings.Contains(msgLower, "no routes") ||
		strings.Contains(msgLower, "no quote") {
		return errx.SwapInsufficientLiquidity()
	}

	// 限流
	if strings.Contains(msgLower, "rate limit") ||
		strings.Contains(msgLower, "too many requests") {
		return errx.SwapRateLimited()
	}

	// 代币不支持
	if strings.Contains(msgLower, "token not found") ||
		strings.Contains(msgLower, "unsupported token") {
		return errx.SwapTokenNotSupported()
	}

	// 金额相关
	if strings.Contains(msgLower, "amount too small") ||
		strings.Contains(msgLower, "minimum") {
		return errx.SwapAmountTooSmall("1.00")
	}
	if strings.Contains(msgLower, "amount too large") ||
		strings.Contains(msgLower, "maximum") {
		return errx.SwapAmountTooLarge("50000.00")
	}

	// 默认返回上游错误消息
	return errx.InvalidParam(msg)
}
