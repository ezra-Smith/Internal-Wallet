package logic

import (
	"context"
	"strconv"

	"internalwallet/common/middleware"
	"internalwallet/common/security"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ChangePasswordLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewChangePasswordLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChangePasswordLogic {
	return &ChangePasswordLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 修改密码（已登录用户）
func (l *ChangePasswordLogic) ChangePassword(in *pb.ChangePasswordReq) (*pb.ChangePasswordResp, error) {
	// 1. 参数验证
	if in == nil || in.OldPassword == "" || in.NewPassword == "" {
		return nil, errx.InvalidParam("invalid params")
	}

	if in.OldPassword == in.NewPassword {
		return nil, errx.InvalidParam("new password must be different from old password")
	}

	// 2. 验证新密码强度
	if err := security.ValidatePasswordStrength(in.NewPassword, 8, false); err != nil {
		return nil, errx.PasswordTooWeak("password too weak: " + err.Error())
	}

	// 3. 获取用户ID
	uid := middleware.GetUserID(l.ctx)
	if uid == "" {
		l.Logger.Error("No user ID in context")
		return nil, errx.Unauthorized("unauthorized")
	}

	userID, err := strconv.ParseInt(uid, 10, 64)
	if err != nil {
		l.Logger.Errorf("Invalid user ID format: %s", uid)
		return nil, errx.InvalidParam("invalid user ID")
	}

	// 4. 获取用户信息
	user, err := l.svcCtx.UserAccountRepository.GetAuthByID(l.ctx, userID)
	if err != nil {
		l.Logger.Errorf("User not found (user_id: %d): %v", userID, err)
		return nil, errx.UserNotFound()
	}

	// 5. 验证旧密码（使用 bcrypt）
	if user.PasswordHash == "" || !security.VerifyPassword(user.PasswordHash, in.OldPassword) {
		l.Logger.Infof("Invalid old password attempt for user %d", userID)
		return nil, errx.InvalidPassword()
	}

	// 6. 使用 bcrypt 生成新密码哈希
	newHash, err := security.HashPassword(in.NewPassword)
	if err != nil {
		l.Logger.Errorf("Failed to hash new password for user %d: %v", userID, err)
		return nil, errx.Internal("internal error")
	}

	// 7. 更新密码
	err = l.svcCtx.UserAccountRepository.UpdatePassword(l.ctx, user.ID, newHash)
	if err != nil {
		l.Logger.Errorf("Failed to update password for user %d: %v", userID, err)
		return nil, errx.DBError()
	}

	// 8. 使所有会话失效（强制重新登录）- 安全最佳实践
	// 密码修改后，用户需要使用新密码重新登录
	if l.svcCtx.UserSessionRepository != nil {
		err = l.svcCtx.UserSessionRepository.InvalidateUserSessions(l.ctx, userID)
		if err != nil {
			l.Logger.Errorf("Failed to invalidate sessions for user %d: %v", userID, err)
			// 不返回错误，密码已经更新成功
		} else {
			l.Logger.Infof("Invalidated all sessions for user %d after password change", userID)
		}
	}

	// 9. 记录审计日志
	l.Logger.Infof("Password changed successfully for user %d (email: %s)", userID, user.Email)

	// 10. （可选）发送通知邮件/短信
	// TODO: 通知用户密码已修改，如果不是本人操作请立即联系客服

	return &pb.ChangePasswordResp{
		Success: true,
		Message: "ok",
	}, nil
}
