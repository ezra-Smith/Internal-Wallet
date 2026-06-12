package logic

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/zeromicro/go-zero/core/logx"

	"internalwallet/proto/pb"
	"internalwallet/services/chainrpc/rpc/internal/svc"
)

type GetTokenBalanceLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetTokenBalanceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetTokenBalanceLogic {
	return &GetTokenBalanceLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ERC20 balanceOf ABI
var erc20BalanceOfABI = `[{
	"constant": true,
	"inputs": [{"name": "_owner", "type": "address"}],
	"name": "balanceOf",
	"outputs": [{"name": "balance", "type": "uint256"}],
	"payable": false,
	"stateMutability": "view",
	"type": "function"
}]`

// 查询代币余额
func (l *GetTokenBalanceLogic) GetTokenBalance(in *pb.GetTokenBalanceReq) (*pb.GetTokenBalanceResp, error) {
	// 参数验证
	if err := l.validateGetTokenBalanceReq(in); err != nil {
		return &pb.GetTokenBalanceResp{
			Success: false,
			Message: fmt.Sprintf("validation failed: %v", err),
		}, nil
	}

	// 根据链类型调用相应的方法
	switch in.Chain {
	case pb.ChainRpcType_CHAIN_TYPE_ETHEREUM, pb.ChainRpcType_CHAIN_TYPE_BSC:
		return l.getETHTokenBalance(in)
	case pb.ChainRpcType_CHAIN_TYPE_TRON:
		return l.getTronTokenBalance(in)
	default:
		return &pb.GetTokenBalanceResp{
			Success: false,
			Message: fmt.Sprintf("unsupported chain type: %v", in.Chain),
		}, nil
	}
}

// validateGetTokenBalanceReq 验证请求参数
func (l *GetTokenBalanceLogic) validateGetTokenBalanceReq(in *pb.GetTokenBalanceReq) error {
	if in == nil {
		return fmt.Errorf("request is nil")
	}

	if in.Address == "" {
		return fmt.Errorf("address is required")
	}

	if in.TokenContract == "" {
		return fmt.Errorf("token contract address is required")
	}

	// 根据链类型验证地址格式
	switch in.Chain {
	case pb.ChainRpcType_CHAIN_TYPE_ETHEREUM, pb.ChainRpcType_CHAIN_TYPE_BSC:
		if !common.IsHexAddress(in.Address) {
			return fmt.Errorf("invalid address format: %s", in.Address)
		}
		if !common.IsHexAddress(in.TokenContract) {
			return fmt.Errorf("invalid token contract address format: %s", in.TokenContract)
		}
	case pb.ChainRpcType_CHAIN_TYPE_TRON:
		if !l.isValidTronAddress(in.Address) {
			return fmt.Errorf("invalid TRON address: %s", in.Address)
		}
		if !l.isValidTronAddress(in.TokenContract) {
			return fmt.Errorf("invalid TRON token contract address: %s", in.TokenContract)
		}
	}

	return nil
}

// isValidTronAddress 验证TRON地址格式
func (l *GetTokenBalanceLogic) isValidTronAddress(addr string) bool {
	if addr == "" {
		return false
	}

	// TRON地址应该是Base58格式且以'T'开头
	if !strings.HasPrefix(addr, "T") {
		return false
	}

	// 检查长度（TRON Base58地址通常是33-35个字符）
	if len(addr) < 33 || len(addr) > 35 {
		return false
	}

	return true
}

// getETHTokenBalance 获取ETH/BSC代币余额
func (l *GetTokenBalanceLogic) getETHTokenBalance(in *pb.GetTokenBalanceReq) (*pb.GetTokenBalanceResp, error) {
	// 获取ETH客户端
	client, err := l.svcCtx.ChainMgr.GetETHClient(in.Chain)
	if err != nil {
		return &pb.GetTokenBalanceResp{
			Success: false,
			Message: fmt.Sprintf("failed to get ETH client: %v", err),
		}, nil
	}

	// 解析地址和合约地址
	address := common.HexToAddress(in.Address)
	contractAddr := common.HexToAddress(in.TokenContract)

	// 解析ERC20 ABI
	parsedABI, err := abi.JSON(strings.NewReader(erc20BalanceOfABI))
	if err != nil {
		return &pb.GetTokenBalanceResp{
			Success: false,
			Message: fmt.Sprintf("failed to parse ERC20 ABI: %v", err),
		}, nil
	}

	// 编码balanceOf函数调用
	data, err := parsedABI.Pack("balanceOf", address)
	if err != nil {
		return &pb.GetTokenBalanceResp{
			Success: false,
			Message: fmt.Sprintf("failed to pack balanceOf function: %v", err),
		}, nil
	}

	// 构建调用消息
	callMsg := ethereum.CallMsg{
		To:   &contractAddr,
		Data: data,
	}

	// 如果指定了区块号，设置区块号
	var blockNumber *big.Int
	if in.BlockNumber != "" {
		if bn, ok := new(big.Int).SetString(in.BlockNumber, 10); ok {
			blockNumber = bn
		}
	}

	// 调用合约
	result, err := client.CallContract(context.Background(), callMsg, blockNumber)
	if err != nil {
		return &pb.GetTokenBalanceResp{
			Success: false,
			Message: fmt.Sprintf("failed to call balanceOf function: %v", err),
		}, nil
	}

	// 解码返回值
	var balance *big.Int
	err = parsedABI.UnpackIntoInterface(&balance, "balanceOf", result)
	if err != nil {
		return &pb.GetTokenBalanceResp{
			Success: false,
			Message: fmt.Sprintf("failed to unpack balanceOf result: %v", err),
		}, nil
	}

	// 尝试获取代币符号和精度（可选）
	tokenSymbol := "UNKNOWN"
	tokenDecimals := uint32(18) // 默认精度

	// 获取代币符号
	if symbol, err := l.getETHTokenSymbol(client, contractAddr, blockNumber); err == nil {
		tokenSymbol = symbol
	}

	// 获取代币精度
	if decimals, err := l.getETHTokenDecimals(client, contractAddr, blockNumber); err == nil {
		tokenDecimals = decimals
	}

	// 格式化余额
	balanceFormatted := l.formatTokenAmount(balance, tokenDecimals)

	l.Logger.Infof("Successfully retrieved ETH/BSC token balance for %s: %s %s",
		in.Address, balanceFormatted, tokenSymbol)

	return &pb.GetTokenBalanceResp{
		Success:          true,
		Message:          "Token balance retrieved successfully",
		Balance:          balance.String(),
		BalanceFormatted: balanceFormatted,
		TokenSymbol:      tokenSymbol,
		TokenDecimals:    tokenDecimals,
		BalanceUsd:       0.0, // 需要外部API支持
	}, nil
}

// getTronTokenBalance 获取TRON代币余额
func (l *GetTokenBalanceLogic) getTronTokenBalance(in *pb.GetTokenBalanceReq) (*pb.GetTokenBalanceResp, error) {
	// 获取TRON客户端
	tronClient, err := l.svcCtx.ChainMgr.GetTronClient()
	if tronClient == nil {
		return &pb.GetTokenBalanceResp{
			Success: false,
			Message: "failed to get TRON client: client is nil",
		}, nil
	}
	if err != nil {
		return &pb.GetTokenBalanceResp{
			Success: false,
			Message: fmt.Sprintf("failed to get TRON client: %v", err),
		}, nil
	}
	defer tronClient.Stop()

	// 调用TRC20 balanceOf函数
	balance, err := tronClient.TRC20ContractBalance(in.Address, in.TokenContract)
	if err != nil {
		return &pb.GetTokenBalanceResp{
			Success: false,
			Message: fmt.Sprintf("failed to call TRC20 balanceOf: %v", err),
		}, nil
	}

	// 尝试获取代币符号和精度（可选）
	tokenSymbol := "UNKNOWN"
	tokenDecimals := uint32(18) // 默认精度

	// 获取代币符号
	if symbol, err := tronClient.TRC20GetSymbol(in.TokenContract); err == nil {
		tokenSymbol = symbol
	}

	// 获取代币精度
	if decimals, err := tronClient.TRC20GetDecimals(in.TokenContract); err == nil {
		tokenDecimals = uint32(decimals.Int64())
	}

	// 格式化余额
	balanceFormatted := l.formatTokenAmount(balance, tokenDecimals)

	l.Logger.Infof("Successfully retrieved TRON token balance for %s: %s %s",
		in.Address, balanceFormatted, tokenSymbol)

	return &pb.GetTokenBalanceResp{
		Success:          true,
		Message:          "TRON token balance retrieved successfully",
		Balance:          balance.String(),
		BalanceFormatted: balanceFormatted,
		TokenSymbol:      tokenSymbol,
		TokenDecimals:    tokenDecimals,
		BalanceUsd:       0.0, // 需要外部API支持
	}, nil
}

// getETHTokenSymbol 获取ETH代币符号
func (l *GetTokenBalanceLogic) getETHTokenSymbol(ethClient *ethclient.Client, contractAddr common.Address, blockNumber *big.Int) (string, error) {
	// ERC20 symbol ABI
	symbolABI := `[{
		"constant": true,
		"inputs": [],
		"name": "symbol",
		"outputs": [{"name": "", "type": "string"}],
		"payable": false,
		"stateMutability": "view",
		"type": "function"
	}]`

	parsedABI, err := abi.JSON(strings.NewReader(symbolABI))
	if err != nil {
		return "", err
	}

	data, err := parsedABI.Pack("symbol")
	if err != nil {
		return "", err
	}

	result, err := ethClient.CallContract(context.Background(), ethereum.CallMsg{
		To:   &contractAddr,
		Data: data,
	}, blockNumber)
	if err != nil {
		return "", err
	}

	var symbol string
	err = parsedABI.UnpackIntoInterface(&symbol, "symbol", result)
	if err != nil {
		return "", err
	}

	return symbol, nil
}

// getETHTokenDecimals 获取ETH代币精度
func (l *GetTokenBalanceLogic) getETHTokenDecimals(ethClient *ethclient.Client, contractAddr common.Address, blockNumber *big.Int) (uint32, error) {
	// ERC20 decimals ABI
	decimalsABI := `[{
		"constant": true,
		"inputs": [],
		"name": "decimals",
		"outputs": [{"name": "", "type": "uint8"}],
		"payable": false,
		"stateMutability": "view",
		"type": "function"
	}]`

	parsedABI, err := abi.JSON(strings.NewReader(decimalsABI))
	if err != nil {
		return 0, err
	}

	data, err := parsedABI.Pack("decimals")
	if err != nil {
		return 0, err
	}

	result, err := ethClient.CallContract(context.Background(), ethereum.CallMsg{
		To:   &contractAddr,
		Data: data,
	}, blockNumber)
	if err != nil {
		return 0, err
	}

	var decimals uint8
	err = parsedABI.UnpackIntoInterface(&decimals, "decimals", result)
	if err != nil {
		return 0, err
	}

	return uint32(decimals), nil
}

// formatTokenAmount 格式化代币数量（根据精度）
func (l *GetTokenBalanceLogic) formatTokenAmount(amount *big.Int, decimals uint32) string {
	if amount == nil {
		return "0"
	}

	// 如果精度为0，直接返回字符串
	if decimals == 0 {
		return amount.String()
	}

	// 创建除数 (10^decimals)
	divisor := new(big.Int)
	divisor.Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)

	// 分离整数和小数部分
	quotient := new(big.Int).Div(amount, divisor)
	remainder := new(big.Int).Mod(amount, divisor)

	// 格式化整数部分
	integerPart := quotient.String()

	// 如果没有小数部分，直接返回
	if remainder.Cmp(big.NewInt(0)) == 0 {
		return integerPart
	}

	// 格式化小数部分
	decimalStr := remainder.String()
	// 补零到指定的精度
	for len(decimalStr) < int(decimals) {
		decimalStr = "0" + decimalStr
	}
	// 移除尾部多余的零
	decimalStr = strings.TrimRight(decimalStr, "0")

	return integerPart + "." + decimalStr
}
