package logic

import (
	"context"
	"fmt"
	"time"

	"internalwallet/proto/pb"
	"internalwallet/services/notification/rpc/internal/model"
	"internalwallet/services/notification/rpc/internal/svc"

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

// RegisterDevice 注册设备（仅用于Web3用户）
// Web3用户使用 device_token 作为设备ID标识，registration_id 为极光推送ID
// 判断逻辑：
//  1. 先通过 registration_id 查询是否已注册 → 存在则更新设备信息
//  2. 如果 registration_id 不存在，再通过 device_token 查询 → 存在则更新 registration_id（极光ID变更场景）
//  3. 两者都不存在 → 创建新设备记录
func (l *RegisterDeviceLogic) RegisterDevice(in *pb.RegisterDeviceRequest) (*pb.RegisterDeviceResponse, error) {
	l.Infof("RegisterDevice request (Web3): registration_id=%s, device_token=%s, platform=%s",
		in.RegistrationId, in.DeviceToken, in.Platform)

	// 1. 参数验证（Web3用户必填项）
	if in.RegistrationId == "" {
		return nil, fmt.Errorf("registration_id is required")
	}
	if in.DeviceToken == "" {
		return nil, fmt.Errorf("device_token is required")
	}

	// 转换平台类型
	var platform string
	switch in.Platform {
	case pb.Platform_IOS:
		platform = "ios"
	case pb.Platform_ANDROID:
		platform = "android"
	default:
		return nil, fmt.Errorf("invalid platform")
	}

	// 2. 先通过 registration_id 查询是否已存在
	existingDeviceByRegID, err := l.svcCtx.DeviceRepo.FindByRegistrationID(l.ctx, in.RegistrationId)

	if err == nil && existingDeviceByRegID != nil {
		// 2a. registration_id 已存在，直接更新设备信息
		l.Infof("Device found by registration_id: device_id=%d, updating device info", existingDeviceByRegID.ID)

		existingDeviceByRegID.Platform = platform
		existingDeviceByRegID.DeviceModel = in.DeviceModel
		existingDeviceByRegID.OSVersion = in.OsVersion
		existingDeviceByRegID.AppVersion = in.AppVersion
		existingDeviceByRegID.DeviceToken = in.DeviceToken
		existingDeviceByRegID.IsActive = true
		existingDeviceByRegID.LastActiveAt = time.Now()

		if err := l.svcCtx.DeviceRepo.Update(l.ctx, existingDeviceByRegID); err != nil {
			l.Errorf("Failed to update device: %v", err)
			return nil, err
		}

		l.Infof("Device updated successfully: device_id=%d", existingDeviceByRegID.ID)

		return &pb.RegisterDeviceResponse{
			DeviceId:    existingDeviceByRegID.ID,
			IsNewDevice: false,
			Message:     "Device updated successfully",
		}, nil
	}

	// 3. registration_id 不存在，检查 device_token 是否存在
	// 场景：同一设备的极光 registration_id 变更了
	existingDeviceByToken, err := l.svcCtx.DeviceRepo.FindByDeviceToken(l.ctx, in.DeviceToken)

	if err == nil && existingDeviceByToken != nil {
		// 3a. device_token 存在但 registration_id 不同，说明同一设备的极光ID变更了
		// 更新旧记录的 registration_id 和其他信息
		l.Infof("Device found by device_token, registration_id changed: device_id=%d, old_reg_id=%s, new_reg_id=%s, updating...",
			existingDeviceByToken.ID, existingDeviceByToken.RegistrationID, in.RegistrationId)

		existingDeviceByToken.RegistrationID = in.RegistrationId // 更新为新的极光ID
		existingDeviceByToken.Platform = platform
		existingDeviceByToken.DeviceModel = in.DeviceModel
		existingDeviceByToken.OSVersion = in.OsVersion
		existingDeviceByToken.AppVersion = in.AppVersion
		existingDeviceByToken.IsActive = true
		existingDeviceByToken.LastActiveAt = time.Now()

		if err := l.svcCtx.DeviceRepo.Update(l.ctx, existingDeviceByToken); err != nil {
			l.Errorf("Failed to update device: %v", err)
			return nil, err
		}

		l.Infof("Device registration_id updated successfully: device_id=%d, new_reg_id=%s",
			existingDeviceByToken.ID, in.RegistrationId)

		return &pb.RegisterDeviceResponse{
			DeviceId:    existingDeviceByToken.ID,
			IsNewDevice: false,
			Message:     "Device registration_id updated successfully",
		}, nil
	}

	// 4. 两者都不存在，创建新设备记录
	l.Infof("Device not found, creating new record: registration_id=%s, device_token=%s", in.RegistrationId, in.DeviceToken)

	newDevice := &model.NotificationDevice{
		UserID:         0, // Web3用户无 user_id
		RegistrationID: in.RegistrationId,
		Platform:       platform,
		DeviceModel:    in.DeviceModel,
		OSVersion:      in.OsVersion,
		AppVersion:     in.AppVersion,
		DeviceToken:    in.DeviceToken,
		IsActive:       true,
		LastActiveAt:   time.Now(),
	}

	if err := l.svcCtx.DeviceRepo.Create(l.ctx, newDevice); err != nil {
		l.Errorf("Failed to create device: %v", err)
		return nil, err
	}

	l.Infof("New device registered successfully: device_id=%d, registration_id=%s, device_token=%s",
		newDevice.ID, in.RegistrationId, in.DeviceToken)

	return &pb.RegisterDeviceResponse{
		DeviceId:    newDevice.ID,
		IsNewDevice: true,
		Message:     "Device registered successfully",
	}, nil
}
