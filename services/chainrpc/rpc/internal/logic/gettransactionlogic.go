package logic

import (
	"context"
	"fmt"
	"math/big"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/fbsobreira/gotron-sdk/pkg/address"
	"github.com/zeromicro/go-zero/core/logx"

	"internalwallet/proto/pb"
	"internalwallet/services/chainrpc/rpc/internal/svc"
)

type GetTransactionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetTransactionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetTransactionLogic {
	return &GetTransactionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetTransaction 查询交易详情
func (l *GetTransactionLogic) GetTransaction(in *pb.GetTransactionReq) (*pb.GetTransactionResp, error) {
	switch in.Chain {
	case pb.ChainRpcType_CHAIN_TYPE_ETHEREUM, pb.ChainRpcType_CHAIN_TYPE_BSC:
		return l.getETHTransaction(in)
	case pb.ChainRpcType_CHAIN_TYPE_TRON:
		return l.getTronTransaction(in)
	default:
		return nil, fmt.Errorf("unsupported chain type: %v", in.Chain)
	}
}

// getETHTransaction 查询 ETH/BSC 交易详情
func (l *GetTransactionLogic) getETHTransaction(in *pb.GetTransactionReq) (*pb.GetTransactionResp, error) {
	// Get ETH client
	client, err := l.svcCtx.ChainMgr.GetETHClient(in.Chain)
	if err != nil {
		return nil, fmt.Errorf("failed to get ETH client: %v", err)
	}

	// Parse transaction hash
	txHash := common.HexToHash(in.TxHash)

	// Get transaction
	tx, isPending, err := client.TransactionByHash(l.ctx, txHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction: %v", err)
	}

	// Get chain ID
	chainID, err := client.ChainID(l.ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get chain ID: %v", err)
	}

	// Get sender address
	signer := types.NewEIP155Signer(chainID)
	from, err := types.Sender(signer, tx)
	if err != nil {
		return nil, fmt.Errorf("failed to get sender: %v", err)
	}

	// Build transaction detail
	transaction := &pb.TransactionDetail{
		TxHash:           tx.Hash().Hex(),
		Chain:            in.Chain,
		BlockNumber:      "",
		BlockHash:        "",
		BlockTimestamp:   0,
		TransactionIndex: 0,
		FromAddress:      from.Hex(),
		ToAddress:        "",
		Value:            tx.Value().String(),
		GasPrice:         tx.GasPrice().String(),
		GasLimit:         tx.Gas(),
		GasUsed:          0,
		GasFee:           "0",
		Status:           pb.TxStatus_TX_STATUS_PENDING,
		Nonce:            strconv.FormatUint(tx.Nonce(), 10),
		InputData:        tx.Data(),
		Confirmations:    0,
	}

	// Set to address if exists
	if tx.To() != nil {
		transaction.ToAddress = tx.To().Hex()
	}

	// Pending tx: receipt not available yet.
	if isPending {
		return &pb.GetTransactionResp{
			Success:     true,
			Message:     "Transaction pending",
			Transaction: transaction,
		}, nil
	}

	// Get transaction receipt (may still be pending on some providers)
	receipt, err := client.TransactionReceipt(l.ctx, txHash)
	if err != nil {
		return &pb.GetTransactionResp{
			Success:     true,
			Message:     "Transaction pending",
			Transaction: transaction,
		}, nil
	}

	// Get current block number for confirmation count
	currentBlock, err := client.BlockNumber(l.ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get current block number: %v", err)
	}
	var confirmations uint64
	if receipt.BlockNumber != nil {
		confirmations = currentBlock - receipt.BlockNumber.Uint64()
	}

	// Get block for timestamp
	block, err := client.BlockByHash(l.ctx, receipt.BlockHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get block: %v", err)
	}

	// Determine transaction status
	var status pb.TxStatus
	if receipt.Status == 1 {
		status = pb.TxStatus_TX_STATUS_CONFIRMED
	} else {
		status = pb.TxStatus_TX_STATUS_FAILED
	}

	transaction.BlockNumber = receipt.BlockNumber.String()
	transaction.BlockHash = receipt.BlockHash.Hex()
	transaction.BlockTimestamp = int64(block.Time())
	transaction.TransactionIndex = uint64(receipt.TransactionIndex)
	transaction.GasUsed = receipt.GasUsed
	transaction.GasFee = new(big.Int).Mul(tx.GasPrice(), new(big.Int).SetUint64(receipt.GasUsed)).String()
	transaction.Status = status
	transaction.Confirmations = confirmations

	// Parse logs if requested
	if in.IncludeLogs {
		for _, log := range receipt.Logs {
			eventLog := &pb.EventLog{
				LogIndex: strconv.Itoa(int(log.Index)),
				Address:  log.Address.Hex(),
				Topics:   make([]string, len(log.Topics)),
				Data:     log.Data,
			}

			for i, topic := range log.Topics {
				eventLog.Topics[i] = topic.Hex()
			}

			transaction.Logs = append(transaction.Logs, eventLog)
		}
	}

	return &pb.GetTransactionResp{
		Success:     true,
		Message:     "Transaction retrieved successfully",
		Transaction: transaction,
	}, nil
}

// getTronTransaction 查询 TRON 交易详情
func (l *GetTransactionLogic) getTronTransaction(in *pb.GetTransactionReq) (*pb.GetTransactionResp, error) {
	// Get TRON client
	tronClient, err := l.svcCtx.ChainMgr.GetTronClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get TRON client: %v", err)
	}
	defer tronClient.Stop()

	// Get current block number for confirmation count
	nowBlock, err := tronClient.GetNowBlock()
	if err != nil {
		return nil, fmt.Errorf("failed to get current block: %v", err)
	}
	var currentBlock int64
	if nowBlock != nil && nowBlock.BlockHeader != nil && nowBlock.BlockHeader.RawData != nil {
		currentBlock = nowBlock.BlockHeader.RawData.Number
	}

	// Prefer transaction info (includes block number + execution result) for status/confirmations.
	status := pb.TxStatus_TX_STATUS_PENDING
	blockNumber := ""
	var confirmations uint64
	feeSun := int64(0)
	txInfo, infoErr := tronClient.GetTransactionInfoByID(in.TxHash)
	if infoErr == nil && txInfo != nil {
		if txInfo.Result == 0 {
			status = pb.TxStatus_TX_STATUS_CONFIRMED
		} else {
			status = pb.TxStatus_TX_STATUS_FAILED
		}
		if txInfo.Fee > 0 {
			// TRON total fee in SUN (includes bandwidth burn, account activation, etc.)
			feeSun = txInfo.Fee
		}
		if txInfo.BlockNumber > 0 {
			blockNumber = fmt.Sprintf("%d", txInfo.BlockNumber)
			if currentBlock > txInfo.BlockNumber {
				confirmations = uint64(currentBlock - txInfo.BlockNumber)
			}
		}
	}

	// Get transaction
	tx, err := tronClient.GetTransactionByID(in.TxHash)
	if err != nil {
		msg := "TRON transaction retrieved successfully"
		if status == pb.TxStatus_TX_STATUS_PENDING {
			msg = "Transaction pending"
		}
		return &pb.GetTransactionResp{
			Success: true,
			Message: msg,
			Transaction: &pb.TransactionDetail{
				TxHash:         in.TxHash,
				Chain:          pb.ChainRpcType_CHAIN_TYPE_TRON,
				BlockNumber:    blockNumber,
				BlockHash:      "",
				BlockTimestamp: 0,
				FromAddress:    "",
				ToAddress:      "",
				Value:          "0",
				GasPrice:       "0",
				GasLimit:       0,
				GasUsed:        0,
				GasFee:         "0",
				Status:         status,
				Nonce:          "0",
				InputData:      []byte{},
				Confirmations:  confirmations,
			},
		}, nil
	}

	if tx == nil || tx.RawData == nil {
		msg := "TRON transaction retrieved successfully"
		if status == pb.TxStatus_TX_STATUS_PENDING {
			msg = "Transaction pending"
		}
		return &pb.GetTransactionResp{
			Success: true,
			Message: msg,
			Transaction: &pb.TransactionDetail{
				TxHash:         in.TxHash,
				Chain:          pb.ChainRpcType_CHAIN_TYPE_TRON,
				BlockNumber:    blockNumber,
				BlockHash:      "",
				BlockTimestamp: 0,
				FromAddress:    "",
				ToAddress:      "",
				Value:          "0",
				GasPrice:       "0",
				GasLimit:       0,
				GasUsed:        0,
				GasFee:         "0",
				Status:         status,
				Nonce:          "0",
				InputData:      []byte{},
				Confirmations:  confirmations,
			},
		}, nil
	}

	// Build transaction detail
	transaction := &pb.TransactionDetail{
		TxHash:         in.TxHash,
		Chain:          pb.ChainRpcType_CHAIN_TYPE_TRON,
		BlockNumber:    blockNumber,
		BlockHash:      "",
		BlockTimestamp: tx.RawData.Timestamp,
		FromAddress:    "", // Will be extracted from contract data
		ToAddress:      "", // Will be extracted from contract data
		Value:          "0",
		GasPrice:       "0", // TRON uses different fee model
		GasLimit:       uint64(tx.RawData.FeeLimit),
		GasUsed:        0,
		// IMPORTANT: For TRON, the authoritative total fee comes from GetTransactionInfoByID().Fee (SUN).
		// tx.Ret[0].Fee is not reliable for total fees (e.g. account activation / other fees).
		GasFee: func() string {
			if feeSun > 0 {
				return strconv.FormatInt(feeSun, 10)
			}
			return "0"
		}(),
		Status:         status,
		Nonce:          "0", // TRON doesn't use nonce
		InputData:      []byte{},
		Confirmations:  confirmations,
	}

	// Extract contract information
	if len(tx.RawData.Contract) > 0 {
		contract := tx.RawData.Contract[0]
		if contract.Parameter != nil {
			transaction.InputData = contract.Parameter.Value

			// Parse contract based on type
			switch contract.Type {
			case 1: // TransferContract - TRX transfer
				from, to, amount := l.parseTrxTransfer(contract.Parameter.Value)
				transaction.FromAddress = from
				transaction.ToAddress = to
				transaction.Value = amount
			case 31: // TriggerSmartContract
				from, to, value, method, tokenValue := l.parseSmartContract(contract.Parameter.Value)
				transaction.FromAddress = from
				transaction.ToAddress = to
				if method == "transfer" && tokenValue != "" {
					// TRC-20 transfer
					transaction.Value = tokenValue
				} else if value != "" {
					// TRX transfer with contract call
					transaction.Value = value
				}
			}
		}
	}

	// Best-effort fallback: if txInfo was not available, try ret[0].Fee.
	if feeSun <= 0 && len(tx.Ret) > 0 && tx.Ret[0].Fee > 0 {
		transaction.GasFee = strconv.FormatInt(tx.Ret[0].Fee, 10)
	}

	return &pb.GetTransactionResp{
		Success:     true,
		Message:     "TRON transaction retrieved successfully",
		Transaction: transaction,
	}, nil
}

// parseTrxTransfer parses TRX transfer contract data
func (l *GetTransactionLogic) parseTrxTransfer(data []byte) (from, to, amount string) {
	// Parse using TRON protocol format
	if len(data) >= 69 { // 21 (from) + 21 (to) + 8 (amount) + 32 (padding) = 82 minimum
		// Extract addresses (21 bytes each in hex format)
		fromBytes := data[0:21]
		toBytes := data[21:42]

		// Convert to hex string
		from = fmt.Sprintf("%x", fromBytes)
		to = fmt.Sprintf("%x", toBytes)

		// Convert hex addresses to base58
		fromAddr := address.HexToAddress(from)
		if fromAddr != nil {
			from = fromAddr.String()
		}
		toAddr := address.HexToAddress(to)
		if toAddr != nil {
			to = toAddr.String()
		}

		// Extract amount (8 bytes, little endian)
		if len(data) >= 69 {
			amountBytes := data[64:72] // Skip padding
			amt := new(big.Int)
			for i := 0; i < 8; i++ {
				amt = amt.Lsh(amt, 8)
				amt = amt.Or(amt, big.NewInt(int64(amountBytes[7-i])))
			}
			amount = amt.String()
		}
	}
	return
}

// parseSmartContract parses smart contract call data
func (l *GetTransactionLogic) parseSmartContract(data []byte) (from, to, value, method, tokenValue string) {
	// Parse smart contract parameter data
	if len(data) >= 69 { // Minimum size for contract call
		// Extract owner address (21 bytes)
		ownerBytes := data[0:21]
		from = fmt.Sprintf("%x", ownerBytes)
		fromAddr := address.HexToAddress(from)
		if fromAddr != nil {
			from = fromAddr.String()
		}

		// Extract contract address (21 bytes)
		if len(data) >= 42 {
			contractBytes := data[21:42]
			contractAddr := fmt.Sprintf("%x", contractBytes)
			contractAddrObj := address.HexToAddress(contractAddr)
			if contractAddrObj != nil {
				to = contractAddrObj.String()
			}
		}

		// Extract call value (8 bytes)
		if len(data) >= 69 {
			valueBytes := data[48:56]
			val := new(big.Int)
			for i := 0; i < 8; i++ {
				val = val.Lsh(val, 8)
				val = val.Or(val, big.NewInt(int64(valueBytes[7-i])))
			}
			value = val.String()
		}

		// Parse function call data for TRC-20 transfers
		if len(data) >= 100 {
			callData := data[69:]
			if len(callData) >= 4 {
				// Check function signature (first 4 bytes)
				funcSig := fmt.Sprintf("%02x%02x%02x%02x",
					callData[0], callData[1], callData[2], callData[3])
				if funcSig == "a9059cbb" {
					method = "transfer"
					// Parse parameters (32 bytes for to address, 32 bytes for amount)
					if len(callData) >= 68 {
						// Extract 'to' address (right-aligned 20 bytes)
						toAddrBytes := callData[16:36]
						toAddrHex := fmt.Sprintf("%x", toAddrBytes)
						toAddrObj := address.HexToAddress(toAddrHex)
						if toAddrObj != nil {
							to = toAddrObj.String()
						}

						// Extract amount (last 32 bytes)
						amountBytes := callData[36:68]
						tokenAmt := new(big.Int)
						for i := 0; i < 32; i++ {
							tokenAmt = tokenAmt.Lsh(tokenAmt, 8)
							tokenAmt = tokenAmt.Or(tokenAmt, big.NewInt(int64(amountBytes[i])))
						}
						tokenValue = tokenAmt.String()
					}
				}
			}
		}
	}
	return
}

// hexToBase58 converts TRON hex address to base58 format
func (l *GetTransactionLogic) hexToBase58(hexAddr string) string {
	if hexAddr == "" {
		return ""
	}

	// Remove 0x prefix if present
	if len(hexAddr) > 2 && hexAddr[:2] == "0x" {
		hexAddr = hexAddr[2:]
	}

	// Convert hex to base58
	addr := address.HexToAddress(hexAddr)
	if addr != nil {
		return addr.String()
	}

	return ""
}
