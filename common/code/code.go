package code

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/redis/go-redis/v9"
)

// Generator 验证码生成器
type Generator struct {
	redisClient *redis.Client
	codeLength  int           // 验证码长度
	expiration  time.Duration // 过期时间
}

// NewGenerator 创建验证码生成器
func NewGenerator(redisClient *redis.Client, codeLength int, expiration time.Duration) *Generator {
	return &Generator{
		redisClient: redisClient,
		codeLength:  codeLength,
		expiration:  expiration,
	}
}

// Generate 生成验证码并存储到 Redis
// recipient: 接收者（邮箱或手机号）
// scene: 场景（register, login, reset_password, withdraw）
// codeType: 类型（email, sms）
func (g *Generator) Generate(ctx context.Context, recipient, scene, codeType string) (string, error) {
	// 1. 生成随机验证码
	code := g.generateRandomCode()

	// 2. 存储到 Redis
	key := g.buildRedisKey(codeType, recipient, scene)
	err := g.redisClient.Set(ctx, key, code, g.expiration).Err()
	if err != nil {
		return "", fmt.Errorf("failed to store code to redis: %w", err)
	}

	return code, nil
}

// Verify 验证验证码（从 Redis 读取并比对）
func (g *Generator) Verify(ctx context.Context, recipient, scene, codeType, inputCode string) (bool, error) {
	key := g.buildRedisKey(codeType, recipient, scene)

	// 从 Redis 获取
	storedCode, err := g.redisClient.Get(ctx, key).Result()
	if err == redis.Nil {
		return false, nil // 验证码不存在或已过期
	}
	if err != nil {
		return false, fmt.Errorf("failed to get code from redis: %w", err)
	}

	// 比对
	return storedCode == inputCode, nil
}

// Delete 删除验证码（验证成功后删除，确保一次性使用）
func (g *Generator) Delete(ctx context.Context, recipient, scene, codeType string) error {
	key := g.buildRedisKey(codeType, recipient, scene)
	return g.redisClient.Del(ctx, key).Err()
}

// generateRandomCode 生成随机数字验证码
func (g *Generator) generateRandomCode() string {
	rand.Seed(time.Now().UnixNano())
	code := ""
	for i := 0; i < g.codeLength; i++ {
		code += fmt.Sprintf("%d", rand.Intn(10))
	}
	return code
}

// buildRedisKey 构建 Redis Key
// 格式：{codeType}:code:{recipient}:{scene}
// 例如：email:code:user@example.com:register
func (g *Generator) buildRedisKey(codeType, recipient, scene string) string {
	return fmt.Sprintf("%s:code:%s:%s", codeType, recipient, scene)
}

// GetTTL 获取验证码剩余过期时间
func (g *Generator) GetTTL(ctx context.Context, recipient, scene, codeType string) (time.Duration, error) {
	key := g.buildRedisKey(codeType, recipient, scene)
	return g.redisClient.TTL(ctx, key).Result()
}
