package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"time"

	"github.com/zeromicro/go-zero/core/logx"

	"internalwallet/proto/pb"
	"internalwallet/services/chainrpc/rpc/internal/svc"
)

// calculateFeeUSD converts a native fee amount (wei/sun) into an approximate USD value using Redis ticker data.
// It follows the same behavior as the previous EstimateGasLogic.calculateFeeUSD implementation.
func calculateFeeUSD(svcCtx *svc.ServiceContext, logger logx.Logger, chainType pb.ChainRpcType, feeAmount *big.Int) string {
	if feeAmount == nil || feeAmount.Sign() <= 0 {
		return "0"
	}

	// Resolve symbol + native decimals.
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
		decimals = 6 // TRX uses SUN (1 TRX = 1,000,000 SUN)
	default:
		return "0"
	}

	// market service stores ticker data in Redis: {"price":"0.1234","ts":1735297200000}
	priceUSD := getPriceFromRedis(svcCtx, logger, symbol)
	if priceUSD <= 0 {
		// Fallbacks for local/dev without Redis.
		var defaultPrice float64
		switch chainType {
		case pb.ChainRpcType_CHAIN_TYPE_ETHEREUM:
			defaultPrice = 3000.0
		case pb.ChainRpcType_CHAIN_TYPE_BSC:
			defaultPrice = 600.0
		case pb.ChainRpcType_CHAIN_TYPE_TRON:
			defaultPrice = 0.1
		default:
			return "0"
		}
		logger.Debugf("Using default price for %s: %.2f USD (Redis unavailable)", symbol, defaultPrice)
		priceUSD = defaultPrice
	}

	// Convert feeAmount into native coin amount (divide by 10^decimals).
	divisor := new(big.Float).SetInt64(1)
	for i := int64(0); i < decimals; i++ {
		divisor.Mul(divisor, big.NewFloat(10))
	}
	coinAmount := new(big.Float).Quo(new(big.Float).SetInt(feeAmount), divisor)

	usdValue, _ := coinAmount.Float64()
	usdTotal := usdValue * priceUSD
	return fmt.Sprintf("%.6f", usdTotal)
}

func getPriceFromRedis(svcCtx *svc.ServiceContext, logger logx.Logger, symbol string) float64 {
	if svcCtx == nil || svcCtx.RedisClient == nil {
		return 0
	}

	tickerHashKey := "binance:tickers"
	if svcCtx.Config.MarketPrice.TickerHashKey != "" {
		tickerHashKey = svcCtx.Config.MarketPrice.TickerHashKey
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	priceData, err := svcCtx.RedisClient.HGet(ctx, tickerHashKey, symbol).Result()
	if err != nil {
		logger.Debugf("Failed to get price from Redis for %s: %v", symbol, err)
		return 0
	}

	var tickerData struct {
		Price string `json:"price"`
		Ts    int64  `json:"ts"`
	}
	if err := json.Unmarshal([]byte(priceData), &tickerData); err != nil {
		logger.Debugf("Failed to parse price data for %s: %v", symbol, err)
		return 0
	}

	price, err := strconv.ParseFloat(tickerData.Price, 64)
	if err != nil {
		logger.Debugf("Failed to parse price string for %s: %v", symbol, err)
		return 0
	}

	return price
}
