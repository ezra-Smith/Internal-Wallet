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

type UpdateInternalWalletAddressLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateInternalWalletAddressLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateInternalWalletAddressLogic {
	return &UpdateInternalWalletAddressLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateInternalWalletAddressLogic) UpdateInternalWalletAddress(in *pb.UpdateInternalWalletAddressReq) (*pb.UpdateInternalWalletAddressResp, error) {
	if in == nil || in.Id == "" {
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

	id, err := strconv.ParseInt(in.Id, 10, 64)
	if err != nil {
		return nil, errx.InvalidParam("invalid id")
	}

	repo := l.svcCtx.MemberInternalAddressRepository
	if repo == nil {
		return nil, errx.ServiceNotAvailable("internal address service")
	}

	row, err := repo.FindByID(l.ctx, id)
	if err != nil || row == nil {
		return nil, errx.AddressNotFound()
	}

	// 验证所有权
	if row.UserID != uid {
		return nil, errx.Forbidden("forbidden")
	}

	// 更新目标用户（账户UID）- 必填参数验证
	lookupType := strings.TrimSpace(strings.ToLower(in.LookupType))
	lookupValue := strings.TrimSpace(in.LookupValue)

	// 验证必填参数：lookup_type 和 lookup_value 必须同时提供
	if lookupType == "" || lookupValue == "" {
		return nil, errx.InvalidParam("lookup_type and lookup_value are required")
	}

	// 验证查找类型
	if lookupType != "uid" && lookupType != "email" {
		return nil, errx.InternalAddressInvalidLookupType()
	}

	// 查找目标用户
	userRepo := l.svcCtx.UserAccountRepository
	if userRepo == nil {
		return nil, errx.ServiceNotAvailable("user service")
	}

	var targetUser *model.UserModel
	switch lookupType {
	case "uid":
		targetUser, err = userRepo.GetByUID(l.ctx, lookupValue)
	case "email":
		targetUser, err = userRepo.GetByEmail(l.ctx, strings.ToLower(lookupValue))
	default:
		return nil, errx.InternalAddressInvalidLookupType()
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

	// 不能修改为自己
	if targetUser.ID == uid {
		return nil, errx.InternalAddressCannotAddSelf()
	}

	// 检查是否与其他地址重复（排除当前地址）
	existing, _ := repo.FindByUserAndTarget(l.ctx, uid, targetUser.ID)
	if existing != nil && existing.ID != id {
		return nil, errx.InternalAddressAlreadyExists()
	}

	// 构建更新字段
	fields := map[string]interface{}{}

	// 更新标签（地址名称）
	if strings.TrimSpace(in.Label) != "" {
		fields["label"] = strings.TrimSpace(in.Label)
	}

	// 更新备注
	if in.Remark != "" {
		fields["remark"] = strings.TrimSpace(in.Remark)
	}

	// 更新目标用户相关字段（必填参数，始终更新）
	// 直接存储原始 lookup_value，不进行脱敏处理
	fields["target_user_id"] = targetUser.ID
	fields["target_user_display"] = lookupValue

	// 设置默认
	if in.SetDefault {
		_ = repo.SetAllDefaultFalseByUser(l.ctx, uid)
		fields["is_default"] = true
	}

	if err := repo.UpdateFields(l.ctx, id, fields); err != nil {
		l.Logger.Errorf("UpdateFields failed: %v", err)
		return nil, errx.DBError()
	}

	return &pb.UpdateInternalWalletAddressResp{Success: true, Message: "ok"}, nil
}
