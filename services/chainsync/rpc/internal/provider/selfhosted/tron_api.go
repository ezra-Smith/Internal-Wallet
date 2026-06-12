package selfhosted

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

func (p *SelfHostedProvider) getTronNowBlock(ctx context.Context) (*TronNowBlockResponse, error) {
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	data, err := p.callTronAPIRaw(ctxWithTimeout, "/wallet/getnowblock", nil)
	if err != nil {
		return nil, err
	}

	var response TronNowBlockResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal getnowblock response: %w", err)
	}

	return &response, nil
}

func (p *SelfHostedProvider) getTronBlockByNumber(ctx context.Context, num uint64) (*TronBlockByNumResponse, error) {
	payload := map[string]interface{}{"num": num}

	ctxWithTimeout, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	data, err := p.callTronAPIRaw(ctxWithTimeout, "/wallet/getblockbynum", payload)
	if err != nil {
		return nil, err
	}

	var response TronBlockByNumResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal getblockbynum response: %w", err)
	}

	return &response, nil
}

func (p *SelfHostedProvider) getTronTransaction(ctx context.Context, value string) (*TronTransactionResponse, error) {
	payload := map[string]interface{}{"value": value}

	data, err := p.callTronAPIRaw(ctx, "/wallet/gettransactionbyid", payload)
	if err != nil {
		return nil, err
	}

	// NOTE: Different TRON nodes / proxies may return either:
	// 1) a raw `TronTransaction` object (most common for /wallet/gettransactionbyid), OR
	// 2) an envelope `{ visible, ret, transaction }` (some proxies).
	//
	// We support both shapes to avoid silently losing `ret.contractRet` (SUCCESS/FAIL),
	// which would otherwise cause tx exec status to stay "PENDING" forever.
	var directTx TronTransaction
	if err := json.Unmarshal(data, &directTx); err == nil && directTx.TxID != "" {
		return &TronTransactionResponse{
			Visible:     true,
			Transaction: directTx,
		}, nil
	}

	var response TronTransactionResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal gettransactionbyid response: %w", err)
	}

	return &response, nil
}

func (p *SelfHostedProvider) getTronAccount(ctx context.Context, address string) (*TronAccountResponse, error) {
	// 使用 visible: true 来支持 Base58 地址格式
	payload := map[string]interface{}{
		"address": address,
		"visible": true,
	}

	data, err := p.callTronAPIRaw(ctx, "/wallet/getaccount", payload)
	if err != nil {
		return nil, err
	}

	var response TronAccountResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal getaccount response: %w", err)
	}

	return &response, nil
}

func (p *SelfHostedProvider) getTronTransactionInfo(ctx context.Context, txHash string) (*TronRPCTransactionInfo, error) {
	payload := map[string]interface{}{"value": txHash}

	data, err := p.callTronAPIRaw(ctx, "/wallet/gettransactioninfobyid", payload)
	if err != nil {
		return nil, fmt.Errorf("failed to get TRON transaction info: %w", err)
	}

	var txInfo TronRPCTransactionInfo
	if err := json.Unmarshal(data, &txInfo); err != nil {
		return nil, fmt.Errorf("failed to unmarshal transaction info: %v", err)
	}

	return &txInfo, nil
}

// TRC20BalanceResponse 表示 TRC20 余额查询的响应
type TRC20BalanceResponse struct {
	Result struct {
		Result  bool   `json:"result"`
		Message string `json:"message"`
	} `json:"result"`
	ConstantResult []string `json:"constant_result"`
}

// getTRC20Balance 查询 TRC20 代币余额
// contractAddress: TRC20 合约地址（Base58 格式）
// ownerAddress: 持币地址（Base58 格式）
// 返回原始余额字符串（需要根据 decimals 转换）
func (p *SelfHostedProvider) getTRC20Balance(ctx context.Context, contractAddress, ownerAddress string) (string, error) {
	// 将 Base58 地址转换为 hex 格式的参数
	addrHex, err := base58ToHex(ownerAddress)
	if err != nil {
		return "0", fmt.Errorf("failed to convert address to hex: %w", err)
	}

	// ABI 编码：地址需要左补零到 32 字节（64 个 hex 字符）
	// TRON 地址是 21 字节（42 个 hex 字符），去掉前缀 41 后是 20 字节（40 个 hex 字符）
	// 所以需要在左边补 24 个零
	addrWithoutPrefix := addrHex[2:] // 去掉 '41' 前缀
	parameter := fmt.Sprintf("%024s%s", "", addrWithoutPrefix)

	payload := map[string]interface{}{
		"owner_address":     ownerAddress,
		"contract_address":  contractAddress,
		"function_selector": "balanceOf(address)",
		"parameter":         parameter,
		"visible":           true,
	}

	data, err := p.callTronAPIRaw(ctx, "/wallet/triggerconstantcontract", payload)
	if err != nil {
		return "0", fmt.Errorf("failed to call TRC20 balanceOf: %w", err)
	}

	var response TRC20BalanceResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return "0", fmt.Errorf("failed to unmarshal TRC20 balance response: %w", err)
	}

	// 检查调用是否成功
	if !response.Result.Result && response.Result.Message != "" {
		return "0", nil // 合约调用失败，返回 0 余额
	}

	if len(response.ConstantResult) == 0 {
		return "0", nil
	}

	// 解析 hex 格式的余额
	balanceHex := response.ConstantResult[0]
	return parseHexBalance(balanceHex), nil
}

// base58ToHex 将 TRON Base58 地址转换为 hex 格式
func base58ToHex(address string) (string, error) {
	alphabet := "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

	// 解码 Base58
	result := make([]byte, 0)
	for _, c := range address {
		idx := -1
		for i, a := range alphabet {
			if a == c {
				idx = i
				break
			}
		}
		if idx == -1 {
			return "", fmt.Errorf("invalid base58 character: %c", c)
		}

		carry := idx
		for i := len(result) - 1; i >= 0; i-- {
			carry += 58 * int(result[i])
			result[i] = byte(carry % 256)
			carry /= 256
		}
		for carry > 0 {
			result = append([]byte{byte(carry % 256)}, result...)
			carry /= 256
		}
	}

	// 添加前导零
	for _, c := range address {
		if c != '1' {
			break
		}
		result = append([]byte{0}, result...)
	}

	// 转换为 hex（去掉最后 4 字节校验和）
	if len(result) < 4 {
		return "", fmt.Errorf("invalid address length")
	}
	addressBytes := result[:len(result)-4]

	hex := ""
	for _, b := range addressBytes {
		hex += fmt.Sprintf("%02x", b)
	}
	return hex, nil
}

// parseHexBalance 解析 hex 格式的余额为字符串
func parseHexBalance(hexStr string) string {
	if hexStr == "" || hexStr == "0" || hexStr == "0000000000000000000000000000000000000000000000000000000000000000" {
		return "0"
	}

	// 移除前导零
	result := hexStr
	for len(result) > 1 && result[0] == '0' {
		result = result[1:]
	}

	// 将 hex 转换为 decimal
	var balance uint64
	for _, c := range result {
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
