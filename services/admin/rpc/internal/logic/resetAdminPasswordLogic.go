package logic

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"internalwallet/pkg/notify"
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
	"internalwallet/common/middleware"
)

type ResetAdminPasswordLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewResetAdminPasswordLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResetAdminPasswordLogic {
	return &ResetAdminPasswordLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ResetAdminPasswordLogic) ResetAdminPassword(in *pb.ResetAdminPasswordRequest) (*pb.ResetAdminPasswordResponse, error) {
	if in == nil || strings.TrimSpace(in.AdminId) == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid params", map[string]string{"admin_id": "required"})
	}
	if l.svcCtx.DB == nil || l.svcCtx.AdminUserRepo == nil || l.svcCtx.PasswordHistoryRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	current, ok := admininterceptor.GetCurrentAdmin(l.ctx)
	if !ok || current == nil {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}

	targetID, parseErr := resp.ParseAdminID(in.AdminId)
	if parseErr != nil || targetID <= 0 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid admin_id", map[string]string{"admin_id": "invalid"})
	}
	target, err := l.svcCtx.AdminUserRepo.FindByID(l.ctx, targetID)
	if err != nil || target == nil {
		return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "NOT_FOUND", "admin not found", nil)
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
	emailSent := false
	tempPasswordResp := tempPass

	ip := middleware.GetClientIP(l.ctx)
	ua := middleware.GetUserAgent(l.ctx)
	detailsBytes, _ := json.Marshal(map[string]interface{}{
		"send_email":      in.SendEmail,
		"require_change":  in.RequireChange,
		"target_username": target.Username,
	})

	if err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		adminRepo := repository.NewAdminUserRepository(tx)
		historyRepo := repository.NewAdminPasswordHistoryRepository(tx)
		auditRepo := repository.NewAdminAuditLogRepository(tx)

		if err := adminRepo.UpdatePassword(l.ctx, targetID, hash, in.RequireChange, current.ID); err != nil {
			return err
		}
		if err := historyRepo.Add(l.ctx, targetID, hash); err != nil {
			return err
		}
		if _, err := adminRepo.IncrementTokenVersion(l.ctx, targetID, current.ID); err != nil {
			return err
		}
		_ = auditRepo.CreateLog(l.ctx, &model.AdminAuditLogModel{
			AdminID:     current.ID,
			Action:      "admin.reset_password",
			TargetType:  "admin",
			TargetID:    resp.AdminIDString(targetID),
			Description: "重置管理员密码: " + target.Username,
			Details:     detailsBytes,
			IP:          ip,
			UserAgent:   ua,
		})
		return nil
	}); err != nil {
		l.Logger.Errorf("reset password failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	// 使所有会话失效
	_ = l.svcCtx.SessionManager.RemoveAll(l.ctx, targetID)

	// 可选发送邮件：失败也不影响主流程，避免幂等键占用导致无法重试
	if in.SendEmail {
		if _, err := notify.SendAdminPasswordResetEmail(target.Username, "Zink Wallet", tempPass, in.RequireChange); err != nil {
			l.Logger.Errorf("send reset password email failed: %v", err)
		} else {
			emailSent = true
			tempPasswordResp = ""
		}
	}

	msg := "密码已重置"
	if emailSent {
		msg = "密码已重置，新密码已发送至邮箱"
	} else if in.SendEmail {
		msg = "密码已重置（邮件发送失败）"
	}
	return &pb.ResetAdminPasswordResponse{
		Success: true,
		Message: msg,
		Data: &pb.ResetAdminPasswordData{
			AdminId:         resp.AdminIDString(targetID),
			PasswordResetAt: formatTime(now),
			EmailSent:       emailSent,
			TempPassword:    tempPasswordResp,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
