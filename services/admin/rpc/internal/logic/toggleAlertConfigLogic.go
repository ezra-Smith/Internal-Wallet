package logic

import (
	"context"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/logx"

	"internalwallet/proto/pb"
	admininterceptor "internalwallet/services/admin/rpc/internal/interceptor"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/svc"
)

type ToggleAlertConfigLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewToggleAlertConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ToggleAlertConfigLogic {
	return &ToggleAlertConfigLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ToggleAlertConfigLogic) ToggleAlertConfig(in *pb.AdminToggleAlertConfigRequest) (*pb.AdminToggleAlertConfigResponse, error) {
	// 获取当前管理员ID
	current, ok := admininterceptor.GetCurrentAdmin(l.ctx)
	if !ok || current == nil {
		l.Logger.Errorf("Failed to get current admin from context")
		return &pb.AdminToggleAlertConfigResponse{
			Success: false,
			Message: "获取管理员信息失败",
		}, nil
	}

	// 1. 参数验证
	if in.ConfigId <= 0 {
		return &pb.AdminToggleAlertConfigResponse{
			Success: false,
			Message: "config_id is required",
		}, nil
	}

	// 2. 检查配置是否存在
	existingConfig, err := l.svcCtx.AlertConfigRepo.FindByID(l.ctx, in.ConfigId)
	if err != nil {
		l.Logger.Errorf("Failed to find config %d: %v", in.ConfigId, err)
		return &pb.AdminToggleAlertConfigResponse{
			Success: false,
			Message: "Alert config not found",
		}, nil
	}

	// 3. 更新启用状态
	if err := l.svcCtx.AlertConfigRepo.UpdateEnabled(l.ctx, in.ConfigId, in.Enabled, current.ID); err != nil {
		l.Logger.Errorf("Failed to toggle alert config: %v", err)
		return &pb.AdminToggleAlertConfigResponse{
			Success: false,
			Message: "Failed to toggle alert config",
		}, err
	}

	// 4. 如果禁用配置，清理Redis中的相关数据
	if !in.Enabled {
		if err := l.clearRedisData(in.ConfigId, existingConfig.TimeWindowSeconds); err != nil {
			l.Logger.Errorf("Failed to clear Redis data for config %d: %v", in.ConfigId, err)
			// 不返回错误，因为主要操作已完成
		}
	}

	// 5. 重新获取更新后的配置
	updatedConfig, _ := l.svcCtx.AlertConfigRepo.FindByID(l.ctx, in.ConfigId)

	l.Logger.Infof("Alert config toggled successfully by admin %d: id=%d, enabled=%v", current.ID, in.ConfigId, in.Enabled)

	// 6. 返回结果
	return &pb.AdminToggleAlertConfigResponse{
		Success: true,
		Message: fmt.Sprintf("Alert config %s successfully", map[bool]string{true: "enabled", false: "disabled"}[in.Enabled]),
		Config:  l.modelToProto(updatedConfig),
	}, nil
}

// clearRedisData 清理Redis中的缓存数据
func (l *ToggleAlertConfigLogic) clearRedisData(configID int64, timeWindowSeconds int) error {
	if l.svcCtx.RedisClient == nil {
		return nil
	}

	// 计算当前时间窗口
	now := time.Now()
	windowStart := now.Truncate(time.Duration(timeWindowSeconds) * time.Second)

	// 删除累计金额数据
	amountKey := fmt.Sprintf("alert:amount:%d:%d", configID, windowStart.Unix())
	l.svcCtx.RedisClient.Del(l.ctx, amountKey)

	// 删除交易详情数据
	detailKey := fmt.Sprintf("alert:details:%d:%d", configID, windowStart.Unix())
	l.svcCtx.RedisClient.Del(l.ctx, detailKey)

	// 删除冷却标记
	cooldownKey := fmt.Sprintf("alert:cooldown:%d", configID)
	l.svcCtx.RedisClient.Del(l.ctx, cooldownKey)

	l.Logger.Infof("Redis data cleared for config %d", configID)
	return nil
}

// modelToProto 将模型转换为Proto消息
func (l *ToggleAlertConfigLogic) modelToProto(config *model.AlertConfigModel) *pb.AdminAlertConfigItem {
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
