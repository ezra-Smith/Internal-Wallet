package logic

import (
	"context"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/chainsync/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type IsInternalAddressLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewIsInternalAddressLogic(ctx context.Context, svcCtx *svc.ServiceContext) *IsInternalAddressLogic {
	return &IsInternalAddressLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *IsInternalAddressLogic) IsInternalAddress(in *pb.IsInternalAddressReq) (*pb.IsInternalAddressResp, error) {
	if in == nil {
		return &pb.IsInternalAddressResp{
			Success:    false,
			IsInternal: false,
			Message:    "invalid request",
		}, nil
	}
	if in.Chain == pb.BlockChainType_CHAIN_TYPE_UNSPECIFIED || strings.TrimSpace(in.Address) == "" {
		return &pb.IsInternalAddressResp{
			Success:    false,
			IsInternal: false,
			Message:    "invalid params",
		}, nil
	}

	if l.svcCtx == nil || l.svcCtx.AddressMonitor == nil {
		return &pb.IsInternalAddressResp{
			Success:    true,
			IsInternal: false,
			Message:    "address monitor not available",
		}, nil
	}

	return &pb.IsInternalAddressResp{
		Success:    true,
		IsInternal: l.svcCtx.AddressMonitor.IsInternalAddress(in.Chain, in.Address),
		Message:    "ok",
	}, nil
}
