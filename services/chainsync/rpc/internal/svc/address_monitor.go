package svc

import (
	"strings"
	"sync"
	"time"

	"internalwallet/proto/pb"
	"internalwallet/services/chainsync/rpc/internal/config"
	"internalwallet/services/chainsync/rpc/internal/provider"
	tronutil "internalwallet/services/chainsync/rpc/internal/provider/tron"
	"internalwallet/services/chainsync/rpc/internal/repository"

	"github.com/zeromicro/go-zero/core/logx"
)

func writeDebugLog(_ map[string]interface{}) {}

// KafkaProducerInterface Kafka生产者接口
type KafkaProducerInterface interface {
	SendMessage(topic string, key string, message interface{}) (string, error)
	Close() error
}

// AddressMonitor 地址监控器
type AddressMonitor struct {
	providerPool               *provider.Pool
	addressMonitorRepo         repository.AddressMonitorRepository
	unconfirmedTransactionRepo repository.UnconfirmedTransactionRepository
	headTracker                *HeadTracker       // chain head cache (latest block per chain)
	redisCacheManager          *RedisCacheManager // 添加Redis缓存管理器
	config                     *config.Config     // 配置信息
	tokenParser                *TokenParser       // 代币解析器

	registry *AddressRegistry

	scanGuards map[pb.BlockChainType]chan struct{} // per-chain scan concurrency guard

	mu sync.RWMutex

	// 控制通道
	stopCh chan struct{}
	wg     sync.WaitGroup

	// Kafka生产者
	kafkaProducer KafkaProducerInterface

	// 地址加载器（用于从signer服务获取监控地址）
	addressLoader *AddressLoader

	// 重试配置
	retryConfig AddressRetryConfig
}

// NewAddressMonitor 创建地址监控器
func NewAddressMonitor(providerPool *provider.Pool, addressMonitorRepo repository.AddressMonitorRepository, unconfirmedTxRepo repository.UnconfirmedTransactionRepository, headTracker *HeadTracker, redisCacheManager *RedisCacheManager, kafkaProducer KafkaProducerInterface, config *config.Config, addressLoader *AddressLoader) *AddressMonitor {
	scanGuards := map[pb.BlockChainType]chan struct{}{
		pb.BlockChainType_CHAIN_TYPE_ETHEREUM: make(chan struct{}, 1),
		pb.BlockChainType_CHAIN_TYPE_BSC:      make(chan struct{}, 1),
		pb.BlockChainType_CHAIN_TYPE_TRON:     make(chan struct{}, 1),
	}

	// 创建代币解析器
	var tokenParser *TokenParser
	var tokenParserErr error

	am := &AddressMonitor{
		providerPool:               providerPool,
		addressMonitorRepo:         addressMonitorRepo,
		unconfirmedTransactionRepo: unconfirmedTxRepo,
		headTracker:                headTracker,
		redisCacheManager:          redisCacheManager,
		config:                     config,
		tokenParser:                nil, // 稍后初始化
		registry:                   NewAddressRegistry(config, addressLoader, addressMonitorRepo),
		scanGuards:                 scanGuards,
		stopCh:                     make(chan struct{}),
		kafkaProducer:              kafkaProducer,
		addressLoader:              addressLoader,
		retryConfig:                initRetryConfig(config),
	}

	// 创建代币解析器，传入地址监控器接口、统一服务商池和配置
	tokenParser, tokenParserErr = NewTokenParser(am, providerPool, config)
	if tokenParserErr != nil {
		logx.Errorf("Failed to create token parser: %v", tokenParserErr)
		tokenParser = nil
	} else {
		logx.Info("✅ Token parser initialized successfully with address monitoring and token caching")
	}
	am.tokenParser = tokenParser

	return am
}

// normalizeMonitoredAddress normalizes addresses for consistent matching.
//
//   - EVM chains (ETH/BSC): store and match in lower-case (addresses are case-insensitive).
//   - TRON: prefer base58 for matching (base58 is case-sensitive), but accept hex/0x forms
//     that appear in TVM logs/topics and normalize them to base58.
func normalizeMonitoredAddress(chain pb.BlockChainType, addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return ""
	}
	switch chain {
	case pb.BlockChainType_CHAIN_TYPE_ETHEREUM, pb.BlockChainType_CHAIN_TYPE_BSC:
		return strings.ToLower(addr)
	case pb.BlockChainType_CHAIN_TYPE_TRON:
		// Most TRON addresses are base58 (T...), keep as-is.
		// However TRC20 logs/topics sometimes expose 20-byte hex (with/without 0x) or 21-byte hex (41...).
		// Normalize those to base58 so matching works against our stored monitored addresses (base58).
		if strings.HasPrefix(addr, "T") {
			return addr
		}

		s := addr
		if strings.HasPrefix(strings.ToLower(s), "0x") {
			s = s[2:]
		}
		s = strings.TrimSpace(s)
		if s == "" {
			return addr
		}
		ls := strings.ToLower(s)

		// Fast hex check (avoid calling tronutil.HexToBase58 on base58 strings).
		isHex := true
		for i := 0; i < len(ls); i++ {
			c := ls[i]
			if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') {
				continue
			}
			isHex = false
			break
		}
		if !isHex {
			return addr
		}

		// 20-byte hex (40 chars) or TRON hex with 41-prefix (42 chars)
		if len(ls) == 40 || len(ls) == 42 {
			if out := tronutil.HexToBase58(ls); out != "" {
				return out
			}
			// Fallback: try adding 41 prefix for 20-byte hex.
			if len(ls) == 40 {
				if out := tronutil.HexToBase58("41" + ls); out != "" {
					return out
				}
			}
		}
		return addr
	default:
		return addr
	}
}

// StartMonitoring 启动地址监控
func (am *AddressMonitor) StartMonitoring() {
	select {
	case <-am.stopCh:
		logx.Info("⏭️ Address monitoring start skipped: already stopped")
		return
	default:
	}

	logx.Info("🔍 Starting multi-chain address monitoring service...")

	// #region agent log
	writeDebugLog(map[string]interface{}{
		"sessionId":    "debug-session",
		"runId":        "pre-fix",
		"hypothesisId": "H2",
		"location":     "address_monitor.go:StartMonitoring",
		"message":      "StartMonitoring invoked",
		"data": map[string]interface{}{
			"addressPoolMonitoring": am.IsAddressPoolMonitoringEnabled(),
		},
		"timestamp": time.Now().UnixMilli(),
	})
	// #endregion

	// 根据配置设置启动延迟
	initialDelay := 3 * time.Second
	if am.config != nil && am.config.Monitoring.AddressMonitoring != nil {
		initialDelay = time.Duration(am.config.Monitoring.AddressMonitoring.InitialDelay) * time.Second
	}

	// 等待服务完全启动
	time.Sleep(initialDelay)

	select {
	case <-am.stopCh:
		logx.Info("⏭️ Address monitoring start aborted: stopped during startup delay")
		return
	default:
	}

	// 启动时执行一次全量刷新（后续由定时刷新兜底）
	am.refreshAddressRegistry("startup")

	select {
	case <-am.stopCh:
		logx.Info("⏭️ Address monitoring start aborted: stopped during initial refresh")
		return
	default:
	}

	// 启动地址注册表定时刷新（copy-on-write + atomic swap）
	am.wg.Add(1)
	go am.addressRegistryRefreshLoop()

	// 启动 Kafka 地址事件消费者（增量更新，定时全量刷新兜底）
	am.startAddressEventConsumer()

	// 启动多链监控循环
	am.wg.Add(1)
	go am.monitoringLoop()

	logx.Info("✅ Multi-chain address monitoring service started successfully")
}

// StartMonitoringWithConfig 根据配置启动地址监控
func (am *AddressMonitor) StartMonitoringWithConfig(cfg *config.Config) {
	am.config = cfg
	if am.registry != nil {
		am.registry.cfg = cfg
	}

	// #region agent log
	writeDebugLog(map[string]interface{}{
		"sessionId":    "debug-session",
		"runId":        "pre-fix",
		"hypothesisId": "H2",
		"location":     "address_monitor.go:StartMonitoringWithConfig",
		"message":      "StartMonitoringWithConfig invoked",
		"data": map[string]interface{}{
			"addressPoolMonitoring": cfg.Monitoring.AddressPoolMonitoring,
			"ethEnabled":            cfg.Chains.Ethereum.Enabled,
			"bscEnabled":            cfg.Chains.BSC.Enabled,
			"tronEnabled":           cfg.Chains.Tron.Enabled,
			"checkInterval":         cfg.Monitoring.AddressMonitoring.CheckInterval,
		},
		"timestamp": time.Now().UnixMilli(),
	})
	// #endregion

	if !cfg.Monitoring.AddressPoolMonitoring {
		logx.Severef("Monitoring.AddressPoolMonitoring=false is no longer supported; chainsync now requires address-pool monitoring")
		return
	}

	logx.Info("🔍 Starting address pool monitoring mode...")
	am.StartMonitoring()
}

// monitoringLoop 监控循环
// scanNewBlocksBatch 扫描新区块并批量过滤相关地址
// processTransactionsBatch 批量处理交易

// saveUnconfirmedTransactionWithIndex 保存未确认交易到数据库（支持 log_index，用于多地址/多Transfer场景）
// isAlreadyFormatted 检查值是否已经格式化（包含单位符号)
// StopMonitoring 停止监控
func (am *AddressMonitor) StopMonitoring() {
	logx.Info("Stopping multi-chain address monitoring service...")

	close(am.stopCh)
	am.wg.Wait()

	logx.Info("✅ Multi-chain address monitoring service stopped")
}

// IsAddressPoolMonitoringEnabled 检查是否启用了地址池监控
func (am *AddressMonitor) IsAddressPoolMonitoringEnabled() bool {
	if am.config == nil {
		return false
	}
	return am.config.Monitoring.AddressPoolMonitoring
}

// IsAddressMonitored 检查指定链上的地址是否被监控
func (am *AddressMonitor) IsAddressMonitored(chain pb.BlockChainType, address string) bool {
	if am.registry == nil {
		return false
	}
	return am.registry.IsMonitored(chain, address)
}

func (am *AddressMonitor) IsInternalAddress(chain pb.BlockChainType, address string) bool {
	if am.registry == nil {
		return false
	}
	return am.registry.IsInternal(chain, address)
}

// GetMonitoringStatus 获取监控状态
func (am *AddressMonitor) GetMonitoringStatus() map[pb.BlockChainType]map[string]bool {
	status := make(map[pb.BlockChainType]map[string]bool)
	if am.registry == nil {
		return status
	}
	snap := am.registry.Snapshot()
	for chainType, cs := range snap.chains {
		if cs == nil || len(cs.addresses) == 0 {
			continue
		}
		m := make(map[string]bool, len(cs.addresses))
		for _, addr := range cs.addresses {
			m[addr] = true
		}
		status[chainType] = m
	}

	return status
}
