package logic

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/redis/go-redis/v9"
)

func verifyBiometricProof(ctx context.Context, svcCtx *svc.ServiceContext, userID int64, scene string, payloadHash string, proof *pb.BiometricProof) error {
	if userID <= 0 {
		return errx.Unauthorized("unauthorized")
	}
	scene = strings.TrimSpace(scene)
	payloadHash = strings.TrimSpace(payloadHash)
	if scene == "" || payloadHash == "" {
		return errx.Internal("internal error")
	}
	if proof == nil {
		// 业务上属于“验证失败”（客户端可回退 trade_password），不应被映射成通用“参数错误”
		return errx.VerifyFailed("biometric required")
	}
	if svcCtx == nil || svcCtx.RedisClient == nil || svcCtx.MemberBiometricCredentialRepository == nil {
		return errx.Internal("biometric unavailable")
	}

	keyID := strings.TrimSpace(proof.KeyId)
	challengeID := strings.TrimSpace(proof.ChallengeId)
	signatureB64 := strings.TrimSpace(proof.Signature)
	if keyID == "" || challengeID == "" || signatureB64 == "" {
		return errx.VerifyFailed("invalid biometric proof")
	}

	cred, err := svcCtx.MemberBiometricCredentialRepository.GetActiveByUserKeyID(ctx, userID, keyID)
	if err != nil {
		return errx.Internal("internal error")
	}
	if cred == nil {
		// 用户侧常见：本地开关=true，但服务端未注册/已解绑/数据库被重置
		return errx.VerifyFailed("biometric key not found")
	}
	if strings.ToUpper(strings.TrimSpace(cred.Algorithm)) != "ES256" {
		return errx.VerifyFailed("biometric algorithm unsupported")
	}

	pub, err := parseES256PublicKey(cred.PublicKeyFormat, cred.PublicKey)
	if err != nil {
		return errx.VerifyFailed("biometric key invalid")
	}

	rec, err := consumeBiometricChallenge(ctx, svcCtx.RedisClient, challengeID)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return errx.VerifyFailed("biometric challenge expired")
		}
		return errx.Internal("internal error")
	}
	if rec == nil || strings.TrimSpace(rec.ChallengeID) == "" || strings.TrimSpace(rec.Challenge) == "" {
		return errx.VerifyFailed("biometric challenge invalid")
	}
	if rec.UserID != userID {
		return errx.VerifyFailed("biometric challenge invalid")
	}
	if strings.TrimSpace(rec.Scene) != scene {
		return errx.VerifyFailed("biometric challenge invalid")
	}
	if strings.TrimSpace(rec.PayloadHash) != payloadHash {
		return errx.VerifyFailed("biometric challenge invalid")
	}
	if rec.ExpiresAt > 0 && time.Now().Unix() > rec.ExpiresAt {
		return errx.VerifyFailed("biometric challenge expired")
	}

	sigBytes, err := decodeBase64Any(signatureB64)
	if err != nil {
		return errx.VerifyFailed("biometric signature invalid")
	}

	// 协议（swagger）约定：对 payload_hash(sha256 hex) 做“裸 ECDSA”签名（ASN.1 DER），
	// 服务端验签应直接以 payload_hash 的 bytes（32字节）作为 digest 进行 VerifyASN1。
	//
	// 注意：challenge 仍然是一次性校验与防重放（GetDel）用途：这里仍校验 challenge_id/user_id/scene/payload_hash 一致。
	hashBytes, err := hex.DecodeString(strings.TrimSpace(rec.PayloadHash))
	if err != nil || len(hashBytes) == 0 {
		return errx.VerifyFailed("biometric signature invalid")
	}
	if !ecdsa.VerifyASN1(pub, hashBytes, sigBytes) {
		// 兼容旧实现：部分历史客户端可能签的是 BIOAUTH1 message 的 SHA256
		msg := biometricSignMessage(userID, scene, rec.ChallengeID, rec.Challenge, rec.PayloadHash)
		if !verifyES256Signature(pub, msg, sigBytes) {
			return errx.VerifyFailed("biometric signature invalid")
		}
	}
	return nil
}

