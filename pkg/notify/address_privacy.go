package notify

import (
	"strings"

	"github.com/shopspring/decimal"
)

// MaskAddress 对区块链地址进行隐私处理
// 显示规则：前6位 + "..." + 后4位
// 例如：0x1234567890abcdef -> 0x1234...cdef
func MaskAddress(address string) string {
	address = strings.TrimSpace(address)
	if address == "" {
		return ""
	}

	// 如果地址长度小于等于10位，不做处理（太短无法隐藏）
	if len(address) <= 10 {
		return address
	}

	// 前6位 + "..." + 后4位
	return address[:6] + "..." + address[len(address)-4:]
}

// FormatAmount 格式化金额显示
// 规则：
// 1. 保留所有有效数字，不强制截断
// 2. 去除尾部的0
// 3. 如果小数位都是0，不显示小数位
// 示例：
//   1.000000 -> 1
//   0.0000001240000 -> 0.000000124
//   1.1234567890 -> 1.123456789
func FormatAmount(amount string) string {
	amount = strings.TrimSpace(amount)
	if amount == "" {
		return "0"
	}

	// 使用 decimal 进行精确计算
	d, err := decimal.NewFromString(amount)
	if err != nil {
		// 解析失败，返回原值
		return amount
	}

	// 转换为字符串（保留所有精度）
	result := d.String()

	// 查找小数点
	dotIndex := strings.Index(result, ".")
	if dotIndex == -1 {
		// 没有小数点，直接返回
		return result
	}

	// 分离整数部分和小数部分
	intPart := result[:dotIndex]
	decPart := result[dotIndex+1:]

	// 去除尾部的0
	decPart = strings.TrimRight(decPart, "0")

	if decPart == "" {
		// 小数部分去除0后为空，只返回整数部分
		return intPart
	}

	return intPart + "." + decPart
}
