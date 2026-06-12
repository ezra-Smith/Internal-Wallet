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

type CreateWithdrawAuditWhitelistBindingBiometricChallengeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateWithdrawAuditWhitelistBindingBiometricChallengeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateWithdrawAuditWhitelistBindingBiometricChallengeLogic {
	return &CreateWithdrawAuditWhitelistBindingBiometricChallengeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 创建提现免审白名单绑定生物识别挑战（用于签名）
func (l *CreateWithdrawAuditWhitelistBindingBiometricChallengeLogic) CreateWithdrawAuditWhitelistBindingBiometricChallenge(in *pb.CreateWithdrawAuditWhitelistBindingBiometricChallengeReq) (*pb.CreateBiometricChallengeResp, error) {
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

	var payloadHash string
	var err error
	if len(in.Items) == 0 {
		payloadHash, err = calcWithdrawAuditWhitelistUnbindAllPayloadHash()
	} else {
		payloadHash, err = calcWithdrawAuditWhitelistBindingPayloadHash(in.Items)
	}
	if err != nil {
		return nil, errx.InvalidParam("invalid params")
	}
	rec, err := createBiometricChallenge(l.ctx, l.svcCtx.RedisClient, userID, biometricSceneWithdrawAuditWhitelistBind, payloadHash, biometricChallengeTTL)
	if err != nil {
		l.Logger.Errorf("create biometric challenge failed: %v", err)
		return nil, errx.Internal("internal error")
	}

	return &pb.CreateBiometricChallengeResp{
		Success:     true,
		Message:     "ok",
		Scene:       biometricSceneWithdrawAuditWhitelistBind,
		ChallengeId: rec.ChallengeID,
		Challenge:   rec.Challenge,
		PayloadHash: rec.PayloadHash,
		ExpiresIn:   int64(biometricChallengeTTL.Seconds()),
	}, nil
}
