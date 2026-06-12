package logic

import (
	"context"
	"strconv"

	"internalwallet/common/middleware"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/model"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type AddWalletAddressLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAddWalletAddressLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddWalletAddressLogic {
	return &AddWalletAddressLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AddWalletAddressLogic) AddWalletAddress(in *pb.AddWalletAddressReq) (*pb.AddWalletAddressResp, error) {
	if in == nil || in.Chain == "" || in.Address == "" {
		return nil, errx.InvalidParam("invalid params")
	}

	uidStr := middleware.GetUserID(l.ctx)
	if uidStr == "" {
		return nil, errx.Unauthorized("unauthorized")
	}
	uid, err := strconv.ParseInt(uidStr, 10, 64)
	if err != nil {
		return nil, errx.InvalidUser()
	}

	// Block adding withdraw address if it hits on-chain sensitive address blacklist.
	if res, err := CheckSensitiveAddressBlacklist(l.ctx, l.svcCtx, in.Chain, in.Address); err != nil {
		l.Logger.Errorf("check sensitive address blacklist failed: %v", err)
		return nil, errx.RiskCheckFailed("risk check failed")
	} else if res != nil && res.IsBlacklisted {
		return nil, errx.RiskCheckFailed(res.Message)
	}

	repo := l.svcCtx.UserWalletAddressRepository

	// 检查地址是否已存在（同一用户、链、地址的组合）
	existingAddrs, err := repo.ListAddresses(l.ctx, uid, "", in.Chain)
	if err != nil {
		l.Logger.Errorf("ListAddresses failed: %v", err)
		return nil, errx.DBError()
	}
	for _, addr := range existingAddrs {
		if addr.Address == in.Address {
			l.Infof("Address already exists: user_id=%d, chain=%s, address=%s", uid, in.Chain, in.Address)
			return nil, errx.AddressAlreadyExists()
		}
	}

	if in.SetDefault {
		_ = repo.SetAllDefaultFalseByUser(l.ctx, uid)
	}

	// Asset 字段不再使用，存空字符串（链上地址不绑定特定资产）
	wa := &model.UserWalletAddressModel{
		UserId:    uid,
		Asset:     "",
		Chain:     in.Chain,
		Address:   in.Address,
		Label:     in.Label,
		MemoTag:   in.MemoTag,
		IsDefault: in.SetDefault,
	}
	if err := repo.CreateAddress(l.ctx, wa); err != nil {
		l.Logger.Errorf("CreateAddress failed: %v", err)
		return nil, errx.DBError()
	}
	return &pb.AddWalletAddressResp{Success: true, Message: "ok"}, nil
}
