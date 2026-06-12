package logic

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/protobuf/proto"

	core "github.com/fbsobreira/gotron-sdk/pkg/proto/core"
	"internalwallet/proto/pb"
	"internalwallet/services/chainrpc/rpc/internal/svc"
)

type BroadcastTransactionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewBroadcastTransactionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BroadcastTransactionLogic {
	return &BroadcastTransactionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *BroadcastTransactionLogic) BroadcastTransaction(in *pb.BroadcastTransactionReq) (*pb.BroadcastTransactionResp, error) {
	if in == nil {
		return &pb.BroadcastTransactionResp{
			Success: false,
			Message: "request is required",
		}, nil
	}

	// 验证链类型
	if in.Chain == pb.ChainRpcType_CHAIN_TYPE_UNSPECIFIED {
		return &pb.BroadcastTransactionResp{
			Success: false,
			Message: "chain type is required",
		}, nil
	}

	// 验证已签名交易
	signedTxHex := strings.TrimSpace(in.SignedTransaction)
	if signedTxHex == "" {
		return &pb.BroadcastTransactionResp{
			Success: false,
			Message: "signed transaction is required",
		}, nil
	}

	// 设置默认值
	waitForReceipt := in.WaitForReceipt
	timeoutSeconds := in.TimeoutSeconds
	if timeoutSeconds <= 0 {
		timeoutSeconds = 30 // 默认30秒
	}

	// 根据链类型广播交易
	switch in.Chain {
	case pb.ChainRpcType_CHAIN_TYPE_ETHEREUM, pb.ChainRpcType_CHAIN_TYPE_BSC:
		return l.broadcastETHTransaction(signedTxHex, in.Chain, waitForReceipt, timeoutSeconds)
	case pb.ChainRpcType_CHAIN_TYPE_TRON:
		return l.broadcastTronTransaction(signedTxHex, waitForReceipt, timeoutSeconds)
	default:
		return &pb.BroadcastTransactionResp{
			Success: false,
			Message: fmt.Sprintf("unsupported chain type: %v", in.Chain),
		}, nil
	}
}

// broadcastETHTransaction 广播 ETH/BSC 交易
func (l *BroadcastTransactionLogic) broadcastETHTransaction(signedTxHex string, chainType pb.ChainRpcType, waitForReceipt bool, timeoutSeconds int32) (*pb.BroadcastTransactionResp, error) {
	// 获取 ETH 客户端
	client, err := l.svcCtx.ChainMgr.GetETHClient(chainType)
	if err != nil {
		l.Errorf("failed to get ETH client: %v", err)
		return &pb.BroadcastTransactionResp{
			Success: false,
			Message: fmt.Sprintf("failed to get chain client: %v", err),
		}, nil
	}

	// 解码已签名交易（RLP编码）
	signedTxBytes, err := hexutil.Decode(signedTxHex)
	if err != nil {
		l.Errorf("failed to decode signed transaction: %v", err)
		return &pb.BroadcastTransactionResp{
			Success: false,
			Message: fmt.Sprintf("invalid transaction hex: %v", err),
		}, nil
	}

	// 解析交易对象
	tx := new(types.Transaction)
	if err := tx.UnmarshalBinary(signedTxBytes); err != nil {
		l.Errorf("failed to unmarshal transaction: %v", err)
		return &pb.BroadcastTransactionResp{
			Success: false,
			Message: fmt.Sprintf("failed to parse transaction: %v", err),
		}, nil
	}

	// 获取交易哈希
	txHash := tx.Hash().Hex()

	// 广播交易
	ctx, cancel := context.WithTimeout(l.ctx, time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	err = client.SendTransaction(ctx, tx)
	if err != nil {
		l.Errorf("failed to send transaction: %v", err)
		return &pb.BroadcastTransactionResp{
			Success:       false,
			Message:       fmt.Sprintf("failed to broadcast transaction: %v", err),
			TxHash:        txHash,
			Status:        pb.TxStatus_TX_STATUS_FAILED,
			BroadcastedAt: time.Now().Unix(),
			Nonce:         tx.Nonce(),
		}, nil
	}

	resp := &pb.BroadcastTransactionResp{
		Success:       true,
		Message:       "Transaction broadcasted successfully",
		TxHash:        txHash,
		Nonce:         tx.Nonce(),
		Status:        pb.TxStatus_TX_STATUS_PENDING,
		BroadcastedAt: time.Now().Unix(),
	}

	// 如果需要等待交易收据
	if waitForReceipt {
		receipt, err := l.waitForReceipt(ctx, client, txHash, timeoutSeconds)
		if err != nil {
			l.Errorf("failed to wait for receipt: %v", err)
			// 即使等待失败，交易已经广播成功
			return resp, nil
		}

		if receipt != nil {
			resp.BlockNumber = receipt.BlockNumber.String()
			resp.GasUsed = receipt.GasUsed
			if receipt.Status == 1 {
				resp.Status = pb.TxStatus_TX_STATUS_CONFIRMED
			} else {
				resp.Status = pb.TxStatus_TX_STATUS_FAILED
				resp.Message = "Transaction failed"
			}
		}
	}

	return resp, nil
}

// broadcastTronTransaction 广播 TRON 交易
func (l *BroadcastTransactionLogic) broadcastTronTransaction(signedTxHex string, waitForReceipt bool, timeoutSeconds int32) (*pb.BroadcastTransactionResp, error) {
	// 获取 TRON 客户端
	tronClient, err := l.svcCtx.ChainMgr.GetTronClient()
	if err != nil {
		l.Errorf("failed to get TRON client: %v", err)
		return &pb.BroadcastTransactionResp{
			Success: false,
			Message: fmt.Sprintf("failed to get TRON client: %v", err),
		}, nil
	}
	defer tronClient.Stop()

	// 解码已签名交易（protobuf编码）
	signedTxHex = strings.TrimSpace(signedTxHex)
	signedTxHex = strings.TrimPrefix(signedTxHex, "0x")

	signedTxBytes, err := hex.DecodeString(signedTxHex)
	if err != nil {
		l.Errorf("failed to decode signed transaction: %v", err)
		return &pb.BroadcastTransactionResp{
			Success: false,
			Message: fmt.Sprintf("invalid transaction hex: %v", err),
		}, nil
	}

	// 解析交易对象
	tx := &core.Transaction{}
	if err := proto.Unmarshal(signedTxBytes, tx); err != nil {
		l.Errorf("failed to unmarshal TRON transaction: %v", err)
		return &pb.BroadcastTransactionResp{
			Success: false,
			Message: fmt.Sprintf("failed to parse transaction: %v", err),
		}, nil
	}

	// 计算交易哈希
	rawDataBytes, err := proto.Marshal(tx.GetRawData())
	if err != nil {
		l.Errorf("failed to marshal transaction raw data: %v", err)
		return &pb.BroadcastTransactionResp{
			Success: false,
			Message: fmt.Sprintf("failed to calculate transaction hash: %v", err),
		}, nil
	}
	hash := sha256.Sum256(rawDataBytes)
	txHash := hex.EncodeToString(hash[:])

	// 广播交易
	broadcastCtx, cancel := context.WithTimeout(l.ctx, time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	// 使用 context（虽然 TRON SDK 可能不支持，但保留以备将来使用）
	_ = broadcastCtx
	result, err := tronClient.Broadcast(tx)
	if err != nil {
		l.Errorf("failed to broadcast TRON transaction: %v", err)
		return &pb.BroadcastTransactionResp{
			Success:       false,
			Message:       fmt.Sprintf("failed to broadcast transaction: %v", err),
			TxHash:        txHash,
			Status:        pb.TxStatus_TX_STATUS_FAILED,
			BroadcastedAt: time.Now().Unix(),
		}, nil
	}

	if !result.Result {
		errorMsg := "unknown error"
		if result.Code != 0 {
			errorMsg = fmt.Sprintf("code: %d", result.Code)
		}
		if len(result.Message) > 0 {
			errorMsg = string(result.Message)
		}
		return &pb.BroadcastTransactionResp{
			Success:       false,
			Message:       fmt.Sprintf("TRON transaction broadcast failed: %s", errorMsg),
			TxHash:        txHash,
			Status:        pb.TxStatus_TX_STATUS_FAILED,
			BroadcastedAt: time.Now().Unix(),
		}, nil
	}

	resp := &pb.BroadcastTransactionResp{
		Success:       true,
		Message:       "Transaction broadcasted successfully",
		TxHash:        txHash,
		Status:        pb.TxStatus_TX_STATUS_PENDING,
		BroadcastedAt: time.Now().Unix(),
	}

	// 如果需要等待交易收据（TRON 暂时不支持，返回成功即可）
	if waitForReceipt {
		l.Infof("wait_for_receipt is not fully supported for TRON, transaction is broadcasted")
		// TODO: 实现 TRON 交易确认等待逻辑
	}

	return resp, nil
}

// waitForReceipt 等待交易收据（仅 ETH/BSC）
func (l *BroadcastTransactionLogic) waitForReceipt(ctx context.Context, client *ethclient.Client, txHash string, timeoutSeconds int32) (*types.Receipt, error) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	deadline := time.Now().Add(time.Duration(timeoutSeconds) * time.Second)
	hash := common.HexToHash(txHash)

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				return nil, fmt.Errorf("timeout waiting for receipt")
			}

			receipt, err := client.TransactionReceipt(ctx, hash)
			if err != nil {
				// 交易可能还在 pending，继续等待
				continue
			}

			return receipt, nil
		}
	}
}
