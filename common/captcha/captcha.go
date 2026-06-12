package captcha

import (
	"context"
	"fmt"
	"time"

	"github.com/mojocn/base64Captcha"
	"github.com/redis/go-redis/v9"
)

// Store 验证码存储接口
type Store interface {
	Set(ctx context.Context, id string, value string) error
	Get(ctx context.Context, id string, clear bool) (string, error)
	Verify(ctx context.Context, id string, answer string, clear bool) bool
}

// RedisStore 基于 Redis 的存储
type RedisStore struct {
	client     *redis.Client
	expiration time.Duration
	prefix     string
}

// NewRedisStore 创建 Redis 存储
func NewRedisStore(client *redis.Client, expiration time.Duration) *RedisStore {
	return NewRedisStoreWithPrefix(client, expiration, "captcha:")
}

// NewRedisStoreWithPrefix 创建带自定义前缀的 Redis 存储（用于区分不同业务的验证码）
func NewRedisStoreWithPrefix(client *redis.Client, expiration time.Duration, prefix string) *RedisStore {
	if prefix == "" {
		prefix = "captcha:"
	}
	return &RedisStore{
		client:     client,
		expiration: expiration,
		prefix:     prefix,
	}
}

// Set 存储验证码
func (s *RedisStore) Set(ctx context.Context, id string, value string) error {
	key := s.prefix + id
	return s.client.Set(ctx, key, value, s.expiration).Err()
}

// Get 获取验证码
func (s *RedisStore) Get(ctx context.Context, id string, clear bool) (string, error) {
	key := s.prefix + id
	val, err := s.client.Get(ctx, key).Result()
	if err != nil {
		return "", err
	}

	if clear {
		s.client.Del(ctx, key)
	}

	return val, nil
}

// Verify 验证验证码
func (s *RedisStore) Verify(ctx context.Context, id string, answer string, clear bool) bool {
	value, err := s.Get(ctx, id, clear)
	if err != nil {
		return false
	}
	return value == answer
}

// Generator 验证码生成器
type Generator struct {
	store  Store
	driver base64Captcha.Driver
}

// NewGenerator 创建验证码生成器
func NewGenerator(store Store) *Generator {
	// 配置验证码样式
	driver := base64Captcha.NewDriverDigit(
		80,  // 高度
		240, // 宽度
		6,   // 验证码长度
		0.7, // 最大倾斜角度
		80,  // 点的数量
	)

	return &Generator{
		store:  store,
		driver: driver,
	}
}

// NewCustomGenerator 创建自定义样式的生成器
func NewCustomGenerator(store Store, height, width, length int, maxSkew float64, dotCount int) *Generator {
	driver := base64Captcha.NewDriverDigit(height, width, length, maxSkew, dotCount)
	return &Generator{
		store:  store,
		driver: driver,
	}
}

// NewStringGenerator 创建字母数字混合验证码
func NewStringGenerator(store Store) *Generator {
	driver := base64Captcha.NewDriverString(
		80,                                     // 高度
		240,                                    // 宽度
		6,                                      // 噪点数量
		base64Captcha.OptionShowHollowLine,     // 显示空心线
		6,                                      // 验证码长度
		"abcdefghijklmnopqrstuvwxyz0123456789", // 字符源
		nil,                                    // 背景色（nil 使用默认）
		nil,                                    // 字体（nil 使用默认）
		nil,                                    // 字体文件（nil 使用默认）
	)

	return &Generator{
		store:  store,
		driver: driver,
	}
}

// CaptchaResponse 验证码响应
type CaptchaResponse struct {
	CaptchaID string `json:"captcha_id"`
	ImageData string `json:"image_data"` // Base64 编码的图片
}

// Generate 生成验证码
func (g *Generator) Generate(ctx context.Context) (*CaptchaResponse, error) {
	// 使用适配器
	store := &storeAdapter{
		store: g.store,
		ctx:   ctx,
	}

	// 生成验证码
	c := base64Captcha.NewCaptcha(g.driver, store)
	id, b64s, answer, err := c.Generate()
	if err != nil {
		return nil, fmt.Errorf("failed to generate captcha: %w", err)
	}

	// answer 用于服务器端验证，已存储在 store 中，这里不需要返回
	_ = answer

	return &CaptchaResponse{
		CaptchaID: id,
		ImageData: b64s,
	}, nil
}

// Verify 验证验证码
func (g *Generator) Verify(ctx context.Context, id string, answer string) bool {
	return g.store.Verify(ctx, id, answer, true)
}

// storeAdapter 适配器，将我们的 Store 接口适配到 base64Captcha.Store
type storeAdapter struct {
	store Store
	ctx   context.Context
}

func (s *storeAdapter) Set(id string, value string) error {
	return s.store.Set(s.ctx, id, value)
}

func (s *storeAdapter) Get(id string, clear bool) string {
	val, _ := s.store.Get(s.ctx, id, clear)
	return val
}

func (s *storeAdapter) Verify(id, answer string, clear bool) bool {
	return s.store.Verify(s.ctx, id, answer, clear)
}
