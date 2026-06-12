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

type GetWeb3UserByDeviceIdLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetWeb3UserByDeviceIdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetWeb3UserByDeviceIdLogic {
	return &GetWeb3UserByDeviceIdLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 根据 device_id 获取 Web3 用户信息
func (l *GetWeb3UserByDeviceIdLogic) GetWeb3UserByDeviceId(in *pb.GetWeb3UserByDeviceIdReq) (*pb.GetWeb3UserByDeviceIdResp, error) {
	deviceID := strings.TrimSpace(in.GetDeviceId())
	if deviceID == "" {
		return nil, errx.Web3DeviceIDRequired()
	}
	if l.svcCtx.Web3UserRepository == nil {
		return nil, errx.Internal("web3 user repository not ready")
	}

	user, err := l.svcCtx.Web3UserRepository.FindByDeviceID(l.ctx, deviceID)
	if err != nil {
		l.Errorf("failed to find web3 user by device_id=%s: %v", deviceID, err)
		return nil, errx.Web3UserNotFound()
	}

	// 更新最后活跃时间
	if updateErr := l.svcCtx.Web3UserRepository.UpdateLastActiveAt(l.ctx, deviceID); updateErr != nil {
		l.Errorf("更新用户活跃时间失败: device_id=%s, error=%v", deviceID, updateErr)
		// 不阻断流程，继续返回用户信息
	}

	resp := &pb.GetWeb3UserByDeviceIdResp{
		Success: true,
		Message: "ok",
		User:    mapWeb3UserToPB(user),
	}
	return resp, nil
}

func mapWeb3UserToPB(m *model.Web3UserModel) *pb.Web3UserInfo {
	if m == nil {
		return nil
	}
	return &pb.Web3UserInfo{
		UserId:             m.ID,
		DeviceId:           m.DeviceID,
		Platform:           m.Platform,
		OsVersion:          m.OsVersion,
		AppVersion:         m.AppVersion,
		DeviceModel:        m.DeviceModel,
		DeviceName:         m.DeviceName,
		PushToken:          valueOrEmpty(m.PushToken),
		Locale:             valueOrEmpty(m.Locale),
		TwoFactorEnabled:   m.TwoFactorEnabled,
		BiometricEnabled:   m.BiometricEnabled,
		HasTradePassword:   m.HasTradePassword,
		LastSecurityUpdate: formatTimeRFC3339(m.LastSecurityUpdate),
		LastActiveAt:       formatTimeRFC3339(m.LastActiveAt),
		CreatedAt:          formatTimeRFC3339(m.CreatedAt),
		UpdatedAt:          formatTimeRFC3339(m.UpdatedAt),
	}
}

func valueOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func formatTimeRFC3339(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}
