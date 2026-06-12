package logic

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	biometricSceneWithdraw                   = "withdraw"
	biometricSceneWithdrawAuditWhitelistBind = "withdraw_audit_whitelist_bind"
	biometricSceneResetTradePassword         = "reset_trade_password"

	biometricChallengeTTL = 120 * time.Second
)

type biometricChallengeRecord struct {
	ChallengeID string `json:"challenge_id"`
	Challenge   string `json:"challenge"`
	UserID      int64  `json:"user_id"`
	Scene       string `json:"scene"`
	PayloadHash string `json:"payload_hash"`
	CreatedAt   int64  `json:"created_at"`
	ExpiresAt   int64  `json:"expires_at"`
}

func biometricChallengeRedisKey(challengeID string) string {
	return "biz:biometric:challenge:" + strings.TrimSpace(challengeID)
}

func createBiometricChallenge(ctx context.Context, rdb *redis.Client, userID int64, scene string, payloadHash string, ttl time.Duration) (*biometricChallengeRecord, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("invalid user_id")
	}
	scene = strings.TrimSpace(scene)
	if scene == "" {
		return nil, fmt.Errorf("invalid scene")
	}
	payloadHash = strings.TrimSpace(payloadHash)
	if payloadHash == "" {
		return nil, fmt.Errorf("invalid payload_hash")
	}
	if ttl <= 0 {
		ttl = biometricChallengeTTL
	}
	if rdb == nil {
		return nil, fmt.Errorf("redis unavailable")
	}

	challengeID := uuid.NewString()
	challenge, err := randomChallengeString(32)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	rec := &biometricChallengeRecord{
		ChallengeID: challengeID,
		Challenge:   challenge,
		UserID:      userID,
		Scene:       scene,
		PayloadHash: payloadHash,
		CreatedAt:   now.Unix(),
		ExpiresAt:   now.Add(ttl).Unix(),
	}
	val, _ := json.Marshal(rec)
	if err := rdb.Set(ctx, biometricChallengeRedisKey(challengeID), val, ttl).Err(); err != nil {
		return nil, err
	}
	return rec, nil
}

func consumeBiometricChallenge(ctx context.Context, rdb *redis.Client, challengeID string) (*biometricChallengeRecord, error) {
	challengeID = strings.TrimSpace(challengeID)
	if challengeID == "" {
		return nil, fmt.Errorf("invalid challenge_id")
	}
	if rdb == nil {
		return nil, fmt.Errorf("redis unavailable")
	}

	key := biometricChallengeRedisKey(challengeID)
	val, err := rdb.GetDel(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	var rec biometricChallengeRecord
	if err := json.Unmarshal([]byte(val), &rec); err != nil {
		return nil, err
	}
	return &rec, nil
}

func randomChallengeString(nbytes int) (string, error) {
	if nbytes <= 0 {
		nbytes = 32
	}
	b := make([]byte, nbytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func biometricSignMessage(userID int64, scene string, challengeID string, challenge string, payloadHash string) []byte {
	scene = strings.TrimSpace(scene)
	challengeID = strings.TrimSpace(challengeID)
	challenge = strings.TrimSpace(challenge)
	payloadHash = strings.TrimSpace(payloadHash)
	return []byte(fmt.Sprintf("BIOAUTH1\nscene:%s\nuser_id:%d\nchallenge_id:%s\nchallenge:%s\npayload_hash:%s\n", scene, userID, challengeID, challenge, payloadHash))
}

func sha256HexBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func sha256HexJSON(v interface{}) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return sha256HexBytes(b), nil
}

type resetTradePasswordBiometricPayload struct {
	Scene            string `json:"scene"`
	UserID           int64  `json:"user_id"`
	NewTradePassword string `json:"new_trade_password"`
}

func calcResetTradePasswordPayloadHash(userID int64, newTradePassword string) (string, error) {
	if userID <= 0 {
		return "", fmt.Errorf("invalid user_id")
	}
	newTradePassword = strings.TrimSpace(newTradePassword)
	if newTradePassword == "" {
		return "", fmt.Errorf("invalid new_trade_password")
	}
	return sha256HexJSON(resetTradePasswordBiometricPayload{
		Scene:            biometricSceneResetTradePassword,
		UserID:           userID,
		NewTradePassword: newTradePassword,
	})
}

func decodeBase64Any(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("empty base64")
	}
	if b, err := base64.StdEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	if b, err := base64.RawStdEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	if b, err := base64.URLEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	if b, err := base64.RawURLEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	return nil, fmt.Errorf("invalid base64")
}

func parseES256PublicKey(format string, pubKey []byte) (*ecdsa.PublicKey, error) {
	format = strings.ToUpper(strings.TrimSpace(format))
	if len(pubKey) == 0 {
		return nil, fmt.Errorf("empty public key")
	}
	switch format {
	case "SPKI_DER_BASE64", "SPKI_DER", "SPKI":
		parsed, err := x509.ParsePKIXPublicKey(pubKey)
		if err != nil {
			return nil, err
		}
		pk, ok := parsed.(*ecdsa.PublicKey)
		if !ok {
			return nil, fmt.Errorf("not ecdsa public key")
		}
		if pk.Curve != elliptic.P256() {
			return nil, fmt.Errorf("unexpected curve")
		}
		return pk, nil
	case "X963_UNCOMPRESSED_BASE64", "X963_UNCOMPRESSED", "X9_63_UNCOMPRESSED":
		x, y := elliptic.Unmarshal(elliptic.P256(), pubKey)
		if x == nil || y == nil {
			return nil, fmt.Errorf("invalid x9.63 public key")
		}
		return &ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}, nil
	default:
		return nil, fmt.Errorf("unsupported public key format")
	}
}

func verifyES256Signature(pub *ecdsa.PublicKey, message []byte, signatureDER []byte) bool {
	if pub == nil || len(message) == 0 || len(signatureDER) == 0 {
		return false
	}
	digest := sha256.Sum256(message)
	return ecdsa.VerifyASN1(pub, digest[:], signatureDER)
}
