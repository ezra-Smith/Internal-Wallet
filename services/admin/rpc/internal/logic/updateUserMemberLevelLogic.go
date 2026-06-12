package logic

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"internalwallet/common/middleware"
	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	admininterceptor "internalwallet/services/admin/rpc/internal/interceptor"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/repository"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"gorm.io/gorm"
)

type UpdateUserMemberLevelLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateUserMemberLevelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateUserMemberLevelLogic {
	return &UpdateUserMemberLevelLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateUserMemberLevelLogic) UpdateUserMemberLevel(in *pb.UpdateUserMemberLevelRequest) (*pb.UpdateUserMemberLevelResponse, error) {
	if in == nil || strings.TrimSpace(in.Uid) == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_UID", "uid required", map[string]string{"uid": "required"})
	}
	if in.MemberLevel < 1 || in.MemberLevel > 5 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_MEMBER_LEVEL", "invalid member_level", map[string]string{"member_level": "invalid"})
	}
	if l.svcCtx.DB == nil || l.svcCtx.UserRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	current, ok := admininterceptor.GetCurrentAdmin(l.ctx)
	if !ok || current == nil {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}
	if _, err := requireAdminReauth(l.ctx, l.svcCtx, in.Reauth); err != nil {
		return nil, err
	}

	uidStr := strings.TrimSpace(in.Uid)
	uid, parseErr := strconv.ParseInt(uidStr, 10, 64)
	if parseErr != nil || uid <= 0 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_UID", "invalid uid", map[string]string{"uid": "invalid"})
	}

	u, err := l.svcCtx.UserRepo.FindByID(l.ctx, uid)
	if err != nil || u == nil {
		return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "USER_NOT_FOUND", "用户不存在", nil)
	}

	oldLevel := u.MemberLevel
	newLevel := in.MemberLevel
	if oldLevel == newLevel {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "SAME_MEMBER_LEVEL", "same member_level", map[string]string{"member_level": "same"})
	}

	now := time.Now()
	ip := middleware.GetClientIP(l.ctx)
	ua := middleware.GetUserAgent(l.ctx)
	detailsBytes, _ := json.Marshal(map[string]interface{}{
		"old_level": oldLevel,
		"new_level": newLevel,
		"reason":    strings.TrimSpace(in.Reason),
	})

	if err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		userRepo := repository.NewUserRepository(tx)
		auditRepo := repository.NewAdminAuditLogRepository(tx)

		if err := userRepo.UpdateMemberLevel(l.ctx, uid, newLevel); err != nil {
			return err
		}
		_ = auditRepo.CreateLog(l.ctx, &model.AdminAuditLogModel{
			AdminID:     current.ID,
			Action:      "user.member_level.update",
			TargetType:  "user",
			TargetID:    uidStr,
			Description: "更新用户会员等级: " + uidStr,
			Details:     detailsBytes,
			IP:          ip,
			UserAgent:   ua,
		})
		return nil
	}); err != nil {
		l.Logger.Errorf("update user member_level failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	return &pb.UpdateUserMemberLevelResponse{
		Success: true,
		Message: resp.Msg(l.ctx, "MEMBER_LEVEL_UPDATED"),
		Data: &pb.UpdateUserMemberLevelData{
			Uid:       uidStr,
			OldLevel:  oldLevel,
			NewLevel:  newLevel,
			UpdatedAt: formatTime(now),
			UpdatedBy: current.Username,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
