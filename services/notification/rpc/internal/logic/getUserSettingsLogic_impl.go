package logic

// 实现文件，简化 getUserSettingsLogic.go

import (
	"context"
	"github.com/zeromicro/go-zero/core/logx"
	"internalwallet/proto/pb"
	"internalwallet/services/notification/rpc/internal/svc"
)

// GetUserSettingsImpl 获取用户推送设置
func GetUserSettingsImpl(ctx context.Context, svcCtx *svc.ServiceContext, logger logx.Logger, in *pb.GetUserSettingsRequest) (*pb.GetUserSettingsResponse, error) {
	// 查询或创建用户设置
	settings, err := svcCtx.SettingsRepo.FindOrCreateByUserID(ctx, in.UserId)
	if err != nil {
		logger.Errorf("Failed to get user settings: %v", err)
		return nil, err
	}

	// 转换为响应
	quietStart := ""
	quietEnd := ""
	if settings.QuietStartTime.Valid {
		quietStart = settings.QuietStartTime.Time.Format("15:04")
	}
	if settings.QuietEndTime.Valid {
		quietEnd = settings.QuietEndTime.Time.Format("15:04")
	}

	return &pb.GetUserSettingsResponse{
		Settings: &pb.UserSettings{
			UserId:            settings.UserID,
			EnableTransaction: settings.EnableTransaction,
			EnableSecurity:    settings.EnableSecurity,
			EnableSystem:      settings.EnableSystem,
			EnablePriceAlert:  settings.EnablePriceAlert,
			QuietStartTime:    quietStart,
			QuietEndTime:      quietEnd,
			Language:          settings.Language,
		},
	}, nil
}
