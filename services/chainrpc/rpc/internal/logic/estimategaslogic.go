package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/fbsobreira/gotron-sdk/pkg/address"
	tronCommon "github.com/fbsobreira/gotron-sdk/pkg/common"
	"github.com/fbsobreira/gotron-sdk/pkg/proto/core"
	"github.com/zeromicro/go-zero/core/logx"

	"internalwallet/proto/pb"
	"internalwallet/services/chainrpc/rpc/internal/svc"
)

type EstimateGasLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewEstimateGasLogic(ctx context.Context, svcCtx *svc.ServiceContext) *EstimateGasLogic {
	return &EstimateGasLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// EstimateGas 估算Gas费用
func (l *EstimateGasLogic) EstimateGas(in *pb.EstimateGasReq) (*pb.EstimateGasResp, error) {
	if in == nil {
		return &pb.EstimateGasResp{
			Success: false,
			Message: "request is required",
		}, nil
	}

	// 验证链类型
	if in.Chain == pb.ChainRpcType_CHAIN_TYPE_UNSPECIFIED {
		return &pb.EstimateGasResp{
			Success: false,
			Message: "chain type is required",
		}, nil
	}

	// 验证地址
	if strings.TrimSpace(in.FromAddress) == "" {
		return &pb.EstimateGasResp{
			Success: false,
			Message: "from_address is required",
		}, nil
	}

	// 根据链类型估算 Gas
	switch in.Chain {
	case pb.ChainRpcType_CHAIN_TYPE_ETHEREUM, pb.ChainRpcType_CHAIN_TYPE_BSC:
		return l.estimateETHGas(in)
	case pb.ChainRpcType_CHAIN_TYPE_TRON:
		return l.estimateTronGas(in)
	default:
		return &pb.EstimateGasResp{
			Success: false,
			Message: fmt.Sprintf("unsupported chain type: %v", in.Chain),
		}, nil
	}
}

// estimateETHGas 估算 ETH/BSC Gas 费用
func (l *EstimateGasLogic) estimateETHGas(in *pb.EstimateGasReq) (*pb.EstimateGasResp, error) {
	// 获取 ETH 客户端
	client, err := l.svcCtx.ChainMgr.GetETHClient(in.Chain)
	if err != nil {
		l.Errorf("failed to get ETH client: %v", err)
		return &pb.EstimateGasResp{
			Success: false,
			Message: fmt.Sprintf("failed to get chain client: %v", err),
		}, nil
	}

	// 解析地址
	fromAddr := ethCommon.HexToAddress(in.FromAddress)
	var toAddr *ethCommon.Address
	if strings.TrimSpace(in.ToAddress) != "" {
		addr := ethCommon.HexToAddress(in.ToAddress)
		toAddr = &addr
	}

	// 解析金额
	var value *big.Int
	if strings.TrimSpace(in.Value) != "" {
		val, ok := new(big.Int).SetString(in.Value, 10)
		if !ok {
			return &pb.EstimateGasResp{
				Success: false,
				Message: fmt.Sprintf("invalid value: %s", in.Value),
			}, nil
		}
		value = val
	} else {
		value = big.NewInt(0)
	}

	// 解析交易数据
	var data []byte
	if strings.TrimSpace(in.Data) != "" {
		// 如果是代币转账，需要构建 transfer 函数调用数据
		if strings.TrimSpace(in.TokenContract) != "" {
			// 代币转账：需要构建 ERC20 transfer 调用
			tokenContract := ethCommon.HexToAddress(in.TokenContract)
			if toAddr == nil || strings.TrimSpace(in.Value) == "" {
				return &pb.EstimateGasResp{
					Success: false,
					Message: "to_address and value are required for token transfer",
				}, nil
			}

			// 使用 ERC20 transfer ABI
			erc20TransferABI := `[{"constant":false,"inputs":[{"name":"_to","type":"address"},{"name":"_value","type":"uint256"}],"name":"transfer","outputs":[{"name":"","type":"bool"}],"payable":false,"stateMutability":"nonpayable","type":"function"}]`
			parsedABI, err := abi.JSON(strings.NewReader(erc20TransferABI))
			if err != nil {
				return &pb.EstimateGasResp{
					Success: false,
					Message: fmt.Sprintf("failed to parse ERC20 ABI: %v", err),
				}, nil
			}

			// 编码 transfer 函数调用
			data, err = parsedABI.Pack("transfer", *toAddr, value)
			if err != nil {
				return &pb.EstimateGasResp{
					Success: false,
					Message: fmt.Sprintf("failed to pack transfer data: %v", err),
				}, nil
			}

			// 代币转账时，to 地址是代币合约，value 为 0
			toAddr = &tokenContract
			value = big.NewInt(0)
		} else {
			// 直接使用提供的 data
			var err error
			data, err = hexutil.Decode(in.Data)
			if err != nil {
				return &pb.EstimateGasResp{
					Success: false,
					Message: fmt.Sprintf("invalid data hex: %v", err),
				}, nil
			}
		}
	}

	// 先查询账户余额，以便给出更友好的错误信息
	balance, err := client.BalanceAt(context.Background(), fromAddr, nil)
	if err != nil {
		l.Errorf("failed to query balance: %v", err)
		// 即使余额查询失败，也继续尝试估算 gas（可能只是 RPC 节点问题）
	} else {
		balanceETH, _ := new(big.Float).Quo(new(big.Float).SetInt(balance), new(big.Float).SetInt64(1e18)).Float64()
		l.Infof("Account balance: %s wei (%.6f ETH)", balance.String(), balanceETH)
	}

	// 获取 Gas Price（需要先知道 gas price 才能估算总费用）
	gasPrice, err := client.SuggestGasPrice(context.Background())
	if err != nil {
		l.Errorf("failed to get gas price: %v", err)
		return &pb.EstimateGasResp{
			Success: false,
			Message: fmt.Sprintf("failed to get gas price: %v", err),
		}, nil
	}

	// 构建 CallMsg
	// 注意：EstimateGas 不需要设置 GasPrice，它会自动估算 gas limit
	msg := ethereum.CallMsg{
		From:  fromAddr,
		To:    toAddr,
		Value: value,
		Data:  data,
	}

	// 估算 Gas Limit
	gasLimit, err := client.EstimateGas(context.Background(), msg)
	if err != nil {
		// 检查是否是余额不足的错误
		errMsg := err.Error()
		if strings.Contains(errMsg, "insufficient funds") || strings.Contains(errMsg, "have") && strings.Contains(errMsg, "want") {
			// 提取余额信息，提供更友好的错误消息
			if balance != nil {
				balanceETH, _ := new(big.Float).Quo(new(big.Float).SetInt(balance), new(big.Float).SetInt64(1e18)).Float64()
				required := new(big.Int).Mul(gasPrice, big.NewInt(21000)) // 使用最小 gas limit 估算
				required.Add(required, value)
				requiredETH, _ := new(big.Float).Quo(new(big.Float).SetInt(required), new(big.Float).SetInt64(1e18)).Float64()
				return &pb.EstimateGasResp{
					Success: false,
					Message: fmt.Sprintf("insufficient funds: balance %.6f ETH, required at least %.6f ETH (including gas fee)", balanceETH, requiredETH),
				}, nil
			}
		}
		l.Errorf("failed to estimate gas: %v", err)
		return &pb.EstimateGasResp{
			Success: false,
			Message: fmt.Sprintf("failed to estimate gas: %v", err),
		}, nil
	}

	// 再次检查余额是否足够（使用估算的 gas limit）
	if balance != nil {
		estimatedFee := new(big.Int).Mul(gasPrice, new(big.Int).SetUint64(gasLimit))
		required := new(big.Int).Add(estimatedFee, value)
		if balance.Cmp(required) < 0 {
			balanceETH, _ := new(big.Float).Quo(new(big.Float).SetInt(balance), new(big.Float).SetInt64(1e18)).Float64()
			requiredETH, _ := new(big.Float).Quo(new(big.Float).SetInt(required), new(big.Float).SetInt64(1e18)).Float64()
			valueETH, _ := new(big.Float).Quo(new(big.Float).SetInt(value), new(big.Float).SetInt64(1e18)).Float64()
			feeETH, _ := new(big.Float).Quo(new(big.Float).SetInt(estimatedFee), new(big.Float).SetInt64(1e18)).Float64()
		return &pb.EstimateGasResp{
			Success: false,
				Message: fmt.Sprintf("insufficient funds: balance %.6f ETH, required %.6f ETH (transfer %.6f ETH + gas fee %.6f ETH)",
					balanceETH, requiredETH, valueETH, feeETH),
		}, nil
		}
	}

	// 计算预估费用（Gas Limit * Gas Price）
	// 注意：gasPrice 已经在上面获取了
	estimatedFee := new(big.Int).Mul(gasPrice, new(big.Int).SetUint64(gasLimit))

	// 计算 USD 价值（需要获取主币价格）
	estimatedFeeUSD := l.calculateFeeUSD(in.Chain, estimatedFee)

	return &pb.EstimateGasResp{
		Success:         true,
		Message:         "Gas estimated successfully",
		GasLimit:        gasLimit,
		GasPrice:        gasPrice.String(),
		EstimatedFee:    estimatedFee.String(),
		EstimatedFeeUsd: estimatedFeeUSD,
	}, nil
}

// estimateTronGas 估算 TRON 手续费
// TRON 使用带宽（Bandwidth）和能量（Energy），不是 Gas
func (l *EstimateGasLogic) estimateTronGas(in *pb.EstimateGasReq) (*pb.EstimateGasResp, error) {
	// TRON 的手续费计算方式不同：
	// 1. 主币转账：消耗带宽（Bandwidth），通常为 268 带宽
	// 2. 代币转账（TRC20）：消耗带宽和能量（Energy）
	// 3. 带宽可以通过冻结 TRX 获得，能量需要冻结 TRX 或租用

	// 获取 TRON 客户端
	tronClient, err := l.svcCtx.ChainMgr.GetTronClient()
	if err != nil {
		l.Errorf("failed to get TRON client: %v", err)
		return &pb.EstimateGasResp{
			Success: false,
			Message: fmt.Sprintf("failed to get TRON client: %v", err),
		}, nil
	}
	defer tronClient.Stop()

	// TRON 主币转账的固定带宽消耗
	var bandwidth uint64 = 268
	var energy uint64 = 0

	// 如果是代币转账，使用 EstimateEnergy API 查询实际能量消耗
	if strings.TrimSpace(in.TokenContract) != "" {
		// 验证必要参数
		if strings.TrimSpace(in.ToAddress) == "" || strings.TrimSpace(in.Value) == "" {
			return &pb.EstimateGasResp{
				Success: false,
				Message: "to_address and value are required for TRC20 token transfer",
			}, nil
		}

		// 解析金额
		amount, ok := new(big.Int).SetString(in.Value, 10)
		if !ok {
			return &pb.EstimateGasResp{
				Success: false,
				Message: fmt.Sprintf("invalid value: %s", in.Value),
			}, nil
		}

		// 构建 TRC20 transfer 调用数据（类似 TRC20Send）
		toAddr, err := address.Base58ToAddress(in.ToAddress)
		if err != nil {
			l.Errorf("failed to parse to_address: %v", err)
			// 如果地址解析失败，使用默认能量值
			energy = 20000
		} else {
			// 构建 transfer 方法调用数据
			// transfer(address,uint256) 方法签名: 0xa9059cbb
			trc20TransferMethodSignature := "0xa9059cbb"
			ab := tronCommon.LeftPadBytes(amount.Bytes(), 32)
			req := trc20TransferMethodSignature + "0000000000000000000000000000000000000000000000000000000000000000"[len(toAddr.Hex())-4:] + toAddr.Hex()[4:]
			req += tronCommon.Bytes2Hex(ab)

			// 解析数据
			dataBytes, err := tronCommon.FromHex(req)
			if err != nil {
				l.Errorf("failed to parse transfer data: %v", err)
				energy = 20000 // 使用默认值
			} else {
				// 构建 TriggerSmartContract
				fromDesc, err := address.Base58ToAddress(in.FromAddress)
				if err != nil {
					l.Errorf("failed to parse from_address: %v", err)
					energy = 20000 // 使用默认值
				} else {
					contractDesc, err := address.Base58ToAddress(in.TokenContract)
					if err != nil {
						l.Errorf("failed to parse token_contract: %v", err)
						energy = 20000 // 使用默认值
					} else {
						ct := &core.TriggerSmartContract{
							OwnerAddress:    fromDesc.Bytes(),
							ContractAddress: contractDesc.Bytes(),
							Data:            dataBytes,
						}

						// 调用 EstimateEnergy API（通过 gRPC 客户端）
						ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
						energyResp, err := tronClient.Client.EstimateEnergy(ctx, ct)
						cancel()
						if err != nil {
							l.Errorf("failed to estimate energy, using default value: %v", err)
							energy = 20000 // 使用默认值
						} else if energyResp != nil && energyResp.EnergyRequired > 0 {
							energy = uint64(energyResp.EnergyRequired)
							l.Infof("Estimated energy for TRC20 transfer: %d", energy)
						} else {
							energy = 20000 // 使用默认值
						}
					}
				}
			}
		}
	}

	// 查询账户的带宽和能量余额
	accountResource, err := tronClient.GetAccountResource(in.FromAddress)
	var bandwidthAvailable int64 = 0
	var energyAvailable int64 = 0
	if err != nil {
		l.Errorf("failed to get account resource, assuming no resources: %v", err)
	} else if accountResource != nil {
		// 计算可用带宽（总带宽 - 已使用带宽）
		bandwidthTotal := accountResource.GetFreeNetLimit() + accountResource.GetNetLimit()
		bandwidthUsed := accountResource.GetFreeNetUsed() + accountResource.GetNetUsed()
		bandwidthAvailable = bandwidthTotal - bandwidthUsed
		if bandwidthAvailable < 0 {
			bandwidthAvailable = 0
		}

		// 计算可用能量（总能量 - 已使用能量）
		energyTotal := accountResource.GetEnergyLimit()
		energyUsed := accountResource.GetEnergyUsed()
		energyAvailable = energyTotal - energyUsed
		if energyAvailable < 0 {
			energyAvailable = 0
		}

		l.Infof("Account resources - Bandwidth: available=%d, required=%d, Energy: available=%d, required=%d",
			bandwidthAvailable, bandwidth, energyAvailable, energy)
	}

	// 计算需要消耗的 TRX（如果资源不足）
	var estimatedFeeSun *big.Int = big.NewInt(0)

	// 检查带宽是否足够
	bandwidthNeeded := int64(bandwidth)
	if bandwidthAvailable < bandwidthNeeded {
		bandwidthShortage := bandwidthNeeded - bandwidthAvailable
		// 获取带宽价格
		bandwidthPriceResp, err := tronClient.GetBandwidthPrices()
		if err != nil {
			l.Errorf("failed to get bandwidth prices, using default: %v", err)
			// 默认：1 带宽 ≈ 1 sun（如果带宽不足）
			estimatedFeeSun.Add(estimatedFeeSun, big.NewInt(bandwidthShortage))
		} else if bandwidthPriceResp != nil && bandwidthPriceResp.Prices != "" {
			// 解析价格（单位：sun per bandwidth）
			price, err := strconv.ParseInt(bandwidthPriceResp.Prices, 10, 64)
			if err != nil {
				l.Errorf("failed to parse bandwidth price, using default: %v", err)
				estimatedFeeSun.Add(estimatedFeeSun, big.NewInt(bandwidthShortage))
			} else {
				bandwidthCost := big.NewInt(price)
				bandwidthCost.Mul(bandwidthCost, big.NewInt(bandwidthShortage))
				estimatedFeeSun.Add(estimatedFeeSun, bandwidthCost)
			}
		} else {
			estimatedFeeSun.Add(estimatedFeeSun, big.NewInt(bandwidthShortage))
		}
	}

	// 检查能量是否足够（仅代币转账需要）
	if energy > 0 && energyAvailable < int64(energy) {
		energyShortage := int64(energy) - energyAvailable
		// 获取能量价格
		energyPriceResp, err := tronClient.GetEnergyPrices()
		if err != nil {
			l.Errorf("failed to get energy prices, using default: %v", err)
			// 默认：1 能量 ≈ 420 sun（如果能量不足，根据市场价格）
			estimatedFeeSun.Add(estimatedFeeSun, big.NewInt(420*energyShortage))
		} else if energyPriceResp != nil && energyPriceResp.Prices != "" {
			// 解析价格（单位：sun per energy）
			price, err := strconv.ParseInt(energyPriceResp.Prices, 10, 64)
			if err != nil {
				l.Errorf("failed to parse energy price, using default: %v", err)
				estimatedFeeSun.Add(estimatedFeeSun, big.NewInt(420*energyShortage))
			} else {
				energyCost := big.NewInt(price)
				energyCost.Mul(energyCost, big.NewInt(energyShortage))
				estimatedFeeSun.Add(estimatedFeeSun, energyCost)
			}
		} else {
			estimatedFeeSun.Add(estimatedFeeSun, big.NewInt(420*energyShortage))
		}
	}

	// 计算 USD 价值
	estimatedFeeUSD := l.calculateFeeUSD(pb.ChainRpcType_CHAIN_TYPE_TRON, estimatedFeeSun)

	// TRON 使用带宽和能量，但为了兼容性，我们使用类似 Gas 的概念
	// Gas Limit = 带宽 + 能量（转换为统一单位）
	gasLimit := bandwidth + energy

	message := "TRON fee estimated successfully"
	if estimatedFeeSun.Cmp(big.NewInt(0)) == 0 {
		message += " (sufficient resources, no TRX needed)"
	} else {
		message += fmt.Sprintf(" (bandwidth: %d, energy: %d)", bandwidth, energy)
	}

	return &pb.EstimateGasResp{
		Success:         true,
		Message:         message,
		GasLimit:        gasLimit,
		GasPrice:        "1", // TRON 不使用 Gas Price，这里返回 1 作为占位符
		EstimatedFee:    estimatedFeeSun.String(),
		EstimatedFeeUsd: estimatedFeeUSD,
	}, nil
}

// calculateFeeUSD 计算手续费 USD 价值
// 从 market 服务存储的 Redis 数据中获取实时价格
func (l *EstimateGasLogic) calculateFeeUSD(chainType pb.ChainRpcType, feeAmount *big.Int) string {
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
		return "0"
	}

	// 从 Redis 获取价格（market 服务存储的数据格式: {"price":"94250.00","ts":1735297200000}）
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
			return "0"
		}
		l.Logger.Debugf("Using default price for %s: %.2f USD (Redis unavailable)", symbol, defaultPrice)
		priceUSD = defaultPrice
	}

	// 转换为主币数量（除以 10^decimals）
	divisor := new(big.Float).SetInt64(1)
	for i := int64(0); i < decimals; i++ {
		divisor.Mul(divisor, big.NewFloat(10))
	}
	coinAmount := new(big.Float).Quo(new(big.Float).SetInt(feeAmount), divisor)

	// 计算 USD 价值
	usdValue, _ := coinAmount.Float64()
	usdTotal := usdValue * priceUSD

	return fmt.Sprintf("%.6f", usdTotal)
}

// getPriceFromRedis 从 Redis 获取交易对价格
// 数据格式来自 market 服务: {"price":"94250.00","ts":1735297200000}
func (l *EstimateGasLogic) getPriceFromRedis(symbol string) float64 {
	if l.svcCtx.RedisClient == nil {
		return 0
	}

	tickerHashKey := "binance:tickers"
	if l.svcCtx.Config.MarketPrice.TickerHashKey != "" {
		tickerHashKey = l.svcCtx.Config.MarketPrice.TickerHashKey
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// 从 Redis Hash 获取价格数据
	priceData, err := l.svcCtx.RedisClient.HGet(ctx, tickerHashKey, symbol).Result()
	if err != nil {
		l.Logger.Debugf("Failed to get price from Redis for %s: %v", symbol, err)
		return 0
	}

	// 解析 JSON 数据
	var tickerData struct {
		Price string `json:"price"`
		Ts    int64  `json:"ts"`
	}
	if err := json.Unmarshal([]byte(priceData), &tickerData); err != nil {
		l.Logger.Debugf("Failed to parse price data for %s: %v", symbol, err)
		return 0
	}

	// 转换为 float64
	price, err := strconv.ParseFloat(tickerData.Price, 64)
	if err != nil {
		l.Logger.Debugf("Failed to parse price string for %s: %v", symbol, err)
		return 0
	}

	return price
}
