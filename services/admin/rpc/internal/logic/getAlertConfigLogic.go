package logic

import (
	"context"

	"github.com/zeromicro/go-zero/core/logx"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/svc"
)

type GetAlertConfigLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetAlertConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetAlertConfigLogic {
	return &GetAlertConfigLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetAlertConfigLogic) GetAlertConfig(in *pb.AdminGetAlertConfigRequest) (*pb.AdminGetAlertConfigResponse, error) {
	// 1. 参数验证
	if in.ConfigId <= 0 {
		return &pb.AdminGetAlertConfigResponse{
			Success: false,
			Message: "config_id is required",
		}, nil
	}

	// 2. 从数据库查询配置
	config, err := l.svcCtx.AlertConfigRepo.FindByID(l.ctx, in.ConfigId)
	if err != nil {
		l.Logger.Errorf("Failed to find config %d: %v", in.ConfigId, err)
		return &pb.AdminGetAlertConfigResponse{
			Success: false,
			Message: "Alert config not found",
		}, nil
	}

	// 3. 返回结果
	return &pb.AdminGetAlertConfigResponse{
		Success: true,
		Message: "Alert config retrieved successfully",
		Config:  l.modelToProto(config),
	}, nil
}

// modelToProto 将模型转换为Proto消息
func (l *GetAlertConfigLogic) modelToProto(config *model.AlertConfigModel) *pb.AdminAlertConfigItem {
	return &pb.AdminAlertConfigItem{
		Id:                      config.ID,
		Name:                    config.Name,
		Description:             config.Description,
		AlertType:               config.AlertType,
		ThresholdUsd:            config.ThresholdUSD.String(),
		TimeWindowSeconds:       int32(config.TimeWindowSeconds),
		MonitorWeb3Withdraw:     config.MonitorWeb3Withdraw,
		MonitorWeb2Withdraw:     config.MonitorWeb2Withdraw,
		MonitorInternalTransfer: config.MonitorInternalTransfer,
		Enabled:                 config.Enabled,
		TestMode:                config.TestMode,
		CooldownSeconds:         int32(config.CooldownSeconds),
		CreatedAt:               config.CreatedAt.Unix(),
		UpdatedAt:               config.UpdatedAt.Unix(),
	}
}
