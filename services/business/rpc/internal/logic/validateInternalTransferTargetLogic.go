package logic

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"internalwallet/common/utils"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/model"
	"internalwallet/services/business/rpc/internal/repository"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type ValidateInternalTransferTargetLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewValidateInternalTransferTargetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ValidateInternalTransferTargetLogic {
	return &ValidateInternalTransferTargetLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ValidateInternalTransferTarget 校验内部转账目标用户是否存在（支持 UID 和邮箱）
func (l *ValidateInternalTransferTargetLogic) ValidateInternalTransferTarget(in *pb.ValidateInternalTransferTargetReq) (*pb.ValidateInternalTransferTargetResp, error) {
	if in == nil || in.ToUserId == "" {
		return &pb.ValidateInternalTransferTargetResp{
			Success: true,
			Message: "ok",
			Exists:  false,
		}, nil
	}

	v := strings.TrimSpace(in.ToUserId)
	if v == "" {
		return &pb.ValidateInternalTransferTargetResp{
			Success: true,
			Message: "ok",
			Exists:  false,
		}, nil
	}

	userRepo := l.svcCtx.UserAccountRepository
	if userRepo == nil {
		return nil, errx.ServiceNotAvailable("user service")
	}

	var user *model.UserModel
	var err error
	var displayName string

	// 1. 尝试作为 UID（纯数字）
	if isDigitsOnly(v) {
		userID, parseErr := strconv.ParseInt(v, 10, 64)
		if parseErr != nil || userID <= 0 {
			return &pb.ValidateInternalTransferTargetResp{
				Success: true,
				Message: "ok",
				Exists:  false,
			}, nil
		}
		user, err = userRepo.GetByID(l.ctx, userID)
		displayName = v
	} else if strings.Contains(v, "@") {
		// 2. 尝试作为 Email（包含 @）
		email := strings.ToLower(strings.TrimSpace(v))
		if !utils.ValidateEmail(email) {
			return &pb.ValidateInternalTransferTargetResp{
				Success: true,
				Message: "ok",
				Exists:  false,
			}, nil
		}
		user, err = userRepo.GetByEmail(l.ctx, email)
		displayName = email
	} else {
		// 格式不支持
		return &pb.ValidateInternalTransferTargetResp{
			Success: true,
			Message: "ok",
			Exists:  false,
		}, nil
	}

	// 检查用户是否存在
	if err != nil {
		if errors.Is(err, repository.ErrProfileNotFound) || errors.Is(err, gorm.ErrRecordNotFound) {
			return &pb.ValidateInternalTransferTargetResp{
				Success:     true,
				Message:     "ok",
				Exists:      false,
				DisplayName: displayName,
			}, nil
		}
		l.Logger.Errorf("ValidateInternalTransferTarget failed: %v", err)
		return nil, errx.DBError()
	}

	if user == nil || user.ID == 0 {
		return &pb.ValidateInternalTransferTargetResp{
			Success:     true,
			Message:     "ok",
			Exists:      false,
			DisplayName: displayName,
		}, nil
	}

	// 用户存在，返回 UID 和邮箱
	displayNameFinal := user.Email
	if displayNameFinal == "" {
		displayNameFinal = strconv.FormatInt(user.ID, 10)
	}

	return &pb.ValidateInternalTransferTargetResp{
		Success:     true,
		Message:     "ok",
		Exists:      true,
		UserId:      strconv.FormatInt(user.ID, 10),
		DisplayName: displayNameFinal,
		Email:       user.Email,
	}, nil
}

// isDigitsOnly 检查字符串是否只包含数字
func isDigitsOnly(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
