package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetClientConfigLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetClientConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetClientConfigLogic {
	return &GetClientConfigLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 获取客户端配置（调用admin服务）
func (l *GetClientConfigLogic) GetClientConfig(in *pb.GetClientConfigReq) (*pb.GetClientConfigResp, error) {
	// 调用admin服务的GetClientConfig接口
	resp, err := l.svcCtx.AdminRpc.GetClientConfig(l.ctx, &pb.GetClientConfigRequest{
		Key: in.GetKey(),
	})
	if err != nil {
		return &pb.GetClientConfigResp{
			Success: false,
			Message: "Failed to get client config",
		}, err
	}

	return &pb.GetClientConfigResp{
		Success: resp.Success,
		Message: resp.Message,
		Data: &pb.BusinessClientConfigData{
			IsShow: resp.Data.IsShow,
		},
	}, nil
}
