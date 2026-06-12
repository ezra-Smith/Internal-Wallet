package svc

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// IdempotencyManager 用于基于 Idempotency-Key 做幂等保护（SETNX）
type IdempotencyManager struct {
	client *redis.Client
}

func NewIdempotencyManager(client *redis.Client) *IdempotencyManager {
	return &IdempotencyManager{client: client}
}

func (m *IdempotencyManager) key(adminID int64, action string, idempotencyKey string) string {
	return fmt.Sprintf("admin:idempotency:%d:%s:%s", adminID, action, idempotencyKey)
}

// Acquire 尝试占用幂等键
// ok=true 表示首次请求；ok=false 表示重复请求
func (m *IdempotencyManager) Acquire(ctx context.Context, adminID int64, action string, idempotencyKey string, ttl time.Duration) (bool, error) {
	if m.client == nil {
		return true, nil
	}
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return m.client.SetNX(ctx, m.key(adminID, action, idempotencyKey), "1", ttl).Result()
}
