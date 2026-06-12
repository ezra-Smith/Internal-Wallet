package logic

import (
	"context"
	"strconv"
	"strings"

	"internalwallet/common/middleware"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateResetTradePasswordBiometricChallengeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateResetTradePasswordBiometricChallengeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateResetTradePasswordBiometricChallengeLogic {
	return &CreateResetTradePasswordBiometricChallengeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 创建重置交易密码生物识别挑战（用于签名）
func (l *CreateResetTradePasswordBiometricChallengeLogic) CreateResetTradePasswordBiometricChallenge(in *pb.CreateResetTradePasswordBiometricChallengeReq) (*pb.CreateBiometricChallengeResp, error) {
	if in == nil {
		return nil, errx.InvalidParam("invalid params")
	}
	uidStr := middleware.GetUserID(l.ctx)
	if uidStr == "" {
		return nil, errx.Unauthorized("unauthorized")
	}
	userID, _ := strconv.ParseInt(strings.TrimSpace(uidStr), 10, 64)
	if userID <= 0 {
		return nil, errx.Unauthorized("unauthorized")
	}
	if l.svcCtx == nil || l.svcCtx.RedisClient == nil {
		return nil, errx.Internal("redis unavailable")
	}

	newTradePassword := strings.TrimSpace(in.NewTradePassword)
	if newTradePassword == "" {
		return nil, errx.NewTradePasswordRequired()
	}
	if len(newTradePassword) < 6 {
		return nil, errx.TradePasswordTooShort(6)
	}

	payloadHash, err := calcResetTradePasswordPayloadHash(userID, newTradePassword)
	if err != nil {
		return nil, errx.InvalidParam("invalid params")
	}
	rec, err := createBiometricChallenge(l.ctx, l.svcCtx.RedisClient, userID, biometricSceneResetTradePassword, payloadHash, biometricChallengeTTL)
	if err != nil {
		l.Logger.Errorf("create biometric challenge failed: %v", err)
		return nil, errx.Internal("internal error")
	}
	return &pb.CreateBiometricChallengeResp{
		Success:     true,
		Message:     "ok",
		Scene:       biometricSceneResetTradePassword,
		ChallengeId: rec.ChallengeID,
		Challenge:   rec.Challenge,
		PayloadHash: rec.PayloadHash,
		ExpiresIn:   int64(biometricChallengeTTL.Seconds()),
	}, nil
}

