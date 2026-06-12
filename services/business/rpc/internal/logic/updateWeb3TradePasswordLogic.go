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

type UpdateWeb3TradePasswordLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateWeb3TradePasswordLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateWeb3TradePasswordLogic {
	return &UpdateWeb3TradePasswordLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateWeb3TradePasswordLogic) UpdateWeb3TradePassword(in *pb.UpdateWeb3TradePasswordReq) (*pb.UpdateWeb3TradePasswordResp, error) {
	// 1. 参数验证
	if in == nil {
		return nil, errx.InvalidParam("invalid params")
	}
	deviceID := strings.TrimSpace(in.DeviceId)
	if deviceID == "" {
		return nil, errx.Web3DeviceIDRequired()
	}
	oldPassword := strings.TrimSpace(in.OldTradePassword)
	if oldPassword == "" {
		return nil, errx.OldTradePasswordRequired()
	}
	newPassword := strings.TrimSpace(in.NewTradePassword)
	if newPassword == "" {
		return nil, errx.NewTradePasswordRequired()
	}
	if len(newPassword) < 6 {
		return nil, errx.TradePasswordTooShort(6)
	}
	if oldPassword == newPassword {
		return nil, errx.TradePasswordSameAsOld()
	}

	// 2. 验证旧密码
	oldHash, hasPassword, err := l.svcCtx.Web3UserRepository.GetTradePasswordHash(l.ctx, deviceID)
	if err != nil {
		l.Logger.Errorf("Failed to get web3 trade password hash for device %s: %v", deviceID, err)
		return nil, errx.Internal("internal error")
	}

	// 如果没有设置过交易密码，不能使用修改接口
	if !hasPassword || oldHash == "" {
		return nil, errx.TradePasswordNotSet()
	}

	// 验证旧密码是否正确
	if !security.VerifyPassword(oldHash, oldPassword) {
		l.Logger.Infof("Invalid old trade password for device %s", deviceID)
		return nil, errx.InvalidOldTradePassword()
	}

	// 3. 哈希并更新新密码
	newHash, err := security.HashPassword(newPassword)
	if err != nil {
		l.Logger.Errorf("Failed to hash new web3 trade password for device %s: %v", deviceID, err)
		return nil, errx.Internal("internal error")
	}

	if err := l.svcCtx.Web3UserRepository.SetTradePassword(l.ctx, deviceID, newHash); err != nil {
		l.Logger.Errorf("Failed to update web3 trade password for device %s: %v", deviceID, err)
		return nil, errx.DBError()
	}

	l.Logger.Infof("Web3 trade password updated successfully for device %s", deviceID)
	return &pb.UpdateWeb3TradePasswordResp{Success: true, Message: "ok"}, nil
}
