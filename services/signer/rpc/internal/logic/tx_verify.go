package logic

import (
	"bytes"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	tronAddress "github.com/fbsobreira/gotron-sdk/pkg/address"
	tronCore "github.com/fbsobreira/gotron-sdk/pkg/proto/core"
	"google.golang.org/protobuf/proto"

	"internalwallet/proto/pb"
)

func validateConsolidationRawTx(req *pb.SignTransactionRequest, rawTxData []byte) error {
	if req == nil {
		return fmt.Errorf("request is nil")
	}
	chain := strings.ToUpper(strings.TrimSpace(req.Chain))
	switch chain {
	case "ETH", "BSC":
		return validateEvmConsolidationRawTx(req, rawTxData)
	case "TRON", "TRN":
		return validateTronConsolidationRawTx(req, rawTxData)
	default:
		return fmt.Errorf("unsupported chain: %s", chain)
	}
}

func validateEvmConsolidationRawTx(req *pb.SignTransactionRequest, rawTxData []byte) error {
	if strings.TrimSpace(req.ToAddress) == "" {
		return fmt.Errorf("to_address is required for consolidation signing")
	}
	if strings.TrimSpace(req.Amount) == "" {
		return fmt.Errorf("amount is required for consolidation signing")
	}
	if !common.IsHexAddress(req.ToAddress) {
		return fmt.Errorf("invalid to_address: %s", strings.TrimSpace(req.ToAddress))
	}
	amount, ok := new(big.Int).SetString(strings.TrimSpace(req.Amount), 10)
	if !ok || amount.Sign() < 0 {
		return fmt.Errorf("invalid amount: %s", strings.TrimSpace(req.Amount))
	}

	tx := new(types.Transaction)
	if err := tx.UnmarshalBinary(rawTxData); err != nil {
		return fmt.Errorf("unmarshal EVM tx: %w", err)
	}
	if tx.To() == nil {
		return fmt.Errorf("contract creation tx is not allowed for consolidation")
	}

	to := common.HexToAddress(strings.TrimSpace(req.ToAddress))
	tokenContract := strings.TrimSpace(req.TokenContract)

	// Native transfer.
	if tokenContract == "" {
		if len(tx.Data()) != 0 {
			return fmt.Errorf("unexpected calldata for native transfer")
		}
		if *tx.To() != to {
			return fmt.Errorf("tx.to mismatch: got=%s want=%s", tx.To().Hex(), to.Hex())
		}
		if tx.Value().Cmp(amount) != 0 {
			return fmt.Errorf("tx.value mismatch: got=%s want=%s", tx.Value().String(), amount.String())
		}
		return nil
	}

	// ERC20 transfer.
	if !common.IsHexAddress(tokenContract) {
		return fmt.Errorf("invalid token_contract: %s", tokenContract)
	}
	contractAddr := common.HexToAddress(tokenContract)
	if *tx.To() != contractAddr {
		return fmt.Errorf("tx.to (token contract) mismatch: got=%s want=%s", tx.To().Hex(), contractAddr.Hex())
	}
	if tx.Value().Sign() != 0 {
		return fmt.Errorf("tx.value must be 0 for token transfer, got=%s", tx.Value().String())
	}
	if err := validateErc20TransferCalldata(tx.Data(), to.Bytes(), amount); err != nil {
		return err
	}
	return nil
}

func validateTronConsolidationRawTx(req *pb.SignTransactionRequest, rawTxData []byte) error {
	if strings.TrimSpace(req.FromAddress) == "" {
		return fmt.Errorf("from_address is required for TRON consolidation signing")
	}
	if strings.TrimSpace(req.ToAddress) == "" {
		return fmt.Errorf("to_address is required for TRON consolidation signing")
	}
	if strings.TrimSpace(req.Amount) == "" {
		return fmt.Errorf("amount is required for TRON consolidation signing")
	}

	fromAddr, err := tronAddress.Base58ToAddress(strings.TrimSpace(req.FromAddress))
	if err != nil {
		return fmt.Errorf("invalid from_address: %v", err)
	}
	toAddr, err := tronAddress.Base58ToAddress(strings.TrimSpace(req.ToAddress))
	if err != nil {
		return fmt.Errorf("invalid to_address: %v", err)
	}
	amount, ok := new(big.Int).SetString(strings.TrimSpace(req.Amount), 10)
	if !ok || amount.Sign() <= 0 {
		return fmt.Errorf("invalid amount: %s", strings.TrimSpace(req.Amount))
	}

	tx := &tronCore.Transaction{}
	if err := proto.Unmarshal(rawTxData, tx); err != nil {
		return fmt.Errorf("unmarshal TRON tx: %w", err)
	}
	raw := tx.GetRawData()
	if raw == nil || len(raw.Contract) == 0 {
		return fmt.Errorf("TRON tx raw_data.contract is empty")
	}
	contract := raw.Contract[0]
	if contract == nil || contract.Parameter == nil {
		return fmt.Errorf("TRON tx contract parameter is missing")
	}

	tokenContract := strings.TrimSpace(req.TokenContract)
	if tokenContract == "" {
		if contract.Type != tronCore.Transaction_Contract_TransferContract {
			return fmt.Errorf("unexpected TRON contract type for native transfer: %v", contract.Type)
		}
		tc := &tronCore.TransferContract{}
		if err := contract.Parameter.UnmarshalTo(tc); err != nil {
			return fmt.Errorf("unmarshal TransferContract: %w", err)
		}
		if !bytes.Equal(tc.OwnerAddress, fromAddr.Bytes()) {
			return fmt.Errorf("owner_address mismatch")
		}
		if !bytes.Equal(tc.ToAddress, toAddr.Bytes()) {
			return fmt.Errorf("to_address mismatch")
		}
		if !amount.IsInt64() || tc.Amount != amount.Int64() {
			return fmt.Errorf("amount mismatch: got=%d want=%s", tc.Amount, amount.String())
		}
		return nil
	}

	contractAddr, err := tronAddress.Base58ToAddress(tokenContract)
	if err != nil {
		return fmt.Errorf("invalid token_contract: %v", err)
	}
	if contract.Type != tronCore.Transaction_Contract_TriggerSmartContract {
		return fmt.Errorf("unexpected TRON contract type for TRC20 transfer: %v", contract.Type)
	}
	sc := &tronCore.TriggerSmartContract{}
	if err := contract.Parameter.UnmarshalTo(sc); err != nil {
		return fmt.Errorf("unmarshal TriggerSmartContract: %w", err)
	}
	if !bytes.Equal(sc.OwnerAddress, fromAddr.Bytes()) {
		return fmt.Errorf("owner_address mismatch")
	}
	if !bytes.Equal(sc.ContractAddress, contractAddr.Bytes()) {
		return fmt.Errorf("contract_address mismatch")
	}
	toEvm20 := toAddr.Bytes()
	if len(toEvm20) != 21 {
		return fmt.Errorf("unexpected TRON to_address length: %d", len(toEvm20))
	}
	if err := validateErc20TransferCalldata(sc.Data, toEvm20[1:], amount); err != nil {
		return err
	}
	return nil
}

func validateErc20TransferCalldata(data []byte, to20 []byte, amount *big.Int) error {
	if amount == nil {
		return fmt.Errorf("amount is nil")
	}
	if len(to20) != 20 {
		return fmt.Errorf("invalid to address length: %d", len(to20))
	}
	if len(data) < 4+32+32 {
		return fmt.Errorf("invalid ERC20 calldata length: %d", len(data))
	}
	// transfer(address,uint256) selector = 0xa9059cbb
	if !bytes.Equal(data[:4], []byte{0xa9, 0x05, 0x9c, 0xbb}) {
		return fmt.Errorf("invalid ERC20 calldata selector")
	}

	toWord := data[4 : 4+32]
	if !bytes.Equal(toWord[12:], to20) {
		return fmt.Errorf("ERC20 transfer(to) mismatch")
	}

	amtWord := data[4+32 : 4+32+32]
	amt := new(big.Int).SetBytes(amtWord)
	if amt.Cmp(amount) != 0 {
		return fmt.Errorf("ERC20 transfer(amount) mismatch: got=%s want=%s", amt.String(), amount.String())
	}
	return nil
}
