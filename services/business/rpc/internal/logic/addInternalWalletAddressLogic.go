package logic

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"internalwallet/common/middleware"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/model"
	"internalwallet/services/business/rpc/internal/repository"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type AddInternalWalletAddressLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAddInternalWalletAddressLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddInternalWalletAddressLogic {
	return &AddInternalWalletAddressLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AddInternalWalletAddressLogic) AddInternalWalletAddress(in *pb.AddInternalWalletAddressReq) (*pb.AddInternalWalletAddressResp, error) {
	if in == nil {
		return nil, errx.InvalidParam("invalid params")
	}

	lookupType := strings.TrimSpace(strings.ToLower(in.LookupType))
	lookupValue := strings.TrimSpace(in.LookupValue)
	label := strings.TrimSpace(in.Label)

	if lookupType == "" || lookupValue == "" {
		return nil, errx.InternalAddressLookupTypeRequired()
	}
	if label == "" {
		return nil, errx.InternalAddressLabelRequired()
	}
	if lookupType != "uid" && lookupType != "email" {
		return nil, errx.InternalAddressInvalidLookupType()
	}

	// 获取当前用户ID
	uidStr := middleware.GetUserID(l.ctx)
	if uidStr == "" {
		return nil, errx.Unauthorized("unauthorized")
	}
	currentUID, err := strconv.ParseInt(uidStr, 10, 64)
	if err != nil {
		return nil, errx.InvalidUser()
	}

	// 查找目标用户
	var targetUser *model.UserModel
	userRepo := l.svcCtx.UserAccountRepository
	if userRepo == nil {
		return nil, errx.ServiceNotAvailable("user service")
	}

	switch lookupType {
	case "uid":
		targetUser, err = userRepo.GetByUID(l.ctx, lookupValue)
	case "email":
		targetUser, err = userRepo.GetByEmail(l.ctx, strings.ToLower(lookupValue))
	}

	if err != nil {
		if errors.Is(err, repository.ErrProfileNotFound) || errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errx.InternalAddressTargetUserNotFound()
		}
		l.Logger.Errorf("lookup user failed: %v", err)
		return nil, errx.DBError()
	}
	if targetUser == nil {
		return nil, errx.InternalAddressTargetUserNotFound()
	}

	// 不能添加自己
	if targetUser.ID == currentUID {
		return nil, errx.InternalAddressCannotAddSelf()
	}

	// 检查是否已存在
	addrRepo := l.svcCtx.MemberInternalAddressRepository
	if addrRepo == nil {
		return nil, errx.ServiceNotAvailable("internal address service")
	}

	existing, _ := addrRepo.FindByUserAndTarget(l.ctx, currentUID, targetUser.ID)
	if existing != nil {
		return nil, errx.InternalAddressAlreadyExists()
	}

	// 设置默认前先取消其他默认
	if in.SetDefault {
		_ = addrRepo.SetAllDefaultFalseByUser(l.ctx, currentUID)
	}

	// 创建内部地址
	addr := &model.MemberInternalAddressModel{
		UserID:            currentUID,
		TargetUserID:      targetUser.ID,
		TargetUserDisplay: in.LookupValue,
		Label:             label,
		Remark:            strings.TrimSpace(in.Remark),
		IsDefault:         in.SetDefault,
	}

	if err := addrRepo.Create(l.ctx, addr); err != nil {
		l.Logger.Errorf("Create internal address failed: %v", err)
		return nil, errx.DBError()
	}

	return &pb.AddInternalWalletAddressResp{
		Success: true,
		Message: "ok",
		Item: &pb.InternalWalletAddressItem{
			Id:                strconv.FormatInt(addr.ID, 10),
			TargetUserId:      strconv.FormatInt(targetUser.ID, 10),
			TargetUserDisplay: in.LookupValue,
			Label:             addr.Label,
			Remark:            addr.Remark,
			IsDefault:         addr.IsDefault,
			CreatedAt:         addr.CreatedAt.Unix(),
		},
	}, nil
}
