package alert

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
)

// Accumulator 金额累加器
type Accumulator struct {
	redis *redis.Client
}

// NewAccumulator 创建累加器
func NewAccumulator(redis *redis.Client) *Accumulator {
	return &Accumulator{
		redis: redis,
	}
}

// Add 累加金额到时间窗口
// 返回: 累加后的总金额, 交易笔数, 错误
func (a *Accumulator) Add(
	ctx context.Context,
	configID int64,
	timeWindowSeconds int,
	amountUSD decimal.Decimal,
) (decimal.Decimal, int64, error) {
	now := time.Now()
	// 根据配置的时间窗口计算窗口起始时间
	windowStart := now.Truncate(time.Duration(timeWindowSeconds) * time.Second)
	windowEnd := windowStart.Add(time.Duration(timeWindowSeconds) * time.Second)
	windowKey := fmt.Sprintf("alert:amount:%d:%d", configID, windowStart.Unix())

	// 使用 Pipeline 原子操作
	pipe := a.redis.Pipeline()
	amountFloat, _ := amountUSD.Float64()
	pipe.HIncrByFloat(ctx, windowKey, "total_amount_usd", amountFloat)
	pipe.HIncrBy(ctx, windowKey, "transaction_count", 1)
	// 过期时间 = 窗口结束时间 + 5分钟余量
	pipe.ExpireAt(ctx, windowKey, windowEnd.Add(5*time.Minute))

	results, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return decimal.Zero, 0, err
	}

	// 获取累加后的金额 (HIncrByFloat 返回 FloatCmd)
	totalAmountFloat, _ := results[0].(*redis.FloatCmd).Result()
	totalAmount := decimal.NewFromFloat(totalAmountFloat)
	// 获取交易笔数 (HIncrBy 返回 IntCmd)
	txCount, _ := results[1].(*redis.IntCmd).Result()

	return totalAmount, txCount, nil
}

// GetTotal 获取当前窗口的累计金额
func (a *Accumulator) GetTotal(
	ctx context.Context,
	configID int64,
	timeWindowSeconds int,
) (decimal.Decimal, int64, error) {
	now := time.Now()
	windowStart := now.Truncate(time.Duration(timeWindowSeconds) * time.Second)
	windowKey := fmt.Sprintf("alert:amount:%d:%d", configID, windowStart.Unix())

	// 从 Redis 获取
	amountStr, err := a.redis.HGet(ctx, windowKey, "total_amount_usd").Result()
	if err == redis.Nil || amountStr == "" {
		return decimal.Zero, 0, nil
	}
	if err != nil {
		return decimal.Zero, 0, err
	}

	totalAmount, _ := decimal.NewFromString(amountStr)
	countStr, _ := a.redis.HGet(ctx, windowKey, "transaction_count").Result()
	count := int64(0)
	if countStr != "" {
		count, _ = ParseInt64(countStr)
	}

	return totalAmount, count, nil
}

// AddTransactionDetail 添加交易详情（用于通知展示）
func (a *Accumulator) AddTransactionDetail(
	ctx context.Context,
	configID int64,
	timeWindowSeconds int,
	detail TransactionDetail,
) error {
	now := time.Now()
	windowStart := now.Truncate(time.Duration(timeWindowSeconds) * time.Second)
	windowEnd := windowStart.Add(time.Duration(timeWindowSeconds) * time.Second)
	detailKey := fmt.Sprintf("alert:details:%d:%d", configID, windowStart.Unix())

	// 格式化详情
	detailStr := fmt.Sprintf("%s|%s|%s|%s|%s",
		detail.Timestamp.Format("2006-01-15 15:04:05"),
		detail.EventType,
		detail.AssetCode,
		detail.Amount.String(),
		detail.AmountUSD.String(),
	)

	// 添加到 List
	err := a.redis.LPush(ctx, detailKey, detailStr).Err()
	if err != nil {
		return err
	}

	// 设置过期时间
	return a.redis.ExpireAt(ctx, detailKey, windowEnd.Add(5*time.Minute)).Err()
}

// GetTransactionDetails 获取交易详情（最近N条）
func (a *Accumulator) GetTransactionDetails(
	ctx context.Context,
	configID int64,
	timeWindowSeconds int,
	limit int64,
) ([]string, error) {
	now := time.Now()
	windowStart := now.Truncate(time.Duration(timeWindowSeconds) * time.Second)
	detailKey := fmt.Sprintf("alert:details:%d:%d", configID, windowStart.Unix())

	return a.redis.LRange(ctx, detailKey, 0, limit-1).Result()
}

// IsInCooldown 检查是否在冷却期内
func (a *Accumulator) IsInCooldown(
	ctx context.Context,
	configID int64,
) bool {
	cooldownKey := fmt.Sprintf("alert:cooldown:%d", configID)
	exists, _ := a.redis.Exists(ctx, cooldownKey).Result()
	return exists > 0
}

// SetCooldown 设置冷却时间
func (a *Accumulator) SetCooldown(
	ctx context.Context,
	configID int64,
	cooldownSeconds int,
) error {
	cooldownKey := fmt.Sprintf("alert:cooldown:%d", configID)
	return a.redis.Set(ctx, cooldownKey, "1", time.Duration(cooldownSeconds)*time.Second).Err()
}

// ParseInt64 解析 int64
func ParseInt64(s string) (int64, error) {
	var result int64
	_, err := fmt.Sscanf(s, "%d", &result)
	return result, err
}
