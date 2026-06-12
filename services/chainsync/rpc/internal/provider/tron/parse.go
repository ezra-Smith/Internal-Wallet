package tron

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/fbsobreira/gotron-sdk/pkg/address"
	"github.com/fbsobreira/gotron-sdk/pkg/common"
	"github.com/fbsobreira/gotron-sdk/pkg/proto/core"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/protobuf/proto"
)

func HexToBase58(hexAddr string) string {
	if hexAddr == "" {
		return ""
	}

	if strings.HasPrefix(hexAddr, "0x") || strings.HasPrefix(hexAddr, "0X") {
		hexAddr = hexAddr[2:]
	}

	if !strings.HasPrefix(hexAddr, "41") {
		addr := address.HexToAddress(hexAddr)
		result := addr.String()
		if result != "" {
			return result
		}
		return ""
	}

	if len(hexAddr) < 40 {
		return ""
	}

	if len(hexAddr) > 42 {
		hexAddr = hexAddr[:42]
	} else if len(hexAddr) == 40 {
		hexAddr = "41" + hexAddr
	}

	addr := address.HexToAddress(hexAddr)
	return addr.String()
}

func ParseTRXTransfer(data []byte) (fromAddr, toAddr, amount string) {
	var transferContract core.TransferContract
	if err := proto.Unmarshal(data, &transferContract); err != nil {
		logx.Errorf("ParseTRXTransfer: failed to unmarshal protobuf: %v", err)
		return parseTRXTransferLegacy(data)
	}

	if len(transferContract.OwnerAddress) > 0 {
		fromAddr = common.Bytes2Hex(transferContract.OwnerAddress)
	}
	if len(transferContract.ToAddress) > 0 {
		toAddr = common.Bytes2Hex(transferContract.ToAddress)
	}
	if transferContract.Amount > 0 {
		amt := new(big.Int).SetInt64(int64(transferContract.Amount))
		amount = amt.String()
	}

	logx.Infof("ParseTRXTransfer: protobuf decoded - from: %s, to: %s, amount: %s", fromAddr, toAddr, amount)
	return
}

func parseTRXTransferLegacy(data []byte) (fromAddr, toAddr, amount string) {
	if len(data) < 72 {
		return
	}

	if len(data) >= 21 {
		fromAddr = common.Bytes2Hex(data[:21])
	}
	if len(data) >= 53 {
		toAddr = common.Bytes2Hex(data[32:53])
	}
	if len(data) >= 72 {
		amountBytes := data[64:72]
		amt := new(big.Int)
		amt.SetBytes(amountBytes)
		amount = amt.String()
	}
	return
}

func ParseTRC10Transfer(data []byte) (fromAddr, toAddr, amount, tokenId string) {
	if len(data) < 76 {
		return
	}

	if len(data) >= 21 {
		fromAddr = common.Bytes2Hex(data[:21])
	}
	if len(data) >= 53 {
		toAddr = common.Bytes2Hex(data[32:53])
	}
	if len(data) >= 72 {
		amountBytes := data[64:72]
		amt := new(big.Int)
		amt.SetBytes(amountBytes)
		amount = amt.String()
	}
	if len(data) >= 76 {
		tokenIdBytes := data[72:76]
		tokenIdInt := new(big.Int)
		tokenIdInt.SetBytes(tokenIdBytes)
		tokenId = tokenIdInt.String()
	}

	return
}

// ParseSmartContract parses TriggerSmartContract parameters.
// Returns hex addresses (without 0x prefix), and tokenValue as base-unit integer string when detected.
func ParseSmartContract(data []byte) (fromAddr, contractAddr, value, method, tokenValue, actualToAddr string) {
	if len(data) < 72 {
		logx.Errorf("ParseSmartContract: insufficient data length: %d, minimum 72", len(data))
		return
	}

	var triggerContract core.TriggerSmartContract
	if err := proto.Unmarshal(data, &triggerContract); err != nil {
		logx.Errorf("ParseSmartContract: failed to unmarshal protobuf: %v", err)
		return ParseSmartContractLegacy(data)
	}

	if len(triggerContract.OwnerAddress) > 0 {
		fromAddr = common.Bytes2Hex(triggerContract.OwnerAddress)
	}

	if len(triggerContract.ContractAddress) > 0 {
		contractAddr = common.Bytes2Hex(triggerContract.ContractAddress)
	}

	if triggerContract.CallValue > 0 {
		val := new(big.Int).SetInt64(int64(triggerContract.CallValue))
		value = val.String()
	}

	if len(triggerContract.Data) >= 4 {
		methodBytes := triggerContract.Data[:4]
		methodSig := fmt.Sprintf("%02x%02x%02x%02x", methodBytes[0], methodBytes[1], methodBytes[2], methodBytes[3])

		if methodSig == "a9059cbb" {
			method = "transfer"

			if len(triggerContract.Data) >= 68 {
				toBytes := triggerContract.Data[16:36]
				if len(toBytes) >= 20 {
					actualToBytes := toBytes[len(toBytes)-20:]
					if len(actualToBytes) == 20 {
						tokenToHex := common.Bytes2Hex(actualToBytes)
						actualToAddr = "41" + tokenToHex
					}
				}

				amtBytes := triggerContract.Data[36:68]
				amt := new(big.Int)
				amt.SetBytes(amtBytes)
				tokenValue = amt.String()
			}
		} else {
			method = fmt.Sprintf("0x%s", methodSig)
		}
	}

	return
}

func ParseSmartContractLegacy(data []byte) (fromAddr, contractAddr, value, method, tokenValue, actualToAddr string) {
	if len(data) >= 21 {
		fromAddr = common.Bytes2Hex(data[:21])
	}
	if len(data) >= 53 {
		contractAddr = common.Bytes2Hex(data[32:53])
	}
	if len(data) >= 72 {
		valueBytes := data[64:72]
		val := new(big.Int)
		val.SetBytes(valueBytes)
		value = val.String()
	}

	if len(data) < 100 {
		return
	}

	contractData := data[72:]
	if len(contractData) < 68 {
		return
	}

	methodBytes := contractData[0:4]
	methodSig := fmt.Sprintf("%02x%02x%02x%02x", methodBytes[0], methodBytes[1], methodBytes[2], methodBytes[3])

	if methodSig != "a9059cbb" {
		method = fmt.Sprintf("0x%s", methodSig)
		return
	}

	method = "transfer"

	if len(contractData) >= 36 {
		toBytes := contractData[16:36]
		if len(toBytes) >= 20 {
			actualToBytes := toBytes[len(toBytes)-20:]
			if len(actualToBytes) == 20 {
				tokenToHex := common.Bytes2Hex(actualToBytes)
				actualToAddr = "41" + tokenToHex
			}
		}
	}

	if len(contractData) >= 68 {
		amtBytes := contractData[36:68]
		amt := new(big.Int)
		amt.SetBytes(amtBytes)
		tokenValue = amt.String()
	}

	return
}

