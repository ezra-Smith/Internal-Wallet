package svc

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"sync"

	"internalwallet/proto/pb"
	"internalwallet/services/chainsync/rpc/internal/config"
	"internalwallet/services/chainsync/rpc/internal/provider"
	tronutil "internalwallet/services/chainsync/rpc/internal/provider/tron"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/zeromicro/go-zero/core/logx"
)

// TokenTransaction 代币交易信息
type TokenTransaction struct {
	// 基础交易信息
	TxHash      string            `json:"tx_hash"`
	Chain       pb.BlockChainType `json:"chain"`
	BlockNumber uint64            `json:"block_number"`
	BlockHash   string            `json:"block_hash"`
	From        string            `json:"from"`
	To          string            `json:"to"`
	Status      uint8             `json:"status"`
	Timestamp   int64             `json:"timestamp"`
	// EventIndex is a stable per-tx event index used to deduplicate and track multiple balance changes.
	// - ERC20/TRC20 Transfer: use chain log_index (parsed to int)
	// - Internal transfers: use offset + trace_address_index
	// - Native/simple transfer: 0
	EventIndex int32 `json:"event_index"`

	// 代币特定信息
	TokenAddress    string `json:"token_address"`
	TokenName       string `json:"token_name"`
	TokenSymbol     string `json:"token_symbol"`
	TokenDecimals   uint8  `json:"token_decimals"`
	TokenAmount     string `json:"token_amount"`
	TokenValue      string `json:"token_value"`      // 转换后的金额（考虑小数位）
	TransactionType string `json:"transaction_type"` // "native" 或 "token"
}

// AddressMonitorInterface 地址监控接口
type AddressMonitorInterface interface {
	IsAddressPoolMonitoringEnabled() bool
	IsAddressMonitored(chain pb.BlockChainType, address string) bool
}

// TokenParser 代币解析器
type TokenParser struct {
	erc20ABI       abi.ABI
	erc20QueryABI  abi.ABI // ERC20查询ABI，用于获取代币信息
	addressMonitor AddressMonitorInterface
	providerPool   *provider.Pool
	config         *config.Config // 配置信息

	// 代币信息缓存
	tokenCache map[string]*TokenInfo // key: chain:address, value: TokenInfo
	cacheMutex sync.RWMutex          // 缓存读写锁
}

// ERC20事件签名
const (
	ERC20TransferEvent = "Transfer(address,address,uint256)"
)

// ERC20标准ABI函数（用于查询代币信息）
var erc20ABI = `[{
	"constant": true,
	"inputs": [],
	"name": "name",
	"outputs": [{"name": "", "type": "string"}],
	"payable": false,
	"stateMutability": "view",
	"type": "function"
}, {
	"constant": true,
	"inputs": [],
	"name": "symbol",
	"outputs": [{"name": "", "type": "string"}],
	"payable": false,
	"stateMutability": "view",
	"type": "function"
}, {
	"constant": true,
	"inputs": [],
	"name": "decimals",
	"outputs": [{"name": "", "type": "uint8"}],
	"payable": false,
	"stateMutability": "view",
	"type": "function"
}, {
	"constant": true,
	"inputs": [],
	"name": "totalSupply",
	"outputs": [{"name": "", "type": "uint256"}],
	"payable": false,
	"stateMutability": "view",
	"type": "function"
}]`

// NewTokenParser 创建代币解析器
func NewTokenParser(addressMonitor AddressMonitorInterface, providerPool *provider.Pool, cfg *config.Config) (*TokenParser, error) {
	// 解析ERC20 Transfer事件ABI
	transferEventABI, err := abi.JSON(strings.NewReader(`[
		{
			"anonymous": false,
			"inputs": [
				{"indexed": true, "name": "from", "type": "address"},
				{"indexed": true, "name": "to", "type": "address"},
				{"indexed": false, "name": "value", "type": "uint256"}
			],
			"name": "Transfer",
			"type": "event"
		}
	]`))
	if err != nil {
		return nil, fmt.Errorf("failed to parse ERC20 Transfer ABI: %v", err)
	}

	// 解析ERC20查询ABI
	queryABI, err := abi.JSON(strings.NewReader(erc20ABI))
	if err != nil {
		return nil, fmt.Errorf("failed to parse ERC20 query ABI: %v", err)
	}

	tp := &TokenParser{
		erc20ABI:       transferEventABI,
		addressMonitor: addressMonitor,
		providerPool:   providerPool,
		config:         cfg,
		tokenCache:     make(map[string]*TokenInfo),
	}

	// 存储查询ABI用于链上查询
	tp.erc20QueryABI = queryABI

	// 初始化预配置的代币信息
	tp.initializePreconfiguredTokens()

	return tp, nil
}

// ParseTransaction 解析交易，区分主币和代币交易
func (tp *TokenParser) ParseTransaction(tx *pb.Transaction) ([]*TokenTransaction, error) {
	if tx == nil {
		return nil, fmt.Errorf("transaction is nil")
	}

	// ============================================================
	// 修复：优先检查 Logs 和 Internal Transactions
	// 对于 DEX Swap 等复杂交易：
	// - Token 转账在 Logs 中（ERC20 Transfer 事件）
	// - ETH 转账可能在 Internal Transactions 中
	// ============================================================

	results := []*TokenTransaction{}

	// 第一步：解析 Logs 中的 ERC20/TRC20 Transfer 事件
	if len(tx.Logs) > 0 {
		erc20Txs, err := tp.parseERC20Transactions(tx)
		if err != nil {
			logx.Errorf("Failed to parse ERC20 transactions from Logs for %s: %v", tx.TxHash, err)
		}
		if len(erc20Txs) > 0 {
			results = append(results, erc20Txs...)
			logx.Debugf("✅ Tx %s: parsed %d token transfers from Logs", tx.TxHash, len(erc20Txs))
		}
	}

	// 第二步：解析 Internal Transactions 中的 ETH 转账
	if len(tx.InternalTransactions) > 0 {
		internalTxs := tp.parseInternalTransactions(tx)
		if len(internalTxs) > 0 {
			results = append(results, internalTxs...)
			logx.Debugf("✅ Tx %s: parsed %d internal transactions", tx.TxHash, len(internalTxs))
		}
	}

	// 第三步：如果以上都没有解析出结果，按原逻辑处理
	if len(results) == 0 {
		if tx.ContractAddress != "" {
			return tp.parseContractTransaction(tx)
		}
		return tp.parseNonContractTransaction(tx)
	}

	// 返回所有解析结果
	logx.Infof("🎯 Tx %s: total parsed %d transactions", tx.TxHash, len(results))
	return results, nil
}

// parseContractTransaction 解析合约交易（有 ContractAddress 但没有 Logs 或 Logs 中无 Transfer）
func (tp *TokenParser) parseContractTransaction(tx *pb.Transaction) ([]*TokenTransaction, error) {
	// 注意：此方法现在只在 ParseTransaction 中没有从 Logs 解析出结果时才会被调用
	// 因此这里不再重复解析 Logs，而是尝试从 Provider 解析的 Value 中获取代币信息
	//
	// 关键安全规则：
	// - 没有 Logs 时，从 input_data 推断 ERC20/TRC20 transfer 会产生“假阳性”（例如 TRON 合约 REVERT 仍有 calldata）
	// - 但这里的“CONFIRMED”在本系统中常被用作“终确认（达到 required confirmations）”；
	//   对于已上链但确认数不足的交易，tx.Status 可能仍是 PENDING。
	// - 只要交易已上链（有 block_number/block_hash），就允许解析 input_data 生成 tokenTx，
	//   由后续确认管理器按 confirmations 再决定何时下发。
	// - **FAILED 必须严格禁止**：即使已上链/有区块信息，也不应解析，否则会把失败交易错误入库。
	if tx.Status == pb.TransactionStatus_TRANSACTION_STATUS_FAILED {
		logx.Infof("⛔️ Skip contract token parsing for failed tx: tx_hash=%s chain=%v status=%v",
			tx.TxHash, tx.Chain, tx.Status,
		)
		return nil, nil
	}
	mined := tx.BlockNumber > 0 || strings.TrimSpace(tx.BlockHash) != ""
	if tx.Status != pb.TransactionStatus_TRANSACTION_STATUS_CONFIRMED && !mined {
		logx.Infof("⛔️ Skip contract token parsing for non-mined tx: tx_hash=%s chain=%v status=%v",
			tx.TxHash, tx.Chain, tx.Status,
		)
		return nil, nil
	}

	// ============================================================
	// ✅ 支持“简单 ERC20 transfer(0xa9059cbb)”在没有 Logs 的情况下解析
	//
	// 很多 Provider（包括 SelfHostedProvider）会对简单 transfer 不拉取收据 Logs。
	// 这时仍然可以从 input_data 解析出：
	// - 实际收款地址（_to）
	// - 转账金额（_value，最小单位 raw）
	// 然后再用合约地址查询 decimals/symbol 计算显示值。
	// ============================================================
	if tx.ContractAddress != "" && len(tx.InputData) >= 4+32+32 {
		// ERC20/TRC20 transfer 方法签名: a9059cbb
		if hex.EncodeToString(tx.InputData[:4]) == "a9059cbb" {
			// param1: address _to (32 bytes, right-most 20 bytes)
			toBytes := tx.InputData[4+12 : 4+32]

			// param2: uint256 _value (32 bytes big-endian)
			amountBytes := tx.InputData[4+32 : 4+64]
			amount := new(big.Int).SetBytes(amountBytes)

			var toAddr string
			switch tx.Chain {
			case pb.BlockChainType_CHAIN_TYPE_ETHEREUM, pb.BlockChainType_CHAIN_TYPE_BSC:
				toAddr = common.BytesToAddress(toBytes).String() // checksum
			case pb.BlockChainType_CHAIN_TYPE_TRON:
				toAddr = tronutil.HexToBase58("41" + hex.EncodeToString(toBytes))
			default:
				toAddr = ""
			}

			// 检查是否与监控地址相关
			if toAddr != "" && tp.isAddressRelevant(tx.Chain, tx.FromAddress, toAddr) {
				tokenInfo := tp.GetTokenInfo(tx.Chain, tx.ContractAddress)
				tokenValue := tp.calculateTokenValue(amount, tokenInfo.Decimals)

				tokenTx := &TokenTransaction{
					TxHash:          tx.TxHash,
					Chain:           tx.Chain,
					BlockNumber:     tx.BlockNumber,
					From:            tx.FromAddress,
					To:              toAddr,
					Status:          uint8(tx.Status),
					Timestamp:       tx.BlockTimestamp,
					EventIndex:      0,
					TokenAddress:    tx.ContractAddress,
					TokenSymbol:     tokenInfo.Symbol,
					TokenName:       tokenInfo.Name,
					TokenDecimals:   tokenInfo.Decimals,
					TokenAmount:     amount.String(), // ✅ raw 最小单位
					TokenValue:      tokenValue,      // ✅ 按 decimals 计算后的显示值
					TransactionType: "token",
				}

				if tx.Chain == pb.BlockChainType_CHAIN_TYPE_TRON {
					logx.Infof("🔄 Contract tx %s: parsed TRC20 transfer from input_data (%s -> %s, %s %s)",
						tx.TxHash, tx.FromAddress, toAddr, tokenValue, tokenInfo.Symbol)
				} else {
					logx.Infof("🔄 Contract tx %s: parsed ERC20 transfer from input_data (%s -> %s, %s %s)",
						tx.TxHash, tx.FromAddress, toAddr, tokenValue, tokenInfo.Symbol)
				}
				return []*TokenTransaction{tokenTx}, nil
			}
		}
	}

	// 方法：尝试从 Provider 解析的 Value 获取代币信息
	if tx.Value != "" && strings.Contains(tx.Value, " ") {
		parts := strings.SplitN(tx.Value, " ", 2)
		if len(parts) == 2 {
			amount := strings.TrimSpace(parts[0])
			tokenSymbol := strings.TrimSpace(parts[1])

			// 排除主币交易（TRX、ETH、BNB 等）
			upperSymbol := strings.ToUpper(tokenSymbol)
			if upperSymbol != "TRX" && upperSymbol != "ETH" && upperSymbol != "BNB" {
				// 检查是否是监控地址相关
				isRelevant := tp.isAddressRelevant(tx.Chain, tx.FromAddress, tx.ToAddress)
				if isRelevant {
					// 获取代币信息
					tokenInfo := tp.GetTokenInfo(tx.Chain, tx.ContractAddress)

					tokenTx := &TokenTransaction{
						TxHash:          tx.TxHash,
						Chain:           tx.Chain,
						BlockNumber:     tx.BlockNumber,
						From:            tx.FromAddress,
						To:              tx.ToAddress,
						Status:          uint8(tx.Status),
						Timestamp:       tx.BlockTimestamp,
						EventIndex:      0,
						TokenAddress:    tx.ContractAddress,
						TokenSymbol:     tokenInfo.Symbol,
						TokenName:       tokenInfo.Name,
						TokenDecimals:   tokenInfo.Decimals,
						TokenAmount:     amount,
						TokenValue:      tx.Value,
						TransactionType: "token",
					}

					logx.Infof("🔄 Contract tx %s: parsed from Provider Value %s", tx.TxHash, tx.Value)
					return []*TokenTransaction{tokenTx}, nil
				}
			}
		}
	}

	// 方法 3：无法解析（没有 Logs，Value 也没有代币信息）
	// 降级为 Debug 日志，因为这是正常的跳过情况（不相关的合约交易）
	logx.Debugf("Contract tx %s (contract: %s) has no Logs or parsed Value - skipping",
		tx.TxHash, tx.ContractAddress)
	return []*TokenTransaction{}, nil
}

// parseNonContractTransaction 解析非合约交易（没有 ContractAddress）
func (tp *TokenParser) parseNonContractTransaction(tx *pb.Transaction) ([]*TokenTransaction, error) {
	// 检查是否是 TRC-10 代币转账（TRON 原生代币，不产生 Logs，没有 ContractAddress）
	if tx.Chain == pb.BlockChainType_CHAIN_TYPE_TRON {
		trc10Tx := tp.parseTRC10Transaction(tx)
		if trc10Tx != nil {
			logx.Debugf("🪙 Transaction %s: TRC-10 transfer", tx.TxHash)
			return []*TokenTransaction{trc10Tx}, nil
		}
	}

	// 否则就是主币交易（native）
	nativeTx := &TokenTransaction{
		TxHash:          tx.TxHash,
		Chain:           tx.Chain,
		BlockNumber:     tx.BlockNumber,
		From:            tx.FromAddress,
		To:              tx.ToAddress,
		Status:          uint8(tx.Status),
		Timestamp:       tx.BlockTimestamp,
		EventIndex:      0,
		TokenAmount:     tx.Value,
		TokenValue:      tx.Value,
		TransactionType: "native",
	}

	logx.Debugf("💰 Transaction %s: native transfer %s", tx.TxHash, tx.Value)
	return []*TokenTransaction{nativeTx}, nil
}
