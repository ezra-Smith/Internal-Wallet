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

type TerminateUserLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewTerminateUserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TerminateUserLogic {
	return &TerminateUserLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *TerminateUserLogic) TerminateUser(in *pb.TerminateUserRequest) (*pb.TerminateUserResponse, error) {
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

	uidStr := strings.TrimSpace(in.Uid)
	uid, err := strconv.ParseInt(uidStr, 10, 64)
	if err != nil || uid <= 0 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_UID", "invalid uid", map[string]string{"uid": "invalid"})
	}

	u, err := l.svcCtx.UserRepo.FindByID(l.ctx, uid)
	if err != nil || u == nil {
		return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "USER_NOT_FOUND", "用户不存在", nil)
	}
	if u.Status == 3 {
		return nil, errx.New(codes.AlreadyExists, 409, errx.CodeConflict, "USER_ALREADY_TERMINATED", "用户已终止", nil)
	}

	handleBalance := strings.TrimSpace(in.HandleBalance)
	if handleBalance == "" {
		handleBalance = "freeze"
	}
	if handleBalance != "freeze" && handleBalance != "withdraw" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_HANDLE_BALANCE", "无效余额处理方式", map[string]string{"handle_balance": "invalid"})
	}
	if handleBalance == "withdraw" && strings.TrimSpace(in.WithdrawAddress) == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_WITHDRAW_ADDRESS", "withdraw_address required", map[string]string{"withdraw_address": "required"})
	}

	// effective_date (optional)
	if strings.TrimSpace(in.EffectiveDate) != "" {
		if _, parseErr := time.ParseInLocation("2006-01-02", strings.TrimSpace(in.EffectiveDate), time.UTC); parseErr != nil {
			return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_EFFECTIVE_DATE", "无效生效日期", map[string]string{"effective_date": "invalid"})
		}
	}

	now := time.Now()
	ip := middleware.GetClientIP(l.ctx)
	ua := middleware.GetUserAgent(l.ctx)
	detailsBytes, _ := json.Marshal(map[string]interface{}{
		"reason":           strings.TrimSpace(in.Reason),
		"effective_date":   strings.TrimSpace(in.EffectiveDate),
		"handle_balance":   handleBalance,
		"withdraw_address": strings.TrimSpace(in.WithdrawAddress),
	})

	if err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		userRepo := repository.NewUserRepository(tx)
		auditRepo := repository.NewAdminAuditLogRepository(tx)

		if err := userRepo.UpdateStatus(l.ctx, uid, 3); err != nil {
			return err
		}
		_ = auditRepo.CreateLog(l.ctx, &model.AdminAuditLogModel{
			AdminID:     current.ID,
			Action:      "user.terminate",
			TargetType:  "user",
			TargetID:    uidStr,
			Description: "终止用户: " + uidStr,
			Details:     detailsBytes,
			IP:          ip,
			UserAgent:   ua,
		})
		return nil
	}); err != nil {
		l.Logger.Errorf("terminate user failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	return &pb.TerminateUserResponse{
		Success: true,
		Message: resp.Msg(l.ctx, "USER_TERMINATED"),
		Data: &pb.TerminateUserData{
			Uid:            uidStr,
			Status:         "terminated",
			TerminatedAt:   formatTime(now),
			TerminatedBy:   current.Username,
			Reason:         strings.TrimSpace(in.Reason),
			BalanceHandled: handleBalance,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
