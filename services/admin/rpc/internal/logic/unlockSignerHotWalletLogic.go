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

type UnlockSignerHotWalletLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUnlockSignerHotWalletLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UnlockSignerHotWalletLogic {
	return &UnlockSignerHotWalletLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UnlockSignerHotWalletLogic) UnlockSignerHotWallet(in *pb.AdminUnlockSignerHotWalletRequest) (*pb.AdminUnlockSignerHotWalletResponse, error) {
	current, ok := admininterceptor.GetCurrentAdmin(l.ctx)
	if !ok || current == nil {
		return &pb.AdminUnlockSignerHotWalletResponse{Success: false, Message: "unauthorized", RequestId: resp.RequestID(l.ctx), Timestamp: resp.Timestamp()}, nil
	}
	if in == nil || strings.TrimSpace(in.UnlockPassword) == "" {
		return &pb.AdminUnlockSignerHotWalletResponse{Success: false, Message: "invalid params", RequestId: resp.RequestID(l.ctx), Timestamp: resp.Timestamp()}, nil
	}
	if l.svcCtx == nil || l.svcCtx.SignerRpc == nil {
		return &pb.AdminUnlockSignerHotWalletResponse{Success: false, Message: "signer service not available", RequestId: resp.RequestID(l.ctx), Timestamp: resp.Timestamp()}, nil
	}

	signerResp, err := l.svcCtx.SignerRpc.UnlockHotWallet(l.ctx, &pb.UnlockHotWalletRequest{
		SeedId:         strings.TrimSpace(in.SeedId),
		UnlockPassword: in.UnlockPassword,
		AdminId:        current.ID,
	})
	if err != nil || signerResp == nil {
		return &pb.AdminUnlockSignerHotWalletResponse{Success: false, Message: "failed to unlock", RequestId: resp.RequestID(l.ctx), Timestamp: resp.Timestamp()}, nil
	}

	ok2 := signerResp.Code == 0 && signerResp.Unlocked
	if ok2 {
		// Best-effort: refresh admin-rpc in-memory wallet state cache immediately
		// to avoid the 2s TTL window causing WALLET_INIT_REQUIRED right after unlock.
		if _, rErr := l.svcCtx.RefreshHotWalletStateCache(l.ctx); rErr != nil {
			l.svcCtx.ForceSetCachedHotWalletState("unlocked")
		}
	}
	return &pb.AdminUnlockSignerHotWalletResponse{
		Success: ok2,
		Message: signerResp.Message,
		Data: &pb.AdminUnlockSignerHotWalletData{
			SeedId:     signerResp.SeedId,
			Unlocked:   signerResp.Unlocked,
			UnlockedAt: signerResp.UnlockedAt,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
