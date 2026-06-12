package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"golang.org/x/crypto/pbkdf2"
)

// EncryptSeedResult 加密结果
type EncryptSeedResult struct {
	Encrypted []byte // 加密后的数据（不包含nonce/IV）
	Salt      []byte // 盐值
	IV        []byte // IV/Nonce
	Hash      string // SHA256哈希
}

// EncryptSeed 加密Master Seed
// 使用AES-256-GCM加密，PBKDF2密钥派生
func EncryptSeed(seed []byte, password string) (*EncryptSeedResult, error) {
	if len(seed) == 0 {
		return nil, errors.New("seed cannot be empty")
	}
	if len(password) < 16 {
		return nil, errors.New("password too weak, minimum 16 characters")
	}

	// 1. 生成随机盐值（32字节）
	salt := make([]byte, 32)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("failed to generate salt: %w", err)
	}

	// 2. 使用PBKDF2从密码派生密钥（100000次迭代，32字节密钥）
	key := pbkdf2.Key([]byte(password), salt, 100000, 32, sha256.New)

	// 3. 创建AES-GCM加密器
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// 4. 生成随机nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// 5. 加密（格式：nonce + encrypted_data）
	encrypted := gcm.Seal(nonce, nonce, seed, nil)

	// 6. 计算Seed的SHA256哈希（用于验证）
	hash := sha256.Sum256(seed)
	hashStr := hex.EncodeToString(hash[:])

	return &EncryptSeedResult{
		Encrypted: encrypted,
		Salt:      salt,
		IV:        nonce,
		Hash:      hashStr,
	}, nil
}

// DecryptSeed 解密Master Seed
func DecryptSeed(encrypted []byte, salt []byte, password string) ([]byte, error) {
	if len(encrypted) == 0 {
		return nil, errors.New("encrypted data cannot be empty")
	}
	if len(salt) == 0 {
		return nil, errors.New("salt cannot be empty")
	}
	if len(password) == 0 {
		return nil, errors.New("password cannot be empty")
	}

	// 1. 从密码派生密钥（使用相同的参数）
	key := pbkdf2.Key([]byte(password), salt, 100000, 32, sha256.New)

	// 2. 创建AES-GCM解密器
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// 3. 提取nonce和密文
	nonceSize := gcm.NonceSize()
	if len(encrypted) < nonceSize {
		return nil, errors.New("invalid encrypted data: too short")
	}

	nonce, ciphertext := encrypted[:nonceSize], encrypted[nonceSize:]

	// 4. 解密
	seed, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	return seed, nil
}

// HashSeed 计算Seed的SHA256哈希（用于验证）
func HashSeed(seed []byte) string {
	hash := sha256.Sum256(seed)
	return hex.EncodeToString(hash[:])
}

// VerifySeedHash 验证Seed的哈希是否匹配
func VerifySeedHash(seed []byte, expectedHash string) bool {
	actualHash := HashSeed(seed)
	return actualHash == expectedHash
}

// GenerateRandomSeed 生成随机Seed（用于测试或初始化）
// seedLength: Seed长度（字节），推荐16-32字节
func GenerateRandomSeed(seedLength int) ([]byte, error) {
	if seedLength < 16 {
		return nil, errors.New("seed length too short, minimum 16 bytes")
	}
	if seedLength > 64 {
		return nil, errors.New("seed length too long, maximum 64 bytes")
	}

	seed := make([]byte, seedLength)
	if _, err := rand.Read(seed); err != nil {
		return nil, fmt.Errorf("failed to generate random seed: %w", err)
	}

	return seed, nil
}

// GenerateStrongPassword 生成强密码（用于测试）
// length: 密码长度（字节），将转换为hex字符串
func GenerateStrongPassword(length int) (string, error) {
	if length < 16 {
		return "", errors.New("password length too short, minimum 16 bytes")
	}

	randomBytes := make([]byte, length)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("failed to generate password: %w", err)
	}

	return hex.EncodeToString(randomBytes), nil
}
