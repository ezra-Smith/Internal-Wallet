package logic

import (
	"context"
	"time"

	"internalwallet/common/errcode"
	"internalwallet/proto/pb"
	"internalwallet/services/signer/rpc/internal/svc"

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

func (l *VerifyHotWalletMnemonicChallengeLogic) VerifyHotWalletMnemonicChallenge(in *pb.VerifyHotWalletMnemonicChallengeRequest) (*pb.VerifyHotWalletMnemonicChallengeResponse, error) {
	if in == nil || in.ChallengeSessionId == "" || len(in.Answers) == 0 {
		return &pb.VerifyHotWalletMnemonicChallengeResponse{Code: int32(errcode.SignerInvalidParams), Message: "invalid params"}, nil
	}

	var ch hotWalletChallengeSession
	if err := l.svcCtx.SignerCache.GetJSON(l.ctx, hotWalletChallengeSessionKeyPrefix+in.ChallengeSessionId, &ch); err != nil {
		return &pb.VerifyHotWalletMnemonicChallengeResponse{Code: int32(errcode.SignerWalletChallengeGone), Message: "challenge not found"}, nil
	}

	now := time.Now()
	if now.After(ch.ExpiresAt) {
		_ = l.svcCtx.SignerCache.Delete(l.ctx, hotWalletChallengeSessionKeyPrefix+in.ChallengeSessionId)
		return &pb.VerifyHotWalletMnemonicChallengeResponse{Code: int32(errcode.SignerWalletChallengeGone), Message: "challenge expired"}, nil
	}

	// attempt accounting
	ch.Attempts++
	if ch.Attempts > ch.MaxAttempts {
		_ = l.svcCtx.SignerCache.Delete(l.ctx, hotWalletChallengeSessionKeyPrefix+in.ChallengeSessionId)
		return &pb.VerifyHotWalletMnemonicChallengeResponse{
			Code:              int32(errcode.SignerChallengeTooManyAttempts),
			Message:           "too many attempts",
			Verified:          false,
			RemainingAttempts: 0,
		}, nil
	}

	// normalize answers into map[index]hash
	ansHash := make(map[int32]string, len(in.Answers))
	for _, a := range in.Answers {
		if a == nil {
			continue
		}
		ansHash[a.Index] = hashWordLower(a.Word)
	}

	incorrect := make([]int32, 0, len(ch.ChallengeIndices))
	for i, idx := range ch.ChallengeIndices {
		if i >= len(ch.ExpectedWordHashes) {
			incorrect = append(incorrect, idx)
			continue
		}
		exp := ch.ExpectedWordHashes[i]
		got, ok := ansHash[idx]
		if !ok || got != exp {
			incorrect = append(incorrect, idx)
		}
	}

	remaining := ch.MaxAttempts - ch.Attempts
	ttl := time.Until(ch.ExpiresAt)
	if ttl < 0 {
		ttl = 0
	}

	if len(incorrect) > 0 {
		// persist attempt count back
		_ = l.svcCtx.SignerCache.SetJSON(l.ctx, hotWalletChallengeSessionKeyPrefix+in.ChallengeSessionId, &ch, ttl)
		return &pb.VerifyHotWalletMnemonicChallengeResponse{
			Code:              int32(errcode.SignerChallengeVerifyFailed),
			Message:           "verification failed",
			Verified:          false,
			IncorrectIndices:  incorrect,
			RemainingAttempts: int32(remaining),
			BackupConfirmed:   false,
		}, nil
	}

	// verified: mark backup confirmed
	if err := l.svcCtx.WalletMasterMnemonicRepo.MarkBackupConfirmed(l.ctx, ch.SeedID, now.Local()); err != nil {
		l.Logger.Errorf("VerifyHotWalletMnemonicChallenge: mark confirmed failed: %v", err)
		return &pb.VerifyHotWalletMnemonicChallengeResponse{
			Code:              int32(errcode.SignerDatabaseError),
			Message:           "database error",
			Verified:          false,
			RemainingAttempts: int32(remaining),
		}, nil
	}

	_ = l.svcCtx.SignerCache.Delete(l.ctx, hotWalletChallengeSessionKeyPrefix+in.ChallengeSessionId)

	return &pb.VerifyHotWalletMnemonicChallengeResponse{
		Code:              0,
		Message:           "success",
		Verified:          true,
		IncorrectIndices:  nil,
		RemainingAttempts: int32(remaining),
		BackupConfirmed:   true,
		VerifiedAt:        now.Format(time.RFC3339),
	}, nil
}
