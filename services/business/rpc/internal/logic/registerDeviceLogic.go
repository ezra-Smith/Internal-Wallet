package logic

import (
	"context"
	"strings"

	"internalwallet/common/middleware"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type RegisterDeviceLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRegisterDeviceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RegisterDeviceLogic {
	return &RegisterDeviceLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// RegisterDevice 注册设备（Web3用户专用）
// 说明：调用 Notification RPC 服务注册设备，平台信息从上下文中获取
func (l *RegisterDeviceLogic) RegisterDevice(in *pb.RegisterDeviceReq) (*pb.RegisterDeviceResp, error) {
	// 1. 从上下文获取平台信息
	platformStr := middleware.GetPlatform(l.ctx)
	appVersion := middleware.GetAppVersion(l.ctx)

	l.Infof("RegisterDevice request: registration_id=%s, device_token=%s, platform=%s (from context)",
		in.RegistrationId, in.DeviceToken, platformStr)

	// 2. 参数验证
	if in.RegistrationId == "" {
		return &pb.RegisterDeviceResp{
			Success: false,
			Message: "registration_id is required",
		}, nil
	}
	if in.DeviceToken == "" {
		return &pb.RegisterDeviceResp{
			Success: false,
			Message: "device_token is required",
		}, nil
	}
	if platformStr == "" {
		return &pb.RegisterDeviceResp{
			Success: false,
			Message: "platform is required (X-Platform header)",
		}, nil
	}

	// 3. 转换平台类型
	var platform pb.Platform
	switch strings.ToLower(platformStr) {
	case "ios":
		platform = pb.Platform_IOS
	case "android":
		platform = pb.Platform_ANDROID
	default:
		return &pb.RegisterDeviceResp{
			Success: false,
			Message: "invalid platform, must be 'ios' or 'android' (X-Platform header)",
		}, nil
	}

	// 4. 优先使用入参，如果入参为空则使用上下文中的值
	deviceModel := in.DeviceModel
	osVersion := in.OsVersion
	finalAppVersion := in.AppVersion
	if finalAppVersion == "" {
		finalAppVersion = appVersion // 使用上下文中的值
	}

	// 5. 调用 Notification RPC 服务注册设备
	resp, err := l.svcCtx.NotificationRpc.RegisterDevice(l.ctx, &pb.RegisterDeviceRequest{
		UserId:         0, // Web3用户无 user_id
		RegistrationId: in.RegistrationId,
		Platform:       platform,
		DeviceModel:    deviceModel,
		OsVersion:      osVersion,
		AppVersion:     finalAppVersion,
		DeviceToken:    in.DeviceToken,
	})

	if err != nil {
		l.Errorf("Failed to register device: %v", err)
		return &pb.RegisterDeviceResp{
			Success: false,
			Message: "Failed to register device: " + err.Error(),
		}, nil
	}

	l.Infof("Device registered successfully: device_id=%d, is_new=%v", resp.DeviceId, resp.IsNewDevice)

	return &pb.RegisterDeviceResp{
		Success:     true,
		Message:     resp.Message,
		DeviceId:    resp.DeviceId,
		IsNewDevice: resp.IsNewDevice,
	}, nil
}
