package logic

import (
	"context"
	"encoding/json"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

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

// GetClientConfig 获取客户端配置
func (l *GetClientConfigLogic) GetClientConfig(in *pb.GetClientConfigRequest) (*pb.GetClientConfigResponse, error) {
	const category = "client_config"
	key := in.GetKey()
	if key == "" {
		key = "is_show" // 默认值
	}

	// 默认值
	data := &pb.ClientConfigData{
		IsShow: false, // 默认关闭
	}

	// 从数据库读取
	if l.svcCtx.SystemConfigRepo != nil {
		config, err := l.svcCtx.SystemConfigRepo.GetOne(l.ctx, category, key)
		if err == nil && config != nil && len(config.Value) > 0 {
			var value bool
			if err := json.Unmarshal(config.Value, &value); err == nil {
				data.IsShow = value
			}
		}
	}

	return &pb.GetClientConfigResponse{
		Success:   true,
		Message:   "ok",
		Data:      data,
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
