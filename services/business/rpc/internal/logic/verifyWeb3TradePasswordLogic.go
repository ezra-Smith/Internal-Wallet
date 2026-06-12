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

type VerifyWeb3TradePasswordLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewVerifyWeb3TradePasswordLogic(ctx context.Context, svcCtx *svc.ServiceContext) *VerifyWeb3TradePasswordLogic {
	return &VerifyWeb3TradePasswordLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *VerifyWeb3TradePasswordLogic) VerifyWeb3TradePassword(in *pb.VerifyWeb3TradePasswordReq) (*pb.VerifyWeb3TradePasswordResp, error) {
	// 1. 参数验证
	deviceID := strings.TrimSpace(in.DeviceId)
	if deviceID == "" {
		return nil, errx.Web3DeviceIDRequired()
	}

	tradePassword := strings.TrimSpace(in.TradePassword)
	if tradePassword == "" {
		return nil, errx.TradePasswordRequired()
	}

	// 2. 检查 Web3UserRepository 是否可用
	if l.svcCtx.Web3UserRepository == nil {
		l.Logger.Error("Web3UserRepository is nil")
		return nil, errx.ServiceNotAvailable("database")
	}

	// 3. 获取交易密码哈希
	hash, hasPassword, err := l.svcCtx.Web3UserRepository.GetTradePasswordHash(l.ctx, deviceID)
	if err != nil {
		l.Logger.Errorf("Failed to get trade password hash for device_id %s: %v", deviceID, err)
		return nil, errx.UserNotFound()
	}

	// 4. 检查是否已设置交易密码
	if !hasPassword || hash == "" {
		return &pb.VerifyWeb3TradePasswordResp{
			Success: true,
			Message: "trade password not set",
			IsValid: false,
		}, nil
	}

	// 5. 验证密码
	isValid := security.VerifyPassword(hash, tradePassword)

	if isValid {
		l.Logger.Infof("Web3 trade password verified successfully for device_id: %s", deviceID)
	} else {
		l.Logger.Infof("Web3 trade password verification failed for device_id: %s", deviceID)
	}

	return &pb.VerifyWeb3TradePasswordResp{
		Success: true,
		Message: "ok",
		IsValid: isValid,
	}, nil
}
