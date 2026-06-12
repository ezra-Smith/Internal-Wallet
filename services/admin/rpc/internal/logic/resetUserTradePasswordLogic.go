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

type ResetUserTradePasswordLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewResetUserTradePasswordLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResetUserTradePasswordLogic {
	return &ResetUserTradePasswordLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ResetUserTradePasswordLogic) ResetUserTradePassword(in *pb.ResetUserTradePasswordRequest) (*pb.ResetUserTradePasswordResponse, error) {
	if in == nil || strings.TrimSpace(in.Uid) == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_UID", "uid required", map[string]string{"uid": "required"})
	}
	if strings.TrimSpace(in.Reason) == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_REASON", "reason required", map[string]string{"reason": "required"})
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

	if u, err := l.svcCtx.UserRepo.FindByID(l.ctx, uid); err != nil || u == nil {
		return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "USER_NOT_FOUND", "用户不存在", nil)
	}

	now := time.Now()
	ip := middleware.GetClientIP(l.ctx)
	ua := middleware.GetUserAgent(l.ctx)
	detailsBytes, _ := json.Marshal(map[string]interface{}{
		"reason": strings.TrimSpace(in.Reason),
	})

	if err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		auditRepo := repository.NewAdminAuditLogRepository(tx)

		// Best-effort: clear trade password hash + flags in users table (schema varies across environments).
		if _, err := tryExecIgnoreMySQLErr(tx, l.ctx, "UPDATE `users` SET `trade_password_hash` = '', `has_trade_password` = 0 WHERE `id` = ?", uid); err != nil {
			return err
		}

		// Best-effort: sync member_security_setting.has_trade_password (if table exists).
		if _, err := tryExecIgnoreMySQLErr(tx, l.ctx, "UPDATE `member_security_setting` SET `has_trade_password` = 0 WHERE `user_id` = ?", uid); err != nil {
			return err
		}

		_ = auditRepo.CreateLog(l.ctx, &model.AdminAuditLogModel{
			AdminID:     current.ID,
			Action:      "user.trade_password.reset",
			TargetType:  "user",
			TargetID:    uidStr,
			Description: "重置用户资金密码: " + uidStr,
			Details:     detailsBytes,
			IP:          ip,
			UserAgent:   ua,
		})
		return nil
	}); err != nil {
		l.Logger.Errorf("reset user trade password failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	return &pb.ResetUserTradePasswordResponse{
		Success: true,
		Message: resp.Msg(l.ctx, "TRADE_PASSWORD_RESET"),
		Data: &pb.ResetUserTradePasswordData{
			Uid:     uidStr,
			ResetAt: formatTime(now),
			ResetBy: current.Username,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
