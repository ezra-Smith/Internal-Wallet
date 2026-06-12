package svc

import (
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"internalwallet/proto/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

// BlockProgressTracker 区块进度追踪器
// 用于跟踪区块扫描进度，确保不会漏扫区块
type BlockProgressTracker struct {
	redisCacheManager *RedisCacheManager
	chain             pb.BlockChainType

	// 内存状态
	confirmedBlock uint64          // 已确认的连续区块（已处理且无间隙）
	pendingBlocks  map[uint64]bool // 已处理但尚未连续的区块
	failedBlocks   map[uint64]int  // 失败的区块及重试次数
	mu             sync.RWMutex

	// 配置
	maxRetries     int // 最大重试次数
	maxPendingSize int // 最大待确认区块数
}

// BlockProgressState 进度状态（用于持久化）
type BlockProgressState struct {
	ConfirmedBlock uint64         `json:"confirmed_block"`
	PendingBlocks  []uint64       `json:"pending_blocks"`
	FailedBlocks   map[uint64]int `json:"failed_blocks"`
	LastUpdate     int64          `json:"last_update"`
}

// NewBlockProgressTracker 创建区块进度追踪器
func NewBlockProgressTracker(redisCacheManager *RedisCacheManager, chain pb.BlockChainType) *BlockProgressTracker {
	bpt := &BlockProgressTracker{
		redisCacheManager: redisCacheManager,
		chain:             chain,
		pendingBlocks:     make(map[uint64]bool),
		failedBlocks:      make(map[uint64]int),
		maxRetries:        DefaultMaxRetries,
		maxPendingSize:    1000,
	}

	// 从 Redis 加载状态
	bpt.loadState()

	return bpt
}

// getRedisKey 获取 Redis 键
func (bpt *BlockProgressTracker) getRedisKey() string {
	return fmt.Sprintf("chainsync:block_progress:%s", bpt.chain.String())
}

// loadState 从 Redis 加载状态
func (bpt *BlockProgressTracker) loadState() {
	if bpt.redisCacheManager == nil {
		return
	}

	key := bpt.getRedisKey()
	value, err := bpt.redisCacheManager.Get(key)
	if err != nil || value == "" {
		return
	}

	var state BlockProgressState
	if err := json.Unmarshal([]byte(value), &state); err != nil {
		logx.Errorf("Failed to unmarshal block progress state for %s: %v", bpt.chain, err)
		return
	}

	bpt.mu.Lock()
	bpt.confirmedBlock = state.ConfirmedBlock
	bpt.failedBlocks = state.FailedBlocks
	if bpt.failedBlocks == nil {
		bpt.failedBlocks = make(map[uint64]int)
	}
	for _, block := range state.PendingBlocks {
		bpt.pendingBlocks[block] = true
	}
	bpt.mu.Unlock()

	logx.Infof("📂 Loaded block progress for %s: confirmed=%d, pending=%d, failed=%d",
		bpt.chain, state.ConfirmedBlock, len(state.PendingBlocks), len(state.FailedBlocks))
}

// saveState 保存状态到 Redis
func (bpt *BlockProgressTracker) saveState() {
	if bpt.redisCacheManager == nil {
		return
	}

	bpt.mu.RLock()
	pendingBlocks := make([]uint64, 0, len(bpt.pendingBlocks))
	for block := range bpt.pendingBlocks {
		pendingBlocks = append(pendingBlocks, block)
	}
	failedBlocks := make(map[uint64]int)
	for block, count := range bpt.failedBlocks {
		failedBlocks[block] = count
	}
	state := BlockProgressState{
		ConfirmedBlock: bpt.confirmedBlock,
		PendingBlocks:  pendingBlocks,
		FailedBlocks:   failedBlocks,
		LastUpdate:     time.Now().Unix(),
	}
	bpt.mu.RUnlock()

	data, err := json.Marshal(state)
	if err != nil {
		logx.Errorf("Failed to marshal block progress state: %v", err)
		return
	}

	key := bpt.getRedisKey()
	ttl := BlockProgressTTLSeconds
	if err := bpt.redisCacheManager.SetWithTTL(key, string(data), ttl); err != nil {
		logx.Errorf("Failed to save block progress state to Redis: %v", err)
	}
}

// GetConfirmedBlock 获取已确认的区块
func (bpt *BlockProgressTracker) GetConfirmedBlock() uint64 {
	bpt.mu.RLock()
	defer bpt.mu.RUnlock()
	return bpt.confirmedBlock
}

// SetConfirmedBlock 设置已确认的区块（仅用于初始化）
func (bpt *BlockProgressTracker) SetConfirmedBlock(block uint64) {
	bpt.mu.Lock()
	if block > bpt.confirmedBlock {
		bpt.confirmedBlock = block
	}
	bpt.mu.Unlock()
	bpt.saveState()
}

// MarkBlockProcessed 标记区块已处理
func (bpt *BlockProgressTracker) MarkBlockProcessed(block uint64) {
	bpt.mu.Lock()
	defer bpt.mu.Unlock()

	// 如果是下一个连续区块，直接更新 confirmedBlock
	if block == bpt.confirmedBlock+1 {
		bpt.confirmedBlock = block
		delete(bpt.failedBlocks, block)

		// 检查并合并待确认的连续区块
		bpt.mergeConfirmedBlocks()
	} else if block > bpt.confirmedBlock+1 {
		// 不是连续的，加入待确认列表
		bpt.pendingBlocks[block] = true
		delete(bpt.failedBlocks, block)
	}

	// 限制 pending 大小
	if len(bpt.pendingBlocks) > bpt.maxPendingSize {
		bpt.trimPendingBlocks()
	}
}

// mergeConfirmedBlocks 合并连续的已确认区块
func (bpt *BlockProgressTracker) mergeConfirmedBlocks() {
	for {
		nextBlock := bpt.confirmedBlock + 1
		if bpt.pendingBlocks[nextBlock] {
			delete(bpt.pendingBlocks, nextBlock)
			bpt.confirmedBlock = nextBlock
		} else {
			break
		}
	}
}

// trimPendingBlocks 裁剪待确认区块列表
func (bpt *BlockProgressTracker) trimPendingBlocks() {
	// 将较老的 pending 区块标记为失败
	blocks := make([]uint64, 0, len(bpt.pendingBlocks))
	for block := range bpt.pendingBlocks {
		blocks = append(blocks, block)
	}
	sort.Slice(blocks, func(i, j int) bool { return blocks[i] < blocks[j] })

	// 保留最新的一半
	removeCount := len(blocks) / 2
	for i := 0; i < removeCount; i++ {
		delete(bpt.pendingBlocks, blocks[i])
		bpt.failedBlocks[blocks[i]] = 1 // 标记为失败
	}
}

// MarkBlockFailed 标记区块处理失败
func (bpt *BlockProgressTracker) MarkBlockFailed(block uint64) {
	bpt.mu.Lock()
	defer bpt.mu.Unlock()

	bpt.failedBlocks[block]++
	delete(bpt.pendingBlocks, block)
}

// GetFailedBlocks 获取失败的区块（可重试的）
func (bpt *BlockProgressTracker) GetFailedBlocks() []uint64 {
	bpt.mu.RLock()
	defer bpt.mu.RUnlock()

	blocks := make([]uint64, 0)
	for block, count := range bpt.failedBlocks {
		if count < bpt.maxRetries {
			blocks = append(blocks, block)
		}
	}
	sort.Slice(blocks, func(i, j int) bool { return blocks[i] < blocks[j] })
	return blocks
}

// GetMissingBlocks 获取缺失的区块（未处理的间隙）
func (bpt *BlockProgressTracker) GetMissingBlocks(endBlock uint64) []uint64 {
	bpt.mu.RLock()
	defer bpt.mu.RUnlock()

	missing := make([]uint64, 0)
	for block := bpt.confirmedBlock + 1; block <= endBlock; block++ {
		if !bpt.pendingBlocks[block] {
			missing = append(missing, block)
		}
	}
	return missing
}

// GetProgress 获取进度信息
func (bpt *BlockProgressTracker) GetProgress() (confirmed uint64, pending int, failed int) {
	bpt.mu.RLock()
	defer bpt.mu.RUnlock()
	return bpt.confirmedBlock, len(bpt.pendingBlocks), len(bpt.failedBlocks)
}

// Flush 刷新状态到 Redis
func (bpt *BlockProgressTracker) Flush() {
	bpt.saveState()
}

// Reset 重置追踪器状态
func (bpt *BlockProgressTracker) Reset(startBlock uint64) {
	bpt.mu.Lock()
	bpt.confirmedBlock = startBlock
	bpt.pendingBlocks = make(map[uint64]bool)
	bpt.failedBlocks = make(map[uint64]int)
	bpt.mu.Unlock()
	bpt.saveState()
}
