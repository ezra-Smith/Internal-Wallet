package svc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/zeromicro/go-zero/core/logx"

	"internalwallet/common/mq"
	"internalwallet/services/chainsync/rpc/internal/model"
)

func (tcm *TransactionConfirmManager) chainIDForChainString(chain string) int64 {
	chain = strings.ToUpper(strings.TrimSpace(chain))
	// Prefer config (single source of truth for mainnet/testnet in this environment).
	if tcm != nil && tcm.config != nil {
		switch chain {
		case "CHAIN_TYPE_ETHEREUM", "ETHEREUM", "ETH":
			if tcm.config.Chains.Ethereum.ChainID > 0 {
				return int64(tcm.config.Chains.Ethereum.ChainID)
			}
		case "CHAIN_TYPE_BSC", "BSC":
			if tcm.config.Chains.BSC.ChainID > 0 {
				return int64(tcm.config.Chains.BSC.ChainID)
			}
		case "CHAIN_TYPE_TRON", "TRON", "TRX":
			if tcm.config.Chains.Tron.ChainID > 0 {
				return int64(tcm.config.Chains.Tron.ChainID)
			}
		}
	}

	// Fallbacks (keep consistent with Business defaults).
	switch chain {
	case "CHAIN_TYPE_ETHEREUM", "ETHEREUM", "ETH":
		return 1
	case "CHAIN_TYPE_BSC", "BSC":
		return 56
	case "CHAIN_TYPE_TRON", "TRON", "TRX":
		return 728126428
	default:
		return 0
	}
}

func txConfirmEventID(tx *model.UnconfirmedTransaction) string {
	if tx == nil {
		return ""
	}
	payload := fmt.Sprintf("%s|%s|%s|%d", strings.TrimSpace(tx.TxHash), strings.TrimSpace(tx.MonitoredAddress), strings.TrimSpace(tx.Source), tx.LogIndex)
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

func (tcm *TransactionConfirmManager) getTransactionTopicForSource(source string) string {
	source = strings.TrimSpace(source)

	// Defaults (explicit per-channel topics)
	defaultDeposit := "wallet.transactions.confirm.deposit"
	defaultWeb3 := "wallet.transactions.confirm.web3"
	defaultVault := "wallet.transactions.confirm.vault"
	defaultManual := "wallet.transactions.confirm.manual"
	defaultUnknown := "wallet.transactions.confirm.unknown"

	if tcm == nil || tcm.config == nil {
		switch mq.AddressMonitorSource(source) {
		case mq.AddressMonitorSourceDeposit:
			return defaultDeposit
		case mq.AddressMonitorSourceWeb3:
			return defaultWeb3
		case mq.AddressMonitorSourceVault, mq.AddressMonitorSourceCompany:
			return defaultVault
		case mq.AddressMonitorSourceManual:
			return defaultManual
		default:
			return defaultUnknown
		}
	}

	topics := tcm.config.Kafka.Topics
	switch mq.AddressMonitorSource(source) {
	case mq.AddressMonitorSourceDeposit:
		if strings.TrimSpace(topics.TransactionDeposit) != "" {
			return topics.TransactionDeposit
		}
		return defaultDeposit
	case mq.AddressMonitorSourceWeb3:
		if strings.TrimSpace(topics.TransactionWeb3) != "" {
			return topics.TransactionWeb3
		}
		return defaultWeb3
	case mq.AddressMonitorSourceVault, mq.AddressMonitorSourceCompany:
		if strings.TrimSpace(topics.TransactionVault) != "" {
			return topics.TransactionVault
		}
		return defaultVault
	case mq.AddressMonitorSourceManual:
		if strings.TrimSpace(topics.TransactionManual) != "" {
			return topics.TransactionManual
		}
		return defaultManual
	default:
		if strings.TrimSpace(topics.TransactionUnknown) != "" {
			return topics.TransactionUnknown
		}
		return defaultUnknown
	}
}

func txConfirmKafkaKey(tx *model.UnconfirmedTransaction) string {
	if tx == nil {
		return ""
	}
	return fmt.Sprintf("%s:%s:%s", strings.TrimSpace(tx.Source), strings.TrimSpace(tx.Chain), strings.TrimSpace(tx.MonitoredAddress))
}

func counterpartySourcesFromBits(bits uint8) []mq.AddressMonitorSource {
	if bits == 0 {
		return nil
	}
	sources := sourcesFromBits(addressSourceBits(bits))
	if len(sources) == 0 {
		return nil
	}
	return sources
}

// enqueueFailedKafkaMessage 将失败的Kafka消息加入重试队列
func (tcm *TransactionConfirmManager) enqueueFailedKafkaMessage(ctx context.Context, tx *model.UnconfirmedTransaction, originalErr error) {
	if tcm.kafkaRetryQueue == nil {
		return
	}

	message := mq.TransactionConfirmMessage{
		Version:                1,
		EventID:                txConfirmEventID(tx),
		Source:                 mq.AddressMonitorSource(tx.Source),
		Direction:              tx.Direction,
		CounterpartyAddress:    tx.CounterpartyAddress,
		MonitoredIsInternal:    tx.MonitoredIsInternal,
		CounterpartyIsInternal: tx.CounterpartyIsInternal,
		CounterpartySources:    counterpartySourcesFromBits(tx.CounterpartySourceBits),
		CounterpartySourceBits: tx.CounterpartySourceBits,
		TxHash:                 tx.TxHash,
		Chain:                  tx.Chain,
		ChainId:                tcm.chainIDForChainString(tx.Chain),
		BlockNumber:            tx.BlockNumber,
		BlockHash:              tx.BlockHash,
		FromAddress:            tx.FromAddress,
		ToAddress:              tx.ToAddress,
		MonitoredAddress:       tx.MonitoredAddress,
		Value:                  tx.Value,
		GasPrice:               tx.GasPrice,
		GasUsed:                tx.GasUsed,
		GasFee:                 tx.GasFee,
		TransactionIndex:       tx.TransactionIndex,
		BlockTimestamp:         tx.BlockTimestamp,
		Confirmations:          tx.Confirmations,
		RequiredConfirmations:  tx.RequiredConfirmations,
		CreatedAt:              tx.CreatedAt,
		TransactionType:        tx.TransactionType,
		LogIndex:               tx.LogIndex,
		TokenAddress:           tx.TokenAddress,
		TokenName:              tx.TokenName,
		TokenSymbol:            tx.TokenSymbol,
		TokenAmount:            tx.TokenAmount,
	}
	message.TokenDecimals = tokenDecimalsForConfirmMessage(tx)

	topic := tcm.getTransactionTopicForSource(tx.Source)
	key := txConfirmKafkaKey(tx)

	if err := tcm.kafkaRetryQueue.EnqueueFailedMessage(
		ctx,
		topic,
		key,
		message,
		"transaction_confirmed",
		tx.TxHash,
		tx.Chain,
		"transaction_confirm_manager",
		0,
	); err != nil {
		logx.Errorf("Failed to enqueue Kafka message for retry: %v (original error: %v)", err, originalErr)
	}
}

// sendToKafka 发送交易到Kafka（统一主币和代币交易）
func (tcm *TransactionConfirmManager) sendToKafka(tx *model.UnconfirmedTransaction) error {
	if tcm.kafkaProducer == nil {
		return fmt.Errorf("kafka producer not initialized")
	}

	message := mq.TransactionConfirmMessage{
		Version:                1,
		EventID:                txConfirmEventID(tx),
		Source:                 mq.AddressMonitorSource(tx.Source),
		Direction:              tx.Direction,
		CounterpartyAddress:    tx.CounterpartyAddress,
		MonitoredIsInternal:    tx.MonitoredIsInternal,
		CounterpartyIsInternal: tx.CounterpartyIsInternal,
		CounterpartySources:    counterpartySourcesFromBits(tx.CounterpartySourceBits),
		CounterpartySourceBits: tx.CounterpartySourceBits,
		TxHash:                 tx.TxHash,
		Chain:                  tx.Chain,
		ChainId:                tcm.chainIDForChainString(tx.Chain),
		BlockNumber:            tx.BlockNumber,
		BlockHash:              tx.BlockHash,
		FromAddress:            tx.FromAddress,
		ToAddress:              tx.ToAddress,
		MonitoredAddress:       tx.MonitoredAddress,
		Value:                  tx.Value,
		GasPrice:               tx.GasPrice,
		GasUsed:                tx.GasUsed,
		GasFee:                 tx.GasFee,
		TransactionIndex:       tx.TransactionIndex,
		BlockTimestamp:         tx.BlockTimestamp,
		Confirmations:          tx.Confirmations,
		RequiredConfirmations:  tx.RequiredConfirmations,
		CreatedAt:              tx.CreatedAt,
		TransactionType:        tx.TransactionType,
		LogIndex:               tx.LogIndex,
		TokenAddress:           tx.TokenAddress,
		TokenName:              tx.TokenName,
		TokenSymbol:            tx.TokenSymbol,
		TokenAmount:            tx.TokenAmount,
	}
	message.TokenDecimals = tokenDecimalsForConfirmMessage(tx)

	messageBytes, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %v", err)
	}

	topic := tcm.getTransactionTopicForSource(tx.Source)
	key := txConfirmKafkaKey(tx)

	messageID, err := tcm.kafkaProducer.SendMessage(topic, key, messageBytes)
	if err != nil {
		return fmt.Errorf("failed to send message: %v", err)
	}

	tx.MessageID = messageID
	tx.MessageTopic = topic

	logx.Infof("✅ Sent %s %s transaction %s to Kafka (topic: %s, key: %s)",
		tx.Source, tx.TransactionType, tx.TxHash, topic, key)

	return nil
}

// tokenDecimalsForConfirmMessage 代币交易必须带 token_decimals；若 DB 未存或为 0 则用链默认值，确保下游不拒绝。
func tokenDecimalsForConfirmMessage(tx *model.UnconfirmedTransaction) uint8 {
	if tx.TransactionType != "token" {
		return 0
	}
	if tx.TokenDecimals != nil && *tx.TokenDecimals > 0 {
		return *tx.TokenDecimals
	}
	return defaultTokenDecimalsForChainString(tx.Chain)
}

func defaultTokenDecimalsForChainString(chain string) uint8 {
	chain = strings.ToUpper(strings.TrimSpace(chain))
	switch {
	case strings.Contains(chain, "TRON") || chain == "TRX":
		return 6
	case strings.Contains(chain, "ETH") || strings.Contains(chain, "BSC"):
		return 18
	default:
		return 18
	}
}

// GetBalanceChangeTopic 获取余额变动主题（供外部使用）
func (tcm *TransactionConfirmManager) GetBalanceChangeTopic() string {
	defaultTopic := "wallet.balance.changes"
	if tcm.config == nil {
		return defaultTopic
	}
	if tcm.config.Kafka.Topics.BalanceChange != "" {
		return tcm.config.Kafka.Topics.BalanceChange
	}
	return defaultTopic
}
