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
	"internalwallet/services/admin/rpc/internal/security"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"gorm.io/gorm"
)

type ResetUserPasswordLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewResetUserPasswordLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResetUserPasswordLogic {
	return &ResetUserPasswordLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ResetUserPasswordLogic) ResetUserPassword(in *pb.ResetUserPasswordRequest) (*pb.ResetUserPasswordResponse, error) {
	if in == nil || strings.TrimSpace(in.Uid) == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_UID", "uid required", map[string]string{"uid": "required"})
	}
	if strings.TrimSpace(in.Reason) == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_REASON", "reason required", map[string]string{"reason": "required"})
	}
	if l.svcCtx.DB == nil || l.svcCtx.UserRepo == nil || l.svcCtx.AdminUserRepo == nil {
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

	minLen := int(l.svcCtx.Config.Security.PasswordMinLength)
	if minLen <= 0 {
		minLen = 10
	}
	tempPass, genErr := security.GenerateTempPassword(minLen + 2)
	if genErr != nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "generate password failed", nil)
	}
	if err := security.ValidatePasswordPolicy(tempPass, minLen); err != nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "generate password failed", nil)
	}
	hash, hashErr := security.HashPassword(tempPass)
	if hashErr != nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "password hash failed", nil)
	}

	now := time.Now()

	ip := middleware.GetClientIP(l.ctx)
	ua := middleware.GetUserAgent(l.ctx)
	detailsBytes, _ := json.Marshal(map[string]interface{}{
		"reason": strings.TrimSpace(in.Reason),
	})

	if err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		auditRepo := repository.NewAdminAuditLogRepository(tx)

		// Update login password hash (users.password_hash).
		res := tx.WithContext(l.ctx).
			Table("users").
			Where("id = ?", uid).
			Updates(map[string]interface{}{
				"password_hash": hash,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}

		_ = auditRepo.CreateLog(l.ctx, &model.AdminAuditLogModel{
			AdminID:     current.ID,
			Action:      "user.password.reset",
			TargetType:  "user",
			TargetID:    uidStr,
			Description: "重置用户登录密码: " + uidStr,
			Details:     detailsBytes,
			IP:          ip,
			UserAgent:   ua,
		})
		return nil
	}); err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "USER_NOT_FOUND", "用户不存在", nil)
		}
		l.Logger.Errorf("reset user password failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	// Best-effort: invalidate all sessions (force relogin).
	if l.svcCtx.DB != nil {
		if err := l.svcCtx.DB.WithContext(l.ctx).
			Table("user_sessions").
			Where("user_id = ? AND is_active = ?", uid, true).
			Update("is_active", false).Error; err != nil {
			// ignore schema differences in some environments
			if _, ok := errx.MySQLTableNotFound(err); !ok {
				l.Logger.Errorf("invalidate user sessions failed: %v", err)
			}
		}
	}

	msg := "密码已重置"
	return &pb.ResetUserPasswordResponse{
		Success: true,
		Message: msg,
		Data: &pb.ResetUserPasswordData{
			Uid:          uidStr,
			TempPassword: tempPass,
			ResetAt:      formatTime(now),
			ResetBy:      current.Username,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
