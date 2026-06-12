package svc

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type MFAFlowManager struct {
	client *redis.Client

	captchaTTL   time.Duration
	tempKeyTTL   time.Duration
	authKeyTTL   time.Duration
	twoFASessTTL time.Duration
}

type MFAAuthSession struct {
	AdminID       int64  `json:"admin_id"`
	ClientIP      string `json:"client_ip,omitempty"`
	UserAgent     string `json:"user_agent,omitempty"`
	PendingSecret string `json:"pending_secret,omitempty"`
	CreatedAtUnix int64  `json:"created_at_unix"`
}

func NewMFAFlowManager(client *redis.Client) *MFAFlowManager {
	return &MFAFlowManager{
		client:       client,
		captchaTTL:   5 * time.Minute,
		tempKeyTTL:   10 * time.Minute,
		authKeyTTL:   15 * time.Minute,
		twoFASessTTL: 10 * time.Minute,
	}
}

func (m *MFAFlowManager) captchaKey(key string) string   { return "admin:mfa:captcha:" + key }
func (m *MFAFlowManager) tempKey(key string) string      { return "admin:mfa:temp:" + key }
func (m *MFAFlowManager) authKey(key string) string      { return "admin:mfa:auth:" + key }
func (m *MFAFlowManager) twoFASessKey(key string) string { return "admin:mfa:2fa_sess:" + key }

func (m *MFAFlowManager) GenerateKey(numBytes int) (string, error) {
	if numBytes <= 0 {
		numBytes = 32
	}
	b := make([]byte, numBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// -------------------- Captcha (key1) --------------------

func (m *MFAFlowManager) StoreCaptcha(ctx context.Context, captchaKey string, answer string) error {
	if m.client == nil {
		return fmt.Errorf("redis not configured")
	}
	captchaKey = strings.TrimSpace(captchaKey)
	if captchaKey == "" {
		return fmt.Errorf("empty captcha key")
	}
	return m.client.Set(ctx, m.captchaKey(captchaKey), strings.TrimSpace(answer), m.captchaTTL).Err()
}

// VerifyAndConsumeCaptcha verifies and always consumes (deletes) the captcha key (one-shot).
func (m *MFAFlowManager) VerifyAndConsumeCaptcha(ctx context.Context, captchaKey string, answer string) (bool, error) {
	if m.client == nil {
		return false, fmt.Errorf("redis not configured")
	}
	captchaKey = strings.TrimSpace(captchaKey)
	if captchaKey == "" {
		return false, nil
	}

	pipe := m.client.TxPipeline()
	get := pipe.Get(ctx, m.captchaKey(captchaKey))
	pipe.Del(ctx, m.captchaKey(captchaKey))
	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return false, err
	}
	if get.Err() == redis.Nil {
		return false, nil
	}
	expected := strings.TrimSpace(get.Val())
	if expected == "" {
		return false, nil
	}
	// Case-insensitive match like base64Captcha does.
	return strings.EqualFold(expected, strings.TrimSpace(answer)), nil
}

// -------------------- Temp key (key2) --------------------

type tempKeyMeta struct {
	ClientIP      string `json:"client_ip,omitempty"`
	UserAgent     string `json:"user_agent,omitempty"`
	CreatedAtUnix int64  `json:"created_at_unix"`
}

func (m *MFAFlowManager) CreateTempKey(ctx context.Context, clientIP, userAgent string) (string, error) {
	if m.client == nil {
		return "", fmt.Errorf("redis not configured")
	}
	key, err := m.GenerateKey(32)
	if err != nil {
		return "", err
	}
	meta := tempKeyMeta{
		ClientIP:      strings.TrimSpace(clientIP),
		UserAgent:     strings.TrimSpace(userAgent),
		CreatedAtUnix: time.Now().Unix(),
	}
	b, _ := json.Marshal(meta)
	if err := m.client.Set(ctx, m.tempKey(key), b, m.tempKeyTTL).Err(); err != nil {
		return "", err
	}
	return key, nil
}

// ConsumeTempKey atomically consumes temp key (one-time). Mismatch also consumes it.
func (m *MFAFlowManager) ConsumeTempKey(ctx context.Context, key, clientIP, userAgent string) (bool, error) {
	if m.client == nil {
		return false, fmt.Errorf("redis not configured")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return false, nil
	}
	val, err := m.client.GetDel(ctx, m.tempKey(key)).Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	var meta tempKeyMeta
	_ = json.Unmarshal([]byte(val), &meta)
	if meta.ClientIP != "" && strings.TrimSpace(clientIP) != "" && meta.ClientIP != strings.TrimSpace(clientIP) {
		return false, nil
	}
	if meta.UserAgent != "" && strings.TrimSpace(userAgent) != "" && meta.UserAgent != strings.TrimSpace(userAgent) {
		return false, nil
	}
	return true, nil
}

// -------------------- Auth key for 2FA bind (key3) --------------------

func (m *MFAFlowManager) CreateAuthKey(ctx context.Context, adminID int64, clientIP, userAgent string) (string, error) {
	if m.client == nil {
		return "", fmt.Errorf("redis not configured")
	}
	key, err := m.GenerateKey(32)
	if err != nil {
		return "", err
	}
	s := MFAAuthSession{
		AdminID:       adminID,
		ClientIP:      strings.TrimSpace(clientIP),
		UserAgent:     strings.TrimSpace(userAgent),
		CreatedAtUnix: time.Now().Unix(),
	}
	b, _ := json.Marshal(s)
	if err := m.client.Set(ctx, m.authKey(key), b, m.authKeyTTL).Err(); err != nil {
		return "", err
	}
	return key, nil
}

func (m *MFAFlowManager) GetAuthSession(ctx context.Context, key, clientIP, userAgent string) (*MFAAuthSession, bool, error) {
	if m.client == nil {
		return nil, false, fmt.Errorf("redis not configured")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, false, nil
	}
	val, err := m.client.Get(ctx, m.authKey(key)).Result()
	if err == redis.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var s MFAAuthSession
	if err := json.Unmarshal([]byte(val), &s); err != nil {
		return nil, false, nil
	}
	if s.ClientIP != "" && strings.TrimSpace(clientIP) != "" && s.ClientIP != strings.TrimSpace(clientIP) {
		return nil, false, nil
	}
	if s.UserAgent != "" && strings.TrimSpace(userAgent) != "" && s.UserAgent != strings.TrimSpace(userAgent) {
		return nil, false, nil
	}
	return &s, true, nil
}

func (m *MFAFlowManager) SetAuthSession(ctx context.Context, key string, s *MFAAuthSession) error {
	if m.client == nil {
		return fmt.Errorf("redis not configured")
	}
	if strings.TrimSpace(key) == "" || s == nil {
		return fmt.Errorf("invalid auth session")
	}
	b, _ := json.Marshal(s)
	return m.client.Set(ctx, m.authKey(strings.TrimSpace(key)), b, m.authKeyTTL).Err()
}

func (m *MFAFlowManager) ConsumeAuthKey(ctx context.Context, key string) error {
	if m.client == nil {
		return fmt.Errorf("redis not configured")
	}
	if strings.TrimSpace(key) == "" {
		return nil
	}
	return m.client.Del(ctx, m.authKey(strings.TrimSpace(key))).Err()
}

// -------------------- 2FA session identifier (key4) --------------------

type TwoFASession struct {
	AdminID       int64  `json:"admin_id"`
	ClientIP      string `json:"client_ip,omitempty"`
	UserAgent     string `json:"user_agent,omitempty"`
	Attempts      int    `json:"attempts"`
	CreatedAtUnix int64  `json:"created_at_unix"`
}

func (m *MFAFlowManager) CreateTwoFASession(ctx context.Context, adminID int64, clientIP, userAgent string) (string, error) {
	if m.client == nil {
		return "", fmt.Errorf("redis not configured")
	}
	key, err := m.GenerateKey(32)
	if err != nil {
		return "", err
	}
	s := TwoFASession{
		AdminID:       adminID,
		ClientIP:      strings.TrimSpace(clientIP),
		UserAgent:     strings.TrimSpace(userAgent),
		Attempts:      0,
		CreatedAtUnix: time.Now().Unix(),
	}
	b, _ := json.Marshal(s)
	if err := m.client.Set(ctx, m.twoFASessKey(key), b, m.twoFASessTTL).Err(); err != nil {
		return "", err
	}
	return key, nil
}

func (m *MFAFlowManager) GetTwoFASession(ctx context.Context, key, clientIP, userAgent string) (*TwoFASession, bool, error) {
	if m.client == nil {
		return nil, false, fmt.Errorf("redis not configured")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, false, nil
	}
	val, err := m.client.Get(ctx, m.twoFASessKey(key)).Result()
	if err == redis.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var s TwoFASession
	if err := json.Unmarshal([]byte(val), &s); err != nil {
		return nil, false, nil
	}
	if s.ClientIP != "" && strings.TrimSpace(clientIP) != "" && s.ClientIP != strings.TrimSpace(clientIP) {
		return nil, false, nil
	}
	if s.UserAgent != "" && strings.TrimSpace(userAgent) != "" && s.UserAgent != strings.TrimSpace(userAgent) {
		return nil, false, nil
	}
	return &s, true, nil
}

func (m *MFAFlowManager) UpdateTwoFASession(ctx context.Context, key string, s *TwoFASession) error {
	if m.client == nil {
		return fmt.Errorf("redis not configured")
	}
	if strings.TrimSpace(key) == "" || s == nil {
		return fmt.Errorf("invalid 2fa session")
	}
	b, _ := json.Marshal(s)
	return m.client.Set(ctx, m.twoFASessKey(strings.TrimSpace(key)), b, m.twoFASessTTL).Err()
}

func (m *MFAFlowManager) ConsumeTwoFASession(ctx context.Context, key string) error {
	if m.client == nil {
		return fmt.Errorf("redis not configured")
	}
	if strings.TrimSpace(key) == "" {
		return nil
	}
	return m.client.Del(ctx, m.twoFASessKey(strings.TrimSpace(key))).Err()
}
