package svc

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// TokenRefreshManager 用于标记某个 token(jti) 是否已刷新（每个 token 只能刷新一次）
type TokenRefreshManager struct {
	client *redis.Client
}

func NewTokenRefreshManager(client *redis.Client) *TokenRefreshManager {
	return &TokenRefreshManager{client: client}
}

func (m *TokenRefreshManager) key(jti string) string {
	return fmt.Sprintf("admin:jwt:refreshed:%s", jti)
}

func (m *TokenRefreshManager) IsRefreshed(ctx context.Context, jti string) (bool, error) {
	if m.client == nil {
		return false, nil
	}
	n, err := m.client.Exists(ctx, m.key(jti)).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (m *TokenRefreshManager) MarkRefreshed(ctx context.Context, jti string, ttl time.Duration) error {
	if m.client == nil {
		return nil
	}
	if ttl <= 0 {
		ttl = time.Hour
	}
	return m.client.Set(ctx, m.key(jti), "1", ttl).Err()
}
