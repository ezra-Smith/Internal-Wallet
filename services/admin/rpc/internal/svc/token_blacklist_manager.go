package svc

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// TokenBlacklistManager 用于将 token jti 加入黑名单（logout/refresh 等场景）
type TokenBlacklistManager struct {
	client *redis.Client
}

func NewTokenBlacklistManager(client *redis.Client) *TokenBlacklistManager {
	return &TokenBlacklistManager{client: client}
}

func (m *TokenBlacklistManager) key(jti string) string {
	return fmt.Sprintf("admin:jwt:blacklist:%s", jti)
}

func (m *TokenBlacklistManager) Blacklist(ctx context.Context, jti string, ttl time.Duration) error {
	if m.client == nil {
		return nil
	}
	if ttl <= 0 {
		ttl = time.Hour
	}
	return m.client.Set(ctx, m.key(jti), "1", ttl).Err()
}

func (m *TokenBlacklistManager) IsBlacklisted(ctx context.Context, jti string) (bool, error) {
	if m.client == nil {
		return false, nil
	}
	n, err := m.client.Exists(ctx, m.key(jti)).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}
