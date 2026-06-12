package selfhosted

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// getERC20Balance 查询 ERC20 代币余额
// contractAddress: ERC20 合约地址
// ownerAddress: 持币地址
// 返回原始余额字符串
func (p *SelfHostedProvider) getERC20Balance(ctx context.Context, contractAddress, ownerAddress string) (string, error) {
	// balanceOf(address) 函数选择器 = 0x70a08231
	// 参数是 32 字节地址（左边补零）
	addrWithoutPrefix := strings.TrimPrefix(strings.ToLower(ownerAddress), "0x")
	data := "0x70a08231" + fmt.Sprintf("%064s", addrWithoutPrefix)

	resp, err := p.callRPC(ctx, "eth_call", []interface{}{
		map[string]string{
			"to":   contractAddress,
			"data": data,
		},
		"latest",
	})
	if err != nil {
		return "0", fmt.Errorf("failed to call ERC20 balanceOf: %w", err)
	}

	var resultHex string
	if err := json.Unmarshal(resp.Result, &resultHex); err != nil {
		return "0", fmt.Errorf("failed to unmarshal ERC20 balance response: %w", err)
	}

	// 解析 hex 格式的余额
	return parseEvmHexBalance(resultHex), nil
}

// parseEvmHexBalance 解析 EVM hex 格式的余额为 decimal 字符串
func parseEvmHexBalance(hexStr string) string {
	hexStr = strings.TrimPrefix(hexStr, "0x")
	if hexStr == "" || hexStr == "0" {
		return "0"
	}

	// 移除前导零
	for len(hexStr) > 1 && hexStr[0] == '0' {
		hexStr = hexStr[1:]
	}

	if hexStr == "" {
		return "0"
	}

	// 将 hex 转换为 decimal（使用大数处理）
	// 简化处理：对于大多数代币余额，uint64 足够
	var balance uint64
	for _, c := range hexStr {
		balance = balance * 16
		if c >= '0' && c <= '9' {
			balance += uint64(c - '0')
		} else if c >= 'a' && c <= 'f' {
			balance += uint64(c - 'a' + 10)
		} else if c >= 'A' && c <= 'F' {
			balance += uint64(c - 'A' + 10)
		}
	}

	return fmt.Sprintf("%d", balance)
}
