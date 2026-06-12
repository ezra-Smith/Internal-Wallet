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

type CreateWithdrawBiometricChallengeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateWithdrawBiometricChallengeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateWithdrawBiometricChallengeLogic {
	return &CreateWithdrawBiometricChallengeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 创建提币生物识别挑战（用于签名）
func (l *CreateWithdrawBiometricChallengeLogic) CreateWithdrawBiometricChallenge(in *pb.CreateWithdrawBiometricChallengeReq) (*pb.CreateBiometricChallengeResp, error) {
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

	payloadHash, err := calcWithdrawPayloadHash(in.Asset, in.Chain, in.Amount, in.Address, in.MemoTag)
	if err != nil {
		return nil, errx.InvalidParam("invalid params")
	}
	rec, err := createBiometricChallenge(l.ctx, l.svcCtx.RedisClient, userID, biometricSceneWithdraw, payloadHash, biometricChallengeTTL)
	if err != nil {
		l.Logger.Errorf("create biometric challenge failed: %v", err)
		return nil, errx.Internal("internal error")
	}

	return &pb.CreateBiometricChallengeResp{
		Success:     true,
		Message:     "ok",
		Scene:       biometricSceneWithdraw,
		ChallengeId: rec.ChallengeID,
		Challenge:   rec.Challenge,
		PayloadHash: rec.PayloadHash,
		ExpiresIn:   int64(biometricChallengeTTL.Seconds()),
	}, nil
}
