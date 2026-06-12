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

type LookupInternalUserLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewLookupInternalUserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LookupInternalUserLogic {
	return &LookupInternalUserLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// LookupInternalUser 查找内部用户（用于添加前预览）
func (l *LookupInternalUserLogic) LookupInternalUser(in *pb.LookupInternalUserReq) (*pb.LookupInternalUserResp, error) {
	if in == nil {
		return nil, errx.InvalidParam("invalid params")
	}

	lookupType := strings.TrimSpace(strings.ToLower(in.LookupType))
	lookupValue := strings.TrimSpace(in.LookupValue)

	if lookupType == "" || lookupValue == "" {
		return &pb.LookupInternalUserResp{
			Success: true,
			Found:   false,
			Message: "lookup_type and lookup_value are required",
		}, nil
	}

	if lookupType != "uid" && lookupType != "email" {
		return &pb.LookupInternalUserResp{
			Success: true,
			Found:   false,
			Message: "lookup_type must be 'uid' or 'email'",
		}, nil
	}

	// 获取当前用户ID（不能查找自己）
	uidStr := middleware.GetUserID(l.ctx)
	currentUID, _ := strconv.ParseInt(uidStr, 10, 64)

	// 查找目标用户
	var targetUser *model.UserModel
	var err error
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
			return &pb.LookupInternalUserResp{
				Success: true,
				Found:   false,
				Message: "user not found",
			}, nil
		}
		l.Logger.Errorf("lookup user failed: %v", err)
		return nil, errx.DBError()
	}

	if targetUser == nil {
		return &pb.LookupInternalUserResp{
			Success: true,
			Found:   false,
			Message: "user not found",
		}, nil
	}

	// 不能添加自己
	if currentUID > 0 && targetUser.ID == currentUID {
		return &pb.LookupInternalUserResp{
			Success: true,
			Found:   false,
			Message: "cannot add yourself",
		}, nil
	}

	// 返回不做脱敏：前端如需脱敏请在展示层处理
	displayName := strconv.FormatInt(targetUser.ID, 10)
	if strings.TrimSpace(targetUser.Email) != "" {
		displayName = strings.ToLower(strings.TrimSpace(targetUser.Email))
	}

	return &pb.LookupInternalUserResp{
		Success:     true,
		Found:       true,
		UserId:      strconv.FormatInt(targetUser.ID, 10),
		DisplayName: displayName,
		Message:     "ok",
	}, nil
}
