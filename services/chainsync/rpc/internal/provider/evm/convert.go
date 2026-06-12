package evm

import (
	"fmt"
	"math/big"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/zeromicro/go-zero/core/logx"

	"internalwallet/proto/pb"
	"internalwallet/services/chainsync/rpc/internal/units"
)

// convertEthTransactionToPbTransaction converts an EVM transaction into pb.Transaction.
// GasUsed/GasFee/status should be filled from receipt by the caller when needed.
func (p *Web3Provider) convertEthTransactionToPbTransaction(tx *types.Transaction, blockNumber uint64, timestamp int64, blockHash string) *pb.Transaction {
	if tx == nil {
		return nil
	}

	fromAddr := p.recoverSenderAddress(tx)

	var toAddr string
	if tx.To() != nil {
		toAddr = tx.To().String()
	}

	// NOTE: tx.Value() is in wei. For native transfers we store a human decimal string
	// (e.g. "1.23") to keep downstream (token parser / deposit confirm) consistent.
	// Token transfers will be parsed from logs/input_data and should not rely on this field.
	nativeValue := units.FormatTokenAmount(tx.Value().String(), 18)
	gasPrice := tx.GasPrice().String()
	nonce := tx.Nonce()
	gasLimit := tx.Gas()

	txType := "native"
	var tokenAddress, tokenValue string
	input := tx.Data()

	// Detect ERC20 transfer(address,uint256)
	if len(input) >= 68 && toAddr != "" {
		methodID := fmt.Sprintf("%x", input[:4])
		if methodID == ERC20TransferMethodID {
			txType = "erc20"
			tokenAddress = toAddr

			if len(input) >= 36 {
				toAddrBytes := input[4:36]
				toAddr = common.BytesToAddress(toAddrBytes).String()
			}

			if len(input) >= 68 {
				amountBytes := input[36:68]
				amount := new(big.Int).SetBytes(amountBytes)
				tokenValue = amount.String()
			}

			logx.Infof("🪙 [%s] ERC20 Transfer detected: contract=%s, to=%s, amount=%s",
				tx.Hash().Hex(), tokenAddress, toAddr, tokenValue)
		}
	}

	txValue := nativeValue
	if txType == "erc20" && tokenValue != "" {
		txValue = tokenValue
		logx.Infof("🪙 [%s] Using token value: %s (native value: %s)", tx.Hash().Hex(), txValue, nativeValue)
	}

	pbTx := &pb.Transaction{
		TxHash:         tx.Hash().Hex(),
		Chain:          p.chain,
		BlockNumber:    blockNumber,
		BlockHash:      blockHash,
		BlockTimestamp: timestamp,
		FromAddress:    fromAddr,
		ToAddress:      toAddr,
		Value:          txValue,
		GasPrice:       gasPrice,
		GasLimit:       strconv.FormatUint(gasLimit, 10),
		Nonce:          fmt.Sprintf("%d", nonce),
		Status:         pb.TransactionStatus_TRANSACTION_STATUS_PENDING,
		InputData:      input,
	}

	if txType == "erc20" {
		pbTx.ContractAddress = tokenAddress
	}

	return pbTx
}

func (p *Web3Provider) recoverSenderAddress(tx *types.Transaction) string {
	txHash := tx.Hash().Hex()
	txType := tx.Type()

	var signer types.Signer
	switch txType {
	case types.LegacyTxType:
		if p.chainID != nil && p.chainID.Sign() > 0 {
			signer = types.NewEIP155Signer(p.chainID)
		} else {
			signer = types.HomesteadSigner{}
		}
	case types.AccessListTxType:
		signer = types.NewEIP2930Signer(p.chainID)
	case types.DynamicFeeTxType:
		signer = types.NewLondonSigner(p.chainID)
	default:
		signer = types.LatestSignerForChainID(p.chainID)
	}

	sender, err := types.Sender(signer, tx)
	if err == nil {
		return sender.String()
	}

	logx.Debugf("Primary signer failed for tx %s (type=%d): %v, trying fallback", txHash, txType, err)
	return p.recoverSenderFallback(tx)
}

func (p *Web3Provider) recoverSenderFallback(tx *types.Transaction) string {
	txHash := tx.Hash().Hex()

	var signers []types.Signer
	if p.chainID != nil && p.chainID.Sign() > 0 {
		signers = []types.Signer{
			types.NewEIP155Signer(p.chainID),
			types.NewLondonSigner(p.chainID),
			types.NewEIP2930Signer(p.chainID),
			types.LatestSignerForChainID(p.chainID),
			types.HomesteadSigner{},
			types.FrontierSigner{},
		}
	} else {
		signers = []types.Signer{
			types.HomesteadSigner{},
			types.FrontierSigner{},
		}
	}

	for i, signer := range signers {
		sender, err := types.Sender(signer, tx)
		if err == nil {
			if i > 0 {
				logx.Infof("✅ Recovered sender for tx %s using fallback signer #%d: %s", txHash, i, sender.String())
			}
			return sender.String()
		}
	}

	logx.Errorf("❌ Failed to recover sender for tx %s (type=%d, chainID=%v) with all %d signer types",
		txHash, tx.Type(), p.chainID, len(signers))
	return ""
}
