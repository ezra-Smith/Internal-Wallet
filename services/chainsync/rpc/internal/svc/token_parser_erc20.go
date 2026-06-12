package svc

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/zeromicro/go-zero/core/logx"

	"internalwallet/proto/pb"
)

// parseERC20Transactions 解析ERC20交易
func (tp *TokenParser) parseERC20Transactions(tx *pb.Transaction) ([]*TokenTransaction, error) {
	var tokenTxs []*TokenTransaction

	if len(tx.Logs) == 0 {
		return tokenTxs, nil
	}

	for _, log := range tx.Logs {
		tokenTx, err := tp.parseERC20Transfer(tx, log)
		if err != nil {
			logx.Errorf("Failed to parse ERC20 transfer for %s: %v", tx.TxHash, err)
			continue
		}
		if tokenTx != nil {
			tokenTxs = append(tokenTxs, tokenTx)
		}
	}

	return tokenTxs, nil
}

// parseERC20Transfer 解析ERC20 Transfer事件
func (tp *TokenParser) parseERC20Transfer(tx *pb.Transaction, log *pb.TransactionLog) (*TokenTransaction, error) {
	if len(log.Topics) < 3 {
		return nil, nil
	}

	topic0 := log.Topics[0]
	if !strings.HasPrefix(topic0, "0x") {
		topic0 = "0x" + topic0
	}
	if len(topic0) < 10 {
		return nil, nil
	}

	eventSignature := topic0[:10]
	transferSignature := crypto.Keccak256Hash([]byte(ERC20TransferEvent)).Hex()[:10]
	if eventSignature != transferSignature {
		return nil, nil
	}

	var dataHex string
	if len(log.Data) > 0 {
		dataHex = hex.EncodeToString(log.Data)
	} else {
		return nil, fmt.Errorf("empty log data")
	}

	amount := new(big.Int)
	amount.SetBytes(common.FromHex(dataHex))

	extractTopicAddress := func(topic string) (common.Address, bool) {
		t := topic
		if strings.HasPrefix(t, "0x") {
			t = t[2:]
		}
		if len(t) < 40 {
			return common.Address{}, false
		}
		return common.HexToAddress(t[len(t)-40:]), true
	}

	fromAddr, ok := extractTopicAddress(log.Topics[1])
	if !ok {
		return nil, nil
	}
	toAddr, ok := extractTopicAddress(log.Topics[2])
	if !ok {
		return nil, nil
	}
	from := fromAddr.String()
	to := toAddr.String()

	if !tp.isAddressRelevant(tx.Chain, from, to) {
		return nil, nil
	}

	tokenInfo := tp.GetTokenInfo(tx.Chain, log.Address)
	tokenValue := tp.calculateTokenValue(amount, tokenInfo.Decimals)
	eventIndex, ok := parseEventIndexFromLogIndex(log.LogIndex)
	if !ok {
		// Fallback: keep 0 (may reduce dedup quality, but avoids dropping the tx).
		eventIndex = 0
		logx.Infof("⚠️  Failed to parse log_index for tx %s: log_index=%q", tx.TxHash, log.LogIndex)
	}

	return &TokenTransaction{
		TxHash:          tx.TxHash,
		Chain:           tx.Chain,
		BlockNumber:     tx.BlockNumber,
		From:            from,
		To:              to,
		Status:          uint8(tx.Status),
		Timestamp:       tx.BlockTimestamp,
		EventIndex:      eventIndex,
		TokenAddress:    log.Address,
		TokenName:       tokenInfo.Name,
		TokenSymbol:     tokenInfo.Symbol,
		TokenDecimals:   tokenInfo.Decimals,
		TokenAmount:     amount.String(),
		TokenValue:      tokenValue,
		TransactionType: "token",
	}, nil
}

func parseEventIndexFromLogIndex(s string) (int32, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}

	var (
		v   int64
		err error
	)
	if strings.HasPrefix(strings.ToLower(s), "0x") {
		v, err = strconv.ParseInt(s[2:], 16, 32)
	} else {
		v, err = strconv.ParseInt(s, 10, 32)
	}
	if err != nil {
		return 0, false
	}
	return int32(v), true
}

// isAddressRelevant 检查地址是否相关
func (tp *TokenParser) isAddressRelevant(chain pb.BlockChainType, from, to string) bool {
	if tp.addressMonitor == nil {
		logx.Debugf("Address monitor not configured, processing all transactions for chain %v", chain)
		return true
	}

	if !tp.addressMonitor.IsAddressPoolMonitoringEnabled() {
		logx.Debugf("Address pool monitoring disabled, processing all transactions for chain %v", chain)
		return true
	}

	fromMonitored := tp.addressMonitor.IsAddressMonitored(chain, from)
	toMonitored := tp.addressMonitor.IsAddressMonitored(chain, to)

	isRelevant := fromMonitored || toMonitored
	if isRelevant {
		logx.Debugf("Transaction relevant for chain %v: from=%s (monitored=%t), to=%s (monitored=%t)",
			chain, from, fromMonitored, to, toMonitored)
	} else {
		logx.Debugf("Transaction filtered out for chain %v: from=%s (monitored=%t), to=%s (monitored=%t)",
			chain, from, fromMonitored, to, toMonitored)
	}

	return isRelevant
}

// calculateTokenValue 计算代币价值（考虑小数位）
func (tp *TokenParser) calculateTokenValue(amount *big.Int, decimals uint8) string {
	if amount == nil {
		return "0"
	}

	divisor := new(big.Int)
	divisor.Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)

	result := new(big.Float).SetInt(amount)
	result.Quo(result, new(big.Float).SetInt(divisor))

	return result.Text('f', int(decimals))
}

// FormatTokenAmount 格式化代币金额显示
func (tp *TokenParser) FormatTokenAmount(amount string, decimals uint8) string {
	if amount == "" {
		return "0"
	}

	bigAmount, ok := new(big.Int).SetString(amount, 10)
	if !ok {
		return amount
	}

	return tp.calculateTokenValue(bigAmount, decimals)
}
