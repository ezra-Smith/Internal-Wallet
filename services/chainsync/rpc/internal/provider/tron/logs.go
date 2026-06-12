package tron

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/fbsobreira/gotron-sdk/pkg/common"
	"github.com/zeromicro/go-zero/core/logx"

	"internalwallet/proto/pb"
	"internalwallet/services/chainsync/rpc/internal/units"
)

type TronTransactionInfo struct {
	TxHash      string
	BlockNumber uint64
	Fee         int64
	Result      int32 // 0=success
	Logs        []*TronEventLog
	Receipt     TronTransactionReceipt
}

type TronTransactionReceipt struct {
	EnergyUsage      int64
	EnergyUsageTotal int64
	NetUsage         int64
}

type TronEventLog struct {
	Address string
	Topics  []string
	Data    string
}

type TRC20Transfer struct {
	ContractAddress string
	From            string
	To              string
	Amount          string
}

func (t *TronProvider) GetTransactionInfo(ctx context.Context, txHash string) (*TronTransactionInfo, error) {
	start := time.Now()
	var err error
	defer t.finishRequest(start, &err)

	grpcClient := t.createClient()
	defer grpcClient.Stop()

	if err := t.connectWithTimeout(ctx, grpcClient); err != nil {
		err = fmt.Errorf("connection failed: %w", err)
		return nil, err
	}

	txInfo, err := grpcClient.GetTransactionInfoByID(txHash)
	if err != nil {
		err = fmt.Errorf("get transaction info failed: %w", err)
		return nil, err
	}

	if txInfo == nil {
		err = fmt.Errorf("transaction info not found")
		return nil, err
	}

	info := &TronTransactionInfo{
		TxHash:      txHash,
		BlockNumber: uint64(txInfo.BlockNumber),
		Fee:         txInfo.Fee,
		Result:      int32(txInfo.Result),
	}

	if txInfo.Receipt != nil {
		info.Receipt = TronTransactionReceipt{
			EnergyUsage:      txInfo.Receipt.EnergyUsage,
			EnergyUsageTotal: txInfo.Receipt.EnergyUsageTotal,
			NetUsage:         txInfo.Receipt.NetUsage,
		}
	}

	if len(txInfo.Log) > 0 {
		for _, log := range txInfo.Log {
			tronLog := &TronEventLog{
				Address: common.Bytes2Hex(log.Address),
				Data:    common.Bytes2Hex(log.Data),
			}
			for _, topic := range log.Topics {
				tronLog.Topics = append(tronLog.Topics, common.Bytes2Hex(topic))
			}
			info.Logs = append(info.Logs, tronLog)
		}
	}

	return info, nil
}

// ParseTRC20TransferFromLogs parses TRC-20 Transfer events from logs.
// Transfer signature: keccak256("Transfer(address,address,uint256)")
func (t *TronProvider) ParseTRC20TransferFromLogs(logs []*TronEventLog) []*TRC20Transfer {
	var transfers []*TRC20Transfer

	transferEventSignature := "ddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"

	for _, log := range logs {
		if len(log.Topics) < 3 {
			continue
		}
		if log.Topics[0] != transferEventSignature {
			continue
		}

		transfer := &TRC20Transfer{ContractAddress: log.Address}

		if len(log.Topics[1]) >= 40 {
			fromHex := log.Topics[1][len(log.Topics[1])-40:]
			transfer.From = HexToBase58("41" + fromHex)
		}
		if len(log.Topics[2]) >= 40 {
			toHex := log.Topics[2][len(log.Topics[2])-40:]
			transfer.To = HexToBase58("41" + toHex)
		}

		if log.Data != "" {
			amount := new(big.Int)
			amount.SetString(log.Data, 16)
			transfer.Amount = amount.String()
		}

		transfers = append(transfers, transfer)
		logx.Infof("📋 Parsed TRC-20 Transfer from logs: %s → %s, amount: %s, contract: %s",
			transfer.From, transfer.To, transfer.Amount, transfer.ContractAddress)
	}

	return transfers
}

func (t *TronProvider) GetTransactionWithLogs(ctx context.Context, tx *pb.Transaction) (*pb.Transaction, error) {
	if tx == nil || tx.TxHash == "" {
		return tx, nil
	}

	txInfo, err := t.GetTransactionInfo(ctx, tx.TxHash)
	if err != nil {
		// Some TRON nodes may not serve txInfo for older txs (pruning windows).
		// If we can't fetch txInfo, we MUST NOT assume "confirmed" (especially for contract calls)
		// because TRC20 calldata can look like a transfer even when the contract REVERTs.
		logx.Debugf("Failed to get transaction info for %s: %v (try TronGrid fallback)", tx.TxHash, err)

		// Best-effort fallback: query a public endpoint for authoritative receipt.result.
		if ok := t.tryEnrichFromTronGrid(ctx, tx); ok {
			return tx, nil
		}

		// Conservative fallback: unknown => pending (unless already failed).
		if tx.Status != pb.TransactionStatus_TRANSACTION_STATUS_FAILED {
			tx.Status = pb.TransactionStatus_TRANSACTION_STATUS_PENDING
		}
		return tx, nil
	}

	if txInfo.Result == 0 {
		tx.Status = pb.TransactionStatus_TRANSACTION_STATUS_CONFIRMED
	} else {
		tx.Status = pb.TransactionStatus_TRANSACTION_STATUS_FAILED
	}

	if txInfo.Fee > 0 {
		tx.GasFee = units.FormatTokenAmount(strconv.FormatInt(txInfo.Fee, 10), 6)
	} else {
		tx.GasFee = "0"
	}

	if txInfo.Receipt.EnergyUsageTotal > 0 {
		tx.EnergyUsed = uint64(txInfo.Receipt.EnergyUsageTotal)
	} else if txInfo.Receipt.EnergyUsage > 0 {
		tx.EnergyUsed = uint64(txInfo.Receipt.EnergyUsage)
	}

	tx.BandwidthUsed = uint64(txInfo.Receipt.NetUsage)

	if len(txInfo.Logs) > 0 {
		for _, tronLog := range txInfo.Logs {
			logData, _ := hex.DecodeString(tronLog.Data)
			tx.Logs = append(tx.Logs, &pb.TransactionLog{
				Address:         HexToBase58("41" + tronLog.Address),
				Topics:          tronLog.Topics,
				Data:            logData,
				TransactionHash: tx.TxHash,
			})
		}

		transfers := t.ParseTRC20TransferFromLogs(txInfo.Logs)
		if len(transfers) > 0 {
			for _, transfer := range transfers {
				if tx.ToAddress == "" || tx.ToAddress == tx.ContractAddress {
					tx.ToAddress = transfer.To
				}
				tx.ContractAddress = HexToBase58("41" + transfer.ContractAddress)
			}
		}
	}

	return tx, nil
}

type tronGridTxInfoResp struct {
	Result  string `json:"result"` // "SUCCESS" / "FAILED"
	Receipt struct {
		Result           string `json:"result"` // "SUCCESS" / "REVERT"
		EnergyUsage      int64  `json:"energy_usage"`
		EnergyUsageTotal int64  `json:"energy_usage_total"`
		NetUsage         int64  `json:"net_usage"`
	} `json:"receipt"`
	Fee int64 `json:"fee"` // in SUN
}

func (t *TronProvider) tryEnrichFromTronGrid(ctx context.Context, tx *pb.Transaction) bool {
	if t == nil || tx == nil || tx.TxHash == "" {
		return false
	}

	// NOTE: This is best-effort and should never break ingestion.
	// TronGrid may rate-limit; keep timeouts short and degrade gracefully.
	callCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	body := []byte(fmt.Sprintf(`{"value":"%s"}`, tx.TxHash))
	req, err := http.NewRequestWithContext(callCtx, http.MethodPost, "https://api.trongrid.io/wallet/gettransactioninfobyid", bytes.NewReader(body))
	if err != nil {
		return false
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.ReadAll(io.LimitReader(resp.Body, 1024))
		return false
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return false
	}

	var info tronGridTxInfoResp
	if err := json.Unmarshal(raw, &info); err != nil {
		return false
	}

	receiptResult := strings.ToUpper(strings.TrimSpace(info.Receipt.Result))
	switch receiptResult {
	case "SUCCESS":
		tx.Status = pb.TransactionStatus_TRANSACTION_STATUS_CONFIRMED
	case "REVERT":
		tx.Status = pb.TransactionStatus_TRANSACTION_STATUS_FAILED
	default:
		// Fallback to top-level result if receipt.result missing.
		top := strings.ToUpper(strings.TrimSpace(info.Result))
		if top == "SUCCESS" {
			tx.Status = pb.TransactionStatus_TRANSACTION_STATUS_CONFIRMED
		} else if top == "FAILED" {
			tx.Status = pb.TransactionStatus_TRANSACTION_STATUS_FAILED
		} else if tx.Status != pb.TransactionStatus_TRANSACTION_STATUS_FAILED {
			tx.Status = pb.TransactionStatus_TRANSACTION_STATUS_PENDING
		}
	}

	if info.Fee > 0 {
		tx.GasFee = units.FormatTokenAmount(strconv.FormatInt(info.Fee, 10), 6)
	} else if strings.TrimSpace(tx.GasFee) == "" {
		tx.GasFee = "0"
	}

	energy := info.Receipt.EnergyUsageTotal
	if energy <= 0 {
		energy = info.Receipt.EnergyUsage
	}
	if energy > 0 {
		tx.EnergyUsed = uint64(energy)
	}
	if info.Receipt.NetUsage > 0 {
		tx.BandwidthUsed = uint64(info.Receipt.NetUsage)
	}

	logx.Debugf("TronGrid enriched tx %s: status=%v fee=%s energy=%d net=%d",
		tx.TxHash, tx.Status, tx.GasFee, tx.EnergyUsed, tx.BandwidthUsed,
	)
	return true
}
