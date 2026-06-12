package svc

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"internalwallet/proto/pb"
	"internalwallet/services/chainsync/rpc/internal/config"
	"internalwallet/services/chainsync/rpc/internal/provider"
	"internalwallet/services/chainsync/rpc/internal/repository"

	"github.com/zeromicro/go-zero/core/logx"
)

// BalanceValidator 余额监控校验器
type BalanceValidator struct {
	providerPool       *provider.Pool
	addressMonitorRepo repository.AddressMonitorRepository
	redisCacheManager  *RedisCacheManager
	kafkaProducer      KafkaProducerInterface

	// 配置
	config        *config.Config
	checkInterval time.Duration // 校验间隔
	batchSize     int           // 批量查询大小
	isEnabled     bool          // 是否启用

	// 监控状态
	isRunning       bool
	validationCycle uint64 // 校验周期计数

	// 控制通道
	stopCh   chan struct{}
	wg       sync.WaitGroup
	stopOnce sync.Once

	// 统计信息
	stats *ValidationStats
	mu    sync.RWMutex
}

// ValidationStats 校验统计信息
type ValidationStats struct {
	TotalChecks   uint64    `json:"total_checks"`
	MatchCount    uint64    `json:"match_count"`
	MismatchCount uint64    `json:"mismatch_count"`
	ErrorCount    uint64    `json:"error_count"`
	LastCheckTime time.Time `json:"last_check_time"`
}

// BalanceDifference 余额差异信息
type BalanceDifference struct {
	Address         string `json:"address"`
	Chain           string `json:"chain"`
	CachedBalance   string `json:"cached_balance"`
	ActualBalance   string `json:"actual_balance"`
	LastUpdate      int64  `json:"last_update"`
	ValidationTime  int64  `json:"validation_time"`
	ValidationCycle uint64 `json:"validation_cycle"`
}

// KafkaBalanceMessage Kafka消息结构
type KafkaBalanceMessage struct {
	MessageType string             `json:"message_type"` // "balance_validation"
	Difference  *BalanceDifference `json:"difference"`
	Timestamp   int64              `json:"timestamp"`
	MessageID   string             `json:"message_id"`
}

// NewBalanceValidator 创建余额监控校验器
func NewBalanceValidator(
	providerPool *provider.Pool,
	addressMonitorRepo repository.AddressMonitorRepository,
	redisCacheManager *RedisCacheManager,
	kafkaProducer KafkaProducerInterface,
) *BalanceValidator {
	bv := &BalanceValidator{
		providerPool:       providerPool,
		addressMonitorRepo: addressMonitorRepo,
		redisCacheManager:  redisCacheManager,
		kafkaProducer:      kafkaProducer,
		checkInterval:      DefaultBalanceValidationCheckInterval, // 默认校验间隔
		batchSize:          DefaultBalanceValidationBatchSize,     // 每批处理个数
		isEnabled:          false,                                 // 默认禁用
		stopCh:             make(chan struct{}),
		stats:              &ValidationStats{},
	}

	return bv
}

// StartWithConfig 根据配置启动余额校验器
func (bv *BalanceValidator) StartWithConfig(cfg *config.Config) {
	bv.config = cfg

	select {
	case <-bv.stopCh:
		logx.Info("⏭️ Balance validation start skipped: already stopped")
		return
	default:
	}

	// 检查配置中是否启用了余额校验
	if cfg.Monitoring.BalanceValidation != nil {
		bv.isEnabled = cfg.Monitoring.BalanceValidation.Enabled
		bv.checkInterval = time.Duration(cfg.Monitoring.BalanceValidation.CheckInterval) * time.Minute
		bv.batchSize = cfg.Monitoring.BalanceValidation.BatchSize

		logx.Infof("Balance validation config: enabled=%v, interval=%v, batch_size=%d",
			bv.isEnabled, bv.checkInterval, bv.batchSize)
	}

	if !bv.isEnabled {
		logx.Info("Balance validation is disabled")
		return
	}

	bv.Start()
}

// Start 启动余额校验器
func (bv *BalanceValidator) Start() {
	select {
	case <-bv.stopCh:
		logx.Info("⏭️ Balance validation start skipped: already stopped")
		return
	default:
	}

	logx.Info("🚀 Starting balance validation service...")

	// 等待服务稳定
	time.Sleep(DefaultBalanceValidationStartDelay)

	select {
	case <-bv.stopCh:
		logx.Info("⏭️ Balance validation start aborted: stopped during startup delay")
		return
	default:
	}

	if bv.isRunning {
		logx.Info("Balance validator is already running")
		return
	}
	bv.isRunning = true

	// 启动主校验循环
	bv.wg.Add(1)
	go bv.validationLoop()

	logx.Info("✅ Balance validation service started successfully")
}

// Stop 停止余额校验器
func (bv *BalanceValidator) Stop() {
	bv.stopOnce.Do(func() {
		close(bv.stopCh)
	})

	if !bv.isRunning {
		return
	}

	logx.Info("Stopping balance validation service...")

	bv.isRunning = false
	bv.wg.Wait()

	// 输出最终统计信息
	bv.printStats()

	logx.Info("✅ Balance validation service stopped")
}

// validationLoop 主校验循环
func (bv *BalanceValidator) validationLoop() {
	defer bv.wg.Done()

	ticker := time.NewTicker(bv.checkInterval)
	defer ticker.Stop()

	logx.Infof("Balance validation loop started with interval: %v", bv.checkInterval)

	for {
		select {
		case <-bv.stopCh:
			logx.Info("Balance validation loop stopped")
			return

		case <-ticker.C:
			bv.performValidation()
		}
	}
}

// performValidation 执行余额校验
func (bv *BalanceValidator) performValidation() {
	startTime := time.Now()

	// 增加校验周期计数
	bv.mu.Lock()
	bv.validationCycle++
	currentCycle := bv.validationCycle
	bv.mu.Unlock()

	logx.Infof("🔍 Starting balance validation cycle #%d", currentCycle)

	// 获取所有监控地址
	addresses, err := bv.getMonitoredAddresses()
	if err != nil {
		logx.Errorf("Failed to get monitored addresses: %v", err)
		bv.incrementErrorCount()
		return
	}

	if len(addresses) == 0 {
		logx.Info("No addresses to validate")
		return
	}

	logx.Infof("Found %d addresses to validate", len(addresses))

	// 按链分组处理
	chainAddresses := bv.groupAddressesByChain(addresses)

	// 并行校验各链
	var wg sync.WaitGroup
	for chainType, addrList := range chainAddresses {
		if len(addrList) == 0 {
			continue
		}

		wg.Add(1)
		go func(ct pb.BlockChainType, addrs []string) {
			defer wg.Done()
			bv.validateChainBalances(ct, addrs, currentCycle)
		}(chainType, addrList)
	}

	wg.Wait()

	duration := time.Since(startTime)
	logx.Infof("✅ Balance validation cycle #%d completed in %v", currentCycle, duration)

	// 更新统计信息
	bv.updateLastCheckTime()
}

// getMonitoredAddresses 获取监控地址（复用现有逻辑）
func (bv *BalanceValidator) getMonitoredAddresses() ([]*pb.AddressMonitor, error) {
	if bv.addressMonitorRepo == nil {
		return nil, fmt.Errorf("address monitor repository is nil")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 从数据库获取活跃的监控地址
	monitorGorms, err := bv.addressMonitorRepo.GetActiveMonitors(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("failed to get active monitors: %v", err)
	}

	// 转换为pb.AddressMonitor类型
	var monitors []*pb.AddressMonitor
	for _, monitorGorm := range monitorGorms {
		monitor := &pb.AddressMonitor{
			Address: monitorGorm.Address,
			Chain:   bv.convertToPBChainType(monitorGorm.Chain),
		}
		monitors = append(monitors, monitor)
	}

	// 如果数据库中没有监控地址，使用默认测试地址（与AddressMonitor保持一致）
	if len(monitors) == 0 {
		monitors = bv.getDefaultTestAddresses()
		logx.Infof("Using %d default test addresses for validation", len(monitors))
	}

	return monitors, nil
}

// getDefaultTestAddresses 获取默认测试地址
func (bv *BalanceValidator) getDefaultTestAddresses() []*pb.AddressMonitor {
	var monitors []*pb.AddressMonitor

	// 以太坊测试地址
	ethAddresses := []string{
		"0xf91840b6bf92acab21a8c05977769088c461d43b",
		"0xeab3ee17c45bd1b9fae87bda998c217718cef7e5",
	}

	for _, addr := range ethAddresses {
		monitors = append(monitors, &pb.AddressMonitor{
			Address: addr,
			Chain:   pb.BlockChainType_CHAIN_TYPE_ETHEREUM,
		})
	}

	// BSC测试地址
	bscAddresses := []string{
		"0x37cc65754cf1c48d33390b1a82cda789b413df53",
		"0xEC6056edEe9cb85607fdd9b36B4A65B3ABcA66F4",
	}

	for _, addr := range bscAddresses {
		monitors = append(monitors, &pb.AddressMonitor{
			Address: addr,
			Chain:   pb.BlockChainType_CHAIN_TYPE_BSC,
		})
	}

	// TRON测试地址
	tronAddresses := []string{
		"TUz4nTU75z5oK4pYaVipkSDQ3Bi2DXdQT8",
		"TKdviYoTN5jDbdP7CXcfm2M8dbggpabMFr",
	}

	for _, addr := range tronAddresses {
		monitors = append(monitors, &pb.AddressMonitor{
			Address: addr,
			Chain:   pb.BlockChainType_CHAIN_TYPE_TRON,
		})
	}

	return monitors
}

// groupAddressesByChain 按链分组地址
func (bv *BalanceValidator) groupAddressesByChain(monitors []*pb.AddressMonitor) map[pb.BlockChainType][]string {
	chainAddresses := make(map[pb.BlockChainType][]string)

	for _, monitor := range monitors {
		chainAddresses[monitor.Chain] = append(chainAddresses[monitor.Chain], monitor.Address)
	}

	return chainAddresses
}

// validateChainBalances 校验指定链的地址余额
func (bv *BalanceValidator) validateChainBalances(chainType pb.BlockChainType, addresses []string, cycle uint64) {
	if len(addresses) == 0 {
		return
	}

	logx.Infof("🔍 Validating %d addresses on chain %v", len(addresses), chainType)

	// 获取服务商
	provider, err := bv.providerPool.GetProvider(chainType)
	if err != nil {
		logx.Errorf("Failed to get provider for chain %v: %v", chainType, err)
		bv.incrementErrorCount()
		return
	}

	// 批量校验地址余额
	for i := 0; i < len(addresses); i += bv.batchSize {
		end := i + bv.batchSize
		if end > len(addresses) {
			end = len(addresses)
		}

		batch := addresses[i:end]
		bv.validateAddressBatch(provider, chainType, batch, cycle)
	}
}

// validateAddressBatch 批量校验地址余额
func (bv *BalanceValidator) validateAddressBatch(provider Provider, chainType pb.BlockChainType, addresses []string, cycle uint64) {
	for _, address := range addresses {
		bv.validateSingleAddress(provider, chainType, address, cycle)
	}
}

// validateSingleAddress 校验单个地址余额
func (bv *BalanceValidator) validateSingleAddress(provider Provider, chainType pb.BlockChainType, address string, cycle uint64) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 获取链上实际余额 (需要tokens参数)
	actualBalanceResp, err := provider.GetAddressBalance(ctx, address, []string{})
	if err != nil {
		logx.Errorf("Failed to get balance for address %s on chain %v: %v", address, chainType, err)
		bv.incrementErrorCount()
		return
	}

	// 获取缓存余额
	cacheKey := fmt.Sprintf("balance:%s:%s", chainType.String(), address)
	cachedBalance, err := bv.redisCacheManager.Get(cacheKey)
	if err != nil {
		// 缓存不存在，记录并跳过
		logx.Infof("No cached balance for address %s on chain %v, skipping validation", address, chainType)
		return
	}

	// 获取实际余额字符串
	actualBalance := actualBalanceResp.NativeBalance

	// 比较余额
	bv.compareAndNotify(address, chainType, cachedBalance, actualBalance, cycle)
}

// compareAndNotify 比较余额并发送差异通知
func (bv *BalanceValidator) compareAndNotify(address string, chainType pb.BlockChainType, cachedBalance, actualBalance string, cycle uint64) {
	// 标准化余额值进行比较
	normalizedCached := bv.normalizeBalance(cachedBalance)
	normalizedActual := bv.normalizeBalance(actualBalance)

	bv.incrementTotalChecks()

	if normalizedCached == normalizedActual {
		// 余额一致
		bv.incrementMatchCount()
		logx.Debugf("✅ Balance matched for address %s: %s", address, actualBalance)
		return
	}

	// 余额不一致，记录差异
	bv.incrementMismatchCount()

	logx.Errorf("❌ Balance mismatch for address %s on chain %v: cached=%s, actual=%s",
		address, chainType, cachedBalance, actualBalance)

	// 创建差异信息
	difference := &BalanceDifference{
		Address:         address,
		Chain:           chainType.String(),
		CachedBalance:   cachedBalance,
		ActualBalance:   actualBalance,
		LastUpdate:      time.Now().Unix(),
		ValidationTime:  time.Now().Unix(),
		ValidationCycle: cycle,
	}

	// 发送Kafka消息
	bv.sendDifferenceMessage(difference)
}

// sendDifferenceMessage 发送差异消息到Kafka
func (bv *BalanceValidator) sendDifferenceMessage(difference *BalanceDifference) {
	if bv.kafkaProducer == nil {
		logx.Error("Kafka producer is nil, cannot send difference message")
		return
	}

	// 创建消息
	message := &KafkaBalanceMessage{
		MessageType: "balance_validation",
		Difference:  difference,
		Timestamp:   time.Now().Unix(),
		MessageID: fmt.Sprintf("balance_validation_%d_%s_%s",
			difference.ValidationCycle, difference.Chain, difference.Address),
	}

	// 序列化消息
	messageBytes, err := json.Marshal(message)
	if err != nil {
		logx.Errorf("Failed to marshal balance difference message: %v", err)
		return
	}

	// 确定主题和Key
	topic := "wallet.balance.validation"
	key := fmt.Sprintf("%s:%s", difference.Chain, difference.Address)

	// 发送消息
	messageID, err := bv.kafkaProducer.SendMessage(topic, key, messageBytes)
	if err != nil {
		logx.Errorf("Failed to send balance difference message for address %s: %v",
			difference.Address, err)
		bv.incrementErrorCount()
		return
	}

	logx.Infof("📤 Sent balance difference message for address %s: messageID=%s",
		difference.Address, messageID)
}
