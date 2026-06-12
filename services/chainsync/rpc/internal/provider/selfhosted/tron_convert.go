package selfhosted

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/zeromicro/go-zero/core/logx"

	"internalwallet/proto/pb"
	tronutil "internalwallet/services/chainsync/rpc/internal/provider/tron"
	"internalwallet/services/chainsync/rpc/internal/units"
)

// convertTronTransactionToPb converts parsed TRON tx data into pb.Transaction.
func convertTronTransactionToPb(
	txData *TronTransactionData,
	chain pb.BlockChainType,
	blockNumber uint64,
	blockHash string,
	blockTimestamp int64,
	txIndex int,
) *pb.Transaction {
	txStatus := pb.TransactionStatus_TRANSACTION_STATUS_PENDING
	if len(txData.Ret) > 0 {
		if txData.Ret[0].ContractRet == "SUCCESS" {
			txStatus = pb.TransactionStatus_TRANSACTION_STATUS_CONFIRMED
		} else {
			txStatus = pb.TransactionStatus_TRANSACTION_STATUS_FAILED
			logx.Infof("⚠️ TRON transaction %s failed with status: %s", txData.TxID, txData.Ret[0].ContractRet)
		}
	}

	pbTx := &pb.Transaction{
		Chain:            chain,
		TxHash:           txData.TxID,
		BlockNumber:      blockNumber,
		BlockHash:        blockHash,
		BlockTimestamp:   blockTimestamp,
		TransactionIndex: uint64(txIndex),
		Status:           txStatus,
		GasPrice:         "0",
		Nonce:            "0",
	}

	if len(txData.Contract) == 0 {
		return pbTx
	}

	contract := txData.Contract[0]
	value := contract.Parameter.Value

	switch contract.Type {
	case "TransferContract":
		parseTrxTransferContract(pbTx, &value)
	case "TransferAssetContract":
		parseTrc10TransferContract(pbTx, &value)
	case "TriggerSmartContract":
		parseSmartContractCall(pbTx, &value)
	default:
		if value.OwnerAddress != "" {
			pbTx.FromAddress = tronutil.HexToBase58(value.OwnerAddress)
		}
		if value.ToAddress != "" {
			pbTx.ToAddress = tronutil.HexToBase58(value.ToAddress)
		}
		logx.Debugf("Unknown TRON contract type: %s", contract.Type)
	}

	return pbTx
}

func parseTrxTransferContract(pbTx *pb.Transaction, value *TronValue) {
	if value.OwnerAddress != "" {
		pbTx.FromAddress = tronutil.HexToBase58(value.OwnerAddress)
	}
	if value.ToAddress != "" {
		pbTx.ToAddress = tronutil.HexToBase58(value.ToAddress)
	}
	if value.Amount > 0 {
		pbTx.Value = units.FormatTokenAmount(fmt.Sprintf("%d", value.Amount), 6)
	}
}

func parseTrc10TransferContract(pbTx *pb.Transaction, value *TronValue) {
	if value.OwnerAddress != "" {
		pbTx.FromAddress = tronutil.HexToBase58(value.OwnerAddress)
	}
	if value.ToAddress != "" {
		pbTx.ToAddress = tronutil.HexToBase58(value.ToAddress)
	}

	if value.Amount > 0 {
		pbTx.Value = units.FormatTokenAmount(fmt.Sprintf("%d", value.Amount), 0)
	}
}

func parseSmartContractCall(pbTx *pb.Transaction, value *TronValue) {
	if value.OwnerAddress != "" {
		pbTx.FromAddress = tronutil.HexToBase58(value.OwnerAddress)
	}
	if value.ContractAddress != "" {
		pbTx.ContractAddress = tronutil.HexToBase58(value.ContractAddress)
	}

	if value.Data != "" {
		parseTrc20TransferFromCallData(pbTx, value.Data)
	} else if value.CallValue > 0 {
		pbTx.Value = units.FormatTokenAmount(fmt.Sprintf("%d", value.CallValue), 6)
	} else {
		pbTx.Value = "0"
	}
}

func parseTrc20TransferFromCallData(pbTx *pb.Transaction, dataHex string) bool {
	if len(dataHex) < 8 || dataHex[:8] != "a9059cbb" {
		return false
	}

	if len(dataHex) >= 72 {
		toHex := dataHex[32:72]
		pbTx.ToAddress = tronutil.HexToBase58("41" + toHex)
	}

	if len(dataHex) >= 136 {
		amountHex := dataHex[72:136]
		amount := new(big.Int)
		amount.SetString(amountHex, 16)
		pbTx.Value = units.FormatTokenAmount(amount.String(), 6)
		return true
	}

	return false
}

func enrichTronTransactionWithLogs(pbTx *pb.Transaction, txInfo *TronRPCTransactionInfo) {
	if txInfo == nil || len(txInfo.Log) == 0 {
		return
	}

	pbTx.TxHash = txInfo.ID
	if txInfo.Result == "SUCCESS" {
		pbTx.Status = pb.TransactionStatus_TRANSACTION_STATUS_CONFIRMED
	} else if txInfo.Result == "REVERT" {
		pbTx.Status = pb.TransactionStatus_TRANSACTION_STATUS_FAILED
	}

	if txInfo.BlockNumber > 0 {
		pbTx.BlockNumber = uint64(txInfo.BlockNumber)
	}

	if txInfo.Fee > 0 {
		pbTx.GasFee = units.FormatTokenAmount(fmt.Sprintf("%d", txInfo.Fee), 6)
	} else {
		pbTx.GasFee = "0"
	}

	if txInfo.Receipt.EnergyUsageTotal > 0 {
		pbTx.GasUsed = uint64(txInfo.Receipt.EnergyUsageTotal)
	} else if txInfo.Receipt.EnergyUsage > 0 {
		pbTx.GasUsed = uint64(txInfo.Receipt.EnergyUsage)
	}

	for _, tronLog := range txInfo.Log {
		logData := []byte{}
		if tronLog.Data != "" {
			if decoded, err := hex.DecodeString(tronLog.Data); err == nil {
				logData = decoded
			}
		}

		pbTx.Logs = append(pbTx.Logs, &pb.TransactionLog{
			Address:         tronutil.HexToBase58("41" + tronLog.Address),
			Topics:          tronLog.Topics,
			Data:            logData,
			TransactionHash: pbTx.TxHash,
		})
	}
}

func updateTransactionWithTransferEvents(pbTx *pb.Transaction, transfers []*tronutil.TRC20Transfer) {
	if len(transfers) == 0 {
		return
	}

	firstTransfer := transfers[0]

	if pbTx.ToAddress == "" || pbTx.ToAddress == pbTx.ContractAddress {
		pbTx.ToAddress = firstTransfer.To
		pbTx.FromAddress = firstTransfer.From
	}

	if pbTx.ContractAddress == "" {
		pbTx.ContractAddress = tronutil.HexToBase58("41" + firstTransfer.ContractAddress)
	}

	logx.Infof("🏦 Detected %d TRC-20 Transfer(s) in transaction %s", len(transfers), pbTx.TxHash)
}

func parseTrc20TransfersFromLogs(logs []*TronEventLog) []*tronutil.TRC20Transfer {
	transfers := make([]*tronutil.TRC20Transfer, 0)
	transferEventSignature := "ddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"

	for _, log := range logs {
		if len(log.Topics) < 3 {
			continue
		}
		if strings.TrimPrefix(strings.ToLower(log.Topics[0]), "0x") != transferEventSignature {
			continue
		}

		transfer := &tronutil.TRC20Transfer{ContractAddress: log.Address}

		if len(log.Topics[1]) >= 40 {
			fromHex := log.Topics[1][len(log.Topics[1])-40:]
			transfer.From = tronutil.HexToBase58("41" + fromHex)
		}
		if len(log.Topics[2]) >= 40 {
			toHex := log.Topics[2][len(log.Topics[2])-40:]
			transfer.To = tronutil.HexToBase58("41" + toHex)
		}

		if log.Data != "" {
			amount := new(big.Int)
			amount.SetString(strings.TrimPrefix(log.Data, "0x"), 16)
			transfer.Amount = amount.String()
		}

		transfers = append(transfers, transfer)
	}

	return transfers
}

