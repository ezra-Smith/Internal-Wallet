package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetSwapConfigDetailLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetSwapConfigDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetSwapConfigDetailLogic {
	return &GetSwapConfigDetailLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetSwapConfigDetailLogic) GetSwapConfigDetail(in *pb.AdminGetSwapConfigDetailRequest) (*pb.AdminGetSwapConfigDetailResponse, error) {
	if l.svcCtx.SwapRpc == nil {
		l.Logger.Errorf("Swap service client is not available")
		return &pb.AdminGetSwapConfigDetailResponse{
			Success: false,
			Message: "Swap服务未配置或未启动",
		}, nil
	}

	resp, err := l.svcCtx.SwapRpc.GetSwapConfigDetail(l.ctx, &pb.SwapGetSwapConfigDetailRequest{
		Id: in.Id,
	})
	if err != nil {
		l.Logger.Errorf("Failed to call swap.GetSwapConfigDetail: %v", err)
		return &pb.AdminGetSwapConfigDetailResponse{
			Success: false,
			Message: "调用Swap服务失败",
		}, nil
	}

	return &pb.AdminGetSwapConfigDetailResponse{
		Success: resp.Success,
		Message: resp.Message,
		Config:  mapSwapConfigItem(resp.Config),
	}, nil
}
