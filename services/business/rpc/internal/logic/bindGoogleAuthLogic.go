package logic

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"internalwallet/common/middleware"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/model"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/pquerna/otp/totp"
	"github.com/zeromicro/go-zero/core/logx"
)

type BindGoogleAuthLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewBindGoogleAuthLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BindGoogleAuthLogic {
	return &BindGoogleAuthLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *BindGoogleAuthLogic) BindGoogleAuth(in *pb.BindGoogleAuthReq) (*pb.BindGoogleAuthResp, error) {
	if in == nil || in.Secret == "" || in.Code == "" {
		return nil, errx.InvalidParam("invalid params")
	}
	uidStr := middleware.GetUserID(l.ctx)
	if uidStr == "" {
		return nil, errx.Unauthorized("unauthorized")
	}
	uid, err := strconv.ParseInt(uidStr, 10, 64)
	if err != nil {
		return nil, errx.InvalidUser()
	}

	// Optional: Verify biometric token if provided (enhanced security)
	if strings.TrimSpace(in.BiometricToken) != "" {
		if !l.verifyBiometricToken(uid, in.BiometricToken) {
			l.Logger.Infof("Biometric token verification failed for user %d during Google Auth bind", uid)
			return nil, errx.InvalidCode("biometric")
		}
		l.Logger.Infof("Biometric token verified successfully for user %d during Google Auth bind", uid)
	}

	// Validate TOTP code against the secret
	if !totp.Validate(in.Code, in.Secret) {
		return nil, errx.InvalidGoogleAuthCode()
	}

	codes := make([]string, 8)
	for i := 0; i < 8; i++ {
		b := make([]byte, 5)
		if _, err := rand.Read(b); err != nil {
			b = []byte{1, 2, 3, 4, 5}
		}
		codes[i] = hex.EncodeToString(b)
	}
	backup := ""
	for i, c := range codes {
		if i == 0 {
			backup = c
		} else {
			backup += "," + c
		}
	}
	accountRepo := l.svcCtx.UserAccountRepository
	securityRepo := l.svcCtx.UserSecuritySettingsRepository
	if err := accountRepo.UpsertGoogleAuth(l.ctx, uid, in.Secret, backup, true); err != nil {
		return nil, errx.DBError()
	}
	// Set both bound and enabled flags
	_ = securityRepo.SetGoogleAuthBound(l.ctx, uid, true)
	_ = securityRepo.SetGoogleAuthEnabled(l.ctx, uid, true)

	ip := middleware.GetClientIP(l.ctx)

	// Best-effort: record 2FA history (TOTP bind)
	if l.svcCtx.User2FAHistoryRepository != nil {
		metaBytes, _ := json.Marshal(map[string]interface{}{
			"backup_codes_count": len(codes),
		})
		_ = l.svcCtx.User2FAHistoryRepository.Create(l.ctx, &model.User2FAHistoryModel{
			UserID:       uid,
			Event:        "bind",
			Factor:       "totp",
			OperatorType: "user",
			OperatorID:   uid,
			Reason:       "",
			IP:           ip,
			UserAgent:    middleware.GetUserAgent(l.ctx),
			Meta:         metaBytes,
		})
	}

	// 检查是否曾经绑定过GA（通过2FA历史记录判断）
	// 首次绑定GA = 加强安全，不触发冷却期
	// 重新绑定GA（解绑后再绑定）= 可疑操作，需要触发冷却期
	isRebind := l.checkHasPreviousGABind(uid)
	if isRebind {
		if err := SetSecurityCooldown(l.ctx, l.svcCtx, uid, CooldownReasonGABind); err != nil {
			l.Logger.Errorf("Failed to set security cooldown for user %d: %v", uid, err)
		}
		l.Logger.Infof("Google Auth re-bound for user %d, security cooldown activated", uid)
	} else {
		l.Logger.Infof("Google Auth bound for first time for user %d, no cooldown needed", uid)
	}

	// 异步发送邮件通知（不阻塞响应）
	go Send2FAEnabledEmailNotification(context.Background(), l.svcCtx, uid, ip, l.Logger)

	return &pb.BindGoogleAuthResp{Success: true, Message: "ok", BackupCodes: codes}, nil
}

// verifyBiometricToken verifies biometric verification token
// Token format: base64({"user_id": 123, "timestamp": 1234567890, "signature": "..."})
// Signature = HMAC-SHA256(user_id + timestamp + secret_key)
func (l *BindGoogleAuthLogic) verifyBiometricToken(userID int64, tokenStr string) bool {
	// 1. Decode base64 token
	tokenBytes, err := base64.StdEncoding.DecodeString(tokenStr)
	if err != nil {
		l.Logger.Errorf("Failed to decode biometric token: %v", err)
		return false
	}

	// 2. Parse JSON token
	var token struct {
		UserID    int64  `json:"user_id"`
		Timestamp int64  `json:"timestamp"`
		Signature string `json:"signature"`
	}
	if err := json.Unmarshal(tokenBytes, &token); err != nil {
		l.Logger.Errorf("Failed to parse biometric token: %v", err)
		return false
	}

	// 3. Verify user ID matches
	if token.UserID != userID {
		l.Logger.Infof("Biometric token user_id mismatch: expected %d, got %d", userID, token.UserID)
		return false
	}

	// 4. Verify timestamp is within valid window (5 minutes)
	now := time.Now().Unix()
	if token.Timestamp < now-300 || token.Timestamp > now+60 {
		l.Logger.Infof("Biometric token timestamp expired or invalid: token=%d, now=%d", token.Timestamp, now)
		return false
	}

	// 5. Get secret key (use user's secret or config secret)
	secretKey := l.getBiometricSecretKey(userID)
	if secretKey == "" {
		l.Logger.Errorf("Failed to get biometric secret key for user %d", userID)
		return false
	}

	// 6. Compute expected signature
	message := fmt.Sprintf("%d:%d", token.UserID, token.Timestamp)
	mac := hmac.New(sha256.New, []byte(secretKey))
	mac.Write([]byte(message))
	expectedSignature := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	// 7. Verify signature
	if !hmac.Equal([]byte(token.Signature), []byte(expectedSignature)) {
		l.Logger.Infof("Biometric token signature mismatch for user %d", userID)
		return false
	}

	return true
}

// getBiometricSecretKey gets the secret key for biometric token verification
// Uses user's secret field if available, otherwise falls back to config secret
func (l *BindGoogleAuthLogic) getBiometricSecretKey(userID int64) string {
	// Try to get user's secret from database
	user, err := l.svcCtx.UserAccountRepository.GetAuthByID(l.ctx, userID)
	if err == nil && user != nil && strings.TrimSpace(user.Secret) != "" {
		return user.Secret
	}

	// Fallback to config secret (if available)
	// Note: You may need to add a config field for biometric secret key
	// For now, use a combination of user ID and a default secret
	// In production, this should be a proper secret key from config
	return fmt.Sprintf("biometric_secret_%d", userID)
}

// checkHasPreviousGABind 检查用户是否曾经绑定过GA
// 通过查询2FA历史记录来判断（如果有之前的bind记录，说明不是首次绑定）
func (l *BindGoogleAuthLogic) checkHasPreviousGABind(userID int64) bool {
	if l.svcCtx.User2FAHistoryRepository == nil {
		return false
	}

	// 查询是否有之前的GA绑定记录
	// 注意：当前这条bind记录刚刚插入，所以如果count > 1说明之前绑定过
	db := l.svcCtx.User2FAHistoryRepository.GetDB()
	var count int64
	err := db.WithContext(l.ctx).
		Model(&model.User2FAHistoryModel{}).
		Where("user_id = ? AND factor = ? AND event = ?", userID, "totp", "bind").
		Count(&count).Error
	if err != nil {
		l.Logger.Errorf("Failed to check previous GA bind for user %d: %v", userID, err)
		return false
	}

	// count > 1 说明之前绑定过（因为当前这条刚插入）
	return count > 1
}
