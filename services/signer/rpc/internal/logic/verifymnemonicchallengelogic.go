package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"

	"internalwallet/common/errcode"
	"internalwallet/common/middleware"
	"internalwallet/proto/pb"
	"internalwallet/services/signer/rpc/internal/svc"
)

type VerifyMnemonicChallengeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewVerifyMnemonicChallengeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *VerifyMnemonicChallengeLogic {
	return &VerifyMnemonicChallengeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// VerifyMnemonicChallenge 验证助记词挑战
// 业务规则：
// - 每次挑战最多允许3次尝试
// - 验证成功后标记用户已确认备份
// - 所有验证尝试记入审计日志
// - 连续失败3次需等待30分钟后重新发起挑战
func (l *VerifyMnemonicChallengeLogic) VerifyMnemonicChallenge(in *pb.VerifyMnemonicChallengeRequest) (*pb.VerifyMnemonicChallengeResponse, error) {
	// 获取审计信息
	userID := middleware.GetUserID(l.ctx)
	clientIP := middleware.GetClientIP(l.ctx)

	l.Logger.Infof("VerifyMnemonicChallenge called | UserID: %s, IP: %s, SessionID: %s",
		userID, clientIP, in.SessionId)

	// 1. 参数验证
	if in.SessionId == "" {
		return &pb.VerifyMnemonicChallengeResponse{
			Code:    int32(errcode.SignerInvalidParams),
			Message: "session_id is required",
		}, nil
	}

	if len(in.Answers) == 0 {
		return &pb.VerifyMnemonicChallengeResponse{
			Code:    int32(errcode.SignerInvalidParams),
			Message: "answers cannot be empty",
		}, nil
	}

	// 2. 从Redis加载挑战会话
	session, err := l.loadSession(in.SessionId)
	if err != nil {
		l.Logger.Errorf("VerifyMnemonicChallenge: session not found | SessionID: %s, Error: %v",
			in.SessionId, err)
		return &pb.VerifyMnemonicChallengeResponse{
			Code:    50013, // 挑战会话不存在
			Message: "challenge session not found or expired",
		}, nil
	}

	// 3. 检查会话是否过期
	if time.Now().After(session.ExpiresAt) {
		l.Logger.Infof("VerifyMnemonicChallenge: session expired | SessionID: %s, ExpiresAt: %s",
			in.SessionId, session.ExpiresAt.Format(time.RFC3339))
		return &pb.VerifyMnemonicChallengeResponse{
			Code:    50010, // 挑战会话已过期
			Message: "challenge session has expired",
		}, nil
	}

	// 4. 检查尝试次数
	if session.Attempts >= session.MaxAttempts {
		l.Logger.Infof("VerifyMnemonicChallenge: max attempts exceeded | SessionID: %s, Attempts: %d",
			in.SessionId, session.Attempts)
		return &pb.VerifyMnemonicChallengeResponse{
			Code:              50012, // 尝试次数过多
			Message:           "too many attempts, please request a new challenge",
			Verified:          false,
			RemainingAttempts: 0,
		}, nil
	}

	// 5. 验证答案数量
	if len(in.Answers) != len(session.ChallengeIndices) {
		return &pb.VerifyMnemonicChallengeResponse{
			Code:    int32(errcode.SignerInvalidParams),
			Message: fmt.Sprintf("expected %d answers, got %d", len(session.ChallengeIndices), len(in.Answers)),
		}, nil
	}

	// 6. 验证答案
	incorrectIndices := make([]int32, 0)
	answerMap := make(map[int32]string)
	for _, answer := range in.Answers {
		answerMap[answer.Index] = strings.ToLower(strings.TrimSpace(answer.Word))
	}

	allCorrect := true
	for i, expectedIndex := range session.ChallengeIndices {
		userAnswer, exists := answerMap[expectedIndex]
		if !exists {
			allCorrect = false
			incorrectIndices = append(incorrectIndices, expectedIndex)
			continue
		}

		expectedWord := strings.ToLower(strings.TrimSpace(session.ExpectedWords[i]))
		if userAnswer != expectedWord {
			allCorrect = false
			incorrectIndices = append(incorrectIndices, expectedIndex)
		}
	}

	// 7. 更新尝试次数
	session.Attempts++
	remainingAttempts := session.MaxAttempts - session.Attempts

	// 保存更新后的会话
	if err := l.updateSession(session); err != nil {
		l.Logger.Errorf("VerifyMnemonicChallenge: failed to update session | Error: %v", err)
	}

	// 8. 验证成功
	if allCorrect {
		// 标记用户已确认备份
		// - 更新 users 表的 mnemonic_backup_confirmed 字段
		if err := l.markBackupConfirmed(session.UserID); err != nil {
			l.Logger.Errorf("VerifyMnemonicChallenge: failed to mark backup confirmed | UserID: %d, Error: %v",
				session.UserID, err)
		}

		verifiedAt := time.Now()

		l.Logger.Infof("✓ Mnemonic challenge verified successfully | UserID: %d, IP: %s, SessionID: %s, Purpose: %s",
			session.UserID, clientIP, in.SessionId, session.Purpose)

		return &pb.VerifyMnemonicChallengeResponse{
			Code:              0,
			Message:           "verification successful",
			Verified:          true,
			BackupConfirmed:   true,
			VerifiedAt:        verifiedAt.Format(time.RFC3339),
			RemainingAttempts: int32(remainingAttempts),
		}, nil
	}

	// 9. 验证失败 - 更新失败统计
	go func() {
		if backups, err := l.svcCtx.MnemonicBackupRepo.GetByUserID(session.UserID); err == nil && len(backups) > 0 {
			if err := l.svcCtx.MnemonicBackupRepo.IncrementFailedVerifies(backups[0].ID); err != nil {
				l.Logger.Errorf("Failed to increment failed verifies: %v", err)
			}
		}
	}()

	l.Logger.Infof("✗ Mnemonic challenge verification failed | UserID: %d, IP: %s, SessionID: %s, Attempts: %d/%d, IncorrectIndices: %v",
		session.UserID, clientIP, in.SessionId, session.Attempts, session.MaxAttempts, incorrectIndices)

	return &pb.VerifyMnemonicChallengeResponse{
		Code:              50011, // 助记词验证失败
		Message:           "verification failed, incorrect words",
		Verified:          false,
		BackupConfirmed:   false,
		IncorrectIndices:  incorrectIndices,
		RemainingAttempts: int32(remainingAttempts),
	}, nil
}

// loadSession 从Redis加载挑战会话
func (l *VerifyMnemonicChallengeLogic) loadSession(sessionID string) (*ChallengeSession, error) {
	key := challengePrefix + sessionID

	// 从Redis读取
	data, err := l.svcCtx.RedisClient.Get(l.ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("session not found or expired")
		}
		return nil, fmt.Errorf("failed to load session from redis: %w", err)
	}

	// 反序列化JSON
	var session ChallengeSession
	if err := json.Unmarshal([]byte(data), &session); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	l.Logger.Infof("✓ Session loaded from Redis | Key: %s, UserID: %d, Attempts: %d/%d",
		key, session.UserID, session.Attempts, session.MaxAttempts)

	return &session, nil
}

// updateSession 更新挑战会话（更新尝试次数）
func (l *VerifyMnemonicChallengeLogic) updateSession(session *ChallengeSession) error {
	key := challengePrefix + session.SessionID

	// 序列化为JSON
	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	// 计算剩余TTL
	ttl := time.Until(session.ExpiresAt)
	if ttl <= 0 {
		l.Logger.Infof("Session already expired, not updating | Key: %s", key)
		return nil
	}

	// 保存到Redis（使用剩余的TTL）
	err = l.svcCtx.RedisClient.Set(l.ctx, key, string(data), ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to update session in redis: %w", err)
	}

	l.Logger.Infof("✓ Session updated in Redis | Key: %s, Attempts: %d/%d, RemainingTTL: %v",
		key, session.Attempts, session.MaxAttempts, ttl)

	return nil
}

// markBackupConfirmed 标记用户已确认备份
func (l *VerifyMnemonicChallengeLogic) markBackupConfirmed(userID int64) error {
	// 获取用户的助记词备份记录
	backups, err := l.svcCtx.MnemonicBackupRepo.GetByUserID(userID)
	if err != nil {
		return fmt.Errorf("failed to query backup records: %w", err)
	}

	if len(backups) == 0 {
		l.Logger.Infof("markBackupConfirmed: no backup found for user %d", userID)
		return nil
	}

	// 更新第一个备份记录（最新的）
	backup := backups[0]

	// 更新备份确认状态
	if err := l.svcCtx.MnemonicBackupRepo.UpdateBackupConfirmed(backup.ID); err != nil {
		return fmt.Errorf("failed to update backup confirmed status: %w", err)
	}

	// 增加成功验证次数
	if err := l.svcCtx.MnemonicBackupRepo.IncrementSuccessfulVerifies(backup.ID); err != nil {
		l.Logger.Errorf("Failed to increment successful verifies: %v", err)
		// 不返回错误，继续执行
	}

	// 更新最后挑战时间
	if err := l.svcCtx.MnemonicBackupRepo.UpdateLastChallengedAt(backup.ID); err != nil {
		l.Logger.Errorf("Failed to update last challenged at: %v", err)
		// 不返回错误，继续执行
	}

	l.Logger.Infof("✓ User backup confirmed | UserID: %d, BackupID: %d", userID, backup.ID)
	return nil
}
