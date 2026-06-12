package svc

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// TwoFAManager 管理 2FA 临时 token 的状态（次数/过期/一次性）
type TwoFAManager struct {
	client *redis.Client
	ttl    time.Duration
}

func NewTwoFAManager(client *redis.Client, ttl time.Duration) *TwoFAManager {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &TwoFAManager{
		client: client,
		ttl:    ttl,
	}
}

func (m *TwoFAManager) key(jti string) string {
	return fmt.Sprintf("admin:2fa:%s", jti)
}

func (m *TwoFAManager) Create(ctx context.Context, jti string) error {
	if m.client == nil {
		return nil
	}
	return m.client.Set(ctx, m.key(jti), "0", m.ttl).Err()
}

func (m *TwoFAManager) GetAttempts(ctx context.Context, jti string) (attempts int, exists bool, err error) {
	if m.client == nil {
		return 0, true, nil
	}
	val, err := m.client.Get(ctx, m.key(jti)).Result()
	if err == redis.Nil {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	n, parseErr := strconv.Atoi(val)
	if parseErr != nil {
		return 0, true, nil
	}
	return n, true, nil
}

func (m *TwoFAManager) IncrementAttempts(ctx context.Context, jti string) (int, error) {
	if m.client == nil {
		return 0, nil
	}
	n, err := m.client.Incr(ctx, m.key(jti)).Result()
	if err != nil {
		return 0, err
	}
	// 刷新 TTL，保证在有效期内的尝试可持续记录
	_ = m.client.Expire(ctx, m.key(jti), m.ttl).Err()
	return int(n), nil
}

func (m *TwoFAManager) Consume(ctx context.Context, jti string) error {
	if m.client == nil {
		return nil
	}
	return m.client.Del(ctx, m.key(jti)).Err()
}
