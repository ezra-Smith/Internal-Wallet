package logic

import (
	"context"
	"strconv"
	"strings"

	"internalwallet/common/middleware"
	"internalwallet/common/security"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/model"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type RegisterBiometricKeyLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRegisterBiometricKeyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RegisterBiometricKeyLogic {
	return &RegisterBiometricKeyLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 我的页-使用设置-生物识别（Web2）
func (l *RegisterBiometricKeyLogic) RegisterBiometricKey(in *pb.RegisterBiometricKeyReq) (*pb.RegisterBiometricKeyResp, error) {
	if in == nil {
		return nil, errx.InvalidParam("invalid params")
	}
	uidStr := middleware.GetUserID(l.ctx)
	if uidStr == "" {
		return nil, errx.Unauthorized("unauthorized")
	}
	uid, err := parseUserID(uidStr)
	if err != nil || uid <= 0 {
		return nil, errx.Unauthorized("unauthorized")
	}

	if l.svcCtx == nil || l.svcCtx.UserAccountRepository == nil || l.svcCtx.MemberBiometricCredentialRepository == nil {
		return nil, errx.Internal("db unavailable")
	}

	// Verify trade password (required for enabling biometric key)
	tradePassword := strings.TrimSpace(in.TradePassword)
	if tradePassword == "" {
		return nil, errx.TradePasswordRequired()
	}
	tradePasswordHash, hasTradePassword, err := l.svcCtx.UserAccountRepository.GetTradePasswordHash(l.ctx, uid)
	if err != nil {
		l.Logger.Errorf("Failed to get trade password hash: %v", err)
		return nil, errx.Internal("internal error")
	}
	if !hasTradePassword || strings.TrimSpace(tradePasswordHash) == "" {
		return nil, errx.TradePasswordNotSet()
	}
	if !security.VerifyPassword(tradePasswordHash, tradePassword) {
		return nil, errx.InvalidTradePassword()
	}

	keyID := strings.TrimSpace(in.KeyId)
	if keyID == "" {
		return nil, errx.InvalidParamWithFields("invalid params", map[string]string{"key_id": "required"})
	}
	algorithm := strings.ToUpper(strings.TrimSpace(in.Algorithm))
	if algorithm == "" {
		algorithm = "ES256"
	}
	if algorithm != "ES256" {
		return nil, errx.InvalidParamWithFields("invalid params", map[string]string{"algorithm": "unsupported"})
	}
	pubKeyFormat := strings.ToUpper(strings.TrimSpace(in.PublicKeyFormat))
	if pubKeyFormat == "" {
		return nil, errx.InvalidParamWithFields("invalid params", map[string]string{"public_key_format": "required"})
	}
	pubKeyBytes, err := decodeBase64Any(in.PublicKey)
	if err != nil {
		return nil, errx.InvalidParamWithFields("invalid params", map[string]string{"public_key": "invalid"})
	}
	if _, err := parseES256PublicKey(pubKeyFormat, pubKeyBytes); err != nil {
		return nil, errx.InvalidParamWithFields("invalid params", map[string]string{"public_key": "invalid"})
	}

	cred := &model.MemberBiometricCredentialModel{
		UserId:          uid,
		KeyId:           keyID,
		Algorithm:       algorithm,
		PublicKeyFormat: pubKeyFormat,
		PublicKey:       pubKeyBytes,
		Status:          1,
		Platform:        strings.TrimSpace(middleware.GetPlatform(l.ctx)),
		DeviceId:        strings.TrimSpace(middleware.GetDeviceID(l.ctx)),
		AppVersion:      strings.TrimSpace(middleware.GetAppVersion(l.ctx)),
	}
	if err := l.svcCtx.MemberBiometricCredentialRepository.Upsert(l.ctx, cred); err != nil {
		l.Logger.Errorf("Upsert biometric credential failed: %v", err)
		return nil, errx.Internal("internal error")
	}

	return &pb.RegisterBiometricKeyResp{Success: true, Message: "ok", KeyId: keyID}, nil
}

func parseUserID(uidStr string) (int64, error) {
	// local helper: keep parsing consistent across logics
	return strconv.ParseInt(strings.TrimSpace(uidStr), 10, 64)
}
