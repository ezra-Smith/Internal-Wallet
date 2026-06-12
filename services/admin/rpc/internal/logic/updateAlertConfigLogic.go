package logic

import (
	"context"
	"errors"
	"fmt"

	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/logx"

	"internalwallet/proto/pb"
	admininterceptor "internalwallet/services/admin/rpc/internal/interceptor"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/svc"
)

type UpdateAlertConfigLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateAlertConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateAlertConfigLogic {
	return &UpdateAlertConfigLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateAlertConfigLogic) UpdateAlertConfig(in *pb.AdminUpdateAlertConfigRequest) (*pb.AdminUpdateAlertConfigResponse, error) {
	// 获取当前管理员ID
	current, ok := admininterceptor.GetCurrentAdmin(l.ctx)
	if !ok || current == nil {
		l.Logger.Errorf("Failed to get current admin from context")
		return &pb.AdminUpdateAlertConfigResponse{
			Success: false,
			Message: "获取管理员信息失败",
		}, nil
	}

	// 1. 参数验证
	if err := l.validateRequest(in); err != nil {
		l.Logger.Errorf("Invalid request: %v", err)
		return &pb.AdminUpdateAlertConfigResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	// 2. 检查配置是否存在
	existingConfig, err := l.svcCtx.AlertConfigRepo.FindByID(l.ctx, in.ConfigId)
	if err != nil {
		l.Logger.Errorf("Failed to find config %d: %v", in.ConfigId, err)
		return &pb.AdminUpdateAlertConfigResponse{
			Success: false,
			Message: "Alert config not found",
		}, nil
	}

	// 3. 检查名称是否与其他配置冲突
	if in.Name != existingConfig.Name {
		conflictConfig, err := l.svcCtx.AlertConfigRepo.FindByName(l.ctx, in.Name)
		if err == nil && conflictConfig != nil && conflictConfig.ID != in.ConfigId {
			return &pb.AdminUpdateAlertConfigResponse{
				Success: false,
				Message: fmt.Sprintf("Alert config with name '%s' already exists", in.Name),
			}, nil
		}
	}

	// 4. 转换阈值
	thresholdUSD, err := decimal.NewFromString(in.ThresholdUsd)
	if err != nil {
		return &pb.AdminUpdateAlertConfigResponse{
			Success: false,
			Message: "Invalid threshold_usd format",
		}, nil
	}

	// 5. 更新配置
	existingConfig.Name = in.Name
	existingConfig.Description = in.Description
	existingConfig.ThresholdUSD = thresholdUSD
	existingConfig.TimeWindowSeconds = int(in.TimeWindowSeconds)
	existingConfig.MonitorWeb3Withdraw = in.MonitorWeb3Withdraw
	existingConfig.MonitorWeb2Withdraw = in.MonitorWeb2Withdraw
	existingConfig.MonitorInternalTransfer = in.MonitorInternalTransfer
	existingConfig.Enabled = in.Enabled
	existingConfig.TestMode = in.TestMode
	existingConfig.CooldownSeconds = int(in.CooldownSeconds)
	existingConfig.UpdatedBy = current.ID

	// 6. 保存更新
	if err := l.svcCtx.AlertConfigRepo.Update(l.ctx, existingConfig); err != nil {
		l.Logger.Errorf("Failed to update alert config: %v", err)
		return &pb.AdminUpdateAlertConfigResponse{
			Success: false,
			Message: "Failed to update alert config",
		}, err
	}

	l.Logger.Infof("Alert config updated successfully: id=%d, name=%s", in.ConfigId, in.Name)

	// 7. 返回结果
	return &pb.AdminUpdateAlertConfigResponse{
		Success: true,
		Message: "Alert config updated successfully",
		Config:  l.modelToProto(existingConfig),
	}, nil
}

// validateRequest 验证请求参数
func (l *UpdateAlertConfigLogic) validateRequest(in *pb.AdminUpdateAlertConfigRequest) error {
	if in.ConfigId <= 0 {
		return errors.New("config_id is required")
	}
	if in.Name == "" {
		return errors.New("name is required")
	}
	threshold, err := decimal.NewFromString(in.ThresholdUsd)
	if err != nil || threshold.LessThanOrEqual(decimal.Zero) {
		return errors.New("threshold_usd must be greater than 0")
	}
	if in.TimeWindowSeconds < 60 {
		return errors.New("time_window_seconds must be at least 60 seconds")
	}
	if in.CooldownSeconds < 0 {
		return errors.New("cooldown_seconds cannot be negative")
	}
	if !in.MonitorWeb3Withdraw && !in.MonitorWeb2Withdraw && !in.MonitorInternalTransfer {
		return errors.New("at least one monitor type must be enabled")
	}
	return nil
}

// modelToProto 将模型转换为Proto消息
func (l *UpdateAlertConfigLogic) modelToProto(config *model.AlertConfigModel) *pb.AdminAlertConfigItem {
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
