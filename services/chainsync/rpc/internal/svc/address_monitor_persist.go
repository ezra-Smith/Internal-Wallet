package svc

import (
	"context"
	"fmt"
	"strings"
	"time"

	"internalwallet/proto/pb"
	chainModel "internalwallet/services/chainsync/rpc/internal/model"

	"github.com/zeromicro/go-zero/core/logx"
)

// saveUnconfirmedTransactionWithIndex 保存未确认交易到数据库（支持 log_index，用于多地址/多Transfer场景）
func (am *AddressMonitor) saveUnconfirmedTransactionWithIndex(
	tx *pb.Transaction,
	monitoredAddress string,
	source string,
	direction string,
	counterpartyAddress string,
	monitoredIsInternal bool,
	counterpartyIsInternal bool,
	counterpartySourceBits uint8,
	tokenTx *TokenTransaction,
	logIndex int32,
) error {
	if am.unconfirmedTransactionRepo == nil {
		return fmt.Errorf("unconfirmedTransaction repository not available")
	}
	if strings.TrimSpace(source) == "" {
		return fmt.Errorf("source is empty")
	}
	if strings.TrimSpace(direction) == "" {
		return fmt.Errorf("direction is empty")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 检查是否已存在（基于 tx_hash + monitored_address + log_index 去重）
	// 说明：同一交易的同一个事件，可能同时影响多个监控地址（from/to 两端）；
	// 这里必须按“监控地址维度”保存，否则会丢掉收款地址一侧，导致充值无法入账。
	existingTx, err := am.unconfirmedTransactionRepo.GetByTxHashAndAddressSource(ctx, tx.TxHash, monitoredAddress, source, logIndex)
	if err != nil {
		logx.Errorf("Failed to check existing transaction %s (log_index=%d): %v",
			tx.TxHash, logIndex, err)
		return err
	}

	if existingTx != nil {
		if tx != nil && strings.EqualFold(tx.TxHash, "0x1bb4a04aa7bb56e9973c1416bb7db5d599dfadef6f588bcf10e96431a4fdd5c9") {
			// #region agent log
			writeDebugLog(map[string]interface{}{
				"sessionId":    "debug-session",
				"runId":        "pre-fix",
				"hypothesisId": "H4",
				"location":     "address_monitor.go:saveUnconfirmedTransactionWithIndex:exists",
				"message":      "unconfirmed tx already exists",
				"data": map[string]interface{}{
					"txHash":      tx.TxHash,
					"logIndex":    logIndex,
					"monitorAddr": monitoredAddress,
				},
				"timestamp": time.Now().UnixMilli(),
			})
			// #endregion
		}
		logx.Debugf("Transaction %s (log_index=%d) already exists (first_monitored_address=%s), skipping duplicate save",
			tx.TxHash, logIndex, existingTx.MonitoredAddress)
		return nil
	}

	// 获取链的确认数配置
	requiredConfirmations := int32(12)
	if am.config != nil {
		switch tx.Chain {
		case pb.BlockChainType_CHAIN_TYPE_ETHEREUM:
			requiredConfirmations = int32(am.config.Chains.Ethereum.Confirmations)
		case pb.BlockChainType_CHAIN_TYPE_BSC:
			requiredConfirmations = int32(am.config.Chains.BSC.Confirmations)
		case pb.BlockChainType_CHAIN_TYPE_TRON:
			requiredConfirmations = int32(am.config.Chains.Tron.Confirmations)
		}
	}

	// 使用 tokenTx 的信息（更准确）
	fromAddress := tokenTx.From
	toAddress := tokenTx.To
	value := tokenTx.TokenValue

	// TokenValue 已经是格式化的纯数值（不带单位）
	// 代币单位通过 TokenSymbol 字段单独存储

	// 获取 gas 信息（保持原有逻辑）
	gasUsed := tx.GasUsed
	gasFee := tx.GasFee
	gasPrice := tx.GasPrice

	// 如果需要，查询完整交易信息获取 gas
	if (gasUsed == 0 || gasFee == "") && am.providerPool != nil {
		provider, err := am.providerPool.GetProvider(tx.Chain)
		if err == nil && provider != nil {
			fullTx, err := provider.GetTransaction(ctx, tx.TxHash)
			if err == nil && fullTx != nil {
				if fullTx.GasUsed > 0 {
					gasUsed = fullTx.GasUsed
				}
				if fullTx.GasFee != "" {
					gasFee = fullTx.GasFee
				}
				if fullTx.GasPrice != "" {
					gasPrice = fullTx.GasPrice
				}
			}
		}
	}

	// 计算 gas_fee（如果需要）
	if gasFee == "" && gasPrice != "" && gasUsed > 0 {
		gasFee = calculateGasFee(gasPrice, gasUsed)
	}

	// 格式化 gas 相关字段
	formattedGasPrice := gasPrice
	if tx.Chain == pb.BlockChainType_CHAIN_TYPE_ETHEREUM || tx.Chain == pb.BlockChainType_CHAIN_TYPE_BSC {
		formattedGasPrice = formatGasPriceToGwei(gasPrice)
	}

	formattedGasFee := gasFee
	if gasFee != "" && !isAlreadyFormatted(gasFee) {
		formattedGasFee = formatGasFee(gasFee, tx.Chain)
	}

	// 统一时间戳为秒级
	blockTimestamp := tx.BlockTimestamp
	if tx.Chain == pb.BlockChainType_CHAIN_TYPE_TRON && blockTimestamp > 1e12 {
		blockTimestamp = blockTimestamp / 1000
	}

	// 初始化链上执行状态（robust 模式：允许 unknown 入库，但确认/下发前会二次校验回执）
	now := time.Now().Local()
	execStatus := chainModel.ExecStatusUnknown
	var execCheckedAt *time.Time
	var execNextCheckAt *time.Time
	execErrMsg := ""
	switch tx.Status {
	case pb.TransactionStatus_TRANSACTION_STATUS_CONFIRMED:
		execStatus = chainModel.ExecStatusConfirmed
		execCheckedAt = &now
	case pb.TransactionStatus_TRANSACTION_STATUS_FAILED:
		execStatus = chainModel.ExecStatusFailed
		execCheckedAt = &now
		execErrMsg = "tx status failed at ingestion"
	default:
		// PENDING/UNSPECIFIED: throttle subsequent receipt checks (done by confirm manager)
		execStatus = chainModel.ExecStatusUnknown
		execNextCheckAt = func() *time.Time {
			t := now.Add(30 * time.Second)
			return &t
		}()
	}

	// 创建未确认交易记录
	unconfirmedTx := &chainModel.UnconfirmedTransaction{
		TxHash:                 tx.TxHash,
		Chain:                  tx.Chain.String(),
		BlockNumber:            tx.BlockNumber,
		BlockHash:              tx.BlockHash,
		FromAddress:            fromAddress,
		ToAddress:              toAddress,
		MonitoredAddress:       monitoredAddress,
		Source:                 source,
		Direction:              direction,
		CounterpartyAddress:    counterpartyAddress,
		MonitoredIsInternal:    monitoredIsInternal,
		CounterpartyIsInternal: counterpartyIsInternal,
		CounterpartySourceBits: counterpartySourceBits,
		Value:                  value,
		GasPrice:               formattedGasPrice,
		GasUsed:                gasUsed,
		GasFee:                 formattedGasFee,
		TransactionIndex:       uint32(tx.TransactionIndex),
		BlockTimestamp:         blockTimestamp,

		ExecStatus:       execStatus,
		ExecCheckedAt:    execCheckedAt,
		ExecNextCheckAt:  execNextCheckAt,
		ExecErrorMessage: execErrMsg,

		Status:                0,
		Confirmations:         0,
		RequiredConfirmations: requiredConfirmations,
		MaxRetries:            3,
		LogIndex:              logIndex,
		TransactionType:       tokenTx.TransactionType, // "native" 或 "token"
		TokenAddress:          tokenTx.TokenAddress,
		TokenName:             tokenTx.TokenName,
		TokenSymbol:           tokenTx.TokenSymbol,
		TokenAmount:           tokenTx.TokenAmount,
	}

	// TRON 链特殊处理：填充 Energy 和 Bandwidth 字段
	if tx.Chain == pb.BlockChainType_CHAIN_TYPE_TRON {
		// ✅ 直接从 Transaction 的专用字段读取（已在扫块时解析）
		unconfirmedTx.EnergyUsed = tx.EnergyUsed
		unconfirmedTx.BandwidthUsed = tx.BandwidthUsed
		logx.Debugf("TRON tx %s: Energy=%d, Bandwidth=%d", tx.TxHash, tx.EnergyUsed, tx.BandwidthUsed)
	}

	// 代币交易必须持久化 TokenDecimals，下游 Business 要求必须传递，否则拒绝入账。
	if tokenTx.TransactionType == "token" {
		decimals := tokenTx.TokenDecimals
		if decimals == 0 {
			decimals = defaultTokenDecimalsForChain(tx.Chain)
			logx.Infof("Token tx %s: token_decimals was 0, using chain default %d for %v", tx.TxHash, decimals, tx.Chain)
		}
		unconfirmedTx.TokenDecimals = &decimals
	}

	// 保存到数据库
	if err := am.unconfirmedTransactionRepo.Create(ctx, unconfirmedTx); err != nil {
		logx.Errorf("Failed to save unconfirmed transaction %s for address %s: %v",
			tx.TxHash, monitoredAddress, err)
		return err
	}

	if tx != nil && strings.EqualFold(tx.TxHash, "0x1bb4a04aa7bb56e9973c1416bb7db5d599dfadef6f588bcf10e96431a4fdd5c9") {
		// #region agent log
		writeDebugLog(map[string]interface{}{
			"sessionId":    "debug-session",
			"runId":        "pre-fix",
			"hypothesisId": "H4",
			"location":     "address_monitor.go:saveUnconfirmedTransactionWithIndex:created",
			"message":      "unconfirmed tx saved",
			"data": map[string]interface{}{
				"txHash":      tx.TxHash,
				"logIndex":    logIndex,
				"monitorAddr": monitoredAddress,
				"tokenType":   tokenTx.TransactionType,
				"tokenSymbol": tokenTx.TokenSymbol,
				"value":       tokenTx.TokenValue,
			},
			"timestamp": time.Now().UnixMilli(),
		})
		// #endregion
	}

	logx.Infof("✅ Saved %s transaction %s for monitored address %s (log_index=%d)",
		tokenTx.TransactionType, tx.TxHash, monitoredAddress, logIndex)
	logx.Infof("   From: %s → To: %s", fromAddress, toAddress)
	logx.Infof("   Value: %s", value)
	if tokenTx.TransactionType == "token" {
		logx.Infof("   Token: %s (%s)", tokenTx.TokenSymbol, tokenTx.TokenAddress)
	}

	return nil
}

// defaultTokenDecimalsForChain 返回链上代币默认精度（当解析得到 0 时使用，确保下游必收 token_decimals）。
func defaultTokenDecimalsForChain(chain pb.BlockChainType) uint8 {
	switch chain {
	case pb.BlockChainType_CHAIN_TYPE_TRON:
		return 6
	case pb.BlockChainType_CHAIN_TYPE_ETHEREUM, pb.BlockChainType_CHAIN_TYPE_BSC:
		return 18
	default:
		return DefaultTokenDecimals
	}
}
