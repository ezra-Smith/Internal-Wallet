package logic

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"time"

	"github.com/zeromicro/go-zero/core/logx"

	"internalwallet/common/errcode"
	"internalwallet/common/middleware"
	"internalwallet/common/utils"
	"internalwallet/proto/pb"
	"internalwallet/services/signer/rpc/internal/svc"
)

type GetMnemonicChallengeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetMnemonicChallengeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetMnemonicChallengeLogic {
	return &GetMnemonicChallengeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ChallengeSession 挑战会话结构（存储在Redis）
type ChallengeSession struct {
	SessionID        string    `json:"session_id"`
	UserID           int64     `json:"user_id"`
	Purpose          string    `json:"purpose"`
	ChallengeIndices []int32   `json:"challenge_indices"`
	ExpectedWords    []string  `json:"expected_words"` // 正确答案（不返回给客户端）
	Attempts         int       `json:"attempts"`       // 已尝试次数
	MaxAttempts      int       `json:"max_attempts"`   // 最大尝试次数（3次）
	CreatedAt        time.Time `json:"created_at"`
	ExpiresAt        time.Time `json:"expires_at"`
}

const (
	challengePrefix   = "mnemonic_challenge:"
	challengeDuration = 5 * time.Minute
	maxAttempts       = 3
	numChallengeWords = 3 // 挑战3个单词
)

// GetMnemonicChallenge 获取助记词验证挑战
// 业务规则：
// - 随机选取3个不连续的位置
// - 挑战会话有效期5分钟
// - 同一用户每小时最多发起5次挑战
func (l *GetMnemonicChallengeLogic) GetMnemonicChallenge(in *pb.GetMnemonicChallengeRequest) (*pb.GetMnemonicChallengeResponse, error) {
	// 获取审计信息
	userID := middleware.GetUserID(l.ctx)
	clientIP := middleware.GetClientIP(l.ctx)

	l.Logger.Infof("GetMnemonicChallenge called | UserID: %s, IP: %s, RequestUserID: %d, Purpose: %s",
		userID, clientIP, in.UserId, in.Purpose)

	// 1. 参数验证
	if in.UserId <= 0 {
		return &pb.GetMnemonicChallengeResponse{
			Code:    int32(errcode.SignerInvalidParams),
			Message: "user_id must be greater than 0",
		}, nil
	}

	if in.Purpose == "" {
		in.Purpose = "verify_backup" // 默认目的
	}

	// 验证purpose是否合法
	validPurposes := map[string]bool{
		"verify_backup":  true,
		"recovery":       true,
		"security_check": true,
	}
	if !validPurposes[in.Purpose] {
		return &pb.GetMnemonicChallengeResponse{
			Code:    int32(errcode.SignerInvalidParams),
			Message: "invalid purpose, must be one of: verify_backup, recovery, security_check",
		}, nil
	}

	// 2. 检查频率限制（每小时最多5次）
	if err := l.checkRateLimit(in.UserId); err != nil {
		l.Logger.Infof("GetMnemonicChallenge rate limit exceeded | UserID: %d, IP: %s",
			in.UserId, clientIP)
		return &pb.GetMnemonicChallengeResponse{
			Code:    int32(errcode.SignerRateLimitExceeded),
			Message: "too many challenge requests, please try again later",
		}, nil
	}

	// 3.从用户的种子或备份中获取助记词
	// 从user_mnemonic_backup表中读取加密的助记词
	mnemonic := l.getUserMnemonic(in.UserId)
	if mnemonic == "" {
		l.Logger.Errorf("GetMnemonicChallenge: user mnemonic not found | UserID: %d", in.UserId)
		return &pb.GetMnemonicChallengeResponse{
			Code:    int32(errcode.SignerMnemonicNotFound),
			Message: "user mnemonic not found, please backup first",
		}, nil
	}

	// 4. 解析助记词
	words := parseMnemonic(mnemonic)
	if len(words) != 12 && len(words) != 24 {
		l.Logger.Errorf("GetMnemonicChallenge: invalid mnemonic length | UserID: %d, Length: %d",
			in.UserId, len(words))
		return &pb.GetMnemonicChallengeResponse{
			Code:    int32(errcode.SignerInternalError),
			Message: "invalid mnemonic format",
		}, nil
	}

	// 5. 随机选取3个不连续的位置
	challengeIndices, err := generateChallengeIndices(len(words), numChallengeWords)
	if err != nil {
		l.Logger.Errorf("GetMnemonicChallenge: failed to generate indices | Error: %v", err)
		return &pb.GetMnemonicChallengeResponse{
			Code:    int32(errcode.SignerInternalError),
			Message: "failed to generate challenge",
		}, nil
	}

	// 6. 获取正确答案
	expectedWords := make([]string, len(challengeIndices))
	for i, idx := range challengeIndices {
		expectedWords[i] = words[idx]
	}

	// 7. 创建挑战会话
	sessionID := generateSessionID(in.UserId)
	expiresAt := time.Now().Add(challengeDuration)

	session := &ChallengeSession{
		SessionID:        sessionID,
		UserID:           in.UserId,
		Purpose:          in.Purpose,
		ChallengeIndices: challengeIndices,
		ExpectedWords:    expectedWords,
		Attempts:         0,
		MaxAttempts:      maxAttempts,
		CreatedAt:        time.Now(),
		ExpiresAt:        expiresAt,
	}

	// 8. 保存到Redis
	if err := l.saveSession(session); err != nil {
		l.Logger.Errorf("GetMnemonicChallenge: failed to save session | Error: %v", err)
		return &pb.GetMnemonicChallengeResponse{
			Code:    int32(errcode.SignerInternalError),
			Message: "failed to create challenge session",
		}, nil
	}

	// 9. 更新挑战统计（非阻塞，失败不影响主流程）
	go func() {
		if backups, err := l.svcCtx.MnemonicBackupRepo.GetByUserID(in.UserId); err == nil && len(backups) > 0 {
			if err := l.svcCtx.MnemonicBackupRepo.IncrementChallengeCount(backups[0].ID); err != nil {
				l.Logger.Errorf("Failed to increment challenge count: %v", err)
			}
			if err := l.svcCtx.MnemonicBackupRepo.UpdateLastChallengedAt(backups[0].ID); err != nil {
				l.Logger.Errorf("Failed to update last challenged at: %v", err)
			}
		}
	}()

	// 10. 记录审计日志
	l.Logger.Infof("✓ Mnemonic challenge created | UserID: %d, IP: %s, SessionID: %s, Indices: %v, Purpose: %s, ExpiresAt: %s",
		in.UserId, clientIP, sessionID, challengeIndices, in.Purpose, expiresAt.Format(time.RFC3339))

	return &pb.GetMnemonicChallengeResponse{
		Code:             0,
		Message:          "challenge created successfully",
		SessionId:        sessionID,
		ChallengeIndices: challengeIndices,
		ExpiresAt:        expiresAt.Format(time.RFC3339),
	}, nil
}

// checkRateLimit 检查频率限制（每小时最多5次）
func (l *GetMnemonicChallengeLogic) checkRateLimit(userID int64) error {
	key := fmt.Sprintf("mnemonic_challenge_rate:%d", userID)

	// 使用Redis INCR命令实现计数器
	count, err := l.svcCtx.RedisClient.Incr(l.ctx, key).Result()
	if err != nil {
		l.Logger.Errorf("Failed to increment rate limit counter: %v", err)
		return err
	}

	// 第一次请求时设置1小时过期
	if count == 1 {
		err = l.svcCtx.RedisClient.Expire(l.ctx, key, time.Hour).Err()
		if err != nil {
			l.Logger.Errorf("Failed to set rate limit expiration: %v", err)
			// 不返回错误，继续执行
		}
	}

	// 检查是否超过限制（每小时5次）
	if count > 5 {
		l.Logger.Infof("Rate limit exceeded for user %d: %d requests in this hour", userID, count)
		return fmt.Errorf("rate limit exceeded: %d requests in this hour", count)
	}

	l.Logger.Infof("Rate limit check passed for user %d: %d/5 requests", userID, count)
	return nil
}

// getUserMnemonic 获取用户的助记词
// 从数据库读取用户的加密助记词并解密
func (l *GetMnemonicChallengeLogic) getUserMnemonic(userID int64) string {
	// 从数据库获取用户的助记词备份记录
	backups, err := l.svcCtx.MnemonicBackupRepo.GetByUserID(userID)
	if err != nil {
		l.Logger.Errorf("getUserMnemonic: failed to query database | UserID: %d, Error: %v", userID, err)
		return ""
	}

	if len(backups) == 0 {
		l.Logger.Infof("getUserMnemonic: no mnemonic backup found | UserID: %d", userID)
		return ""
	}

	// 使用第一个备份记录（最新的）
	backup := backups[0]

	// 检查加密字段是否完整
	if len(backup.MnemonicEncrypted) == 0 || len(backup.EncryptionSalt) == 0 || len(backup.EncryptionIV) == 0 {
		l.Logger.Errorf("getUserMnemonic: incomplete encryption data | UserID: %d, BackupID: %d", userID, backup.ID)
		return ""
	}

	// 解密助记词
	password := l.svcCtx.Config.Security.EncryptionPassword
	mnemonic, err := decryptMnemonic(backup.MnemonicEncrypted, backup.EncryptionSalt, backup.EncryptionIV, password)
	if err != nil {
		l.Logger.Errorf("getUserMnemonic: failed to decrypt mnemonic | UserID: %d, BackupID: %d, Error: %v",
			userID, backup.ID, err)
		return ""
	}

	l.Logger.Infof("✓ getUserMnemonic: mnemonic retrieved successfully | UserID: %d, BackupID: %d, WordCount: %d",
		userID, backup.ID, backup.WordCount)

	return mnemonic
}

// decryptMnemonic 解密助记词
// 使用 AES-256-GCM 解密存储的助记词
// 注意：encrypted 已经包含了 nonce（由 EncryptSeed 生成的格式：nonce + ciphertext）
func decryptMnemonic(encrypted []byte, salt []byte, iv []byte, password string) (string, error) {
	if len(encrypted) == 0 {
		return "", fmt.Errorf("encrypted data cannot be empty")
	}
	if len(salt) == 0 {
		return "", fmt.Errorf("salt cannot be empty")
	}
	if len(password) == 0 {
		return "", fmt.Errorf("password cannot be empty")
	}

	// encrypted 已经包含了完整的 nonce + ciphertext（由 utils.EncryptSeed 生成）
	// 直接使用 DecryptSeed 解密，不需要再合并 IV
	decrypted, err := utils.DecryptSeed(encrypted, salt, password)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt: %w", err)
	}

	return string(decrypted), nil
}

// parseMnemonic 解析助记词字符串为单词数组
func parseMnemonic(mnemonic string) []string {
	// 按空格分割
	words := make([]string, 0)
	word := ""
	for _, ch := range mnemonic {
		if ch == ' ' || ch == '\t' || ch == '\n' {
			if word != "" {
				words = append(words, word)
				word = ""
			}
		} else {
			word += string(ch)
		}
	}
	if word != "" {
		words = append(words, word)
	}
	return words
}

// generateChallengeIndices 生成不连续的随机挑战位置
func generateChallengeIndices(totalWords, numChallenges int) ([]int32, error) {
	if numChallenges > totalWords {
		return nil, fmt.Errorf("numChallenges (%d) cannot exceed totalWords (%d)", numChallenges, totalWords)
	}

	// 生成所有可能的位置
	available := make([]int, totalWords)
	for i := 0; i < totalWords; i++ {
		available[i] = i
	}

	// 随机打乱
	for i := len(available) - 1; i > 0; i-- {
		j, err := randomInt(i + 1)
		if err != nil {
			return nil, err
		}
		available[i], available[j] = available[j], available[i]
	}

	// 选取前N个（确保不连续）
	indices := make([]int32, 0, numChallenges)
	for _, idx := range available {
		// 检查是否与已选位置连续
		isAdjacent := false
		for _, selected := range indices {
			if abs(int32(idx)-selected) == 1 {
				isAdjacent = true
				break
			}
		}

		if !isAdjacent {
			indices = append(indices, int32(idx))
			if len(indices) == numChallenges {
				break
			}
		}
	}

	// 如果无法找到足够的不连续位置（边缘情况），放宽要求
	if len(indices) < numChallenges {
		indices = make([]int32, 0, numChallenges)
		for i := 0; i < numChallenges; i++ {
			indices = append(indices, int32(available[i]))
		}
	}

	return indices, nil
}

// randomInt 生成 [0, max) 范围内的随机整数
func randomInt(max int) (int, error) {
	nBig, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0, err
	}
	return int(nBig.Int64()), nil
}

// abs 返回绝对值
func abs(x int32) int32 {
	if x < 0 {
		return -x
	}
	return x
}

// generateSessionID 生成会话ID
func generateSessionID(userID int64) string {
	timestamp := time.Now().UnixNano()
	return fmt.Sprintf("challenge_%d_%d", userID, timestamp)
}

// saveSession 保存挑战会话到Redis
func (l *GetMnemonicChallengeLogic) saveSession(session *ChallengeSession) error {
	key := challengePrefix + session.SessionID

	// 序列化为JSON
	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	// 保存到Redis并设置过期时间
	err = l.svcCtx.RedisClient.Set(l.ctx, key, string(data), challengeDuration).Err()
	if err != nil {
		return fmt.Errorf("failed to save session to redis: %w", err)
	}

	l.Logger.Infof("✓ Session saved to Redis | Key: %s, TTL: %v, DataSize: %d bytes",
		key, challengeDuration, len(data))

	return nil
}
