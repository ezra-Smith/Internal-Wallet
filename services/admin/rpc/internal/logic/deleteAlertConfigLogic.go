package logic

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/logx"

	"internalwallet/proto/pb"
	admininterceptor "internalwallet/services/admin/rpc/internal/interceptor"
	"internalwallet/services/admin/rpc/internal/svc"
)

type DeleteAlertConfigLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteAlertConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteAlertConfigLogic {
	return &DeleteAlertConfigLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *DeleteAlertConfigLogic) DeleteAlertConfig(in *pb.AdminDeleteAlertConfigRequest) (*pb.AdminDeleteAlertConfigResponse, error) {
	// 获取当前管理员ID（用于审计）
	current, ok := admininterceptor.GetCurrentAdmin(l.ctx)
	if !ok || current == nil {
		l.Logger.Errorf("Failed to get current admin from context")
		return &pb.AdminDeleteAlertConfigResponse{
			Success: false,
			Message: "获取管理员信息失败",
		}, nil
	}

	// 1. 参数验证
	if in.ConfigId <= 0 {
		return &pb.AdminDeleteAlertConfigResponse{
			Success: false,
			Message: "config_id is required",
		}, nil
	}

	// 2. 检查配置是否存在
	existingConfig, err := l.svcCtx.AlertConfigRepo.FindByID(l.ctx, in.ConfigId)
	if err != nil {
		l.Logger.Errorf("Failed to find config %d: %v", in.ConfigId, err)
		return &pb.AdminDeleteAlertConfigResponse{
			Success: false,
			Message: "Alert config not found",
		}, nil
	}

	// 3. 软删除配置
	if err := l.svcCtx.AlertConfigRepo.Delete(l.ctx, in.ConfigId); err != nil {
		l.Logger.Errorf("Failed to delete alert config: %v", err)
		return &pb.AdminDeleteAlertConfigResponse{
			Success: false,
			Message: "Failed to delete alert config",
		}, err
	}

	// 4. 清理Redis中的相关缓存数据
	if err := l.clearRedisData(in.ConfigId, existingConfig.TimeWindowSeconds); err != nil {
		l.Logger.Errorf("Failed to clear Redis data for config %d: %v", in.ConfigId, err)
		// 不返回错误，因为主要操作已完成
	}

	l.Logger.Infof("Alert config deleted successfully by admin %d: id=%d, name=%s", current.ID, in.ConfigId, existingConfig.Name)

	// 5. 返回结果
	return &pb.AdminDeleteAlertConfigResponse{
		Success: true,
		Message: "Alert config deleted successfully",
	}, nil
}

// clearRedisData 清理Redis中的缓存数据
func (l *DeleteAlertConfigLogic) clearRedisData(configID int64, timeWindowSeconds int) error {
	if l.svcCtx.RedisClient == nil {
		return errors.New("Redis client is not available")
	}

	// 计算当前时间窗口
	now := time.Now()
	windowStart := now.Truncate(time.Duration(timeWindowSeconds) * time.Second)

	// 删除累计金额数据
	amountKey := fmt.Sprintf("alert:amount:%d:%d", configID, windowStart.Unix())
	if err := l.svcCtx.RedisClient.Del(l.ctx, amountKey).Err(); err != nil {
		return err
	}

	// 删除交易详情数据
	detailKey := fmt.Sprintf("alert:details:%d:%d", configID, windowStart.Unix())
	if err := l.svcCtx.RedisClient.Del(l.ctx, detailKey).Err(); err != nil {
		return err
	}

	// 删除冷却标记
	cooldownKey := fmt.Sprintf("alert:cooldown:%d", configID)
	if err := l.svcCtx.RedisClient.Del(l.ctx, cooldownKey).Err(); err != nil {
		return err
	}

	l.Logger.Infof("Redis data cleared for config %d", configID)
	return nil
}
