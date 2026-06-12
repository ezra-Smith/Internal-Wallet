package svc

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// SessionManager 管理 Admin 的服务端会话（基于 Redis）
//
// 约定：
// - 每个 access token 发行时会生成 jti，并写入 session key。
// - 每次请求都校验该 session key 是否存在，从而支持主动失效（logout、重置密码等）。
type SessionManager struct {
	client *redis.Client
}

func NewSessionManager(client *redis.Client) *SessionManager {
	return &SessionManager{client: client}
}

func (m *SessionManager) sessionKey(adminID int64, jti string) string {
	return fmt.Sprintf("admin:session:%d:%s", adminID, jti)
}

func (m *SessionManager) sessionsSetKey(adminID int64) string {
	return fmt.Sprintf("admin:sessions:%d", adminID)
}

func (m *SessionManager) Add(ctx context.Context, adminID int64, jti string, expUnix int64) error {
	if m.client == nil {
		return nil
	}
	ttl := time.Until(time.Unix(expUnix, 0))
	if ttl <= 0 {
		ttl = time.Hour
	}
	key := m.sessionKey(adminID, jti)
	setKey := m.sessionsSetKey(adminID)

	pipe := m.client.Pipeline()
	pipe.Set(ctx, key, "1", ttl)
	pipe.SAdd(ctx, setKey, jti)
	// setKey 用于批量清理，过期时间设为 token ttl 的 2 倍即可
	pipe.Expire(ctx, setKey, ttl*2)
	_, err := pipe.Exec(ctx)
	return err
}

func (m *SessionManager) Remove(ctx context.Context, adminID int64, jti string) error {
	if m.client == nil {
		return nil
	}
	key := m.sessionKey(adminID, jti)
	setKey := m.sessionsSetKey(adminID)

	pipe := m.client.Pipeline()
	pipe.Del(ctx, key)
	pipe.SRem(ctx, setKey, jti)
	_, err := pipe.Exec(ctx)
	return err
}

func (m *SessionManager) Exists(ctx context.Context, adminID int64, jti string) (bool, error) {
	if m.client == nil {
		// 无 Redis 时降级为“始终存在”（生产环境不建议）
		return true, nil
	}
	key := m.sessionKey(adminID, jti)
	n, err := m.client.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (m *SessionManager) RemoveAll(ctx context.Context, adminID int64) error {
	if m.client == nil {
		return nil
	}
	setKey := m.sessionsSetKey(adminID)
	jtis, err := m.client.SMembers(ctx, setKey).Result()
	if err != nil {
		return err
	}
	pipe := m.client.Pipeline()
	for _, jti := range jtis {
		pipe.Del(ctx, m.sessionKey(adminID, jti))
	}
	pipe.Del(ctx, setKey)
	_, execErr := pipe.Exec(ctx)
	return execErr
}

func (m *SessionManager) RemoveAllExcept(ctx context.Context, adminID int64, keepJTI string) error {
	if m.client == nil {
		return nil
	}
	setKey := m.sessionsSetKey(adminID)
	jtis, err := m.client.SMembers(ctx, setKey).Result()
	if err != nil {
		return err
	}
	pipe := m.client.Pipeline()
	for _, jti := range jtis {
		if jti == keepJTI {
			continue
		}
		pipe.Del(ctx, m.sessionKey(adminID, jti))
		pipe.SRem(ctx, setKey, jti)
	}
	_, execErr := pipe.Exec(ctx)
	return execErr
}
