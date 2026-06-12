package logic

import (
	"context"

	"github.com/zeromicro/go-zero/core/logx"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/svc"
)

type GetAlertConfigWithStatsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetAlertConfigWithStatsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetAlertConfigWithStatsLogic {
	return &GetAlertConfigWithStatsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetAlertConfigWithStatsLogic) GetAlertConfigWithStats(in *pb.AdminGetAlertConfigWithStatsRequest) (*pb.AdminGetAlertConfigWithStatsResponse, error) {
	// 1. 参数验证
	if in.ConfigId <= 0 {
		return &pb.AdminGetAlertConfigWithStatsResponse{
			Success: false,
			Message: "config_id is required",
		}, nil
	}

	// 2. 从数据库查询配置
	config, err := l.svcCtx.AlertConfigRepo.FindByID(l.ctx, in.ConfigId)
	if err != nil {
		l.Logger.Errorf("Failed to find config %d: %v", in.ConfigId, err)
		return &pb.AdminGetAlertConfigWithStatsResponse{
			Success: false,
			Message: "Alert config not found",
		}, nil
	}

	// 3. 获取实时统计数据（从Redis）
	// 注意：这部分逻辑将来会移到business服务中
	// 目前先返回空统计数据，实际统计由business服务处理
	stats := &pb.AdminAlertConfigStats{
		CurrentWindowAmount:      "0",
		CurrentWindowTxCount:     0,
		ProgressPercentage:       "0",
		IsInCooldown:             false,
		CooldownRemainingSeconds: 0,
	}

	// 4. 返回结果
	return &pb.AdminGetAlertConfigWithStatsResponse{
		Success: true,
		Message: "Alert config with stats retrieved successfully",
		Config:  l.modelToProto(config),
		Stats:   stats,
	}, nil
}

// modelToProto 将模型转换为Proto消息
func (l *GetAlertConfigWithStatsLogic) modelToProto(config *model.AlertConfigModel) *pb.AdminAlertConfigItem {
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
