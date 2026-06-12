package cache

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisCache 基于 go-redis/v9 的缓存实现
// 提供与 cache.go 类似的接口，但使用 go-redis/v9 作为底层客户端
type RedisCache struct {
	client *redis.Client
	prefix string // 键前缀，用于区分不同业务
}

// NewRedisCache 创建一个新的 Redis 缓存实例
func NewRedisCache(client *redis.Client, prefix string) *RedisCache {
	return &RedisCache{
		client: client,
		prefix: prefix,
	}
}

// key 添加前缀
func (c *RedisCache) key(k string) string {
	if c.prefix == "" {
		return k
	}
	return c.prefix + ":" + k
}

// Set 设置字符串值
func (c *RedisCache) Set(ctx context.Context, key string, value string, expiration time.Duration) error {
	return c.client.Set(ctx, c.key(key), value, expiration).Err()
}

// Get 获取字符串值
func (c *RedisCache) Get(ctx context.Context, key string) (string, error) {
	val, err := c.client.Get(ctx, c.key(key)).Result()
	if err == redis.Nil {
		return "", errors.New("key not found")
	}
	return val, err
}

// SetJSON 设置 JSON 对象
func (c *RedisCache) SetJSON(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.client.Set(ctx, c.key(key), data, expiration).Err()
}

// GetJSON 获取 JSON 对象
func (c *RedisCache) GetJSON(ctx context.Context, key string, value interface{}) error {
	data, err := c.client.Get(ctx, c.key(key)).Result()
	if err == redis.Nil {
		return errors.New("key not found")
	}
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(data), value)
}

// Delete 删除键
func (c *RedisCache) Delete(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}

	prefixedKeys := make([]string, len(keys))
	for i, key := range keys {
		prefixedKeys[i] = c.key(key)
	}

	return c.client.Del(ctx, prefixedKeys...).Err()
}

// Exists 检查键是否存在
func (c *RedisCache) Exists(ctx context.Context, key string) (bool, error) {
	n, err := c.client.Exists(ctx, c.key(key)).Result()
	return n > 0, err
}

// Expire 设置过期时间
func (c *RedisCache) Expire(ctx context.Context, key string, expiration time.Duration) error {
	return c.client.Expire(ctx, c.key(key), expiration).Err()
}

// TTL 获取剩余过期时间
func (c *RedisCache) TTL(ctx context.Context, key string) (time.Duration, error) {
	return c.client.TTL(ctx, c.key(key)).Result()
}

// Incr 自增
func (c *RedisCache) Incr(ctx context.Context, key string) (int64, error) {
	return c.client.Incr(ctx, c.key(key)).Result()
}

// IncrBy 增加指定值
func (c *RedisCache) IncrBy(ctx context.Context, key string, value int64) (int64, error) {
	return c.client.IncrBy(ctx, c.key(key), value).Result()
}

// Decr 自减
func (c *RedisCache) Decr(ctx context.Context, key string) (int64, error) {
	return c.client.Decr(ctx, c.key(key)).Result()
}

// DecrBy 减少指定值
func (c *RedisCache) DecrBy(ctx context.Context, key string, value int64) (int64, error) {
	return c.client.DecrBy(ctx, c.key(key), value).Result()
}

// SetNX 仅当键不存在时设置（用于分布式锁）
func (c *RedisCache) SetNX(ctx context.Context, key string, value string, expiration time.Duration) (bool, error) {
	return c.client.SetNX(ctx, c.key(key), value, expiration).Result()
}

// Hash 操作 --------------

// HSet 设置哈希字段
func (c *RedisCache) HSet(ctx context.Context, key string, field string, value interface{}) error {
	return c.client.HSet(ctx, c.key(key), field, value).Err()
}

// HGet 获取哈希字段
func (c *RedisCache) HGet(ctx context.Context, key string, field string) (string, error) {
	val, err := c.client.HGet(ctx, c.key(key), field).Result()
	if err == redis.Nil {
		return "", errors.New("field not found")
	}
	return val, err
}

// HGetAll 获取所有哈希字段
func (c *RedisCache) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	return c.client.HGetAll(ctx, c.key(key)).Result()
}

// HDel 删除哈希字段
func (c *RedisCache) HDel(ctx context.Context, key string, fields ...string) error {
	return c.client.HDel(ctx, c.key(key), fields...).Err()
}

// HExists 检查哈希字段是否存在
func (c *RedisCache) HExists(ctx context.Context, key string, field string) (bool, error) {
	return c.client.HExists(ctx, c.key(key), field).Result()
}

// List 操作 --------------

// LPush 从左侧插入列表
func (c *RedisCache) LPush(ctx context.Context, key string, values ...interface{}) error {
	return c.client.LPush(ctx, c.key(key), values...).Err()
}

// RPush 从右侧插入列表
func (c *RedisCache) RPush(ctx context.Context, key string, values ...interface{}) error {
	return c.client.RPush(ctx, c.key(key), values...).Err()
}

// LPop 从左侧弹出
func (c *RedisCache) LPop(ctx context.Context, key string) (string, error) {
	val, err := c.client.LPop(ctx, c.key(key)).Result()
	if err == redis.Nil {
		return "", errors.New("list is empty")
	}
	return val, err
}

// RPop 从右侧弹出
func (c *RedisCache) RPop(ctx context.Context, key string) (string, error) {
	val, err := c.client.RPop(ctx, c.key(key)).Result()
	if err == redis.Nil {
		return "", errors.New("list is empty")
	}
	return val, err
}

// LRange 获取列表范围
func (c *RedisCache) LRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return c.client.LRange(ctx, c.key(key), start, stop).Result()
}

// LLen 获取列表长度
func (c *RedisCache) LLen(ctx context.Context, key string) (int64, error) {
	return c.client.LLen(ctx, c.key(key)).Result()
}

// Set 操作 --------------

// SAdd 添加集合成员
func (c *RedisCache) SAdd(ctx context.Context, key string, members ...interface{}) error {
	return c.client.SAdd(ctx, c.key(key), members...).Err()
}

// SRem 移除集合成员
func (c *RedisCache) SRem(ctx context.Context, key string, members ...interface{}) error {
	return c.client.SRem(ctx, c.key(key), members...).Err()
}

// SMembers 获取所有集合成员
func (c *RedisCache) SMembers(ctx context.Context, key string) ([]string, error) {
	return c.client.SMembers(ctx, c.key(key)).Result()
}

// SIsMember 检查是否是集合成员
func (c *RedisCache) SIsMember(ctx context.Context, key string, member interface{}) (bool, error) {
	return c.client.SIsMember(ctx, c.key(key), member).Result()
}

// SCard 获取集合成员数量
func (c *RedisCache) SCard(ctx context.Context, key string) (int64, error) {
	return c.client.SCard(ctx, c.key(key)).Result()
}

// Sorted Set 操作 --------------

// ZAdd 添加有序集合成员
func (c *RedisCache) ZAdd(ctx context.Context, key string, members ...redis.Z) error {
	return c.client.ZAdd(ctx, c.key(key), members...).Err()
}

// ZRem 移除有序集合成员
func (c *RedisCache) ZRem(ctx context.Context, key string, members ...interface{}) error {
	return c.client.ZRem(ctx, c.key(key), members...).Err()
}

// ZRange 按索引范围获取有序集合成员（分数从小到大）
func (c *RedisCache) ZRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return c.client.ZRange(ctx, c.key(key), start, stop).Result()
}

// ZRevRange 按索引范围获取有序集合成员（分数从大到小）
func (c *RedisCache) ZRevRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return c.client.ZRevRange(ctx, c.key(key), start, stop).Result()
}

// ZRangeByScore 按分数范围获取有序集合成员
func (c *RedisCache) ZRangeByScore(ctx context.Context, key string, min, max string) ([]string, error) {
	return c.client.ZRangeByScore(ctx, c.key(key), &redis.ZRangeBy{
		Min: min,
		Max: max,
	}).Result()
}

// ZCard 获取有序集合成员数量
func (c *RedisCache) ZCard(ctx context.Context, key string) (int64, error) {
	return c.client.ZCard(ctx, c.key(key)).Result()
}

// ZScore 获取成员分数
func (c *RedisCache) ZScore(ctx context.Context, key string, member string) (float64, error) {
	return c.client.ZScore(ctx, c.key(key), member).Result()
}

// 高级功能 --------------

// Lock 获取分布式锁
// 返回 unlock 函数，调用它来释放锁
func (c *RedisCache) Lock(ctx context.Context, key string, expiration time.Duration) (unlock func() error, err error) {
	lockKey := c.key("lock:" + key)

	// 尝试获取锁
	ok, err := c.client.SetNX(ctx, lockKey, "locked", expiration).Result()
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("failed to acquire lock")
	}

	// 返回解锁函数
	unlock = func() error {
		return c.client.Del(ctx, lockKey).Err()
	}

	return unlock, nil
}

// LockWithRetry 获取分布式锁（带重试）
// maxRetries: 最大重试次数
// retryDelay: 重试间隔
func (c *RedisCache) LockWithRetry(ctx context.Context, key string, expiration time.Duration, maxRetries int, retryDelay time.Duration) (unlock func() error, err error) {
	for i := 0; i <= maxRetries; i++ {
		unlock, err = c.Lock(ctx, key, expiration)
		if err == nil {
			return unlock, nil
		}

		if i < maxRetries {
			time.Sleep(retryDelay)
		}
	}

	return nil, errors.New("failed to acquire lock after retries")
}

// GetClient 获取底层 Redis 客户端（用于高级操作）
func (c *RedisCache) GetClient() *redis.Client {
	return c.client
}
