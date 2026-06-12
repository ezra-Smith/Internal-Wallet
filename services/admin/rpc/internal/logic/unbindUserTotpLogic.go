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

type UnbindUserTotpLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUnbindUserTotpLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UnbindUserTotpLogic {
	return &UnbindUserTotpLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UnbindUserTotpLogic) UnbindUserTotp(in *pb.UnbindUserTotpRequest) (*pb.UnbindUserTotpResponse, error) {
	if in == nil || strings.TrimSpace(in.Uid) == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_UID", "uid required", map[string]string{"uid": "required"})
	}
	if strings.TrimSpace(in.Reason) == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_REASON", "reason required", map[string]string{"reason": "required"})
	}
	if l.svcCtx.DB == nil || l.svcCtx.UserRepo == nil || l.svcCtx.User2FAHistoryRepo == nil {
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

	now := time.Now()
	ip := middleware.GetClientIP(l.ctx)
	ua := middleware.GetUserAgent(l.ctx)

	// Do not record secrets.
	detailsBytes, _ := json.Marshal(map[string]interface{}{
		"reason": strings.TrimSpace(in.Reason),
	})

	if err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		auditRepo := repository.NewAdminAuditLogRepository(tx)
		historyRepo := repository.NewUser2FAHistoryRepository(tx)

		// Best-effort: update business security settings table.
		if _, err := tryExecIgnoreMySQLErr(tx, l.ctx, "UPDATE `member_security_setting` SET `google_auth_enabled` = 0, `google_auth_bound` = 0 WHERE `user_id` = ?", uid); err != nil {
			return err
		}

		// Best-effort: clear 2FA flags/secrets in users table (schema varies across environments).
		// - new schema: google_auth_secret / is_2fa_enabled
		// - legacy schema: secret / backup_codes / enabled
		if _, err := tryExecIgnoreMySQLErr(tx, l.ctx, "UPDATE `users` SET `is_2fa_enabled` = 0 WHERE `id` = ?", uid); err != nil {
			return err
		}
		if _, err := tryExecIgnoreMySQLErr(tx, l.ctx, "UPDATE `users` SET `google_auth_secret` = '' WHERE `id` = ?", uid); err != nil {
			return err
		}
		if _, err := tryExecIgnoreMySQLErr(tx, l.ctx, "UPDATE `users` SET `secret` = '', `backup_codes` = '', `enabled` = 0 WHERE `id` = ?", uid); err != nil {
			return err
		}

		// Insert history record.
		if err := historyRepo.Create(l.ctx, &model.User2FAHistoryModel{
			UserID:       uid,
			Event:        "admin_unbind",
			Factor:       "totp",
			OperatorType: "admin",
			OperatorID:   current.ID,
			Reason:       strings.TrimSpace(in.Reason),
			IP:           ip,
			UserAgent:    ua,
			Meta:         []byte("{}"),
		}); err != nil {
			// ignore if migration not applied yet
			if _, ok := errx.MySQLTableNotFound(err); ok {
				// ignore
			} else if _, ok := errx.MySQLColumnNotFound(err); ok {
				// ignore
			} else {
				return err
			}
		}

		_ = auditRepo.CreateLog(l.ctx, &model.AdminAuditLogModel{
			AdminID:     current.ID,
			Action:      "user.2fa.unbind",
			TargetType:  "user",
			TargetID:    uidStr,
			Description: "解绑用户 TOTP: " + uidStr,
			Details:     detailsBytes,
			IP:          ip,
			UserAgent:   ua,
		})
		return nil
	}); err != nil {
		l.Logger.Errorf("unbind user totp failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	_ = u // keep for future extensions (e.g., notify user)
	return &pb.UnbindUserTotpResponse{
		Success: true,
		Message: resp.Msg(l.ctx, "TWO_FA_UNBOUND"),
		Data: &pb.UnbindUserTotpData{
			Uid:              uidStr,
			TwoFactorEnabled: false,
			UnboundAt:        formatTime(now),
			UnboundBy:        current.Username,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}

func tryExecIgnoreMySQLErr(tx *gorm.DB, ctx context.Context, sql string, values ...interface{}) (rowsAffected int64, err error) {
	if tx == nil {
		return 0, nil
	}
	res := tx.WithContext(ctx).Exec(sql, values...)
	if res.Error != nil {
		// Ignore schema differences for best-effort compatibility.
		if _, ok := errx.MySQLTableNotFound(res.Error); ok {
			return 0, nil
		}
		if _, ok := errx.MySQLColumnNotFound(res.Error); ok {
			return 0, nil
		}
		return 0, res.Error
	}
	return res.RowsAffected, nil
}
