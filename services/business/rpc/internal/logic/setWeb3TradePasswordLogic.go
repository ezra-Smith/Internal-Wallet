package logic

import (
	"context"
	"strings"

	"internalwallet/common/security"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type SetWeb3TradePasswordLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSetWeb3TradePasswordLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SetWeb3TradePasswordLogic {
	return &SetWeb3TradePasswordLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SetWeb3TradePasswordLogic) SetWeb3TradePassword(in *pb.SetWeb3TradePasswordReq) (*pb.SetWeb3TradePasswordResp, error) {
	// 1. 参数验证
	deviceID := strings.TrimSpace(in.DeviceId)
	if deviceID == "" {
		return nil, errx.Web3DeviceIDRequired()
	}

	tradePassword := strings.TrimSpace(in.TradePassword)
	if tradePassword == "" {
		return nil, errx.TradePasswordRequired()
	}
	if len(tradePassword) < 6 {
		return nil, errx.TradePasswordTooShort(6)
	}

	// 2. 检查 Web3UserRepository 是否可用
	if l.svcCtx.Web3UserRepository == nil {
		l.Logger.Error("Web3UserRepository is nil")
		return nil, errx.ServiceNotAvailable("database")
	}

	// 3. 验证 Web3 用户是否存在
	user, err := l.svcCtx.Web3UserRepository.FindByDeviceID(l.ctx, deviceID)
	if err != nil {
		l.Logger.Errorf("Failed to get Web3 user by device_id %s: %v", deviceID, err)
		return nil, errx.UserNotFound()
	}
	if user == nil {
		return nil, errx.UserNotFound()
	}

	// 4. 哈希交易密码
	hash, err := security.HashPassword(tradePassword)
	if err != nil {
		l.Logger.Errorf("Failed to hash trade password for device_id %s: %v", deviceID, err)
		return nil, errx.Internal("internal error")
	}

	// 5. 保存交易密码
	if err := l.svcCtx.Web3UserRepository.SetTradePassword(l.ctx, deviceID, hash); err != nil {
		l.Logger.Errorf("Failed to set trade password for device_id %s: %v", deviceID, err)
		return nil, errx.DBError()
	}

	l.Logger.Infof("Web3 trade password set successfully for device_id: %s", deviceID)
	return &pb.SetWeb3TradePasswordResp{
		Success: true,
		Message: "ok",
	}, nil
}
