package selfhosted

import (
	"context"
	"encoding/json"
	"math/big"
	"strconv"

	"github.com/ethereum/go-ethereum/common"

	"internalwallet/proto/pb"
	"internalwallet/services/chainsync/rpc/internal/units"
)

func (p *SelfHostedProvider) parseEthereumTransaction(tx map[string]interface{}, transactionIndex uint32, block *pb.ChainBlockInfo) *pb.Transaction {
	hash, ok := tx["hash"].(string)
	if !ok {
		return nil
	}

	fromRaw, _ := tx["from"].(string)
	toRaw, _ := tx["to"].(string)
	valueHex, _ := tx["value"].(string)
	gasPriceHex, _ := tx["gasPrice"].(string)

	var from, to string
	if fromRaw != "" {
		from = common.HexToAddress(fromRaw).String()
	}
	if toRaw != "" {
		to = common.HexToAddress(toRaw).String()
	}

	var (
		gasUsed        uint64
		gasFee         string
		blockTimestamp = block.Timestamp
		// Default to pending: being included in a block does NOT imply EVM execution success.
		txStatus = pb.TransactionStatus_TRANSACTION_STATUS_PENDING
	)

	receiptResp, err := p.callRPC(context.Background(), "eth_getTransactionReceipt", []interface{}{hash})
	if err == nil {
		var receipt map[string]interface{}
		if json.Unmarshal(receiptResp.Result, &receipt) == nil {
			// Set status from receipt.status when available (0x1 success / 0x0 revert).
			// Without receipt.status, being "in a block" does NOT mean success for EVM.
			if s, ok := receipt["status"].(string); ok && s != "" {
				if s == "0x1" {
					txStatus = pb.TransactionStatus_TRANSACTION_STATUS_CONFIRMED
				} else if s == "0x0" {
					txStatus = pb.TransactionStatus_TRANSACTION_STATUS_FAILED
				}
			}
			if gasUsedStr, ok := receipt["gasUsed"].(string); ok {
				if parsed, parseErr := strconv.ParseUint(gasUsedStr[2:], 16, 64); parseErr == nil {
					gasUsed = parsed
				}
			}
			if effectiveGasPrice, ok := receipt["effectiveGasPrice"].(string); ok && effectiveGasPrice != "" {
				gasPriceHex = effectiveGasPrice
			}
		}
	}

	formattedValue := valueHex
	if valueHex != "" {
		valueBigInt := parseHexToBigInt(valueHex)
		if valueBigInt != nil {
			formattedValue = units.FormatTokenAmount(valueBigInt.String(), 18)
		}
	}

	formattedGasPrice := gasPriceHex
	if gasPriceHex != "" {
		gasPriceBigInt := parseHexToBigInt(gasPriceHex)
		if gasPriceBigInt != nil {
			formattedGasPrice = units.FormatTokenAmount(gasPriceBigInt.String(), 9)
		}
	}

	if gasPriceHex != "" && gasUsed > 0 {
		gasPriceInt := parseHexToBigInt(gasPriceHex)
		if gasPriceInt != nil {
			gasUsedInt := new(big.Int).SetUint64(gasUsed)
			gasFeeInt := new(big.Int).Mul(gasPriceInt, gasUsedInt)
			gasFee = units.FormatTokenAmount(gasFeeInt.String(), 18)
		}
	}

	return &pb.Transaction{
		TxHash:           hash,
		Chain:            p.config.Chain,
		BlockNumber:      block.BlockNumber,
		BlockHash:        block.BlockHash,
		BlockTimestamp:   blockTimestamp,
		FromAddress:      from,
		ToAddress:        to,
		Value:            formattedValue,
		GasPrice:         formattedGasPrice,
		GasUsed:          gasUsed,
		GasFee:           gasFee,
		TransactionIndex: uint64(transactionIndex),
		Status:           txStatus,
	}
}

