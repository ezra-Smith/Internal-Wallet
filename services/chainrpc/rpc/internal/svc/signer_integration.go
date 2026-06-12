package svc

import (
	"context"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/logx"

	"internalwallet/proto/pb"
)

// SignTransactionWithSigner 使用签名服务对交易进行签名（带重试机制）
func (m *ChainClientManager) SignTransactionWithSigner(ctx context.Context, req *pb.SignTransactionRequest) (*pb.SignTransactionResponse, error) {
	const maxRetries = 3
	const initialBackoff = 100 * time.Millisecond

	// Get signer client
	signerClient := m.GetSignerClient()
	if signerClient == nil {
		return nil, fmt.Errorf("signer client not available")
	}

	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		// Call signer service
		resp, err := signerClient.SignTransaction(ctx, req)
		if err != nil {
			lastErr = err

			// Check if it's a connection error that might be resolved with etcd discovery
			if attempt < maxRetries-1 {
				backoff := time.Duration(attempt+1) * initialBackoff
				logx.Errorf("Failed to sign transaction (attempt %d/%d): %v, retrying in %v",
					attempt+1, maxRetries, err, backoff)

				// Wait before retry
				select {
				case <-ctx.Done():
					return nil, fmt.Errorf("context cancelled during retry: %v", ctx.Err())
				case <-time.After(backoff):
					continue
				}
			}
			return nil, fmt.Errorf("failed to sign transaction after %d attempts: %v", maxRetries, lastErr)
		}

		// Check response code
		if resp.Code != 0 {
			// For service errors (like invalid parameters), don't retry
			return nil, fmt.Errorf("signer service returned error: %s", resp.Message)
		}

		logx.Infof("Transaction signed successfully, tx_hash: %s", resp.TxHash)
		return resp, nil
	}

	return nil, fmt.Errorf("failed to sign transaction after %d attempts: %v", maxRetries, lastErr)
}

// BuildAndSignTransaction 构建并签名交易的完整流程示例
func (m *ChainClientManager) BuildAndSignTransaction(ctx context.Context, chainType pb.ChainRpcType,
	fromAddress, toAddress, amount, derivationPath string) (*pb.SignTransactionResponse, error) {

	// This is an example of the complete flow:
	// 1. Build unsigned transaction using the appropriate logic
	// 2. Call signer service to sign the transaction

	// For demonstration, we'll create a simple sign request
	// In practice, you would call the BuildNativeTransfer logic first

	// Validate required parameters
	if fromAddress == "" || toAddress == "" || amount == "" {
		return nil, fmt.Errorf("missing required parameters: fromAddress=%s, toAddress=%s, amount=%s",
			fromAddress, toAddress, amount)
	}

	// Generate a proper request ID if not provided
	requestID := fmt.Sprintf("chainrpc_%d", time.Now().UnixNano())

	signReq := &pb.SignTransactionRequest{
		RequestId:      requestID,
		Chain:          chainType.String(),
		RawTransaction: "0x...", // This would be the raw transaction from BuildNativeTransfer
		OperationType:  2,       // 2=withdrawal
		Amount:         amount,
		ToAddress:      toAddress,
		FromAddress:    fromAddress,
		AssetSymbol:    getAssetSymbol(chainType),
		Requester:      "chainrpc_service",
	}

	// Validate the request before sending
	if err := ValidateSignRequest(signReq); err != nil {
		return nil, fmt.Errorf("invalid sign request: %v", err)
	}

	return m.SignTransactionWithSigner(ctx, signReq)
}

// getAssetSymbol returns the asset symbol for the given chain
func getAssetSymbol(chainType pb.ChainRpcType) string {
	switch chainType {
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

// ValidateSignRequest 验证签名请求参数
func ValidateSignRequest(req *pb.SignTransactionRequest) error {
	if req == nil {
		return fmt.Errorf("sign request is nil")
	}

	if req.RequestId == "" {
		return fmt.Errorf("request ID is empty")
	}
	if req.Chain == "" {
		return fmt.Errorf("chain type is empty")
	}

	if req.RawTransaction == "" || req.RawTransaction == "0x..." {
		return fmt.Errorf("raw transaction is empty or placeholder")
	}

	if req.ToAddress == "" {
		return fmt.Errorf("to address is empty")
	}

	if req.FromAddress == "" {
		return fmt.Errorf("from address is empty")
	}

	if req.Amount == "" {
		return fmt.Errorf("amount is empty")
	}

	return nil
}

// ValidateSignerResponse 验证签名服务响应
func ValidateSignerResponse(resp *pb.SignTransactionResponse) error {
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
