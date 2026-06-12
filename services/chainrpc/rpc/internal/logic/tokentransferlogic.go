package logic

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/fbsobreira/gotron-sdk/pkg/proto/core"
	"google.golang.org/protobuf/proto"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/fbsobreira/gotron-sdk/pkg/client"
	"github.com/zeromicro/go-zero/core/logx"
	"internalwallet/proto/pb"
	"internalwallet/services/chainrpc/rpc/internal/svc"
	"internalwallet/services/chainrpc/rpc/internal/utils"
)

type TokenTransferLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewTokenTransferLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TokenTransferLogic {
	return &TokenTransferLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ERC20 Transfer方法的ABI
var erc20TransferABI = `[{"constant":false,"inputs":[{"name":"_to","type":"address"},{"name":"_value","type":"uint256"}],"name":"transfer","outputs":[{"name":"","type":"bool"}],"payable":false,"stateMutability":"nonpayable","type":"function"}]`

// 构建代币转账交易
func (l *TokenTransferLogic) TokenTransfer(in *pb.TokenTransferReq) (*pb.TransactionResp, error) {
	// 验证输入参数
	if err := l.validateTokenTransferReq(in); err != nil {
		return nil, fmt.Errorf("validation failed: %v", err)
	}

	switch in.Chain {
	case pb.ChainRpcType_CHAIN_TYPE_ETHEREUM, pb.ChainRpcType_CHAIN_TYPE_BSC:
		return l.ETHTokenTransfer(in)
	case pb.ChainRpcType_CHAIN_TYPE_TRON:
		return l.TronTokenTransfer(in)
	default:
		return nil, fmt.Errorf("unsupported chain type: %v", in.Chain)
	}
}

// validateTokenTransferReq 验证代币转账请求参数
func (l *TokenTransferLogic) validateTokenTransferReq(in *pb.TokenTransferReq) error {
	if in == nil {
		return fmt.Errorf("request is nil")
	}

	if in.FromAddress == "" {
		return fmt.Errorf("from address is required")
	}

	if in.ToAddress == "" {
		return fmt.Errorf("to address is required")
	}

	if in.TokenContract == "" {
		return fmt.Errorf("token contract address is required")
	}

	if in.Amount == "" {
		return fmt.Errorf("amount is required")
	}

	// 验证金额格式
	_, ok := new(big.Int).SetString(in.Amount, 10)
	if !ok {
		return fmt.Errorf("invalid amount format: %s", in.Amount)
	}

	// 根据链类型验证地址格式
	switch in.Chain {
	case pb.ChainRpcType_CHAIN_TYPE_ETHEREUM, pb.ChainRpcType_CHAIN_TYPE_BSC:
		// 以太坊地址验证
		if !common.IsHexAddress(in.FromAddress) {
			return fmt.Errorf("invalid from address format: %s", in.FromAddress)
		}
		if !common.IsHexAddress(in.ToAddress) {
			return fmt.Errorf("invalid to address format: %s", in.ToAddress)
		}
		if !common.IsHexAddress(in.TokenContract) {
			return fmt.Errorf("invalid token contract address format: %s", in.TokenContract)
		}
	case pb.ChainRpcType_CHAIN_TYPE_TRON:
		// TRON地址验证
		if !l.isValidTronAddress(in.FromAddress) {
			return fmt.Errorf("invalid from TRON address: %s", in.FromAddress)
		}
		if !l.isValidTronAddress(in.ToAddress) {
			return fmt.Errorf("invalid to TRON address: %s", in.ToAddress)
		}
		if !l.isValidTronAddress(in.TokenContract) {
			return fmt.Errorf("invalid token contract TRON address: %s", in.TokenContract)
		}
	}

	return nil
}

// isValidTronAddress 验证TRON地址格式
func (l *TokenTransferLogic) isValidTronAddress(addr string) bool {
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

// ETHTokenTransfer ETH/BSC代币转账（ERC20）
func (l *TokenTransferLogic) ETHTokenTransfer(in *pb.TokenTransferReq) (*pb.TransactionResp, error) {
	// 获取ETH客户端
	ethClient, err := l.svcCtx.ChainMgr.GetETHClient(in.Chain)
	if err != nil {
		return nil, fmt.Errorf("failed to get ETH client: %v", err)
	}

	// 解析地址
	fromAddr := common.HexToAddress(in.FromAddress)
	toAddr := common.HexToAddress(in.ToAddress)
	tokenContract := common.HexToAddress(in.TokenContract)

	// 解析金额
	amount, ok := new(big.Int).SetString(in.Amount, 10)
	if !ok {
		return nil, fmt.Errorf("invalid amount: %s", in.Amount)
	}

	// 获取nonce（使用 NonceManager 防止并发 nonce 冲突）
	var nonce uint64
	releaseNonce := func(bool) {} // 默认空操作
	if in.Nonce > 0 {
		nonce = in.Nonce
	} else if l.svcCtx.NonceManager != nil {
		n, release, err := l.svcCtx.NonceManager.AcquireNonce(
			context.Background(),
			utils.ChainTypeToString(in.Chain),
			in.FromAddress,
			ethClient,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to get nonce: %v", err)
		}
		nonce = n
		releaseNonce = release
	} else {
		n, err := ethClient.PendingNonceAt(context.Background(), fromAddr)
		if err != nil {
			return nil, fmt.Errorf("failed to get nonce: %v", err)
		}
		nonce = n
	}

	// 获取gas price
	var gasPrice *big.Int
	if in.GasPrice > 0 {
		gasPrice = big.NewInt(int64(in.GasPrice))
	} else {
		gp, err := ethClient.SuggestGasPrice(context.Background())
		if err != nil {
			return nil, fmt.Errorf("failed to get gas price: %v", err)
		}
		gasPrice = gp
	}
	
	// 确保 gas price 不低于链的最小值（特别是 BSC 链要求至少 1 Gwei）
	var minGasPrice *big.Int
	if in.Chain == pb.ChainRpcType_CHAIN_TYPE_BSC {
		minGasPrice = big.NewInt(1_000_000_000) // 1 gwei
	}
	if minGasPrice != nil && gasPrice.Cmp(minGasPrice) < 0 {
		l.Logger.Infof("suggested gas price below minimum, clamping: chain=%v suggested=%v min=%v", in.Chain, gasPrice, minGasPrice)
		gasPrice = new(big.Int).Set(minGasPrice)
	}

	// 解析ERC20 Transfer ABI
	parsedABI, err := abi.JSON(strings.NewReader(erc20TransferABI))
	if err != nil {
		return nil, fmt.Errorf("failed to parse ERC20 ABI: %v", err)
	}

	// 编码transfer函数调用数据
	data, err := parsedABI.Pack("transfer", toAddr, amount)
	if err != nil {
		return nil, fmt.Errorf("failed to pack transfer data: %v", err)
	}

	// 设置gas limit
	var gasLimit uint64
	if in.GasLimit > 0 {
		gasLimit = in.GasLimit
	} else {
		// 估算gas
		msg := ethereum.CallMsg{
			From:  fromAddr,
			To:    &tokenContract,
			Value: big.NewInt(0),
			Data:  data,
		}
		estimatedGas, err := ethClient.EstimateGas(context.Background(), msg)
		if err != nil {
			// 如果估算失败，使用默认值
			gasLimit = 60000
		} else {
			gasLimit = estimatedGas
		}
	}
	// 使用 defer 确保 releaseNonce 在所有退出路径（签名失败、广播失败等）都被调用
	broadcastOK := false
	defer func() {
		releaseNonce(broadcastOK)
	}()

	// 创建交易 ,由于是代币转账，需要设置value为0
	tx := types.NewTransaction(nonce, tokenContract, big.NewInt(0), gasLimit, gasPrice, data)
	rawTxBytes, err := tx.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal transaction: %v", err)
	}
	signReq := &pb.SignTransactionRequest{
		RequestId:      fmt.Sprintf("native_%d_%d", time.Now().Unix(), nonce),
		Chain:          utils.ChainTypeToString(in.Chain),
		RawTransaction: hexutil.Encode(rawTxBytes),
		OperationType:  2, // 2=withdrawal
		Amount:         in.Amount,
		ToAddress:      in.ToAddress,
		FromAddress:    in.FromAddress,
		AssetSymbol:    getAssetSymbol(in.Chain),
		Requester:      "chainrpc_service",
	}
	signResp, err := l.svcCtx.ChainMgr.SignTransactionWithSigner(context.Background(), signReq)
	if err != nil {
		return nil, fmt.Errorf("failed to sign transaction: %v", err)
	}
	if err := validateSignerResponse(signResp); err != nil {
		return nil, fmt.Errorf("invalid signer response: %v", err)
	}

	//Decode signed transaction
	signedTx, err := hexutil.Decode(signResp.Signature)
	if err != nil {
		return nil, fmt.Errorf("failed to decode signed transaction: %v", err)
	}
	signedTxObj := new(types.Transaction)
	if err := signedTxObj.UnmarshalBinary(signedTx); err != nil {
		return nil, fmt.Errorf("failed to unmarshal signed transaction: %v", err)
	}
	err = ethClient.SendTransaction(context.Background(), signedTxObj)
	if err != nil {
		return &pb.TransactionResp{
			Success:       false,
			Message:       fmt.Sprintf("failed to send transaction: %v", err),
			TxHash:        "",
			GasUsed:       0,
			Status:        pb.TxStatus_TX_STATUS_FAILED,
			BroadcastedAt: time.Now().Unix(),
			Nonce:         signedTxObj.Nonce(),
		}, nil
	}
	broadcastOK = true
	actualFee := new(big.Int).Mul(signedTxObj.GasPrice(), new(big.Int).SetUint64(signedTxObj.Gas()))
	return &pb.TransactionResp{
		Success:       true,
		Message:       "Transaction sent successfully",
		TxHash:        signedTxObj.Hash().Hex(),
		GasUsed:       actualFee.Uint64(),
		Status:        pb.TxStatus_TX_STATUS_PENDING,
		BroadcastedAt: time.Now().Unix(),
		Nonce:         signedTxObj.Nonce(),
	}, nil
}

// TronTokenTransfer TRON代币转账（TRC20）
func (l *TokenTransferLogic) TronTokenTransfer(in *pb.TokenTransferReq) (*pb.TransactionResp, error) {
	// 获取TRON客户端
	tronClient, err := l.svcCtx.ChainMgr.GetTronClient()
	if tronClient == nil {
		return nil, fmt.Errorf("failed to get TRON client: client is nil")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get TRON client: %v", err)
	}
	defer tronClient.Stop()

	// 解析金额（TRC20代币金额通常是最小单位）
	amount, ok := new(big.Int).SetString(in.Amount, 10)
	if !ok {
		return nil, fmt.Errorf("invalid amount: %s", in.Amount)
	}

	// 获取代币精度（用于日志和验证）
	tokenDecimals, err := tronClient.TRC20GetDecimals(in.TokenContract)
	if err != nil {
		l.Logger.Error("Failed to get token decimals, using default 18: " + err.Error())
	} else {
		l.Logger.Infof("Token decimals: %d", tokenDecimals.Int64())
	}

	// 获取当前区块号
	currentBlock, err := tronClient.GetNowBlock()
	if err != nil {
		return nil, fmt.Errorf("failed to get current block: %v", err)
	}
	var blockNumber string
	if currentBlock != nil && currentBlock.BlockHeader != nil && currentBlock.BlockHeader.RawData != nil {
		blockNumber = fmt.Sprintf("%d", currentBlock.BlockHeader.RawData.Number)
		l.Logger.Infof("Current block number: %s", blockNumber)
	}

	// 设置能量限制（类似于gas limit）
	feeLimit := int64(100000000) // 100 TRX

	// 调用gotron-sdk的TRC20Send方法创建交易
	// 注意：amount参数需要是*big.Int类型
	tx, err := tronClient.TRC20Send(in.FromAddress, in.ToAddress, in.TokenContract, amount, feeLimit)
	if err != nil {
		return nil, fmt.Errorf("failed to create TRC20 transfer: %v", err)
	}
	// 将Transaction对象序列化为hex
	txBytes, err := proto.Marshal(tx.Transaction)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal transaction: %v", err)
	}
	signReq := &pb.SignTransactionRequest{
		RequestId:      fmt.Sprintf("native_%d_%d", time.Now().Unix(), 0),
		Chain:          utils.ChainTypeToString(in.Chain),
		RawTransaction: hexutil.Encode(txBytes), // 传递Transaction对象的hex编码
		OperationType:  2,                       // 2=withdrawal
		Amount:         in.Amount,
		ToAddress:      in.ToAddress,
		FromAddress:    in.FromAddress,
		AssetSymbol:    getAssetSymbol(in.Chain),
		Requester:      "chainrpc_service",
	}
	signResp, err := l.svcCtx.ChainMgr.SignTransactionWithSigner(context.Background(), signReq)
	if err != nil {
		return nil, fmt.Errorf("failed to sign transaction: %v", err)
	}
	if err := validateSignerResponse(signResp); err != nil {
		return nil, fmt.Errorf("invalid signer response: %v", err)
	}
	//解码签名后的交易hex
	signedTxBytes, err := hexutil.Decode(signResp.Signature)
	if err != nil {
		return nil, fmt.Errorf("failed to decode signed transaction: %v", err)
	}
	//将字节数组反序列化为Transaction对象
	signedTxObj := &core.Transaction{}
	if err := proto.Unmarshal(signedTxBytes, signedTxObj); err != nil {
		return nil, fmt.Errorf("failed to unmarshal TRON transaction: %v", err)
	}
	// 计算交易哈希
	rawDataBytes, err := proto.Marshal(signedTxObj.GetRawData())
	if err != nil {
		return nil, fmt.Errorf("failed to marshal transaction raw data: %v", err)
	}
	hash := sha256.Sum256(rawDataBytes)
	txHash := hex.EncodeToString(hash[:])
	// 广播签名后的交易
	result, err := tronClient.Broadcast(signedTxObj)
	if err != nil {
		return nil, fmt.Errorf("failed to broadcast TRON transaction: %v", err)
	}
	if !result.Result {
		return &pb.TransactionResp{
			Success:       false,
			Message:       "TRON Token transaction sent failed",
			TxHash:        txHash,
			Status:        pb.TxStatus_TX_STATUS_FAILED,
			BroadcastedAt: time.Now().Unix(),
		}, nil
	} else {
		return &pb.TransactionResp{
			Success:       true,
			Message:       "TRON Token transaction sent successfully",
			TxHash:        txHash,
			Status:        pb.TxStatus_TX_STATUS_PENDING,
			BroadcastedAt: time.Now().Unix(),
			BlockNumber:   blockNumber,
		}, nil
	}
}

// getTronTokenSymbol 获取TRC20代币符号
func (l *TokenTransferLogic) getTronTokenSymbol(tronClient *client.GrpcClient, tokenContract string) (string, error) {
	// 调用gotron-sdk的TRC20GetSymbol方法
	symbol, err := tronClient.TRC20GetSymbol(tokenContract)
	if err != nil {
		return "", err
	}
	return symbol, nil
}

// getTokenSymbol 获取ERC20代币符号
func (l *TokenTransferLogic) getTokenSymbol(ethClient *ethclient.Client, tokenContract common.Address) (string, error) {
	// ERC20的symbol函数ABI
	symbolABI := `[{"constant":true,"inputs":[],"name":"symbol","outputs":[{"name":"","type":"string"}],"payable":false,"stateMutability":"view","type":"function"}]`

	parsedABI, err := abi.JSON(strings.NewReader(symbolABI))
	if err != nil {
		return "", err
	}

	// 调用symbol函数
	data, err := parsedABI.Pack("symbol")
	if err != nil {
		return "", err
	}

	result, err := ethClient.CallContract(context.Background(), ethereum.CallMsg{
		To:   &tokenContract,
		Data: data,
	}, nil)
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

// validateSignerResponse 验证签名服务响应
func (l *TokenTransferLogic) validateSignerResponse(resp *pb.SignTransactionResponse) error {
	if resp == nil {
		return fmt.Errorf("signer response is nil")
	}

	if resp.Code != 0 {
		return fmt.Errorf("signer failed with code %d: %s", resp.Code, resp.Message)
	}

	if resp.Signature == "" {
		return fmt.Errorf("signature is empty in signer response")
	}

	if resp.TxHash == "" {
		return fmt.Errorf("transaction hash is empty in signer response")
	}

	return nil
}
