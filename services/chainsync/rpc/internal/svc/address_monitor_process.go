package svc

import (
	"context"
	"fmt"
	"internalwallet/common/mq"
	"strings"
	"time"

	"internalwallet/proto/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

func (am *AddressMonitor) processTransactionsBatch(transactions []*pb.Transaction) error {
	if len(transactions) == 0 {
		return nil
	}

	logx.Infof("🔄 Processing batch of %d transactions", len(transactions))

	successCount := 0

	for _, tx := range transactions {
		if err := am.processTransactionWithErrorHandling(tx); err != nil {
			logx.Errorf("❌ Failed to process transaction %s: %v", tx.TxHash, err)
			return err
		} else {
			successCount++
		}
	}

	logx.Infof("✅ Batch processing completed: %d success, %d failed", successCount, 0)
	return nil
}

// processTransactionWithErrorHandling 处理单个交易（带错误处理）
func (am *AddressMonitor) processTransactionWithErrorHandling(tx *pb.Transaction) (err error) {
	defer func() {
		if r := recover(); r != nil {
			logx.Errorf("🚨 Panic recovered while processing transaction %s: %v", tx.TxHash, r)
			err = fmt.Errorf("panic while processing transaction %s: %v", tx.TxHash, r)
		}
	}()

	return am.processTransaction(tx)
}

type monitoredTransferEvent struct {
	monitoredAddress       string
	source                 mq.AddressMonitorSource
	direction              string // in/out
	counterpartyAddress    string
	monitoredIsInternal    bool
	counterpartyIsInternal bool
	counterpartySourceBits uint8
	tokenTx                *TokenTransaction
}

func sourcesFromBits(bits addressSourceBits) []mq.AddressMonitorSource {
	out := make([]mq.AddressMonitorSource, 0, 4)
	if bits&addressSourceDeposit != 0 {
		out = append(out, mq.AddressMonitorSourceDeposit)
	}
	if bits&addressSourceCompany != 0 {
		out = append(out, mq.AddressMonitorSourceCompany)
	}
	if bits&addressSourceVault != 0 {
		out = append(out, mq.AddressMonitorSourceVault)
	}
	if bits&addressSourceWeb3 != 0 {
		out = append(out, mq.AddressMonitorSourceWeb3)
	}
	if bits&addressSourceManual != 0 {
		out = append(out, mq.AddressMonitorSourceManual)
	}
	return out
}

func isInternalFromBits(bits addressSourceBits) bool {
	return bits&(addressSourceDeposit|addressSourceCompany|addressSourceVault) != 0
}

// extractMonitoredTransferEventsFromTokenTxs fans out token transfer events into per-(monitored_address, source) units.
// This preserves multi-source membership and enables independent downstream processing.
func (am *AddressMonitor) extractMonitoredTransferEventsFromTokenTxs(tokenTxs []*TokenTransaction, chain pb.BlockChainType) []monitoredTransferEvent {
	if am.registry == nil {
		return nil
	}
	snap := am.registry.Snapshot()
	delta := am.registry.getDelta()
	cs := snap.chains[chain]
	if (cs == nil || len(cs.sourceBitsByAddr) == 0) && (delta.chains[chain] == nil || len(delta.chains[chain]) == 0) {
		return nil
	}

	out := make([]monitoredTransferEvent, 0, len(tokenTxs))

	emit := func(monitoredAddr string, monitoredBits addressSourceBits, direction string, counterpartyRaw string, counterpartyBits addressSourceBits, tokenTx *TokenTransaction) {
		if monitoredAddr == "" || tokenTx == nil {
			return
		}

		counterparty := strings.TrimSpace(counterpartyRaw)
		if counterparty == "" {
			// Avoid failing ingestion due to missing counterparty; keep as empty string (DB default is empty).
			counterparty = ""
		}

		for _, src := range sourcesFromBits(monitoredBits) {
			out = append(out, monitoredTransferEvent{
				monitoredAddress:       monitoredAddr,
				source:                 src,
				direction:              direction,
				counterpartyAddress:    counterparty,
				monitoredIsInternal:    isInternalFromBits(monitoredBits),
				counterpartyIsInternal: isInternalFromBits(counterpartyBits),
				counterpartySourceBits: uint8(counterpartyBits),
				tokenTx:                tokenTx,
			})
		}
	}

	for _, tokenTx := range tokenTxs {
		if tokenTx == nil {
			continue
		}

		// TokenParser for EVM may return checksum addresses; registry stores EVM addresses lower-case.
		fromNorm := normalizeMonitoredAddress(chain, tokenTx.From)
		toNorm := normalizeMonitoredAddress(chain, tokenTx.To)

		var fromBits addressSourceBits
		if fromNorm != "" {
			fromBits = am.registry.effectiveBits(chain, fromNorm, snap, delta)
		}
		var toBits addressSourceBits
		if toNorm != "" {
			toBits = am.registry.effectiveBits(chain, toNorm, snap, delta)
		}

		// Self-transfer: emit only one OUT event to avoid duplicates.
		if fromNorm != "" && fromNorm == toNorm && fromBits != 0 {
			emit(fromNorm, fromBits, "out", tokenTx.To, fromBits, tokenTx)
			continue
		}

		if fromNorm != "" && fromBits != 0 {
			emit(fromNorm, fromBits, "out", tokenTx.To, toBits, tokenTx)
		}
		if toNorm != "" && toBits != 0 {
			emit(toNorm, toBits, "in", tokenTx.From, fromBits, tokenTx)
		}
	}

	return out
}

// processTransaction 处理交易
func (am *AddressMonitor) processTransaction(tx *pb.Transaction) error {
	logx.Infof("📝 Processing transaction: %s", tx.TxHash)

	if tx != nil && strings.EqualFold(tx.TxHash, "0x1bb4a04aa7bb56e9973c1416bb7db5d599dfadef6f588bcf10e96431a4fdd5c9") {
		// #region agent log
		writeDebugLog(map[string]interface{}{
			"sessionId":    "debug-session",
			"runId":        "pre-fix",
			"hypothesisId": "H1",
			"location":     "address_monitor.go:processTransaction:entry",
			"message":      "matched tx hash in processTransaction",
			"data": map[string]interface{}{
				"chain":        tx.Chain.String(),
				"from":         tx.FromAddress,
				"to":           tx.ToAddress,
				"status":       tx.Status.String(),
				"blockNumber":  tx.BlockNumber,
				"blockHash":    tx.BlockHash,
				"blockTime":    tx.BlockTimestamp,
				"txValue":      tx.Value,
				"contractAddr": tx.ContractAddress,
			},
			"timestamp": time.Now().UnixMilli(),
		})
		// #endregion
	}

	// 🆕 补充获取 Logs（如果需要）
	if am.shouldFetchLogs(tx) {
		if err := am.enrichTransactionWithLogs(tx); err != nil {
			logx.Errorf("⚠️ Failed to enrich transaction %s with logs: %v", tx.TxHash, err)
			// 继续处理，即使获取 Logs 失败
		}
	}

	// ✅ 检查交易状态，只保存成功的交易
	if tx.Status == pb.TransactionStatus_TRANSACTION_STATUS_FAILED {
		logx.Infof("❌ Transaction %s failed, skipping save to database", tx.TxHash)
		return nil
	}

	// Robust 策略：允许 PENDING/UNSPECIFIED 入库（exec_status=unknown），但后续下发 Kafka 前会二次校验链上回执。
	// 这样既避免“状态未知导致漏单”，又避免“仅按 confirmations 误入账”。
	if tx.Status == pb.TransactionStatus_TRANSACTION_STATUS_PENDING || tx.Status == pb.TransactionStatus_TRANSACTION_STATUS_UNSPECIFIED {
		logx.Infof("⚠️ Transaction %s has status %s, will be saved as exec_status=unknown (robust mode)",
			tx.TxHash, tx.Status.String())
	}

	// 使用代币解析器解析交易
	var tokenTxs []*TokenTransaction
	if am.tokenParser != nil {
		parsedTxs, err := am.tokenParser.ParseTransaction(tx)
		if err != nil {
			logx.Errorf("Failed to parse token transactions for %s: %v", tx.TxHash, err)
			return fmt.Errorf("parse token transactions: %w", err)
		}
		tokenTxs = parsedTxs
	} else {
		// 如果没有代币解析器，只处理主币交易
		tokenTxs = []*TokenTransaction{
			{
				TxHash:          tx.TxHash,
				Chain:           tx.Chain,
				BlockNumber:     tx.BlockNumber,
				BlockHash:       tx.BlockHash,
				From:            tx.FromAddress,
				To:              tx.ToAddress,
				Status:          uint8(tx.Status),
				Timestamp:       tx.BlockTimestamp,
				EventIndex:      0,
				TokenAmount:     tx.Value,
				TokenValue:      tx.Value,
				TransactionType: "native",
				TokenDecimals:   DefaultTokenDecimals,
			},
		}
	}

	// 检查是否启用地址池监控
	if !am.IsAddressPoolMonitoringEnabled() {
		// 全链扫描模式：只解析，不入库
		logx.Infof("📊 [Full Scan Mode] Transaction %s parsed (%d token txs), but not saved to DB",
			tx.TxHash, len(tokenTxs))
		return nil
	}

	// 地址池监控模式：提取监控地址及其相关的转账
	events := am.extractMonitoredTransferEventsFromTokenTxs(tokenTxs, tx.Chain)

	if tx != nil && strings.EqualFold(tx.TxHash, "0x1bb4a04aa7bb56e9973c1416bb7db5d599dfadef6f588bcf10e96431a4fdd5c9") {
		hasTarget := false
		targetAddrLower := strings.ToLower("0x81962f7083b23b2b3d2f3cd1ee7ac038591e7a90")
		for _, evt := range events {
			if strings.EqualFold(evt.monitoredAddress, targetAddrLower) {
				hasTarget = true
				break
			}
		}
		// #region agent log
		writeDebugLog(map[string]interface{}{
			"sessionId":    "debug-session",
			"runId":        "pre-fix",
			"hypothesisId": "H3",
			"location":     "address_monitor.go:processTransaction:monitorMatch",
			"message":      "monitored addresses resolved for tx",
			"data": map[string]interface{}{
				"txHash":          tx.TxHash,
				"monitoredCount":  len(events),
				"hasTargetAddr":   hasTarget,
				"targetAddrLower": targetAddrLower,
			},
			"timestamp": time.Now().UnixMilli(),
		})
		// #endregion
	}

	if len(events) == 0 {
		logx.Debugf("No monitored addresses found in transaction %s", tx.TxHash)
		return nil
	}

	logx.Infof("📊 Found %d monitored transfer events in transaction %s", len(events), tx.TxHash)

	for _, evt := range events {
		if evt.tokenTx == nil {
			continue
		}
		if err := am.saveUnconfirmedTransactionWithIndex(
			tx,
			evt.monitoredAddress,
			string(evt.source),
			evt.direction,
			evt.counterpartyAddress,
			evt.monitoredIsInternal,
			evt.counterpartyIsInternal,
			evt.counterpartySourceBits,
			evt.tokenTx,
			evt.tokenTx.EventIndex,
		); err != nil {
			return err
		}
	}

	return nil
}

// shouldFetchLogs 判断是否需要补充获取完整交易信息（Logs 和 Internal Transactions）
func (am *AddressMonitor) shouldFetchLogs(tx *pb.Transaction) bool {
	// Only enrich in address pool monitoring mode. Full chain scanning mode should be lightweight
	// and must not trigger per-tx receipt/log fetching.
	if !am.IsAddressPoolMonitoringEnabled() {
		return false
	}

	// For EVM chains, if tx status is not known (pending/unspecified), fetch receipt to get
	// definitive status (and logs for contract interactions). This is only called for relevant txs.
	if tx.Chain == pb.BlockChainType_CHAIN_TYPE_ETHEREUM || tx.Chain == pb.BlockChainType_CHAIN_TYPE_BSC {
		if tx.Status == pb.TransactionStatus_TRANSACTION_STATUS_PENDING || tx.Status == pb.TransactionStatus_TRANSACTION_STATUS_UNSPECIFIED {
			return true
		}
	}

	// 🆕 优先检查：如果有 Logs 但缺少 Internal Transactions，且是复杂合约调用
	if len(tx.Logs) > 0 && len(tx.InternalTransactions) == 0 {
		// 对于 ETH/BSC，如果有 InputData（合约调用）且 Logs 存在，可能需要 Internal Transactions
		if tx.Chain == pb.BlockChainType_CHAIN_TYPE_ETHEREUM || tx.Chain == pb.BlockChainType_CHAIN_TYPE_BSC {
			if len(tx.InputData) > 0 && tx.ToAddress != "" {
				return true // 需要获取 Internal Transactions
			}
		}
	}

	// 如果已经有 Logs 和 Internal Transactions，不需要再获取
	if len(tx.Logs) > 0 && len(tx.InternalTransactions) > 0 {
		return false
	}

	// 如果完全没有 Logs，按原逻辑判断
	if len(tx.Logs) == 0 {
		// 如果有合约地址，可能需要 Logs
		if tx.ContractAddress != "" {
			return true
		}

		// 如果有 InputData 且不为空，可能是合约调用
		if len(tx.InputData) > 0 {
			return true
		}

		// 对于 TRON，如果有合约地址或 ToAddress，可能需要 Logs
		if tx.Chain == pb.BlockChainType_CHAIN_TYPE_TRON {
			// 如果有合约地址，一定需要 Logs（智能合约调用）
			if tx.ContractAddress != "" {
				return true
			}
			// 如果 ToAddress 不为空且 Value 为 0，很可能是代币交易（value已不再包含单位）
			if tx.ToAddress != "" && (tx.Value == "0" || tx.Value == "") {
				return true
			}
		}

		// 对于 ETH/BSC，如果有合约地址或 ToAddress 且 Value 为 0，可能是代币交易
		if tx.Chain == pb.BlockChainType_CHAIN_TYPE_ETHEREUM || tx.Chain == pb.BlockChainType_CHAIN_TYPE_BSC {
			// 如果有合约地址且 Value 为 0，可能是代币交易
			if tx.ContractAddress != "" && (tx.Value == "0" || tx.Value == "0x0") {
				return true
			}
			// 如果 ToAddress 存在且 Value 为 0，也可能是代币交易
			if tx.ToAddress != "" && (tx.Value == "0" || tx.Value == "0x0") {
				return true
			}
		}
	}

	return false
}

// enrichTransactionWithLogs 补充获取交易的 Logs
func (am *AddressMonitor) enrichTransactionWithLogs(tx *pb.Transaction) error {
	if am.providerPool == nil {
		return fmt.Errorf("provider pool not available")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 从 provider 池获取 provider
	provider, err := am.providerPool.GetProvider(tx.Chain)
	if err != nil {
		return fmt.Errorf("failed to get provider: %w", err)
	}

	// 获取完整交易信息（包含 Logs）
	fullTx, err := provider.GetTransaction(ctx, tx.TxHash)
	if err != nil {
		return fmt.Errorf("failed to get transaction: %w", err)
	}

	// 补充 Logs、Internal Transactions 和其他可能缺失的信息
	if fullTx != nil {
		if fullTx.Status != pb.TransactionStatus_TRANSACTION_STATUS_UNSPECIFIED {
			tx.Status = fullTx.Status
		}

		if len(fullTx.Logs) > 0 {
			tx.Logs = fullTx.Logs
			logx.Infof("📋 Enriched transaction %s with %d logs", tx.TxHash, len(fullTx.Logs))
		}

		// 🆕 补充 Internal Transactions（智能合约内部的 ETH 转账）
		if len(fullTx.InternalTransactions) > 0 {
			tx.InternalTransactions = fullTx.InternalTransactions
			logx.Infof("🔄 Enriched transaction %s with %d internal transactions", tx.TxHash, len(fullTx.InternalTransactions))
		}

		// 同时补充其他可能缺失的字段
		if tx.BlockNumber == 0 && fullTx.BlockNumber > 0 {
			tx.BlockNumber = fullTx.BlockNumber
		}
		if tx.BlockHash == "" && fullTx.BlockHash != "" {
			tx.BlockHash = fullTx.BlockHash
		}
		if tx.GasFee == "" && fullTx.GasFee != "" {
			tx.GasFee = fullTx.GasFee
		}
		if tx.GasUsed == 0 && fullTx.GasUsed > 0 {
			tx.GasUsed = fullTx.GasUsed
		}
	}

	return nil
}

// filterTransactionsForMonitoredAddresses 过滤出监控地址相关的交易
// 注意：此方法假设已经在地址池监控模式下调用
func (am *AddressMonitor) filterTransactionsForMonitoredAddresses(transactions []*pb.Transaction, chainType pb.BlockChainType) []*pb.Transaction {
	if len(transactions) == 0 {
		return transactions
	}

	if am.registry == nil {
		return []*pb.Transaction{}
	}
	snap := am.registry.Snapshot()
	delta := am.registry.getDelta()
	cs := snap.chains[chainType]
	if (cs == nil || len(cs.sourceBitsByAddr) == 0) && (delta.chains[chainType] == nil || len(delta.chains[chainType]) == 0) {
		logx.Infof("⚠️ Address pool monitoring enabled but no monitored addresses loaded for chain %v", chainType)
		return []*pb.Transaction{}
	}

	var filteredTxs []*pb.Transaction
	for _, tx := range transactions {
		fromAddr := normalizeMonitoredAddress(chainType, tx.FromAddress)
		toAddr := normalizeMonitoredAddress(chainType, tx.ToAddress)
		// 检查发送方
		if fromAddr != "" && am.registry.effectiveBits(chainType, fromAddr, snap, delta) != 0 {
			filteredTxs = append(filteredTxs, tx)
			continue
		}
		// 检查接收方
		if toAddr != "" && am.registry.effectiveBits(chainType, toAddr, snap, delta) != 0 {
			filteredTxs = append(filteredTxs, tx)
		}
	}

	return filteredTxs
}

// checkMonitoredAddress 检查交易是否匹配监控地址
func (am *AddressMonitor) checkMonitoredAddress(tx *pb.Transaction) (string, bool) {
	if tx == nil {
		return "", false
	}
	if am.registry == nil {
		return "", false
	}
	snap := am.registry.Snapshot()
	delta := am.registry.getDelta()
	cs := snap.chains[tx.Chain]
	if (cs == nil || len(cs.sourceBitsByAddr) == 0) && (delta.chains[tx.Chain] == nil || len(delta.chains[tx.Chain]) == 0) {
		return "", false
	}

	// 检查发送方
	fromAddr := normalizeMonitoredAddress(tx.Chain, tx.FromAddress)
	if fromAddr != "" && am.registry.effectiveBits(tx.Chain, fromAddr, snap, delta) != 0 {
		return fromAddr, true
	}
	// 检查接收方
	toAddr := normalizeMonitoredAddress(tx.Chain, tx.ToAddress)
	if toAddr != "" && am.registry.effectiveBits(tx.Chain, toAddr, snap, delta) != 0 {
		return toAddr, true
	}
	return "", false
}
