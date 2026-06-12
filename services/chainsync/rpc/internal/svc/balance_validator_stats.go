package svc

import (
	"time"

	"github.com/zeromicro/go-zero/core/logx"

	"internalwallet/proto/pb"
)

// normalizeBalance 标准化余额值
func (bv *BalanceValidator) normalizeBalance(balance string) string {
	if balance == "" {
		return "0"
	}
	return balance
}

// 统计信息更新方法
func (bv *BalanceValidator) incrementTotalChecks() {
	bv.mu.Lock()
	bv.stats.TotalChecks++
	bv.mu.Unlock()
}

func (bv *BalanceValidator) incrementMatchCount() {
	bv.mu.Lock()
	bv.stats.MatchCount++
	bv.mu.Unlock()
}

func (bv *BalanceValidator) incrementMismatchCount() {
	bv.mu.Lock()
	bv.stats.MismatchCount++
	bv.mu.Unlock()
}

func (bv *BalanceValidator) incrementErrorCount() {
	bv.mu.Lock()
	bv.stats.ErrorCount++
	bv.mu.Unlock()
}

func (bv *BalanceValidator) updateLastCheckTime() {
	bv.mu.Lock()
	bv.stats.LastCheckTime = time.Now()
	bv.mu.Unlock()
}

// GetStats 获取统计信息
func (bv *BalanceValidator) GetStats() *ValidationStats {
	bv.mu.RLock()
	defer bv.mu.RUnlock()

	return &ValidationStats{
		TotalChecks:   bv.stats.TotalChecks,
		MatchCount:    bv.stats.MatchCount,
		MismatchCount: bv.stats.MismatchCount,
		ErrorCount:    bv.stats.ErrorCount,
		LastCheckTime: bv.stats.LastCheckTime,
	}
}

// printStats 打印统计信息
func (bv *BalanceValidator) printStats() {
	stats := bv.GetStats()
	logx.Infof("📊 Balance Validation Statistics:")
	logx.Infof("   Total Checks:   %d", stats.TotalChecks)
	logx.Infof("   Match Count:    %d", stats.MatchCount)
	logx.Infof("   Mismatch Count: %d", stats.MismatchCount)
	logx.Infof("   Error Count:    %d", stats.ErrorCount)
	if stats.TotalChecks > 0 {
		matchRate := float64(stats.MatchCount) / float64(stats.TotalChecks) * 100
		logx.Infof("   Match Rate:     %.2f%%", matchRate)
	}
}

// IsEnabled 检查是否启用
func (bv *BalanceValidator) IsEnabled() bool {
	return bv.isEnabled
}

// IsRunning 检查是否运行中
func (bv *BalanceValidator) IsRunning() bool {
	return bv.isRunning
}

// convertToPBChainType 将字符串转换为pb.BlockChainType
func (bv *BalanceValidator) convertToPBChainType(chainStr string) pb.BlockChainType {
	switch chainStr {
	case "ethereum", "ETH", "eth":
		return pb.BlockChainType_CHAIN_TYPE_ETHEREUM
	case "bsc", "BSC":
		return pb.BlockChainType_CHAIN_TYPE_BSC
	case "tron", "TRON", "trx":
		return pb.BlockChainType_CHAIN_TYPE_TRON
	default:
		return pb.BlockChainType_CHAIN_TYPE_ETHEREUM
	}
}
