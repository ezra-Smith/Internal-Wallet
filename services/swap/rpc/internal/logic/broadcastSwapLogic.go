package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"internalwallet/proto/pb"
	"internalwallet/services/swap/rpc/internal/svc"
)

type BroadcastSwapLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewBroadcastSwapLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BroadcastSwapLogic {
	return &BroadcastSwapLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *BroadcastSwapLogic) BroadcastSwap(in *pb.BroadcastSwapRequest) (*pb.BroadcastSwapResponse, error) {
	// 1. 验证请求参数
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if strings.TrimSpace(in.SwapId) == "" {
		return nil, status.Error(codes.InvalidArgument, "swap_id is required")
	}
	// chain_id 可选，用于验证（如果提供）

	// 2. 检查必需的服务依赖
	if l.svcCtx.DB == nil || l.svcCtx.TxRepo == nil {
		return nil, status.Error(codes.Internal, "database not configured")
	}
	if l.svcCtx.SignerRpc == nil {
		return nil, status.Error(codes.Internal, "signer service not configured")
	}
	if l.svcCtx.EVM == nil {
		return nil, status.Error(codes.Internal, "evm client not configured")
	}

	// 3. 从数据库获取swap记录
	swapID, err := parseSwapID(in.SwapId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid swap_id")
	}

	swapRecord, err := l.svcCtx.TxRepo.FindByID(l.ctx, swapID)
	if err != nil {
		l.Errorf("Failed to get swap record: %v", err)
		return nil, status.Error(codes.NotFound, "swap record not found")
	}

	// 4. 验证swap记录状态和chain_id
	if in.ChainId > 0 && swapRecord.ChainID != in.ChainId {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("chain_id mismatch: expected %d, got %d", swapRecord.ChainID, in.ChainId))
	}
	if swapRecord.Status != "created" {
		return nil, status.Error(codes.FailedPrecondition, fmt.Sprintf("invalid swap status: %s (expected: created)", swapRecord.Status))
	}

	// 验证交易数据完整性
	if strings.TrimSpace(swapRecord.TxFrom) == "" {
		return nil, status.Error(codes.FailedPrecondition, "transaction data incomplete: missing tx_from")
	}
	if strings.TrimSpace(swapRecord.TxTo) == "" {
		return nil, status.Error(codes.FailedPrecondition, "transaction data incomplete: missing tx_to")
	}
	if strings.TrimSpace(swapRecord.TxData) == "" {
		return nil, status.Error(codes.FailedPrecondition, "transaction data incomplete: missing tx_data")
	}

	// 检查并记录 SignatureData（OKX 特定：某些交易需要额外的签名数据）
	var signatureData []string
	if swapRecord.TxSignatureData != "" {
		if err := json.Unmarshal([]byte(swapRecord.TxSignatureData), &signatureData); err != nil {
			l.Infof("Failed to parse signature data", logx.Field("error", err))
		} else if len(signatureData) > 0 {
			l.Infow("Transaction has extra signature data from OKX",
				logx.Field("swap_id", swapID),
				logx.Field("signature_data_count", len(signatureData)),
				logx.Field("signature_data", signatureData),
			)
			// TODO: 根据 OKX 文档，这些签名数据需要在签名过程中正确应用
			// 当前实现：记录日志以便排查
			// 未来优化：需要根据具体的 DEX 协议正确处理这些数据
		}
	}

	// 5. 从数据库记录构建未签名交易
	// 获取nonce（优先使用数据库中的，否则查询链上）
	var nonceUint64 uint64
	savedNonce := strings.TrimSpace(swapRecord.TxNonce)
	if savedNonce != "" {
		if _, err := fmt.Sscanf(savedNonce, "%d", &nonceUint64); err != nil {
			l.Infow("Invalid saved nonce, will query from chain", logx.Field("error", err))
			savedNonce = ""
		}
	}

	if savedNonce == "" {
		// 从RPC获取nonce
		ethClient, err := l.svcCtx.EVM.GetClient(l.ctx, swapRecord.ChainID)
		if err != nil {
			l.Errorf("Failed to get EVM client for nonce: %v", err)
			return nil, status.Error(codes.Internal, "failed to connect to chain")
		}
		fromAddr := strings.TrimSpace(swapRecord.TxFrom)
		if !strings.HasPrefix(fromAddr, "0x") {
			fromAddr = "0x" + fromAddr
		}
		nonceVal, err := ethClient.PendingNonceAt(l.ctx, common.HexToAddress(fromAddr))
		if err != nil {
			l.Errorf("Failed to get nonce: %v", err)
			return nil, status.Error(codes.Internal, "failed to get nonce")
		}
		nonceUint64 = nonceVal
	}

	// 解析交易参数（从数据库）
	value, ok := new(big.Int).SetString(strings.TrimSpace(swapRecord.TxValue), 10)
	if !ok {
		l.Errorf("Invalid tx_value in database: %s", swapRecord.TxValue)
		return nil, status.Error(codes.Internal, "invalid transaction data: value")
	}

	gasLimit, ok := new(big.Int).SetString(strings.TrimSpace(swapRecord.TxGas), 10)
	if !ok || gasLimit.Sign() <= 0 {
		l.Errorf("Invalid tx_gas in database: %s", swapRecord.TxGas)
		return nil, status.Error(codes.Internal, "invalid transaction data: gas")
	}

	// 解析gas price（优先使用数据库中的，否则查询链上）
	var gasPrice *big.Int
	savedGasPrice := strings.TrimSpace(swapRecord.TxGasPrice)
	if savedGasPrice != "" && savedGasPrice != "0" {
		var ok bool
		gasPrice, ok = new(big.Int).SetString(savedGasPrice, 10)
		if !ok || gasPrice.Sign() <= 0 {
			l.Infow("Invalid saved gas_price, will query from chain", logx.Field("saved_gas_price", savedGasPrice))
			savedGasPrice = ""
		}
	}

	if savedGasPrice == "" || savedGasPrice == "0" {
		ethClient, err := l.svcCtx.EVM.GetClient(l.ctx, swapRecord.ChainID)
		if err != nil {
			return nil, status.Error(codes.Internal, "failed to connect to chain")
		}
		gasPrice, err = ethClient.SuggestGasPrice(l.ctx)
		if err != nil {
			l.Errorf("Failed to get gas price: %v", err)
			return nil, status.Error(codes.Internal, "failed to get gas price")
		}
	}

	// 解析data（从数据库）
	dataHex := strings.TrimSpace(swapRecord.TxData)
	if !strings.HasPrefix(dataHex, "0x") {
		dataHex = "0x" + dataHex
	}
	data, err := hexutil.Decode(dataHex)
	if err != nil {
		l.Errorf("Invalid tx_data in database: %v", err)
		return nil, status.Error(codes.Internal, "invalid transaction data: data")
	}

	// 解析to地址（从数据库）
	toAddr := strings.TrimSpace(swapRecord.TxTo)
	if !strings.HasPrefix(toAddr, "0x") {
		toAddr = "0x" + toAddr
	}
	toAddress, err := hexutil.Decode(toAddr)
	if err != nil || len(toAddress) != 20 {
		l.Errorf("Invalid tx_to in database: %s", swapRecord.TxTo)
		return nil, status.Error(codes.Internal, "invalid transaction data: to address")
	}
	to := common.BytesToAddress(toAddress)

	// 构建EVM交易（Legacy Transaction）
	unsignedTx := types.NewTransaction(
		nonceUint64,
		to,
		value,
		gasLimit.Uint64(),
		gasPrice,
		data,
	)

	// 序列化未签名交易为RLP hex
	rawTxBytes, err := unsignedTx.MarshalBinary()
	if err != nil {
		l.Errorf("Failed to marshal transaction: %v", err)
		return nil, status.Error(codes.Internal, "failed to serialize transaction")
	}
	rawTxHex := hexutil.Encode(rawTxBytes)

	// 6. 调用Signer服务签名交易
	chainName := getChainName(swapRecord.ChainID)
	requestID := fmt.Sprintf("swap_%d_%d", swapID, time.Now().Unix())

	signReq := &pb.SignTransactionRequest{
		RequestId:      requestID,
		Chain:          chainName,
		FromAddress:    swapRecord.WalletAddress,
		RawTransaction: rawTxHex,
		OperationType:  6, // 6 = other/swap
		Amount:         swapRecord.FromAmount,
		ToAddress:      swapRecord.ToToken,
		AssetSymbol:    "", // 可选：可以从swap记录中获取token symbol
		TokenContract:  swapRecord.FromToken,
		Requester:      "swap_service",
	}

	signResp, err := l.svcCtx.SignerRpc.SignTransaction(l.ctx, signReq)
	if err != nil {
		l.Errorf("Failed to sign transaction: %v", err)
		// 更新swap状态为failed
		now := time.Now().Local()
		swapRecord.Status = "failed"
		swapRecord.ErrorCode = "SIGN_FAILED"
		errMsg := fmt.Sprintf("signature service error: %v", err)
		swapRecord.ErrorMessage = &errMsg
		swapRecord.UpdatedAt = now

		if updateErr := l.svcCtx.TxRepo.UpdateFields(l.ctx, swapID, map[string]any{
			"status":        swapRecord.Status,
			"error_code":    swapRecord.ErrorCode,
			"error_message": swapRecord.ErrorMessage,
		}); updateErr != nil {
			l.Errorw("Failed to update swap record after sign failure",
				logx.Field("error", updateErr),
				logx.Field("swap_id", swapID),
			)
		}
		return nil, status.Error(codes.Internal, "failed to sign transaction")
	}

	if signResp.Code != 0 {
		l.Errorf("Signer returned error: code=%d, message=%s", signResp.Code, signResp.Message)
		now := time.Now().Local()
		swapRecord.Status = "failed"
		swapRecord.ErrorCode = "SIGN_FAILED"
		errMsg := fmt.Sprintf("signer error: %s", signResp.Message)
		swapRecord.ErrorMessage = &errMsg
		swapRecord.UpdatedAt = now

		if updateErr := l.svcCtx.TxRepo.UpdateFields(l.ctx, swapID, map[string]any{
			"status":        swapRecord.Status,
			"error_code":    swapRecord.ErrorCode,
			"error_message": swapRecord.ErrorMessage,
		}); updateErr != nil {
			l.Errorw("Failed to update swap record after signer error",
				logx.Field("error", updateErr),
				logx.Field("swap_id", swapID),
			)
		}
		return nil, status.Error(codes.Internal, fmt.Sprintf("signature failed: %s", signResp.Message))
	}

	// 7. 解析签名后的交易
	signedTxHex := signResp.Signature
	if !strings.HasPrefix(signedTxHex, "0x") {
		signedTxHex = "0x" + signedTxHex
	}

	signedTxBytes, err := hexutil.Decode(signedTxHex)
	if err != nil {
		l.Errorf("Failed to decode signed transaction: %v", err)
		return nil, status.Error(codes.Internal, "failed to decode signed transaction")
	}

	signedTx := new(types.Transaction)
	if err := signedTx.UnmarshalBinary(signedTxBytes); err != nil {
		l.Errorf("Failed to unmarshal signed transaction: %v", err)
		return nil, status.Error(codes.Internal, "failed to parse signed transaction")
	}

	// 8. 获取EVM RPC客户端并广播交易
	ethClient, err := l.svcCtx.EVM.GetClient(l.ctx, swapRecord.ChainID)
	if err != nil {
		l.Errorf("Failed to get EVM client for chain %d: %v", swapRecord.ChainID, err)
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to connect to chain: %v", err))
	}

	if err := ethClient.SendTransaction(l.ctx, signedTx); err != nil {
		l.Errorf("Failed to broadcast transaction: %v", err)
		// 更新swap状态为failed
		now := time.Now().Local()
		swapRecord.Status = "failed"
		swapRecord.ErrorCode = "BROADCAST_FAILED"
		errMsg := fmt.Sprintf("broadcast error: %v", err)
		swapRecord.ErrorMessage = &errMsg
		swapRecord.TxHash = signedTx.Hash().Hex()
		swapRecord.UpdatedAt = now

		if updateErr := l.svcCtx.TxRepo.UpdateFields(l.ctx, swapID, map[string]any{
			"status":        swapRecord.Status,
			"error_code":    swapRecord.ErrorCode,
			"error_message": swapRecord.ErrorMessage,
			"tx_hash":       swapRecord.TxHash,
		}); updateErr != nil {
			l.Errorw("Failed to update swap record after broadcast failure",
				logx.Field("error", updateErr),
				logx.Field("swap_id", swapID),
			)
		}

		// 返回 gRPC 错误而不是 Success: false 的响应体
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to broadcast transaction: %v", err))
	}

	// 9. 广播成功，更新数据库记录
	txHash := signedTx.Hash().Hex()
	now := time.Now().Local()
	swapRecord.Status = "pending"
	swapRecord.TxHash = txHash
	swapRecord.BroadcastedAt = &now
	swapRecord.UpdatedAt = now
	swapRecord.ErrorCode = ""
	swapRecord.ErrorMessage = nil

	// 详细记录更新信息
	l.Infow("Attempting to update swap record",
		logx.Field("swap_id", swapID),
		logx.Field("tx_hash", txHash),
		logx.Field("status", "pending"),
		logx.Field("broadcasted_at", now.Format(time.RFC3339)),
	)

	updateFields := map[string]any{
		"status":         swapRecord.Status,
		"tx_hash":        swapRecord.TxHash,
		"broadcasted_at": swapRecord.BroadcastedAt,
		"error_code":     "",
		"error_message":  nil,
	}

	if err := l.svcCtx.TxRepo.UpdateFields(l.ctx, swapID, updateFields); err != nil {
		l.Errorw("Failed to update swap record after broadcast",
			logx.Field("error", err),
			logx.Field("swap_id", swapID),
			logx.Field("tx_hash", txHash),
			logx.Field("update_fields", updateFields),
		)
		// 虽然更新数据库失败，但交易已经广播，仍然返回成功
	} else {
		l.Infow("Successfully updated swap record",
			logx.Field("swap_id", swapID),
			logx.Field("tx_hash", txHash),
		)
	}

	l.Infof("Swap transaction broadcasted successfully: swap_id=%s, tx_hash=%s, chain_id=%d", in.SwapId, txHash, swapRecord.ChainID)

	return &pb.BroadcastSwapResponse{
		Success: true,
		Message: "transaction broadcasted successfully",
		Data: &pb.BroadcastSwapData{
			SwapId:        in.SwapId,
			TxHash:        txHash,
			Status:        pb.SwapTxStatus_SWAP_TX_STATUS_PENDING,
			BroadcastedAt: now.Unix(),
		},
	}, nil
}

// parseSwapID converts swap_id string to int64
func parseSwapID(swapID string) (int64, error) {
	swapID = strings.TrimSpace(swapID)
	if swapID == "" {
		return 0, fmt.Errorf("empty swap_id")
	}
	// 假设swap_id是数字类型的ID，可以直接解析
	var id int64
	if _, err := fmt.Sscanf(swapID, "%d", &id); err != nil {
		return 0, err
	}
	return id, nil
}

// getChainName maps chain ID to chain name for Signer service
func getChainName(chainID int64) string {
	switch chainID {
	case 1:
		return "ETH"
	case 56:
		return "BSC"
	case 137:
		return "POLYGON"
	case 42161:
		return "ARBITRUM"
	case 10:
		return "OPTIMISM"
	case 43114:
		return "AVALANCHE"
	case 250:
		return "FANTOM"
	case 8453:
		return "BASE"
	default:
		// 返回默认值或使用通用名称
		return fmt.Sprintf("CHAIN_%d", chainID)
	}
}
