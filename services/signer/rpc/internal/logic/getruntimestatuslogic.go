package logic

import (
	"context"

	"internalwallet/common/errcode"
	"internalwallet/proto/pb"
	"internalwallet/services/signer/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetRuntimeStatusLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetRuntimeStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetRuntimeStatusLogic {
	return &GetRuntimeStatusLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetRuntimeStatusLogic) GetRuntimeStatus(in *pb.GetRuntimeStatusRequest) (*pb.GetRuntimeStatusResponse, error) {
	seedID := defaultHotWalletSeedID
	if in != nil {
		seedID = normalizeSeedID(in.SeedId)
	}

	// Guard: service context / DB might not be ready yet (e.g. cold start).
	// Never panic; return a stable response so callers can show a meaningful status.
	if l.svcCtx == nil || l.svcCtx.WalletRuntime == nil {
		return &pb.GetRuntimeStatusResponse{
			Code:            int32(errcode.SignerInternalError),
			Message:         "signer runtime not ready",
			SeedId:          seedID,
			WalletState:     walletStateUnknown,
			BackupConfirmed: false,
			UnlockedAt:      "",
		}, nil
	}
	if l.svcCtx.WalletMasterMnemonicRepo == nil {
		return &pb.GetRuntimeStatusResponse{
			Code:            int32(errcode.SignerInternalError),
			Message:         "signer db not ready",
			SeedId:          seedID,
			WalletState:     walletStateUnknown,
			BackupConfirmed: false,
			UnlockedAt:      "",
		}, nil
	}

	snap, err := getHotWalletState(l.ctx, l.svcCtx, seedID)
	if err != nil {
		l.Logger.Errorf("GetRuntimeStatus failed: %v", err)
		return &pb.GetRuntimeStatusResponse{
			Code:    int32(errcode.SignerInternalError),
			Message: "internal error",
			SeedId:  seedID,
			// best-effort: still return something stable for UI
			WalletState:     walletStateUnknown,
			BackupConfirmed: false,
			UnlockedAt:      "",
		}, nil
	}

	return &pb.GetRuntimeStatusResponse{
		Code:            0,
		Message:         "success",
		SeedId:          snap.SeedID,
		WalletState:     snap.State,
		BackupConfirmed: snap.BackupConfirmed,
		UnlockedAt:      snap.UnlockedAtRFC3339,
	}, nil
}
