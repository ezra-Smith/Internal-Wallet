package logic

import (
	"context"
	"strings"
	"time"

	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/model"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateWeb3UserLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateWeb3UserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateWeb3UserLogic {
	return &CreateWeb3UserLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateWeb3UserLogic) CreateWeb3User(in *pb.CreateWeb3UserReq) (*pb.CreateWeb3UserResp, error) {
	// 验证 device_id
	deviceID := strings.TrimSpace(in.DeviceId)
	if deviceID == "" {
		return nil, errx.Web3DeviceIDRequired()
	}

	// 检查用户是否已存在
	existingUser, err := l.svcCtx.Web3UserRepository.FindByDeviceID(l.ctx, deviceID)
	if err == nil && existingUser != nil {
		// 用户已存在，更新最后活跃时间
		if updateErr := l.svcCtx.Web3UserRepository.UpdateLastActiveAt(l.ctx, deviceID); updateErr != nil {
			l.Errorf("更新用户活跃时间失败: device_id=%s, error=%v", deviceID, updateErr)
			// 不阻断流程，继续返回成功
		}
		return &pb.CreateWeb3UserResp{
			Success:  true,
			Message:  "ok",
			DeviceId: existingUser.DeviceID,
			UserId:   existingUser.ID,
		}, nil
	}

	// 创建新的 Web3 用户
	now := time.Now()
	web3User := &model.Web3UserModel{
		DeviceID:     deviceID,
		Platform:     in.Platform,
		OsVersion:    in.OsVersion,
		AppVersion:   in.AppVersion,
		DeviceModel:  in.DeviceModel,
		DeviceName:   in.DeviceName,
		LastActiveAt: &now,
		CreatedAt:    &now,
		UpdatedAt:    &now,
	}

	if in.PushToken != "" {
		web3User.PushToken = &in.PushToken
	}
	if in.Locale != "" {
		web3User.Locale = &in.Locale
	}

	// 保存到数据库
	if err := l.svcCtx.Web3UserRepository.GetDB().WithContext(l.ctx).Create(web3User).Error; err != nil {
		l.Errorf("创建 Web3 用户失败: %v", err)
		return nil, errx.Web3UserCreateFailed()
	}

	l.Infof("成功创建 Web3 用户: device_id=%s, user_id=%d", deviceID, web3User.ID)

	return &pb.CreateWeb3UserResp{
		Success:  true,
		Message:  "ok",
		DeviceId: web3User.DeviceID,
		UserId:   web3User.ID,
	}, nil
}
