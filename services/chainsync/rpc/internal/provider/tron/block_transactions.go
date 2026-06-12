package tron

import (
	"context"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/fbsobreira/gotron-sdk/pkg/common"
	"github.com/zeromicro/go-zero/core/logx"

	"internalwallet/proto/pb"
	"internalwallet/services/chainsync/rpc/internal/units"
)

func (t *TronProvider) GetBlockTransactions(ctx context.Context, blockNumber uint64) ([]*pb.Transaction, error) {
	start := time.Now()
	var err error
	defer t.finishRequest(start, &err)

	var lastErr error
	maxRetries := 3

	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			logx.Infof("TRON GetBlockTransactions retry %d/%d", i, maxRetries-1)
			time.Sleep(time.Duration(i) * time.Second)
		}

		grpcClient := t.createClient()
		if err := t.connectWithTimeout(ctx, grpcClient); err != nil {
			grpcClient.Stop()
			lastErr = fmt.Errorf("connection failed: %w", err)
			continue
		}

		block, err := grpcClient.GetBlockByNum(int64(blockNumber))
		grpcClient.Stop()

		if err != nil {
			lastErr = fmt.Errorf("get block by number failed: %w", err)
			continue
		}

		transactions := make([]*pb.Transaction, 0)
		if block == nil || len(block.Transactions) == 0 {
			logx.Infof("📦 No transactions found in TRON block %d", blockNumber)
			return transactions, nil
		}

		var blockHash string
		if len(block.Blockid) == 32 {
			blockHash = hex.EncodeToString(block.Blockid)
		} else if block.BlockHeader != nil {
			blockHash = t.calculateTronBlockIDFromHeader(block.BlockHeader)
		}

		blockTimestamp := block.BlockHeader.RawData.Timestamp
		if blockTimestamp > 1e12 {
			blockTimestamp = blockTimestamp / 1000
		}

		for txIndex := range block.Transactions {
			txID := common.Bytes2Hex(block.Transactions[txIndex].Txid)

			transaction := &pb.Transaction{
				TxHash:           txID,
				Chain:            pb.BlockChainType_CHAIN_TYPE_TRON,
				BlockNumber:      blockNumber,
				BlockHash:        blockHash,
				BlockTimestamp:   blockTimestamp,
				TransactionIndex: uint64(txIndex),
				Value:            "0",
				GasPrice:         "0",
				GasUsed:          0,
				// IMPORTANT: do not leave empty, downstream treats empty as "unknown" and may store NULL.
				// For TRON, fee can legitimately be 0 (bandwidth/energy covered). Persist as "0".
				GasFee:           "0",
				// Conservative default: do not assume success without ret/txInfo.
				Status:           pb.TransactionStatus_TRANSACTION_STATUS_PENDING,
				Nonce:            "0",
				InputData:        []byte{},
			}

			txExt := block.Transactions[txIndex]
			if txExt == nil || txExt.Transaction == nil || txExt.Transaction.RawData == nil {
				transactions = append(transactions, transaction)
				continue
			}

			txRawData := txExt.Transaction.RawData
			if len(txRawData.Contract) > 0 {
				contract := txRawData.Contract[0]
				transaction.InputData = contract.Parameter.Value

				switch contract.Type {
				case 1: // TransferContract (TRX)
					fromAddr, toAddr, amount := ParseTRXTransfer(contract.Parameter.Value)
					if fromAddr != "" {
						transaction.FromAddress = HexToBase58(fromAddr)
					}
					if toAddr != "" {
						transaction.ToAddress = HexToBase58(toAddr)
					}
					if amount != "" {
						transaction.Value = units.FormatTokenAmount(amount, 6)
					}
				case 2: // TransferAssetContract (TRC-10)
					fromAddr, toAddr, amount, _ := ParseTRC10Transfer(contract.Parameter.Value)
					if fromAddr != "" {
						transaction.FromAddress = HexToBase58(fromAddr)
					}
					if toAddr != "" {
						transaction.ToAddress = HexToBase58(toAddr)
					}
					if amount != "" {
						transaction.Value = units.FormatTokenAmount(amount, 6)
					}
				case 31: // TriggerSmartContract (TRC-20 / contract call)
					fromAddr, contractAddr, callValue, method, tokenValue, actualToAddr := ParseSmartContract(contract.Parameter.Value)
					if fromAddr != "" {
						transaction.FromAddress = HexToBase58(fromAddr)
					}

					contractBase58 := HexToBase58(contractAddr)
					transaction.ContractAddress = contractBase58
					transaction.ToAddress = contractBase58

					if tokenValue != "" && method == "transfer" && actualToAddr != "" {
						transaction.ToAddress = HexToBase58(actualToAddr)
						transaction.Value = units.FormatTokenAmount(tokenValue, 6)
					} else if callValue != "" {
						transaction.Value = units.FormatTokenAmount(callValue, 6)
					} else {
						transaction.Value = "0"
					}
				default:
					logx.Debugf("Contract type %d found in tx %s", contract.Type, txID)
				}
			}

			if len(txExt.Transaction.Ret) > 0 {
				ret := txExt.Transaction.Ret[0]
				if ret.Ret == 0 {
					transaction.Status = pb.TransactionStatus_TRANSACTION_STATUS_CONFIRMED
				} else {
					transaction.Status = pb.TransactionStatus_TRANSACTION_STATUS_FAILED
				}

				if ret.Fee > 0 {
					// ret.Fee is TRON fee in SUN (not "gas used").
					// Keep GasUsed=0 so downstream can still fetch txInfo (GetTransactionInfoByID)
					// and overwrite GasFee with the authoritative total fee (including "other fees"
					// like account activation), when available.
					transaction.GasFee = units.FormatTokenAmount(strconv.FormatInt(ret.Fee, 10), 6)
				} else if len(txRawData.Contract) > 0 && txRawData.Contract[0].Type == 31 && txRawData.FeeLimit > 0 {
					// FeeLimit is a cap, not actual consumed fee. Keep as-is for best-effort display,
					// but still allow downstream to fetch txInfo for authoritative fee.
					transaction.GasFee = units.FormatTokenAmount(strconv.FormatInt(txRawData.FeeLimit, 10), 6)
				}
			}

			transactions = append(transactions, transaction)
		}

		return transactions, nil
	}

	err = lastErr
	return nil, err
}
