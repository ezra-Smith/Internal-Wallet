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

type VerifyHotWalletMnemonicChallengeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewVerifyHotWalletMnemonicChallengeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *VerifyHotWalletMnemonicChallengeLogic {
	return &VerifyHotWalletMnemonicChallengeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *VerifyHotWalletMnemonicChallengeLogic) VerifyHotWalletMnemonicChallenge(in *pb.AdminVerifyHotWalletMnemonicChallengeRequest) (*pb.AdminVerifyHotWalletMnemonicChallengeResponse, error) {
	if current, ok := admininterceptor.GetCurrentAdmin(l.ctx); !ok || current == nil {
		return &pb.AdminVerifyHotWalletMnemonicChallengeResponse{Success: false, Message: "unauthorized", RequestId: resp.RequestID(l.ctx), Timestamp: resp.Timestamp()}, nil
	}
	if in == nil || strings.TrimSpace(in.ChallengeSessionId) == "" || len(in.Answers) == 0 {
		return &pb.AdminVerifyHotWalletMnemonicChallengeResponse{Success: false, Message: "invalid params", RequestId: resp.RequestID(l.ctx), Timestamp: resp.Timestamp()}, nil
	}
	if l.svcCtx == nil || l.svcCtx.SignerRpc == nil {
		return &pb.AdminVerifyHotWalletMnemonicChallengeResponse{Success: false, Message: "signer service not available", RequestId: resp.RequestID(l.ctx), Timestamp: resp.Timestamp()}, nil
	}

	answers := make([]*pb.MnemonicAnswer, 0, len(in.Answers))
	for _, a := range in.Answers {
		if a == nil {
			continue
		}
		answers = append(answers, &pb.MnemonicAnswer{
			Index: a.Index,
			Word:  a.Word,
		})
	}

	signerResp, err := l.svcCtx.SignerRpc.VerifyHotWalletMnemonicChallenge(l.ctx, &pb.VerifyHotWalletMnemonicChallengeRequest{
		ChallengeSessionId: strings.TrimSpace(in.ChallengeSessionId),
		Answers:            answers,
	})
	if err != nil || signerResp == nil {
		return &pb.AdminVerifyHotWalletMnemonicChallengeResponse{Success: false, Message: "failed to verify", RequestId: resp.RequestID(l.ctx), Timestamp: resp.Timestamp()}, nil
	}

	ok2 := signerResp.Code == 0 && signerResp.Verified
	return &pb.AdminVerifyHotWalletMnemonicChallengeResponse{
		Success: ok2,
		Message: signerResp.Message,
		Data: &pb.AdminVerifyHotWalletMnemonicChallengeData{
			Verified:          signerResp.Verified,
			BackupConfirmed:   signerResp.BackupConfirmed,
			IncorrectIndices:  signerResp.IncorrectIndices,
			RemainingAttempts: signerResp.RemainingAttempts,
			VerifiedAt:        signerResp.VerifiedAt,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
