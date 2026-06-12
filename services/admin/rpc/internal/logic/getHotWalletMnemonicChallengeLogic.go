package logic

import (
	"context"
	"strings"

	"internalwallet/proto/pb"
	admininterceptor "internalwallet/services/admin/rpc/internal/interceptor"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetHotWalletMnemonicChallengeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetHotWalletMnemonicChallengeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetHotWalletMnemonicChallengeLogic {
	return &GetHotWalletMnemonicChallengeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetHotWalletMnemonicChallengeLogic) GetHotWalletMnemonicChallenge(in *pb.AdminGetHotWalletMnemonicChallengeRequest) (*pb.AdminGetHotWalletMnemonicChallengeResponse, error) {
	if current, ok := admininterceptor.GetCurrentAdmin(l.ctx); !ok || current == nil {
		return &pb.AdminGetHotWalletMnemonicChallengeResponse{Success: false, Message: "unauthorized", RequestId: resp.RequestID(l.ctx), Timestamp: resp.Timestamp()}, nil
	}
	if in == nil || strings.TrimSpace(in.InitSessionId) == "" {
		return &pb.AdminGetHotWalletMnemonicChallengeResponse{Success: false, Message: "invalid params", RequestId: resp.RequestID(l.ctx), Timestamp: resp.Timestamp()}, nil
	}
	if l.svcCtx == nil || l.svcCtx.SignerRpc == nil {
		return &pb.AdminGetHotWalletMnemonicChallengeResponse{Success: false, Message: "signer service not available", RequestId: resp.RequestID(l.ctx), Timestamp: resp.Timestamp()}, nil
	}

	signerResp, err := l.svcCtx.SignerRpc.GetHotWalletMnemonicChallenge(l.ctx, &pb.GetHotWalletMnemonicChallengeRequest{
		InitSessionId: strings.TrimSpace(in.InitSessionId),
	})
	if err != nil || signerResp == nil {
		return &pb.AdminGetHotWalletMnemonicChallengeResponse{Success: false, Message: "failed to get challenge", RequestId: resp.RequestID(l.ctx), Timestamp: resp.Timestamp()}, nil
	}

	ok2 := signerResp.Code == 0
	return &pb.AdminGetHotWalletMnemonicChallengeResponse{
		Success: ok2,
		Message: signerResp.Message,
		Data: &pb.AdminGetHotWalletMnemonicChallengeData{
			ChallengeSessionId: signerResp.ChallengeSessionId,
			ChallengeIndices:   signerResp.ChallengeIndices,
			ExpiresAt:          signerResp.ExpiresAt,
			RemainingAttempts:  signerResp.RemainingAttempts,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
