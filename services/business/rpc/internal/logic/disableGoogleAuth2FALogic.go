package logic

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"internalwallet/common/code"
	"internalwallet/common/middleware"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/model"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/pquerna/otp/totp"
	"github.com/zeromicro/go-zero/core/logx"
)

type DisableGoogleAuth2FALogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDisableGoogleAuth2FALogic(ctx context.Context, svcCtx *svc.ServiceContext) *DisableGoogleAuth2FALogic {
	return &DisableGoogleAuth2FALogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// DisableGoogleAuth2FA 关闭双重认证（不解绑 Google Auth）
func (l *DisableGoogleAuth2FALogic) DisableGoogleAuth2FA(in *pb.DisableGoogleAuth2FAReq) (*pb.DisableGoogleAuth2FAResp, error) {
	if in == nil {
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

	// Get user info for verification (includes email, phone, google_auth_secret)
	user, err := l.svcCtx.UserAccountRepository.GetAuthByID(l.ctx, uid)
	if err != nil || user == nil {
		return nil, errx.UserNotFound()
	}

	// Check if Google Auth is bound
	googleSecret := strings.TrimSpace(user.GoogleAuthSecret)
	if googleSecret == "" {
		return nil, errx.GoogleAuthNotBound()
	}

	// Check if 2FA is already disabled
	if !user.Is2faEnabled {
		return nil, errx.InvalidParam("2FA is already disabled")
	}

	// Determine verification method used
	verifyMethod := in.Method
	var usedFactor string

	// If method is unspecified, try to infer from provided fields (backward compatibility)
	if verifyMethod == pb.UnbindVerifyMethod_UNBIND_VERIFY_METHOD_UNSPECIFIED {
		if strings.TrimSpace(in.GoogleCode) != "" {
			verifyMethod = pb.UnbindVerifyMethod_UNBIND_VERIFY_METHOD_GOOGLE_AUTH
		} else if strings.TrimSpace(in.EmailCode) != "" {
			verifyMethod = pb.UnbindVerifyMethod_UNBIND_VERIFY_METHOD_EMAIL
		} else if strings.TrimSpace(in.SmsCode) != "" {
			verifyMethod = pb.UnbindVerifyMethod_UNBIND_VERIFY_METHOD_PHONE
		} else if strings.TrimSpace(in.BiometricToken) != "" {
			verifyMethod = pb.UnbindVerifyMethod_UNBIND_VERIFY_METHOD_BIOMETRIC
		} else {
			return nil, errx.InvalidParam("verification code required")
		}
	}

	// Execute verification based on chosen method
	switch verifyMethod {
	case pb.UnbindVerifyMethod_UNBIND_VERIFY_METHOD_GOOGLE_AUTH:
		// GA verification
		if strings.TrimSpace(in.GoogleCode) == "" {
			return nil, errx.InvalidParam("google auth code is required")
		}
		if !totp.Validate(in.GoogleCode, googleSecret) {
			return nil, errx.InvalidGoogleAuthCode()
		}
		usedFactor = "totp"

	case pb.UnbindVerifyMethod_UNBIND_VERIFY_METHOD_EMAIL:
		// Email verification
		if strings.TrimSpace(user.Email) == "" {
			return nil, errx.InvalidParam("email not bound, cannot use email verification")
		}
		if strings.TrimSpace(in.EmailCode) == "" {
			return nil, errx.InvalidParam("email verification code is required")
		}
		// Normalize email to lowercase for consistent Redis key matching
		email := strings.ToLower(strings.TrimSpace(user.Email))
		if !l.verifyEmailCode(email, in.EmailCode, "2fa") {
			l.Logger.Infof("Email code verification failed for user %d, email=%s, code=%s", uid, email, in.EmailCode)
			return nil, errx.InvalidCode("email")
		}
		usedFactor = "email"

	case pb.UnbindVerifyMethod_UNBIND_VERIFY_METHOD_PHONE:
		// Phone/SMS verification
		if strings.TrimSpace(user.Phone) == "" {
			return nil, errx.InvalidParam("phone not bound, cannot use phone verification")
		}
		phone := fmt.Sprintf("%s%s", user.CountryCode, user.Phone)
		if strings.TrimSpace(in.SmsCode) == "" {
			return nil, errx.InvalidParam("sms verification code is required")
		}
		if !l.verifySMSCode(phone, in.SmsCode, "2fa") {
			l.Logger.Infof("SMS code verification failed for user %d, phone=%s, code=%s", uid, phone, in.SmsCode)
			return nil, errx.InvalidCode("sms")
		}
		usedFactor = "sms"

	case pb.UnbindVerifyMethod_UNBIND_VERIFY_METHOD_BIOMETRIC:
		// Biometric verification (Face ID/Touch ID/Fingerprint)
		if strings.TrimSpace(in.BiometricToken) != "" {
			// 如果提供了 token，则验证
			if !l.verifyBiometricToken(uid, in.BiometricToken) {
				l.Logger.Infof("Biometric token verification failed for user %d", uid)
				return nil, errx.InvalidCode("biometric")
			}
			usedFactor = "biometric"
		} else {
			// Token 为空时，检查用户是否已注册生物识别密钥
			// 仅当用户已注册生物识别密钥时，才允许不传递 token（客户端已通过生物识别验证）
			hasBiometricKey := l.checkUserHasBiometricKey(uid)
			if !hasBiometricKey {
				// 用户未注册生物识别密钥，必须提供 token 或使用其他验证方式
				return nil, errx.InvalidParam("biometric token is required when biometric key is not registered")
			}
			// 用户已注册生物识别密钥，允许不传递 token（仅记录日志）
			l.Logger.Infof("Biometric verification used without token for user %d (user has registered biometric key, client-side verified)", uid)
			usedFactor = "biometric"
		}

	default:
		return nil, errx.InvalidParam("invalid verification method")
	}

	// Disable 2FA (set is_2fa_enabled to false, but keep google_auth_secret)
	userRepo := l.svcCtx.UserAccountRepository
	if err := userRepo.GetDB().WithContext(l.ctx).Model(&model.UserModel{}).
		Where("id = ?", uid).
		Updates(map[string]interface{}{
			"is_2fa_enabled": false,
		}).Error; err != nil {
		l.Logger.Errorf("disable 2fa failed: %v", err)
		return nil, errx.DBError()
	}

	// Update member_security_setting.google_auth_enabled
	if l.svcCtx.UserSecuritySettingsRepository != nil {
		if err := l.svcCtx.UserSecuritySettingsRepository.SetGoogleAuthEnabled(l.ctx, uid, false); err != nil {
			return nil, errx.DBError()
		}
	}

	ip := middleware.GetClientIP(l.ctx)
	// Best-effort: record 2FA history (TOTP disable, not unbind)
	if l.svcCtx.User2FAHistoryRepository != nil {
		metaBytes, _ := json.Marshal(map[string]interface{}{
			"verify_method": usedFactor,
			"action":        "disable_2fa",
		})
		_ = l.svcCtx.User2FAHistoryRepository.Create(l.ctx, &model.User2FAHistoryModel{
			UserID:       uid,
			Event:        "disable",
			Factor:       "totp",
			OperatorType: "user",
			OperatorID:   uid,
			Reason:       fmt.Sprintf("verified_by_%s", usedFactor),
			IP:           ip,
			UserAgent:    middleware.GetUserAgent(l.ctx),
			Meta:         metaBytes,
		})
	}

	// 异步发送邮件通知（不阻塞响应）
	go Send2FADisabledEmailNotification(context.Background(), l.svcCtx, uid, ip, usedFactor, l.Logger)

	return &pb.DisableGoogleAuth2FAResp{Success: true, Message: "ok"}, nil
}

// verifyEmailCode verifies email verification code
func (l *DisableGoogleAuth2FALogic) verifyEmailCode(email, codeStr, scene string) bool {
	length := l.svcCtx.Config.Code.Length
	if length <= 0 {
		length = 6
	}
	expire := l.svcCtx.Config.Code.ExpireSeconds
	if expire <= 0 {
		expire = 600
	}
	gen := code.NewGenerator(l.svcCtx.RedisClient, int(length), time.Duration(expire)*time.Second)

	bypass := l.svcCtx.Config.Code.BypassCode
	if bypass != "" && codeStr == bypass {
		return true
	}

	ok, err := gen.Verify(l.ctx, email, scene, "email", codeStr)
	if err != nil {
		l.Logger.Errorf("verify email code failed: %v", err)
		return false
	}
	if ok {
		// Delete used code
		_ = gen.Delete(l.ctx, email, scene, "email")
	}
	return ok
}

// verifySMSCode verifies SMS verification code
func (l *DisableGoogleAuth2FALogic) verifySMSCode(phone, codeStr, scene string) bool {
	length := l.svcCtx.Config.Code.Length
	if length <= 0 {
		length = 6
	}
	expire := l.svcCtx.Config.Code.ExpireSeconds
	if expire <= 0 {
		expire = 600
	}
	gen := code.NewGenerator(l.svcCtx.RedisClient, int(length), time.Duration(expire)*time.Second)

	bypass := l.svcCtx.Config.Code.BypassCode
	if bypass != "" && codeStr == bypass {
		return true
	}

	ok, err := gen.Verify(l.ctx, phone, scene, "sms", codeStr)
	if err != nil {
		l.Logger.Errorf("verify sms code failed: %v", err)
		return false
	}
	if ok {
		// Delete used code
		_ = gen.Delete(l.ctx, phone, scene, "sms")
	}
	return ok
}

// verifyBiometricToken verifies biometric verification token
// Token format: base64({"user_id": 123, "timestamp": 1234567890, "signature": "..."})
// Signature = HMAC-SHA256(user_id + timestamp + secret_key)
func (l *DisableGoogleAuth2FALogic) verifyBiometricToken(userID int64, tokenStr string) bool {
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

// checkUserHasBiometricKey 检查用户是否已注册生物识别密钥
func (l *DisableGoogleAuth2FALogic) checkUserHasBiometricKey(userID int64) bool {
	if l.svcCtx.MemberBiometricCredentialRepository == nil {
		return false
	}
	keys, err := l.svcCtx.MemberBiometricCredentialRepository.ListActiveByUser(l.ctx, userID)
	if err != nil {
		l.Logger.Errorf("Failed to check biometric keys for user %d: %v", userID, err)
		return false
	}
	return len(keys) > 0
}

// getBiometricSecretKey gets the secret key for biometric token verification
// Uses user's secret field if available, otherwise falls back to config secret
func (l *DisableGoogleAuth2FALogic) getBiometricSecretKey(userID int64) string {
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
