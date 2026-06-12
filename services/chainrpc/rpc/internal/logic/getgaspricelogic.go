package logic

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"internalwallet/proto/pb"
	"internalwallet/services/chainrpc/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetGasPriceLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetGasPriceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetGasPriceLogic {
	return &GetGasPriceLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetGasPrice 获取推荐Gas价格（分级：慢/标准/快/极速）
func (l *GetGasPriceLogic) GetGasPrice(in *pb.GetGasPriceReq) (*pb.GetGasPriceResp, error) {
	if in == nil {
		return &pb.GetGasPriceResp{
			Success: false,
			Message: "request is required",
		}, nil
	}

	// 验证链类型
	if in.Chain == pb.ChainRpcType_CHAIN_TYPE_UNSPECIFIED {
		return &pb.GetGasPriceResp{
			Success: false,
			Message: "chain type is required",
		}, nil
	}

	// 根据链类型获取 Gas 价格
	switch in.Chain {
	case pb.ChainRpcType_CHAIN_TYPE_ETHEREUM, pb.ChainRpcType_CHAIN_TYPE_BSC:
		return l.getEVMGasPrice(in)
	case pb.ChainRpcType_CHAIN_TYPE_TRON:
		return &pb.GetGasPriceResp{
			Success: false,
			Message: "TRON does not use EVM gas price. Use EstimateTronFee / GetTronAccountResources instead.",
		}, nil
	default:
		return &pb.GetGasPriceResp{
			Success: false,
			Message: fmt.Sprintf("unsupported chain type: %v", in.Chain),
		}, nil
	}
}

// getEVMGasPrice 获取 EVM 链（ETH/BSC/Polygon）的分级 Gas 价格
func (l *GetGasPriceLogic) getEVMGasPrice(in *pb.GetGasPriceReq) (*pb.GetGasPriceResp, error) {
	// 获取 ETH 客户端
	client, err := l.svcCtx.ChainMgr.GetETHClient(in.Chain)
	if err != nil {
		l.Errorf("failed to get ETH client: %v", err)
		return &pb.GetGasPriceResp{
			Success: false,
			Message: fmt.Sprintf("failed to get chain client: %v", err),
		}, nil
	}

	// 获取当前建议的 Gas Price（标准速度）
	standardGasPrice, err := client.SuggestGasPrice(context.Background())
	if err != nil {
		l.Errorf("failed to get gas price: %v", err)
		return &pb.GetGasPriceResp{
			Success: false,
			Message: fmt.Sprintf("failed to get gas price: %v", err),
		}, nil
	}

	minGasPrice := minGasPriceWei(in.Chain)
	if minGasPrice != nil && standardGasPrice.Cmp(minGasPrice) < 0 {
		l.Infof("suggested gas price below minimum, clamping: chain=%v suggested=%v min=%v", in.Chain, standardGasPrice, minGasPrice)
		standardGasPrice = new(big.Int).Set(minGasPrice)
	}

	// 获取当前区块号
	blockNumber, err := client.BlockNumber(context.Background())
	if err != nil {
		l.Infof("failed to get block number: %v", err)
	}

	// 计算不同速度级别的 Gas Price
	// 慢速：标准价格的 0.8 倍（20% 折扣，约 2 分钟确认）
	slowGasPrice := clampGasPrice(scaleGasPrice(standardGasPrice, 80, 100), minGasPrice)

	// 快速：标准价格的 1.2 倍（20% 溢价，约 15 秒确认）
	fastGasPrice := clampGasPrice(scaleGasPrice(standardGasPrice, 120, 100), minGasPrice)

	// 极速：标准价格的 1.5 倍（50% 溢价，约 10 秒确认）
	instantGasPrice := clampGasPrice(scaleGasPrice(standardGasPrice, 150, 100), minGasPrice)

	// 估算 Gas Limit（根据交易类型，默认使用 swap）
	transactionType := strings.TrimSpace(in.TransactionType)
	if transactionType == "" {
		transactionType = "swap" // 默认按 Swap 估算（UI 场景）
	}
	gasLimit := estimateGasLimit(in.Chain, transactionType)

	// 如果用户指定了速度级别，只返回该级别的价格
	if in.Speed != pb.GasSpeed_GAS_SPEED_UNSPECIFIED {
		var selectedPrice *big.Int
		var selectedSpeed pb.GasSpeed
		var estimatedTime int32

		switch in.Speed {
		case pb.GasSpeed_GAS_SPEED_SLOW:
			selectedPrice = slowGasPrice
			selectedSpeed = pb.GasSpeed_GAS_SPEED_SLOW
			estimatedTime = 120 // 2 分钟
		case pb.GasSpeed_GAS_SPEED_STANDARD:
			selectedPrice = standardGasPrice
			selectedSpeed = pb.GasSpeed_GAS_SPEED_STANDARD
			estimatedTime = 30 // 30 秒
		case pb.GasSpeed_GAS_SPEED_FAST:
			selectedPrice = fastGasPrice
			selectedSpeed = pb.GasSpeed_GAS_SPEED_FAST
			estimatedTime = 15 // 15 秒
		case pb.GasSpeed_GAS_SPEED_INSTANT:
			selectedPrice = instantGasPrice
			selectedSpeed = pb.GasSpeed_GAS_SPEED_INSTANT
			estimatedTime = 10 // 10 秒
		}

		return &pb.GetGasPriceResp{
			Success:      true,
			Message:      "ok",
			Prices:       []*pb.GasPriceInfo{buildGasPriceInfo(selectedSpeed, selectedPrice, estimatedTime, gasLimit)},
			CurrentBlock: fmt.Sprintf("%d", blockNumber),
			Timestamp:    time.Now().Unix(),
		}, nil
	}

	// 返回所有速度级别的价格
	return &pb.GetGasPriceResp{
		Success: true,
		Message: "ok",
		Prices: []*pb.GasPriceInfo{
			buildGasPriceInfo(pb.GasSpeed_GAS_SPEED_SLOW, slowGasPrice, 120, gasLimit),
			buildGasPriceInfo(pb.GasSpeed_GAS_SPEED_STANDARD, standardGasPrice, 30, gasLimit),
			buildGasPriceInfo(pb.GasSpeed_GAS_SPEED_FAST, fastGasPrice, 15, gasLimit),
			buildGasPriceInfo(pb.GasSpeed_GAS_SPEED_INSTANT, instantGasPrice, 10, gasLimit),
		},
		CurrentBlock: fmt.Sprintf("%d", blockNumber),
		Timestamp:    time.Now().Unix(),
	}, nil
}

// weiToGwei 将 Wei 转换为 Gwei
func weiToGwei(wei *big.Int) string {
	gwei := new(big.Float).Quo(new(big.Float).SetInt(wei), new(big.Float).SetInt64(1e9))
	return gwei.Text('f', 2) // 保留2位小数
}

// minGasPriceWei returns chain-specific minimum gas price in wei.
func minGasPriceWei(chain pb.ChainRpcType) *big.Int {
	switch chain {
	case pb.ChainRpcType_CHAIN_TYPE_BSC:
		return big.NewInt(1_000_000_000) // 1 gwei
	default:
		return nil
	}
}

func clampGasPrice(price *big.Int, min *big.Int) *big.Int {
	if min == nil {
		return price
	}
	if price.Cmp(min) < 0 {
		return new(big.Int).Set(min)
	}
	return price
}

func scaleGasPrice(price *big.Int, numerator int64, denominator int64) *big.Int {
	scaled := new(big.Int).Mul(price, big.NewInt(numerator))
	return scaled.Div(scaled, big.NewInt(denominator))
}

// estimateGasLimit 根据交易类型估算 Gas Limit
func estimateGasLimit(chain pb.ChainRpcType, transactionType string) uint64 {
	transactionType = strings.ToLower(strings.TrimSpace(transactionType))

	switch chain {
	case pb.ChainRpcType_CHAIN_TYPE_ETHEREUM, pb.ChainRpcType_CHAIN_TYPE_BSC:
		// EVM 链的典型 Gas Limit
		switch transactionType {
		case "native_transfer", "native":
			return 21000 // ETH/BNB/MATIC 转账标准 Gas Limit
		case "token_transfer", "token", "erc20":
			return 65000 // ERC20 转账典型值
		case "approve":
			return 50000 // ERC20 授权
		case "swap", "uniswap", "pancakeswap":
			return 200000 // DEX Swap 典型值（Uniswap/PancakeSwap）
		case "swap_with_permit":
			return 250000 // Swap + Permit 签名
		case "add_liquidity":
			return 300000 // 添加流动性
		case "remove_liquidity":
			return 250000 // 移除流动性
		case "nft_transfer", "erc721", "erc1155":
			return 100000 // NFT 转账
		default:
			// 默认返回一个安全的中等值
			return 100000
		}

	default:
		return 100000 // 默认值
	}
}

// buildGasPriceInfo 构建包含完整费用信息的 GasPriceInfo
func buildGasPriceInfo(speed pb.GasSpeed, gasPrice *big.Int, estimatedTime int32, gasLimit uint64) *pb.GasPriceInfo {
	// 计算最大费用：gasPrice * gasLimit
	maxFeeWei := new(big.Int).Mul(gasPrice, big.NewInt(int64(gasLimit)))

	return &pb.GasPriceInfo{
		Speed:                speed,
		GasPrice:             gasPrice.String(),
		GasPriceGwei:         weiToGwei(gasPrice),
		EstimatedTimeSeconds: estimatedTime,
		GasLimit:             gasLimit,
		MaxFeeWei:            maxFeeWei.String(),
		MaxFeeGwei:           weiToGwei(maxFeeWei),
	}
}
