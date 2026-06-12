package logic

import (
	"context"
	"crypto/rand"
	"math/big"
	"time"

	"internalwallet/common/errcode"
	"internalwallet/common/utils"
	"internalwallet/proto/pb"
	"internalwallet/services/signer/rpc/internal/repository"
	"internalwallet/services/signer/rpc/internal/svc"

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

func (l *GetHotWalletMnemonicChallengeLogic) GetHotWalletMnemonicChallenge(in *pb.GetHotWalletMnemonicChallengeRequest) (*pb.GetHotWalletMnemonicChallengeResponse, error) {
	if in == nil || in.InitSessionId == "" {
		return &pb.GetHotWalletMnemonicChallengeResponse{Code: int32(errcode.SignerInvalidParams), Message: "invalid params"}, nil
	}

	var sess hotWalletInitSession
	if err := l.svcCtx.SignerCache.GetJSON(l.ctx, hotWalletInitSessionKeyPrefix+in.InitSessionId, &sess); err != nil {
		return &pb.GetHotWalletMnemonicChallengeResponse{Code: int32(errcode.SignerWalletInitSessionGone), Message: "init session not found"}, nil
	}
	if time.Now().After(sess.ExpiresAt) {
		_ = l.svcCtx.SignerCache.Delete(l.ctx, hotWalletInitSessionKeyPrefix+in.InitSessionId)
		return &pb.GetHotWalletMnemonicChallengeResponse{Code: int32(errcode.SignerWalletInitSessionGone), Message: "init session expired"}, nil
	}

	row, err := l.svcCtx.WalletMasterMnemonicRepo.FindBySeedID(l.ctx, sess.SeedID)
	if err != nil {
		if err == repository.ErrWalletMasterMnemonicNotFound {
			return &pb.GetHotWalletMnemonicChallengeResponse{Code: int32(errcode.SignerWalletUninitialized), Message: "wallet not initialized"}, nil
		}
		l.Logger.Errorf("GetHotWalletMnemonicChallenge: query failed: %v", err)
		return &pb.GetHotWalletMnemonicChallengeResponse{Code: int32(errcode.SignerDatabaseError), Message: "database error"}, nil
	}

	plain, dErr := utils.DecryptSeed(row.MnemonicEncrypted, row.MnemonicSalt, sess.UnlockPassword)
	if dErr != nil {
		return &pb.GetHotWalletMnemonicChallengeResponse{Code: int32(errcode.SignerWalletUnlockFailed), Message: "decrypt mnemonic failed"}, nil
	}
	words := splitMnemonicWords(string(plain))
	if len(words) != hotWalletWordCount {
		return &pb.GetHotWalletMnemonicChallengeResponse{Code: int32(errcode.SignerInternalError), Message: "invalid mnemonic length"}, nil
	}

	indices, iErr := randUniqueIndices(hotWalletWordCount, 4)
	if iErr != nil {
		l.Logger.Errorf("GetHotWalletMnemonicChallenge: rand indices failed: %v", iErr)
		return &pb.GetHotWalletMnemonicChallengeResponse{Code: int32(errcode.SignerInternalError), Message: "internal error"}, nil
	}

	hashes := make([]string, 0, len(indices))
	for _, idx := range indices {
		hashes = append(hashes, hashWordLower(words[idx]))
	}

	now := time.Now()
	expiresAt := now.Add(hotWalletChallengeSessionTTL)
	ch := hotWalletChallengeSession{
		ChallengeSessionID: "ch_" + randHex(18),
		SeedID:             sess.SeedID,
		ChallengeIndices:   indices,
		ExpectedWordHashes: hashes,
		Attempts:           0,
		MaxAttempts:        hotWalletChallengeMaxTry,
		CreatedAt:          now,
		ExpiresAt:          expiresAt,
	}
	if err := l.svcCtx.SignerCache.SetJSON(l.ctx, hotWalletChallengeSessionKeyPrefix+ch.ChallengeSessionID, &ch, hotWalletChallengeSessionTTL); err != nil {
		l.Logger.Errorf("GetHotWalletMnemonicChallenge: save session failed: %v", err)
		return &pb.GetHotWalletMnemonicChallengeResponse{Code: int32(errcode.SignerInternalError), Message: "cache error"}, nil
	}

	return &pb.GetHotWalletMnemonicChallengeResponse{
		Code:               0,
		Message:            "success",
		ChallengeSessionId: ch.ChallengeSessionID,
		ChallengeIndices:   indices,
		ExpiresAt:          expiresAt.Format(time.RFC3339),
		RemainingAttempts:  int32(hotWalletChallengeMaxTry),
	}, nil
}

func randUniqueIndices(n int, k int) ([]int32, error) {
	if n <= 0 || k <= 0 || k > n {
		return nil, nil
	}
	used := make(map[int]struct{}, k)
	out := make([]int32, 0, k)
	for len(out) < k {
		r, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
		if err != nil {
			return nil, err
		}
		idx := int(r.Int64())
		if _, ok := used[idx]; ok {
			continue
		}
		used[idx] = struct{}{}
		out = append(out, int32(idx))
	}
	return out, nil
}
