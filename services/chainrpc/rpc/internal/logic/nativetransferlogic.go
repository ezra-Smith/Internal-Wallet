package logic

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/fbsobreira/gotron-sdk/pkg/proto/core"
	"google.golang.org/protobuf/proto"
	"internalwallet/services/chainrpc/rpc/internal/utils"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/zeromicro/go-zero/core/logx"

	"internalwallet/proto/pb"
	"internalwallet/services/chainrpc/rpc/internal/svc"
)

type NativeTransferLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

// isValidTronAddress validates TRON address format
func isValidTronAddress(addr string) bool {
	if addr == "" {
		return false
	}

	// TRON addresses should be Base58 format and start with 'T'
	// Typical TRON address length is 33-34 characters in Base58
	if !strings.HasPrefix(addr, "T") {
		return false
	}

	// Check length (TRON Base58 addresses are typically 33-34 characters)
	if len(addr) < 33 || len(addr) > 35 {
		return false
	}

	// Additional validation could be added here if needed
	// For now, basic format check should be sufficient
	return true
}

func NewNativeTransferLogic(ctx context.Context, svcCtx *svc.ServiceContext) *NativeTransferLogic {
	return &NativeTransferLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// NativeTransfer 主币转账交易
func (l *NativeTransferLogic) NativeTransfer(in *pb.NativeTransferReq) (*pb.TransactionResp, error) {
	switch in.Chain {
	case pb.ChainRpcType_CHAIN_TYPE_ETHEREUM, pb.ChainRpcType_CHAIN_TYPE_BSC:
		return l.ETHNativeTransfer(in)
	case pb.ChainRpcType_CHAIN_TYPE_TRON:
		return l.TronNativeTransfer(in)
	default:
		return nil, fmt.Errorf("unsupported chain type: %v", in.Chain)
	}
}

// ETHNativeTransfer  ETH/BSC 主币转账交易
func (l *NativeTransferLogic) ETHNativeTransfer(in *pb.NativeTransferReq) (*pb.TransactionResp, error) {
	// Get ETH client
	client, err := l.svcCtx.ChainMgr.GetETHClient(in.Chain)
	if err != nil {
		return nil, fmt.Errorf("failed to get ETH client: %v", err)
	}

	// Parse addresses
	fromAddr := common.HexToAddress(in.FromAddress)
	toAddr := common.HexToAddress(in.ToAddress)

	// Parse amount
	amount, ok := new(big.Int).SetString(in.Amount, 10)
	if !ok {
		return nil, fmt.Errorf("invalid amount: %s", in.Amount)
	}

	// 获取 nonce（使用 NonceManager 防止并发 nonce 冲突）
	var nonce uint64
	releaseNonce := func(bool) {} // 默认空操作
	if in.Nonce > 0 {
		nonce = in.Nonce
	} else if l.svcCtx.NonceManager != nil {
		n, release, err := l.svcCtx.NonceManager.AcquireNonce(
			context.Background(),
			utils.ChainTypeToString(in.Chain),
			in.FromAddress,
			client,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to get nonce: %v", err)
		}
		nonce = n
		releaseNonce = release
	} else {
		n, err := client.PendingNonceAt(context.Background(), fromAddr)
		if err != nil {
			return nil, fmt.Errorf("failed to get nonce: %v", err)
		}
		nonce = n
	}
	// 获取 gas price
	var gasPrice *big.Int
	if in.GasPrice > 0 {
		gasPrice = big.NewInt(int64(in.GasPrice))
		if !ok {
			return nil, fmt.Errorf("invalid gas price: %d", in.GasPrice)
		}
	} else {
		gp, err := client.SuggestGasPrice(context.Background())
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

	// 使用 defer 确保 releaseNonce 在所有退出路径（签名失败、广播失败等）都被调用
	broadcastOK := false
	defer func() {
		releaseNonce(broadcastOK)
	}()

	// 设置 gas limit
	var gasLimit = in.GasLimit
	if gasLimit == 0 {
		// Estimate gas
		msg := ethereum.CallMsg{
			From:  fromAddr,
			To:    &toAddr,
			Value: amount,
		}
		estimatedGas, err := client.EstimateGas(context.Background(), msg)
		if err != nil {
			return nil, fmt.Errorf("failed to estimate gas: %v", err)
		}
		gasLimit = estimatedGas
	}

	// Create transaction
	tx := types.NewTransaction(nonce, toAddr, amount, gasLimit, gasPrice, nil)

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

	//Validate signer response
	if err := validateSignerResponse(signResp); err != nil {
		return nil, fmt.Errorf("invalid signer response: %v", err)
	}

	//Decode signed transaction
	signedTx, err := hexutil.Decode(signResp.Signature)
	if err != nil {
		return nil, fmt.Errorf("failed to decode signed transaction: %v", err)
	}

	//Parse signed transaction
	signedTxObj := new(types.Transaction)
	if err := signedTxObj.UnmarshalBinary(signedTx); err != nil {
		return nil, fmt.Errorf("failed to unmarshal signed transaction: %v", err)
	}

	//Send transaction
	err = client.SendTransaction(context.Background(), signedTxObj)
	if err != nil {
		return nil, fmt.Errorf("failed to send transaction: %v", err)
	}
	broadcastOK = true

	//Calculate actual fee
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

	//----------------这里暂时使用自己的私钥进行测试
	// Get chain ID
	//chainID, err := client.ChainID(context.Background())
	//if err != nil {
	//	return nil, fmt.Errorf("failed to get chain ID: %v", err)
	//}
	//
	//privateKey, err := crypto.HexToECDSA("XXXXXXX")
	//if err != nil {
	//	log.Fatal(err)
	//}
	//signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), privateKey)
	//if err != nil {
	//	log.Fatal(err)
	//}
	//ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	//err = client.SendTransaction(ctx, signedTx)
	//cancel()
	//if err != nil {
	//	return nil, fmt.Errorf("failed to send transaction: %v", err)
	//}
	//
	//var blockNumber string
	//var status = pb.TxStatus_TX_STATUS_PENDING
	//if in.WaitForReceipt {
	//	txReceipt, err := bind.WaitMined(context.Background(), client, signedTx)
	//	if err != nil {
	//		fmt.Errorf("failed to get transaction receipt: %v", err)
	//	} else {
	//		blockNumber = txReceipt.BlockNumber.String()
	//		status = pb.TxStatus(txReceipt.Status)
	//	}
	//
	//} else {
	//	currentBlockNumber, err := client.BlockNumber(context.Background())
	//	if err != nil {
	//		blockNumber = ""
	//	} else {
	//		blockNumber = strconv.FormatUint(currentBlockNumber, 10)
	//	}
	//}
	//return &pb.TransactionResp{
	//	Success:       true,
	//	Message:       "Transaction sent successfully",
	//	TxHash:        signedTx.Hash().Hex(),
	//	GasUsed:       signedTx.GasPrice().Uint64(),
	//	Status:        status,
	//	BroadcastedAt: time.Now().Unix(),
	//	Nonce:         signedTx.Nonce(),
	//	BlockNumber:   blockNumber,
	//}, nil
}

// TronNativeTransfer 构建 TRON 主币转账交易
func (l *NativeTransferLogic) TronNativeTransfer(in *pb.NativeTransferReq) (*pb.TransactionResp, error) {
	// Get TRON client
	tronClient, err := l.svcCtx.ChainMgr.GetTronClient()
	if tronClient == nil {
		return nil, fmt.Errorf("failed to get TRON client: client is nil")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get TRON client: %v", err)
	}
	defer tronClient.Stop()

	// Parse amount (TRX uses SUN as unit, 1 TRX = 1,000,000 SUN)
	amount, ok := new(big.Int).SetString(in.Amount, 10)
	if !ok {
		return nil, fmt.Errorf("invalid amount: %s", in.Amount)
	}

	if !isValidTronAddress(in.FromAddress) {
		return nil, fmt.Errorf("invalid from address: %s", in.FromAddress)
	}

	if !isValidTronAddress(in.ToAddress) {
		return nil, fmt.Errorf("invalid to address: %s", in.ToAddress)
	}

	tx, err := tronClient.Transfer(in.FromAddress, in.ToAddress, amount.Int64())
	if err != nil {
		return nil, fmt.Errorf("failed to create TRON transfer: %v", err)
	}

	currentBlock, err := tronClient.GetNowBlock()
	if err != nil {
		return nil, fmt.Errorf("failed to get current block: %v", err)
	}
	var blockNumber string
	if currentBlock != nil && currentBlock.BlockHeader != nil && currentBlock.BlockHeader.RawData != nil {
		blockNumber = strconv.FormatInt(currentBlock.BlockHeader.RawData.Number, 10)
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

	//Validate signer response
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
			Message:       "TRON transaction sent failed",
			TxHash:        txHash,
			Status:        pb.TxStatus_TX_STATUS_FAILED,
			BroadcastedAt: time.Now().Unix(),
		}, nil
	} else {
		return &pb.TransactionResp{
			Success:       true,
			Message:       "TRON transaction sent successfully",
			TxHash:        txHash,
			Status:        pb.TxStatus_TX_STATUS_PENDING,
			BroadcastedAt: time.Now().Unix(),
			BlockNumber:   blockNumber,
		}, nil
	}

	//privateKeyBytes, _ := hex.DecodeString("XXXXXX")
	//ecdsaPrivKey, err := crypto.ToECDSA(privateKeyBytes)
	//if err != nil {
	//	log.Fatal(err)
	//}
	//signedTx, err := transaction.SignTransactionECDSA(tx.Transaction, ecdsaPrivKey)
	//if err != nil {
	//	log.Fatal("Failed to sign transaction: %V", err)
	//}
	//result, err := tronClient.Broadcast(signedTx)
	//if err != nil {
	//	return nil, fmt.Errorf("failed to broadcast transaction: %v", err)
	//}
	//if result.Result {
	//	return &pb.TransactionResp{
	//		Success:       true,
	//		Message:       "Transaction sent successfully",
	//		TxHash:        txHash,
	//		GasUsed:       0, // TRON doesn't use gas
	//		Status:        pb.TxStatus_TX_STATUS_PENDING,
	//		BroadcastedAt: time.Now().Unix(),
	//		Nonce:         0, // TRON doesn't use nonce
	//		BlockNumber:   blockNumber,
	//	}, nil
	//} else {
	//	return &pb.TransactionResp{
	//		Success:     false,
	//		Message:     "TRON transaction broadcast failed",
	//		Status:      pb.TxStatus_TX_STATUS_FAILED,
	//		TxHash:      txHash,
	//		Nonce:       0, // TRON doesn't use nonce
	//		BlockNumber: blockNumber,
	//	}, fmt.Errorf("broadcast transaction failed: %v", result.Message)
	//}
}

// getAssetSymbol returns the asset symbol for the given chain
func getAssetSymbol(chain pb.ChainRpcType) string {
	switch chain {
	case pb.ChainRpcType_CHAIN_TYPE_ETHEREUM:
		return "ETH"
	case pb.ChainRpcType_CHAIN_TYPE_BSC:
		return "BNB"
	case pb.ChainRpcType_CHAIN_TYPE_TRON:
		return "TRX"
	default:
		return "UNKNOWN"
	}
}

// validateSignerResponse validates the signer response
func validateSignerResponse(resp *pb.SignTransactionResponse) error {
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
