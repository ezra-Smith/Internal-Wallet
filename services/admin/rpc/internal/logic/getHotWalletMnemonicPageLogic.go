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

func (l *GetHotWalletMnemonicPageLogic) GetHotWalletMnemonicPage(in *pb.AdminGetHotWalletMnemonicPageRequest) (*pb.AdminGetHotWalletMnemonicPageResponse, error) {
	if current, ok := admininterceptor.GetCurrentAdmin(l.ctx); !ok || current == nil {
		return &pb.AdminGetHotWalletMnemonicPageResponse{Success: false, Message: "unauthorized", RequestId: resp.RequestID(l.ctx), Timestamp: resp.Timestamp()}, nil
	}
	if in == nil || strings.TrimSpace(in.InitSessionId) == "" || in.PageIndex < 0 {
		return &pb.AdminGetHotWalletMnemonicPageResponse{Success: false, Message: "invalid params", RequestId: resp.RequestID(l.ctx), Timestamp: resp.Timestamp()}, nil
	}
	if l.svcCtx == nil || l.svcCtx.SignerRpc == nil {
		return &pb.AdminGetHotWalletMnemonicPageResponse{Success: false, Message: "signer service not available", RequestId: resp.RequestID(l.ctx), Timestamp: resp.Timestamp()}, nil
	}

	signerResp, err := l.svcCtx.SignerRpc.GetHotWalletMnemonicPage(l.ctx, &pb.GetHotWalletMnemonicPageRequest{
		InitSessionId: strings.TrimSpace(in.InitSessionId),
		PageIndex:     in.PageIndex,
	})
	if err != nil || signerResp == nil {
		return &pb.AdminGetHotWalletMnemonicPageResponse{Success: false, Message: "failed to fetch page", RequestId: resp.RequestID(l.ctx), Timestamp: resp.Timestamp()}, nil
	}

	ok2 := signerResp.Code == 0
	return &pb.AdminGetHotWalletMnemonicPageResponse{
		Success: ok2,
		Message: signerResp.Message,
		Data: &pb.AdminGetHotWalletMnemonicPageData{
			PageIndex:  signerResp.PageIndex,
			PageSize:   signerResp.PageSize,
			TotalPages: signerResp.TotalPages,
			Words:      signerResp.Words,
			HasPrev:    signerResp.HasPrev,
			HasNext:    signerResp.HasNext,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
