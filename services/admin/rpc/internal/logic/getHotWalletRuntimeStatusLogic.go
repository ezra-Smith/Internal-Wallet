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

type GetHotWalletRuntimeStatusLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetHotWalletRuntimeStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetHotWalletRuntimeStatusLogic {
	return &GetHotWalletRuntimeStatusLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetHotWalletRuntimeStatusLogic) GetHotWalletRuntimeStatus(in *pb.AdminGetHotWalletRuntimeStatusRequest) (*pb.AdminGetHotWalletRuntimeStatusResponse, error) {
	if current, ok := admininterceptor.GetCurrentAdmin(l.ctx); !ok || current == nil {
		return &pb.AdminGetHotWalletRuntimeStatusResponse{
			Success:   false,
			Message:   "unauthorized",
			RequestId: resp.RequestID(l.ctx),
			Timestamp: resp.Timestamp(),
		}, nil
	}

	seedID := ""
	if in != nil {
		seedID = strings.TrimSpace(in.SeedId)
	}

	if l.svcCtx == nil || l.svcCtx.SignerRpc == nil {
		return &pb.AdminGetHotWalletRuntimeStatusResponse{
			Success:   false,
			Message:   "signer service not available",
			RequestId: resp.RequestID(l.ctx),
			Timestamp: resp.Timestamp(),
		}, nil
	}

	st, err := l.svcCtx.SignerRpc.GetRuntimeStatus(l.ctx, &pb.GetRuntimeStatusRequest{SeedId: seedID})
	if err != nil || st == nil {
		return &pb.AdminGetHotWalletRuntimeStatusResponse{
			Success:   false,
			Message:   "failed to query signer status",
			RequestId: resp.RequestID(l.ctx),
			Timestamp: resp.Timestamp(),
		}, nil
	}

	ok := st.Code == 0
	msg := st.Message
	if msg == "" {
		if ok {
			msg = "ok"
		} else {
			msg = "error"
		}
	}

	return &pb.AdminGetHotWalletRuntimeStatusResponse{
		Success: ok,
		Message: msg,
		Data: &pb.AdminGetHotWalletRuntimeStatusData{
			SeedId:          st.SeedId,
			WalletState:     st.WalletState,
			BackupConfirmed: st.BackupConfirmed,
			UnlockedAt:      st.UnlockedAt,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
