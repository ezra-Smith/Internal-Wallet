package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListWithdrawAuditWhitelistBindableNetworksLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListWithdrawAuditWhitelistBindableNetworksLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListWithdrawAuditWhitelistBindableNetworksLogic {
	return &ListWithdrawAuditWhitelistBindableNetworksLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListWithdrawAuditWhitelistBindableNetworksLogic) ListWithdrawAuditWhitelistBindableNetworks(in *pb.ListWithdrawAuditWhitelistBindableNetworksReq) (*pb.ListWithdrawAuditWhitelistBindableNetworksResp, error) {
	// In binder flow, we only need chain options; return all supported withdraw chains.
	_ = in

	// ListActionNetworks 现在直接返回所有启用的网络
	netsRes, err := NewListActionNetworksLogic(l.ctx, l.svcCtx).ListActionNetworks(&pb.ListActionNetworksReq{
		Action: "withdraw",
	})
	if err != nil || netsRes == nil || !netsRes.Success {
		return &pb.ListWithdrawAuditWhitelistBindableNetworksResp{Success: true, Items: []*pb.ActionNetworkItem{}}, nil
	}

	return &pb.ListWithdrawAuditWhitelistBindableNetworksResp{Success: true, Items: netsRes.Items}, nil
}
