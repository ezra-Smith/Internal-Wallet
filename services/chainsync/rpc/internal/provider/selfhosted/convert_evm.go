package selfhosted

import (
	"encoding/hex"
	"math/big"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/zeromicro/go-zero/core/logx"

	"internalwallet/proto/pb"
	"internalwallet/services/chainsync/rpc/internal/units"
)

func (p *SelfHostedProvider) convertRPCTransactionToPbTransaction(tx map[string]interface{}, blockNumber uint64, blockTimestamp int64) *pb.Transaction {
	pbTx := &pb.Transaction{
		Chain:          p.config.Chain,
		BlockNumber:    blockNumber,
		BlockTimestamp: blockTimestamp,
		Status:         pb.TransactionStatus_TRANSACTION_STATUS_PENDING,
	}

	if hash, ok := tx["hash"].(string); ok {
		pbTx.TxHash = hash
	}

	if from, ok := tx["from"].(string); ok {
		fromAddr := common.HexToAddress(from)
		pbTx.FromAddress = fromAddr.String()
	}

	toAddr := ""
	if to, ok := tx["to"].(string); ok {
		toAddrObj := common.HexToAddress(to)
		toAddr = toAddrObj.String()
		pbTx.ToAddress = toAddr
	}

	if valueHex, ok := tx["value"].(string); ok && valueHex != "" && valueHex != "0x0" {
		valueBigInt := parseHexToBigInt(valueHex)
		if valueBigInt != nil {
			pbTx.Value = units.FormatTokenAmount(valueBigInt.String(), 18)
		}
	} else {
		pbTx.Value = "0"
	}

	if gasHex, ok := tx["gas"].(string); ok {
		gasBigInt := parseHexToBigInt(gasHex)
		if gasBigInt != nil {
			pbTx.GasLimit = gasBigInt.String()
		}
	}

	if gasPriceHex, ok := tx["gasPrice"].(string); ok {
		gasPriceBigInt := parseHexToBigInt(gasPriceHex)
		if gasPriceBigInt != nil {
			pbTx.GasPrice = units.FormatTokenAmount(gasPriceBigInt.String(), 9)
		}
	}

	if nonceHex, ok := tx["nonce"].(string); ok {
		if nonceInt64, err := strconv.ParseInt(nonceHex, 0, 64); err == nil {
			pbTx.Nonce = strconv.FormatInt(nonceInt64, 10)
		}
	}

	if txIndexHex, ok := tx["transactionIndex"].(string); ok {
		if txIndexUint64, err := strconv.ParseUint(txIndexHex, 0, 64); err == nil {
			pbTx.TransactionIndex = txIndexUint64
		}
	}

	if blockHash, ok := tx["blockHash"].(string); ok {
		pbTx.BlockHash = blockHash
	}

	if input, ok := tx["input"].(string); ok && len(input) >= 10 {
		inputHex := input
		if strings.HasPrefix(inputHex, "0x") {
			inputHex = inputHex[2:]
		}
		if inputHex != "" {
			if decoded, err := hex.DecodeString(inputHex); err == nil {
				pbTx.InputData = decoded
			}
		}

		if strings.HasPrefix(input, "0xa9059cbb") && len(input) >= 138 {
			pbTx.ContractAddress = toAddr

			toParamHex := input[10:74]
			actualAddrHex := toParamHex[len(toParamHex)-40:]
			actualAddr := common.HexToAddress(actualAddrHex)
			pbTx.ToAddress = actualAddr.String()

			amountHex := input[74:138]
			amount := new(big.Int)
			amount.SetString(amountHex, 16)
			pbTx.Value = amount.String()

			logx.Infof("💰 [ERC20] Transfer from %s to %s, amount: %s, contract: %s",
				pbTx.FromAddress, pbTx.ToAddress, pbTx.Value, pbTx.ContractAddress)
		}
	}

	return pbTx
}

