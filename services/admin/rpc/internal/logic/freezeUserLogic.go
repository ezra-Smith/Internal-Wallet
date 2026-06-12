package logic

import (
	"context"
	"encoding/json"
	"fmt"
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

type FreezeUserLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewFreezeUserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FreezeUserLogic {
	return &FreezeUserLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *FreezeUserLogic) FreezeUser(in *pb.FreezeUserRequest) (*pb.FreezeUserResponse, error) {
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
	if u.Status == 2 {
		return nil, errx.New(codes.AlreadyExists, 409, errx.CodeConflict, "USER_ALREADY_FROZEN", "用户已处于冻结状态", nil)
	}
	if u.Status == 3 {
		return nil, errx.New(codes.FailedPrecondition, 409, errx.CodeConflict, "USER_ALREADY_TERMINATED", "用户已终止", nil)
	}

	now := time.Now()
	ip := middleware.GetClientIP(l.ctx)
	ua := middleware.GetUserAgent(l.ctx)
	detailsBytes, _ := json.Marshal(map[string]interface{}{
		"reason":        strings.TrimSpace(in.Reason),
		"freeze_assets": in.FreezeAssets,
		"notify_user":   in.NotifyUser,
	})

	if err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		userRepo := repository.NewUserRepository(tx)
		auditRepo := repository.NewAdminAuditLogRepository(tx)

		if err := userRepo.UpdateStatus(l.ctx, uid, 2); err != nil {
			return err
		}
		_ = auditRepo.CreateLog(l.ctx, &model.AdminAuditLogModel{
			AdminID:     current.ID,
			Action:      "user.freeze",
			TargetType:  "user",
			TargetID:    uidStr,
			Description: "冻结用户: " + uidStr,
			Details:     detailsBytes,
			IP:          ip,
			UserAgent:   ua,
		})
		return nil
	}); err != nil {
		l.Logger.Errorf("freeze user failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	// 清除用户的登录状态，使其所有 JWT token 失效
	if err := l.invalidateUserTokens(uid); err != nil {
		// Token 失效失败不影响冻结流程，仅记录日志
		l.Logger.Errorf("failed to invalidate user tokens: user_id=%d, err=%v", uid, err)
	} else {
		l.Logger.Infof("✓ User tokens invalidated successfully: user_id=%d", uid)
	}

	// Best-effort: freeze user assets (move from available to locked bucket).
	if in.FreezeAssets && l.svcCtx.AccountingRpc != nil {
		balances, err := l.svcCtx.AccountingRpc.GetUserBalances(l.ctx, &pb.GetUserBalancesRequest{
			UserId: uid,
		})
		if err == nil && balances != nil {
			for _, bal := range balances.Items {
				if bal == nil || bal.Available == "0" || bal.Available == "" {
					continue
				}
				// Freeze each asset's available balance.
				freezeResp, _ := l.svcCtx.AccountingRpc.FreezeUserAssets(l.ctx, &pb.FreezeUserAssetsRequest{
					IdempotencyKey: "user:freeze:" + uidStr + ":" + bal.AssetCode + ":" + strconv.FormatInt(now.Unix(), 10),
					BizRef:         "user_freeze:" + uidStr,
					UserId:         uid,
					AssetCode:      bal.AssetCode,
					AmountDecimal:  bal.Available,
					Reason:         strings.TrimSpace(in.Reason),
				})
				if freezeResp != nil && !freezeResp.Success {
					l.Logger.Errorf("freeze assets failed for user %d asset %s: %s", uid, bal.AssetCode, freezeResp.Message)
				}
			}
		} else if err != nil {
			l.Logger.Errorf("get user balances failed for freezing assets: %v", err)
		}
	}

	return &pb.FreezeUserResponse{
		Success: true,
		Message: resp.Msg(l.ctx, "USER_FROZEN"),
		Data: &pb.FreezeUserData{
			Uid:      uidStr,
			Status:   "frozen",
			FrozenAt: formatTime(now),
			FrozenBy: current.Username,
			Reason:   strings.TrimSpace(in.Reason),
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}

// invalidateUserTokens 清除用户的所有登录状态（使 JWT token 失效）
// 通过在 Redis 中设置用户冻结标记，业务服务在验证 token 时会检查此标记
func (l *FreezeUserLogic) invalidateUserTokens(userID int64) error {
	if l.svcCtx.RedisClient == nil {
		l.Logger.Infof("RedisClient not configured, skip token invalidation for user %d", userID)
		return nil
	}

	// 在 Redis 中设置用户冻结标记
	// Key 格式: user:frozen:{user_id}
	// 值为冻结时间戳，TTL 设置为 JWT token 的最大有效期（30 天）
	key := fmt.Sprintf("user:frozen:%d", userID)
	ttl := 30 * 24 * time.Hour // JWT token 的最大有效期（30天）

	if err := l.svcCtx.RedisClient.Set(l.ctx, key, time.Now().Unix(), ttl).Err(); err != nil {
		return fmt.Errorf("failed to set user frozen flag in redis: %w", err)
	}

	l.Logger.Infof("User frozen flag set in Redis: user_id=%d, key=%s, ttl=%v", userID, key, ttl)
	return nil
}
