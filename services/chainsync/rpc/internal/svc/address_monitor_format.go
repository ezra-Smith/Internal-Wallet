package svc

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"internalwallet/proto/pb"
)

func isAlreadyFormatted(valueStr string) bool {
	// 如果值包含常见的代币单位符号，则认为已经格式化
	units := []string{"TRX", "USDT", "USDC", "ETH", "BNB", "BTC"}
	for _, unit := range units {
		if strings.Contains(valueStr, unit) {
			return true
		}
	}
	return false
}

// formatValueForStorage 格式化 value 用于存储（不带单位）
// 将 wei/sun 等最小单位转换为可读的数值，如 "1000000000000000000" -> "1"
func formatValueForStorage(valueStr string, chain pb.BlockChainType) string {
	return formatValueWithDecimals(valueStr, chain, 0, "")
}

// formatValueWithDecimals 格式化金额，支持自定义精度（返回纯数值，不带单位）
// decimals: 小数位数，0 表示使用链默认值
// unit: 单位符号（已废弃，保留参数以兼容），不再使用
// 返回格式化后的纯数值字符串，如 "1.5"、"100"、"0"
func formatValueWithDecimals(valueStr string, chain pb.BlockChainType, decimals int, unit string) string {
	if valueStr == "" || valueStr == "0" || valueStr == "0x0" {
		return "0"
	}

	// 如果已经格式化过（包含单位），先提取数值部分
	if isAlreadyFormatted(valueStr) {
		// 提取数值部分（去掉单位）
		parts := strings.Fields(valueStr)
		if len(parts) > 0 {
			return parts[0]
		}
		return valueStr
	}

	// 根据链类型确定默认精度
	if decimals == 0 {
		switch chain {
		case pb.BlockChainType_CHAIN_TYPE_TRON:
			decimals = 6 // TRON 主币 6 位小数
		case pb.BlockChainType_CHAIN_TYPE_ETHEREUM, pb.BlockChainType_CHAIN_TYPE_BSC:
			decimals = 18
		default:
			decimals = 18
		}
	}

	// 使用 big.Int 处理大数，支持十进制和十六进制格式
	value := new(big.Int)
	if strings.HasPrefix(valueStr, "0x") || strings.HasPrefix(valueStr, "0X") {
		_, ok := value.SetString(valueStr[2:], 16)
		if !ok {
			return valueStr
		}
	} else {
		_, ok := value.SetString(valueStr, 10)
		if !ok {
			return valueStr
		}
	}

	// 如果值为 0，直接返回
	if value.Sign() == 0 {
		return "0"
	}

	// 计算除数 10^decimals
	divisor := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)

	// 计算整数部分和小数部分
	integerPart := new(big.Int).Div(value, divisor)
	remainder := new(big.Int).Mod(value, divisor)

	// 如果没有小数部分
	if remainder.Sign() == 0 {
		return integerPart.String()
	}

	// 格式化小数部分（补零到 decimals 位，然后去掉尾部的零）
	decimalStr := fmt.Sprintf("%0*s", decimals, remainder.String())
	// 确保小数部分长度正确
	if len(decimalStr) < decimals {
		decimalStr = strings.Repeat("0", decimals-len(decimalStr)) + decimalStr
	}
	decimalStr = strings.TrimRight(decimalStr, "0")

	if decimalStr == "" {
		return integerPart.String()
	}

	return fmt.Sprintf("%s.%s", integerPart.String(), decimalStr)
}

// formatGasPriceToGwei 将 gas price 从 wei 转换为 Gwei
// 1 Gwei = 10^9 Wei
func formatGasPriceToGwei(gasPriceWei string) string {
	if gasPriceWei == "" || gasPriceWei == "0" || gasPriceWei == "0x0" {
		return "0 Gwei"
	}

	// 使用 big.Int 处理大数，支持十进制和十六进制格式
	value := new(big.Int)

	// 检查是否是十六进制格式
	if strings.HasPrefix(gasPriceWei, "0x") || strings.HasPrefix(gasPriceWei, "0X") {
		_, ok := value.SetString(gasPriceWei[2:], 16)
		if !ok {
			return gasPriceWei // 解析失败，返回原始值
		}
	} else {
		_, ok := value.SetString(gasPriceWei, 10)
		if !ok {
			return gasPriceWei // 解析失败，返回原始值
		}
	}

	if value.Sign() == 0 {
		return "0 Gwei"
	}

	// 1 Gwei = 10^9 Wei
	gweiDivisor := new(big.Int).Exp(big.NewInt(10), big.NewInt(9), nil)

	// 计算整数部分和小数部分
	integerPart := new(big.Int).Div(value, gweiDivisor)
	remainder := new(big.Int).Mod(value, gweiDivisor)

	// 如果没有小数部分
	if remainder.Sign() == 0 {
		return fmt.Sprintf("%s Gwei", integerPart.String())
	}

	// 格式化小数部分（9位精度）
	decimalStr := fmt.Sprintf("%09d", remainder.Int64())
	decimalStr = strings.TrimRight(decimalStr, "0")

	if decimalStr == "" {
		return fmt.Sprintf("%s Gwei", integerPart.String())
	}

	return fmt.Sprintf("%s.%s Gwei", integerPart.String(), decimalStr)
}

// calculateGasFee 计算 gas fee = gas_price * gas_used
// gasPrice: Wei 单位的 gas 价格（可能是十进制字符串或十六进制字符串）
// gasUsed: gas 使用量
// 返回: Wei 单位的 gas fee 字符串
func calculateGasFee(gasPrice string, gasUsed uint64) string {
	if gasPrice == "" || gasUsed == 0 {
		return "0"
	}

	// 解析 gas_price
	gasPriceInt := new(big.Int)

	// 检查是否是十六进制格式
	if strings.HasPrefix(gasPrice, "0x") || strings.HasPrefix(gasPrice, "0X") {
		_, ok := gasPriceInt.SetString(gasPrice[2:], 16)
		if !ok {
			return "0"
		}
	} else {
		_, ok := gasPriceInt.SetString(gasPrice, 10)
		if !ok {
			return "0"
		}
	}

	// 计算 gas_fee = gas_price * gas_used
	gasUsedInt := new(big.Int).SetUint64(gasUsed)
	gasFee := new(big.Int).Mul(gasPriceInt, gasUsedInt)

	return gasFee.String()
}

// formatGasFee 格式化 gas fee（转换为 ETH/BNB）
func formatGasFee(gasFeeWei string, chain pb.BlockChainType) string {
	if gasFeeWei == "" || gasFeeWei == "0" || gasFeeWei == "0x0" {
		return "0"
	}

	unit := "ETH"
	switch chain {
	case pb.BlockChainType_CHAIN_TYPE_BSC:
		unit = "BNB"
	case pb.BlockChainType_CHAIN_TYPE_TRON:
		unit = "TRX"
	}

	// 使用 big.Int 处理大数，支持十进制和十六进制格式
	value := new(big.Int)
	if strings.HasPrefix(gasFeeWei, "0x") || strings.HasPrefix(gasFeeWei, "0X") {
		_, ok := value.SetString(gasFeeWei[2:], 16)
		if !ok {
			return gasFeeWei
		}
	} else {
		_, ok := value.SetString(gasFeeWei, 10)
		if !ok {
			return gasFeeWei
		}
	}

	if value.Sign() == 0 {
		return "0"
	}

	// 18 位小数（ETH/BNB）
	decimals := 18
	if chain == pb.BlockChainType_CHAIN_TYPE_TRON {
		decimals = 6
	}

	divisor := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
	integerPart := new(big.Int).Div(value, divisor)
	remainder := new(big.Int).Mod(value, divisor)

	if remainder.Sign() == 0 {
		return fmt.Sprintf("%s %s", integerPart.String(), unit)
	}

	decimalStr := fmt.Sprintf("%0*s", decimals, remainder.String())
	if len(decimalStr) < decimals {
		decimalStr = strings.Repeat("0", decimals-len(decimalStr)) + decimalStr
	}
	decimalStr = strings.TrimRight(decimalStr, "0")

	if decimalStr == "" {
		return fmt.Sprintf("%s %s", integerPart.String(), unit)
	}

	return fmt.Sprintf("%s.%s %s", integerPart.String(), decimalStr, unit)
}

// formatValue 根据链类型格式化数值显示
func formatValue(valueStr string, chain pb.BlockChainType) string {
	if valueStr == "" {
		return "0"
	}

	// 根据链类型决定单位和精度
	unit := "TRX"
	decimals := 6 // TRON默认6位小数

	switch chain {
	case pb.BlockChainType_CHAIN_TYPE_ETHEREUM:
		unit = "ETH"
		decimals = 18
	case pb.BlockChainType_CHAIN_TYPE_BSC:
		unit = "BNB"
		decimals = 18
	}

	// 转换字符串为big.Int进行计算
	var value int64
	if len(valueStr) > 0 {
		// 尝试解析为整数
		if val, err := strconv.ParseInt(valueStr, 10, 64); err == nil {
			value = val
		} else {
			// 如果解析失败，返回原始值
			return valueStr + " (raw)"
		}
	}

	// 计算整数和小数部分
	divisor := int64(10)
	for i := 1; i < decimals; i++ {
		divisor *= 10
	}

	whole := value / divisor
	decimal := value % divisor

	// 如果是0，直接返回
	if whole == 0 && decimal == 0 {
		return "0 " + unit
	}

	// 格式化小数部分，去掉尾部零
	decimalStr := fmt.Sprintf("%0*d", decimals, decimal)
	decimalStr = strings.TrimRight(decimalStr, "0")

	if decimalStr == "" {
		return fmt.Sprintf("%d %s", whole, unit)
	}

	return fmt.Sprintf("%d.%s %s", whole, decimalStr, unit)
}
