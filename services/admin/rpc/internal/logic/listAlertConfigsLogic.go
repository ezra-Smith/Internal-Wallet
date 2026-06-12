package logic

import (
	"context"

	"github.com/zeromicro/go-zero/core/logx"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/svc"
)

type ListAlertConfigsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListAlertConfigsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListAlertConfigsLogic {
	return &ListAlertConfigsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListAlertConfigsLogic) ListAlertConfigs(in *pb.AdminListAlertConfigsRequest) (*pb.AdminListAlertConfigsResponse, error) {
	// 从数据库查询所有配置
	configs, err := l.svcCtx.AlertConfigRepo.FindAll(l.ctx)
	if err != nil {
		l.Logger.Errorf("Failed to list alert configs: %v", err)
		return &pb.AdminListAlertConfigsResponse{
			Success: false,
			Message: "Failed to list alert configs",
		}, err
	}

	// 转换为Proto格式
	var items []*pb.AdminAlertConfigItem
	for _, config := range configs {
		items = append(items, l.modelToProto(config))
	}

	return &pb.AdminListAlertConfigsResponse{
		Success: true,
		Message: "Alert configs retrieved successfully",
		Configs: items,
		Total:   int32(len(items)),
	}, nil
}

// modelToProto 将模型转换为Proto消息
func (l *ListAlertConfigsLogic) modelToProto(config *model.AlertConfigModel) *pb.AdminAlertConfigItem {
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
