package logic

import (
	"context"
	"time"

	"internalwallet/common/errcode"
	"internalwallet/common/utils"
	"internalwallet/proto/pb"
	"internalwallet/services/signer/rpc/internal/repository"
	"internalwallet/services/signer/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetHotWalletMnemonicPageLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetHotWalletMnemonicPageLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetHotWalletMnemonicPageLogic {
	return &GetHotWalletMnemonicPageLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetHotWalletMnemonicPageLogic) GetHotWalletMnemonicPage(in *pb.GetHotWalletMnemonicPageRequest) (*pb.GetHotWalletMnemonicPageResponse, error) {
	if in == nil || in.InitSessionId == "" {
		return &pb.GetHotWalletMnemonicPageResponse{Code: int32(errcode.SignerInvalidParams), Message: "invalid params"}, nil
	}
	if in.PageIndex < 0 {
		return &pb.GetHotWalletMnemonicPageResponse{Code: int32(errcode.SignerInvalidParams), Message: "invalid page_index"}, nil
	}

	var sess hotWalletInitSession
	if err := l.svcCtx.SignerCache.GetJSON(l.ctx, hotWalletInitSessionKeyPrefix+in.InitSessionId, &sess); err != nil {
		return &pb.GetHotWalletMnemonicPageResponse{Code: int32(errcode.SignerWalletInitSessionGone), Message: "init session not found"}, nil
	}
	if time.Now().After(sess.ExpiresAt) {
		_ = l.svcCtx.SignerCache.Delete(l.ctx, hotWalletInitSessionKeyPrefix+in.InitSessionId)
		return &pb.GetHotWalletMnemonicPageResponse{Code: int32(errcode.SignerWalletInitSessionGone), Message: "init session expired"}, nil
	}

	row, err := l.svcCtx.WalletMasterMnemonicRepo.FindBySeedID(l.ctx, sess.SeedID)
	if err != nil {
		if err == repository.ErrWalletMasterMnemonicNotFound {
			return &pb.GetHotWalletMnemonicPageResponse{Code: int32(errcode.SignerWalletUninitialized), Message: "wallet not initialized"}, nil
		}
		l.Logger.Errorf("GetHotWalletMnemonicPage: query failed: %v", err)
		return &pb.GetHotWalletMnemonicPageResponse{Code: int32(errcode.SignerDatabaseError), Message: "database error"}, nil
	}

	// 解密助记词（会话期内使用 unlock_password）
	plain, dErr := utils.DecryptSeed(row.MnemonicEncrypted, row.MnemonicSalt, sess.UnlockPassword)
	if dErr != nil {
		return &pb.GetHotWalletMnemonicPageResponse{Code: int32(errcode.SignerWalletUnlockFailed), Message: "decrypt mnemonic failed"}, nil
	}
	words := splitMnemonicWords(string(plain))
	if len(words) != hotWalletWordCount {
		return &pb.GetHotWalletMnemonicPageResponse{Code: int32(errcode.SignerInternalError), Message: "invalid mnemonic length"}, nil
	}

	totalPages := hotWalletWordCount / hotWalletPageSize
	if int(in.PageIndex) >= totalPages {
		return &pb.GetHotWalletMnemonicPageResponse{Code: int32(errcode.SignerInvalidParams), Message: "page_index out of range"}, nil
	}
	start := int(in.PageIndex) * hotWalletPageSize
	end := start + hotWalletPageSize
	pageWords := words[start:end]

	return &pb.GetHotWalletMnemonicPageResponse{
		Code:       0,
		Message:    "success",
		PageIndex:  in.PageIndex,
		PageSize:   hotWalletPageSize,
		TotalPages: int32(totalPages),
		Words:      pageWords,
		HasPrev:    in.PageIndex > 0,
		HasNext:    int(in.PageIndex) < totalPages-1,
	}, nil
}
