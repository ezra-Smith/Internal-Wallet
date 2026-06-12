package selfhosted

import (
	"encoding/hex"
	"math/big"
	"strconv"

	"github.com/zeromicro/go-zero/core/logx"

	"internalwallet/proto/pb"
	tronutil "internalwallet/services/chainsync/rpc/internal/provider/tron"
	"internalwallet/services/chainsync/rpc/internal/units"
)

func (p *SelfHostedProvider) parseTronTransaction(tx *TronTransaction, block *pb.ChainBlockInfo, transactionIndex uint32) *pb.Transaction {
	if len(tx.RawData.Contract) == 0 {
		return nil
	}

	txStatus := pb.TransactionStatus_TRANSACTION_STATUS_PENDING
	if len(tx.Ret) > 0 {
		if tx.Ret[0].Ret == 0 {
			txStatus = pb.TransactionStatus_TRANSACTION_STATUS_CONFIRMED
		} else {
			txStatus = pb.TransactionStatus_TRANSACTION_STATUS_FAILED
			logx.Infof("⚠️ TRON transaction %s failed with ret code: %d", tx.TxID, tx.Ret[0].Ret)
		}
	} else {
		// Conservative: if no ret info is present, we cannot conclude execution success.
		// Treat as pending to avoid false-positive "confirmed" for reverted contract calls.
		txStatus = pb.TransactionStatus_TRANSACTION_STATUS_PENDING
	}

	transaction := &pb.Transaction{
		TxHash:           tx.TxID,
		Chain:            p.config.Chain,
		BlockNumber:      block.BlockNumber,
		BlockHash:        block.BlockHash,
		BlockTimestamp:   block.Timestamp,
		TransactionIndex: uint64(transactionIndex),
		Status:           txStatus,
	}

	contract := tx.RawData.Contract[0]
	fromAddr := tronutil.HexToBase58(contract.Parameter.Value.OwnerAddress)
	toAddr := tronutil.HexToBase58(contract.Parameter.Value.ToAddress)

	transaction.FromAddress = fromAddr
	transaction.ToAddress = toAddr

	switch contract.Type {
	case "TransferContract":
		amount := strconv.FormatInt(contract.Parameter.Value.Amount, 10)
		transaction.Value = units.FormatTokenAmount(amount, 6)
	case "TransferAssetContract":
		amount := strconv.FormatInt(contract.Parameter.Value.Amount, 10)
		transaction.Value = units.FormatTokenAmount(amount, 6)
	case "TriggerSmartContract":
		contractAddr := tronutil.HexToBase58(contract.Parameter.Value.ContractAddress)
		transaction.ContractAddress = contractAddr
		if transaction.ToAddress == "" && contractAddr != "" {
			// Keep ToAddress as contract by default for contract calls; TRC20 transfer below will override it to the real recipient.
			transaction.ToAddress = contractAddr
		}

		if contract.Parameter.Value.Data != "" {
			data := contract.Parameter.Value.Data

			// Preserve original call data for token parsing (TRC20: transfer(address,uint256)).
			if b, err := hex.DecodeString(data); err == nil && len(b) > 0 {
				transaction.InputData = b
			}

			if len(data) >= 8 && data[:8] == "a9059cbb" {
				if len(data) >= 136 {
					toHex := data[32:72]
					if len(toHex) == 40 {
						actualToHex := "41" + toHex
						actualToAddr := tronutil.HexToBase58(actualToHex)

						amountHex := data[72:136]
						amount := new(big.Int)
						amount.SetString(amountHex, 16)

						transaction.ToAddress = actualToAddr
						// Contract call value (TRX) should not be treated as token amount.
						if contract.Parameter.Value.CallValue > 0 {
							transaction.Value = units.FormatTokenAmount(strconv.FormatInt(contract.Parameter.Value.CallValue, 10), 6)
						} else {
							transaction.Value = "0"
						}
					}
				}
			} else {
				if contract.Parameter.Value.CallValue > 0 {
					callValue := strconv.FormatInt(contract.Parameter.Value.CallValue, 10)
					transaction.Value = units.FormatTokenAmount(callValue, 6)
				} else {
					transaction.Value = "0"
				}
			}
		} else {
			if contract.Parameter.Value.CallValue > 0 {
				callValue := strconv.FormatInt(contract.Parameter.Value.CallValue, 10)
				transaction.Value = units.FormatTokenAmount(callValue, 6)
			} else {
				transaction.Value = "0"
			}
		}
	default:
		amount := strconv.FormatInt(contract.Parameter.Value.Amount, 10)
		transaction.Value = units.FormatTokenAmount(amount, 6)
		logx.Debugf("Unknown contract type: %s", contract.Type)
	}

	return transaction
}
