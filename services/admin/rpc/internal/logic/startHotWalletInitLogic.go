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

type StartHotWalletInitLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewStartHotWalletInitLogic(ctx context.Context, svcCtx *svc.ServiceContext) *StartHotWalletInitLogic {
	return &StartHotWalletInitLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *StartHotWalletInitLogic) StartHotWalletInit(in *pb.AdminStartHotWalletInitRequest) (*pb.AdminStartHotWalletInitResponse, error) {
	current, ok := admininterceptor.GetCurrentAdmin(l.ctx)
	if !ok || current == nil {
		return &pb.AdminStartHotWalletInitResponse{
			Success:   false,
			Message:   "unauthorized",
			RequestId: resp.RequestID(l.ctx),
			Timestamp: resp.Timestamp(),
		}, nil
	}
	if in == nil || strings.TrimSpace(in.UnlockPassword) == "" {
		return &pb.AdminStartHotWalletInitResponse{
			Success:   false,
			Message:   "invalid params",
			RequestId: resp.RequestID(l.ctx),
			Timestamp: resp.Timestamp(),
		}, nil
	}
	if l.svcCtx == nil || l.svcCtx.SignerRpc == nil {
		return &pb.AdminStartHotWalletInitResponse{
			Success:   false,
			Message:   "signer service not available",
			RequestId: resp.RequestID(l.ctx),
			Timestamp: resp.Timestamp(),
		}, nil
	}

	seedID := strings.TrimSpace(in.SeedId)
	if seedID == "" {
		seedID = svc.DefaultHotWalletSeedID
	}

	signerResp, err := l.svcCtx.SignerRpc.StartHotWalletInit(l.ctx, &pb.StartHotWalletInitRequest{
		SeedId:   seedID,
		SeedName: svc.DefaultHotWalletSeedID,
		SupportedChains: []string{
			"ETH",
			"BSC",
			"TRON",
		},
		UnlockPassword:  in.UnlockPassword,
		AdminId:         current.ID,
	})
	if err != nil || signerResp == nil {
		return &pb.AdminStartHotWalletInitResponse{
			Success:   false,
			Message:   "failed to start init",
			RequestId: resp.RequestID(l.ctx),
			Timestamp: resp.Timestamp(),
		}, nil
	}

	ok2 := signerResp.Code == 0
	return &pb.AdminStartHotWalletInitResponse{
		Success: ok2,
		Message: signerResp.Message,
		Data: &pb.AdminStartHotWalletInitData{
			SeedId:        signerResp.SeedId,
			InitSessionId: signerResp.InitSessionId,
			WordCount:     signerResp.WordCount,
			PageSize:      signerResp.PageSize,
			TotalPages:    signerResp.TotalPages,
			ExpiresAt:     signerResp.ExpiresAt,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
