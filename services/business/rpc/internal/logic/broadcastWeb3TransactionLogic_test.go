package logic

import (
	"encoding/hex"
	"math/big"
	"testing"

	tronAddress "github.com/fbsobreira/gotron-sdk/pkg/address"
	core "github.com/fbsobreira/gotron-sdk/pkg/proto/core"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

func TestParseTransaction_TronTransfer(t *testing.T) {
	l := &BroadcastWeb3TransactionLogic{}

	from := "TFBnLx8PKnSZivhZZfcGMFCUiCUANhPcjE"
	to := "TZ2QdQfPChGtsy8Y1bFy4eA8J25nZSHYL6"

	fromAddr, err := tronAddress.Base58ToAddress(from)
	if err != nil {
		t.Fatalf("failed to parse from address: %v", err)
	}
	toAddr, err := tronAddress.Base58ToAddress(to)
	if err != nil {
		t.Fatalf("failed to parse to address: %v", err)
	}

	transfer := &core.TransferContract{
		OwnerAddress: fromAddr.Bytes(),
		ToAddress:    toAddr.Bytes(),
		Amount:       10_000_000,
	}
	param, err := anypb.New(transfer)
	if err != nil {
		t.Fatalf("failed to build any: %v", err)
	}

	tx := &core.Transaction{
		RawData: &core.TransactionRaw{
			Contract: []*core.Transaction_Contract{
				{
					Type:      core.Transaction_Contract_TransferContract,
					Parameter: param,
				},
			},
		},
	}

	txBytes, err := proto.Marshal(tx)
	if err != nil {
		t.Fatalf("failed to marshal tx: %v", err)
	}

	info, err := l.parseTransaction(hex.EncodeToString(txBytes), "TRON")
	if err != nil {
		t.Fatalf("parseTransaction error: %v", err)
	}
	if info.FromAddress != from {
		t.Fatalf("from address mismatch: %s", info.FromAddress)
	}
	if info.ToAddress != to {
		t.Fatalf("to address mismatch: %s", info.ToAddress)
	}
	if info.Amount != "10000000" {
		t.Fatalf("amount mismatch: %s", info.Amount)
	}
	if info.AssetCode != "TRX" {
		t.Fatalf("asset code mismatch: %s", info.AssetCode)
	}
}

func TestParseTransaction_TronTrc20Transfer(t *testing.T) {
	l := &BroadcastWeb3TransactionLogic{}

	from := "TFBnLx8PKnSZivhZZfcGMFCUiCUANhPcjE"
	to := "TZ2QdQfPChGtsy8Y1bFy4eA8J25nZSHYL6"
	contract := "TXYZopYRdj2D9XRtbG411XZZ3kM5VkAeBf"

	fromAddr, err := tronAddress.Base58ToAddress(from)
	if err != nil {
		t.Fatalf("failed to parse from address: %v", err)
	}
	toAddr, err := tronAddress.Base58ToAddress(to)
	if err != nil {
		t.Fatalf("failed to parse to address: %v", err)
	}
	contractAddr, err := tronAddress.Base58ToAddress(contract)
	if err != nil {
		t.Fatalf("failed to parse contract address: %v", err)
	}

	amount := big.NewInt(12345)
	data := buildTrc20TransferData(toAddr.Bytes()[1:], amount)

	trigger := &core.TriggerSmartContract{
		OwnerAddress:    fromAddr.Bytes(),
		ContractAddress: contractAddr.Bytes(),
		Data:            data,
	}
	param, err := anypb.New(trigger)
	if err != nil {
		t.Fatalf("failed to build any: %v", err)
	}

	tx := &core.Transaction{
		RawData: &core.TransactionRaw{
			Contract: []*core.Transaction_Contract{
				{
					Type:      core.Transaction_Contract_TriggerSmartContract,
					Parameter: param,
				},
			},
		},
	}

	txBytes, err := proto.Marshal(tx)
	if err != nil {
		t.Fatalf("failed to marshal tx: %v", err)
	}

	info, err := l.parseTransaction(hex.EncodeToString(txBytes), "TRON")
	if err != nil {
		t.Fatalf("parseTransaction error: %v", err)
	}
	if info.FromAddress != from {
		t.Fatalf("from address mismatch: %s", info.FromAddress)
	}
	if info.ToAddress != to {
		t.Fatalf("to address mismatch: %s", info.ToAddress)
	}
	if info.Amount != "12345" {
		t.Fatalf("amount mismatch: %s", info.Amount)
	}
}

func buildTrc20TransferData(to20 []byte, amount *big.Int) []byte {
	data := make([]byte, 0, 4+32+32)
	data = append(data, []byte{0xa9, 0x05, 0x9c, 0xbb}...)

	toPadded := make([]byte, 32)
	copy(toPadded[12:], to20)
	data = append(data, toPadded...)

	amtPadded := make([]byte, 32)
	amtBytes := amount.Bytes()
	copy(amtPadded[32-len(amtBytes):], amtBytes)
	data = append(data, amtPadded...)

	return data
}
