package logic

import (
	"context"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/fbsobreira/gotron-sdk/pkg/address"
	"github.com/fbsobreira/gotron-sdk/pkg/client"
	"github.com/zeromicro/go-zero/core/logx"

	"internalwallet/proto/pb"
	"internalwallet/services/chainrpc/rpc/internal/svc"
)

type GetTokenInfoLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetTokenInfoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetTokenInfoLogic {
	return &GetTokenInfoLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ERC20标准ABI函数（不包括owner，因为不是标准函数）
var erc20ABI = `[{
	"constant": true,
	"inputs": [],
	"name": "name",
	"outputs": [{"name": "", "type": "string"}],
	"payable": false,
	"stateMutability": "view",
	"type": "function"
}, {
	"constant": true,
	"inputs": [],
	"name": "symbol",
	"outputs": [{"name": "", "type": "string"}],
	"payable": false,
	"stateMutability": "view",
	"type": "function"
}, {
	"constant": true,
	"inputs": [],
	"name": "decimals",
	"outputs": [{"name": "", "type": "uint8"}],
	"payable": false,
	"stateMutability": "view",
	"type": "function"
}, {
	"constant": true,
	"inputs": [],
	"name": "totalSupply",
	"outputs": [{"name": "", "type": "uint256"}],
	"payable": false,
	"stateMutability": "view",
	"type": "function"
}]`

// GetTokenInfo 获取代币信息
func (l *GetTokenInfoLogic) GetTokenInfo(in *pb.GetTokenInfoReq) (*pb.GetTokenInfoResp, error) {
	// 参数验证
	if err := l.validateGetTokenInfoReq(in); err != nil {
		return &pb.GetTokenInfoResp{
			Success: false,
			Message: fmt.Sprintf("validation failed: %v", err),
		}, nil
	}

	// 根据链类型调用相应的方法
	switch in.Chain {
	case pb.ChainRpcType_CHAIN_TYPE_ETHEREUM, pb.ChainRpcType_CHAIN_TYPE_BSC:
		return l.getETHTokenInfo(in)
	case pb.ChainRpcType_CHAIN_TYPE_TRON:
		return l.getTronTokenInfo(in)
	default:
		return &pb.GetTokenInfoResp{
			Success: false,
			Message: fmt.Sprintf("unsupported chain type: %v", in.Chain),
		}, nil
	}
}

// validateGetTokenInfoReq 验证请求参数
func (l *GetTokenInfoLogic) validateGetTokenInfoReq(in *pb.GetTokenInfoReq) error {
	if in == nil {
		return fmt.Errorf("request is nil")
	}

	if in.TokenContract == "" {
		return fmt.Errorf("token contract address is required")
	}

	// 根据链类型验证地址格式
	switch in.Chain {
	case pb.ChainRpcType_CHAIN_TYPE_ETHEREUM, pb.ChainRpcType_CHAIN_TYPE_BSC:
		if !common.IsHexAddress(in.TokenContract) {
			return fmt.Errorf("invalid token contract address format: %s", in.TokenContract)
		}
	case pb.ChainRpcType_CHAIN_TYPE_TRON:
		if !l.isValidTronAddress(in.TokenContract) {
			return fmt.Errorf("invalid TRON token contract address: %s", in.TokenContract)
		}
	}

	return nil
}

// isValidTronAddress 验证TRON地址格式
func (l *GetTokenInfoLogic) isValidTronAddress(addr string) bool {
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

// getETHTokenInfo 获取ETH/BSC代币信息
func (l *GetTokenInfoLogic) getETHTokenInfo(in *pb.GetTokenInfoReq) (*pb.GetTokenInfoResp, error) {
	// 获取ETH客户端
	client, err := l.svcCtx.ChainMgr.GetETHClient(in.Chain)
	if err != nil {
		return &pb.GetTokenInfoResp{
			Success: false,
			Message: fmt.Sprintf("failed to get ETH client: %v", err),
		}, nil
	}

	// 解析合约地址
	contractAddr := common.HexToAddress(in.TokenContract)

	// 解析ERC20 ABI
	parsedABI, err := abi.JSON(strings.NewReader(erc20ABI))
	if err != nil {
		return &pb.GetTokenInfoResp{
			Success: false,
			Message: fmt.Sprintf("failed to parse ERC20 ABI: %v", err),
		}, nil
	}

	// 构建TokenInfo对象
	tokenInfo := &pb.TokenInfo{
		ContractAddress: in.TokenContract,
	}

	// 获取代币名称
	if name, err := l.callERC20StringFunction(client, parsedABI, "name", contractAddr); err != nil {
		l.Logger.Error("Failed to get token name: " + err.Error())
		tokenInfo.Name = "Unknown"
	} else {
		tokenInfo.Name = name
	}

	// 获取代币符号
	if symbol, err := l.callERC20StringFunction(client, parsedABI, "symbol", contractAddr); err != nil {
		l.Logger.Error("Failed to get token symbol: " + err.Error())
		tokenInfo.Symbol = "UNKNOWN"
	} else {
		tokenInfo.Symbol = symbol
	}

	// 获取代币精度
	if decimals, err := l.callERC20Uint8Function(client, parsedABI, "decimals", contractAddr); err != nil {
		l.Logger.Error("Failed to get token decimals: " + err.Error())
		tokenInfo.Decimals = 18 // 默认精度
	} else {
		tokenInfo.Decimals = uint32(decimals)
	}

	// 获取总供应量
	if totalSupply, err := l.callERC20Uint256Function(client, parsedABI, "totalSupply", contractAddr); err != nil {
		l.Logger.Error("Failed to get token total supply: " + err.Error())
		tokenInfo.TotalSupply = "0"
		tokenInfo.TotalSupplyFormatted = "0"
	} else {
		tokenInfo.TotalSupply = totalSupply.String()
		// 计算格式化后的总供应量
		if formattedSupply := l.formatTokenAmount(totalSupply, tokenInfo.Decimals); formattedSupply != "" {
			tokenInfo.TotalSupplyFormatted = formattedSupply
		}
	}

	// 获取合约所有者（可选）
	// 尝试多种可能的owner函数名。因为不同的代币可能会有不同的owner函数名。有些合约中也可能没有owner函数。
	owner := l.tryGetContractOwner(client, contractAddr)
	tokenInfo.Owner = owner

	// 默认值（这些信息通常需要外部API支持）
	tokenInfo.Verified = false
	tokenInfo.PriceUsd = 0.0

	l.Logger.Infof("Successfully retrieved ETH/BSC token info for %s (%s)", tokenInfo.Symbol, tokenInfo.Name)

	return &pb.GetTokenInfoResp{
		Success:   true,
		Message:   "Token info retrieved successfully",
		TokenInfo: tokenInfo,
	}, nil
}

// getTronTokenInfo 获取TRON代币信息
func (l *GetTokenInfoLogic) getTronTokenInfo(in *pb.GetTokenInfoReq) (*pb.GetTokenInfoResp, error) {
	// 获取TRON客户端
	tronClient, err := l.svcCtx.ChainMgr.GetTronClient()
	if tronClient == nil {
		return &pb.GetTokenInfoResp{
			Success: false,
			Message: "failed to get TRON client: client is nil",
		}, nil
	}
	if err != nil {
		return &pb.GetTokenInfoResp{
			Success: false,
			Message: fmt.Sprintf("failed to get TRON client: %v", err),
		}, nil
	}
	defer tronClient.Stop()

	// 构建TokenInfo对象
	tokenInfo := &pb.TokenInfo{
		ContractAddress: in.TokenContract,
	}

	// 获取代币名称
	if name, err := tronClient.TRC20GetName(in.TokenContract); err != nil {
		l.Logger.Error("Failed to get TRON token name: " + err.Error())
		tokenInfo.Name = "Unknown"
	} else {
		tokenInfo.Name = name
	}

	// 获取代币符号
	if symbol, err := tronClient.TRC20GetSymbol(in.TokenContract); err != nil {
		l.Logger.Error("Failed to get TRON token symbol: " + err.Error())
		tokenInfo.Symbol = "UNKNOWN"
	} else {
		tokenInfo.Symbol = symbol
	}

	// 获取代币精度
	if decimals, err := tronClient.TRC20GetDecimals(in.TokenContract); err != nil {
		l.Logger.Error("Failed to get TRON token decimals: " + err.Error())
		tokenInfo.Decimals = 18 // 默认精度
	} else {
		tokenInfo.Decimals = uint32(decimals.Int64())
	}

	// 获取总供应量
	if totalSupply, err := l.getTronTotalSupply(tronClient, in.TokenContract); err != nil {
		l.Logger.Error("Failed to get TRON token total supply: " + err.Error())
		tokenInfo.TotalSupply = "0"
		tokenInfo.TotalSupplyFormatted = "0"
	} else {
		tokenInfo.TotalSupply = totalSupply.String()
		// 计算格式化后的总供应量
		if formattedSupply := l.formatTokenAmount(totalSupply, tokenInfo.Decimals); formattedSupply != "" {
			tokenInfo.TotalSupplyFormatted = formattedSupply
		}
	}

	// 获取TRON代币合约所有者信息
	owner := l.tryGetTronContractOwner(tronClient, in.TokenContract)
	tokenInfo.Owner = owner

	// 默认值
	tokenInfo.Verified = false
	tokenInfo.PriceUsd = 0.0

	l.Logger.Infof("Successfully retrieved TRON token info for %s (%s)", tokenInfo.Symbol, tokenInfo.Name)

	return &pb.GetTokenInfoResp{
		Success:   true,
		Message:   "TRON token info retrieved successfully",
		TokenInfo: tokenInfo,
	}, nil
}

// callERC20StringFunction 调用ERC20返��字符串的函数
func (l *GetTokenInfoLogic) callERC20StringFunction(client *ethclient.Client, parsedABI abi.ABI, functionName string, contractAddr common.Address) (string, error) {
	// 编码函数调用
	data, err := parsedABI.Pack(functionName)
	if err != nil {
		return "", fmt.Errorf("failed to pack %s function: %v", functionName, err)
	}

	// 调用合约
	result, err := client.CallContract(context.Background(), ethereum.CallMsg{
		To:   &contractAddr,
		Data: data,
	}, nil)
	if err != nil {
		return "", fmt.Errorf("failed to call %s function: %v", functionName, err)
	}

	// 解码返回值
	var value string
	err = parsedABI.UnpackIntoInterface(&value, functionName, result)
	if err != nil {
		return "", fmt.Errorf("failed to unpack %s result: %v", functionName, err)
	}

	return value, nil
}

// callERC20Uint8Function 调用ERC20返回uint8的函数
func (l *GetTokenInfoLogic) callERC20Uint8Function(client *ethclient.Client, parsedABI abi.ABI, functionName string, contractAddr common.Address) (uint8, error) {
	// 编码函数调用
	data, err := parsedABI.Pack(functionName)
	if err != nil {
		return 0, fmt.Errorf("failed to pack %s function: %v", functionName, err)
	}

	// 调用合约
	result, err := client.CallContract(context.Background(), ethereum.CallMsg{
		To:   &contractAddr,
		Data: data,
	}, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to call %s function: %v", functionName, err)
	}

	// 解码返回值
	var value uint8
	err = parsedABI.UnpackIntoInterface(&value, functionName, result)
	if err != nil {
		return 0, fmt.Errorf("failed to unpack %s result: %v", functionName, err)
	}

	return value, nil
}

// callERC20Uint256Function 调用ERC20返回uint256的函数
func (l *GetTokenInfoLogic) callERC20Uint256Function(client *ethclient.Client, parsedABI abi.ABI, functionName string, contractAddr common.Address) (*big.Int, error) {
	// 编码函数调用
	data, err := parsedABI.Pack(functionName)
	if err != nil {
		return nil, fmt.Errorf("failed to pack %s function: %v", functionName, err)
	}

	// 调用合约
	result, err := client.CallContract(context.Background(), ethereum.CallMsg{
		To:   &contractAddr,
		Data: data,
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to call %s function: %v", functionName, err)
	}

	// 解码返回值
	var value *big.Int
	err = parsedABI.UnpackIntoInterface(&value, functionName, result)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack %s result: %v", functionName, err)
	}

	return value, nil
}

// callERC20AddressFunction 调用ERC20返回地址的函数
func (l *GetTokenInfoLogic) callERC20AddressFunction(client *ethclient.Client, parsedABI abi.ABI, functionName string, contractAddr common.Address) (common.Address, error) {
	// 编码函数调用
	data, err := parsedABI.Pack(functionName)
	if err != nil {
		return common.Address{}, fmt.Errorf("failed to pack %s function: %v", functionName, err)
	}

	// 调用合约
	result, err := client.CallContract(context.Background(), ethereum.CallMsg{
		To:   &contractAddr,
		Data: data,
	}, nil)
	if err != nil {
		return common.Address{}, fmt.Errorf("failed to call %s function: %v", functionName, err)
	}

	// 解码返回值
	var value common.Address
	err = parsedABI.UnpackIntoInterface(&value, functionName, result)
	if err != nil {
		return common.Address{}, fmt.Errorf("failed to unpack %s result: %v", functionName, err)
	}

	return value, nil
}

// formatTokenAmount 格式化代币数量（根据精度）
func (l *GetTokenInfoLogic) formatTokenAmount(amount *big.Int, decimals uint32) string {
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

// tryGetContractOwner 尝试获取合约所有者
func (l *GetTokenInfoLogic) tryGetContractOwner(client *ethclient.Client, contractAddr common.Address) string {
	// 常见的owner函数名称及其ABI
	ownerFuncs := []string{
		`{"constant":true,"inputs":[],"name":"owner","outputs":[{"name":"","type":"address"}],"payable":false,"stateMutability":"view","type":"function"}`,
		`{"constant":true,"inputs":[],"name":"getOwner","outputs":[{"name":"","type":"address"}],"payable":false,"stateMutability":"view","type":"function"}`,
		`{"constant":true,"inputs":[],"name":"admin","outputs":[{"name":"","type":"address"}],"payable":false,"stateMutability":"view","type":"function"}`,
		`{"constant":true,"inputs":[],"name":"owner_","outputs":[{"name":"","type":"address"}],"payable":false,"stateMutability":"view","type":"function"}`,
	}

	for _, funcABI := range ownerFuncs {
		parsedABI, err := abi.JSON(strings.NewReader("[" + funcABI + "]"))
		if err != nil {
			continue
		}

		// 获取函数名
		var funcName string
		parts := strings.Split(funcABI, `"name":"`)
		if len(parts) > 1 {
			funcName = strings.Split(parts[1], `"`)[0]
			l.Logger.Debugf("Trying function: %s", funcName)
		} else {
			continue
		}

		data, err := parsedABI.Pack(funcName)
		if err != nil {
			continue
		}

		// 调用合约
		result, err := client.CallContract(context.Background(), ethereum.CallMsg{
			To:   &contractAddr,
			Data: data,
		}, nil)
		if err != nil {
			continue // 该函数不存在，尝试下一个
		}

		// 解码返回值
		var value common.Address
		err = parsedABI.UnpackIntoInterface(&value, funcName, result)
		if err != nil {
			continue
		}

		// 如果返回的不是零地址，说明找到了owner
		if value != (common.Address{}) {
			l.Logger.Infof("Found contract owner using function %s: %s", funcName, value.Hex())
			return value.Hex()
		}
	}

	l.Logger.Info("No standard owner function found in contract")
	return ""
}

// tryGetTronContractOwner 尝试获取TRON合约所有者
func (l *GetTokenInfoLogic) tryGetTronContractOwner(tronClient *client.GrpcClient, contractAddress string) string {
	// TRON常见的owner函数签名（基于TRC20标准）
	// owner函数的函数选择器：0x8da5cb5b (bytes4(keccak256("owner()")))
	ownerSignatures := []string{
		"0x8da5cb5b", // owner()
		"0x79ba5097", // getOwner() - keccak256("getOwner()")
		"0xa8c646b5", // admin() - keccak256("admin()")
		"0x7952c85f", // owner_() - keccak256("owner_()")
	}

	for _, signature := range ownerSignatures {
		l.Logger.Debugf("Trying TRON owner function with signature: %s", signature)

		// 调用TRC20合约的owner函数
		result, err := tronClient.TRC20Call("", contractAddress, signature, true, 0)
		if err != nil {
			l.Logger.Debugf("Function call failed for signature %s: %v", signature, err)
			continue // 该函数不存在，尝试下一个
		}

		// 检查返回结果
		data := result.GetConstantResult()
		if len(data) == 0 {
			l.Logger.Debugf("No data returned for signature %s", signature)
			continue
		}

		// TRON地址格式：返回的是地址的字节表示
		ownerBytes := data[0]
		if len(ownerBytes) == 0 {
			l.Logger.Debugf("Empty owner data for signature %s", signature)
			continue
		}

		// 检查是否为零地址（全0）
		isZeroAddress := true
		for _, b := range ownerBytes {
			if b != 0 {
				isZeroAddress = false
				break
			}
		}

		if isZeroAddress {
			l.Logger.Debugf("Zero address returned for signature %s", signature)
			continue
		}

		// 将字节转换为TRON Base58地址格式
		// 检查ownerBytes长度，通常应该是20字节
		if len(ownerBytes) != 20 {
			l.Logger.Debugf("Invalid owner address length: %d, expected 20", len(ownerBytes))
			continue
		}

		// 构造TRON地址：前缀0x41 + 20字节地址 = 21字节
		var tronAddressBytes [21]byte
		tronAddressBytes[0] = address.TronBytePrefix // 0x41前缀
		copy(tronAddressBytes[1:], ownerBytes)

		// 创建TRON Address对象并转换为Base58格式
		tronAddr := address.Address(tronAddressBytes[:])
		ownerBase58 := tronAddr.String()

		l.Logger.Infof("Found TRON contract owner using signature %s: %s", signature, ownerBase58)
		return ownerBase58
	}

	l.Logger.Info("No standard owner function found in TRON contract")
	return ""
}

// getTronTotalSupply 获取TRON代币总供应量
func (l *GetTokenInfoLogic) getTronTotalSupply(tronClient *client.GrpcClient, contractAddress string) (*big.Int, error) {
	// TRC20 totalSupply方法的签名是 0x18160ddd
	const trc20TotalSupplySignature = "0x18160ddd"

	result, err := tronClient.TRC20Call("", contractAddress, trc20TotalSupplySignature, true, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to call totalSupply: %v", err)
	}

	data := result.GetConstantResult()
	if len(data) == 0 {
		return nil, fmt.Errorf("no result returned from totalSupply call")
	}

	// 解析返回的数值
	totalSupply := new(big.Int).SetBytes(data[0])
	return totalSupply, nil
}
