package logic

import (
	"context"
	"encoding/json"
	"fmt"
	admininterceptor "internalwallet/services/admin/rpc/internal/interceptor"
	"strconv"
	"strings"
	"time"

	"internalwallet/common/middleware"
	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/repository"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"gorm.io/gorm"
)

type UnlockTradePasswordLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUnlockTradePasswordLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UnlockTradePasswordLogic {
	return &UnlockTradePasswordLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// UnlockTradePassword 解锁用户的交易密码（重置错误次数）
// 此接口专门用于重置交易密码错误次数，不会改变用户的冻结状态
// 适用场景：用户交易密码输入错误被锁定，但账户未被冻结或管理员只想重置密码错误次数
func (l *UnlockTradePasswordLogic) UnlockTradePassword(in *pb.UnlockTradePasswordRequest) (*pb.UnlockTradePasswordResponse, error) {
	// 1. 参数验证
	if in == nil || strings.TrimSpace(in.Uid) == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_UID", "uid required", map[string]string{"uid": "required"})
	}
	if strings.TrimSpace(in.Reason) == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_REASON", "reason required", map[string]string{"reason": "required"})
	}
	if l.svcCtx.DB == nil || l.svcCtx.UserRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	// 2. 获取当前管理员信息
	current, ok := admininterceptor.GetCurrentAdmin(l.ctx)
	if !ok || current == nil {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}

	// 3. 验证用户是否存在
	uidStr := strings.TrimSpace(in.Uid)
	uid, err := strconv.ParseInt(uidStr, 10, 64)
	if err != nil || uid <= 0 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_UID", "invalid uid", map[string]string{"uid": "invalid"})
	}

	u, err := l.svcCtx.UserRepo.FindByID(l.ctx, uid)
	if err != nil || u == nil {
		return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "USER_NOT_FOUND", "用户不存在", nil)
	}

	// 4. 检查用户是否已终止（终止的用户不允许操作）
	if u.Status == 3 {
		return nil, errx.New(codes.FailedPrecondition, 409, errx.CodeConflict, "USER_TERMINATED", "用户已终止，无法操作", nil)
	}

	// 5. 获取当前的错误次数（用于返回给管理员）
	previousAttempts := l.getTradePasswordAttempts(uid)

	// 6. 重置交易密码错误次数
	if err := l.resetTradePasswordAttemptsInRedis(uid); err != nil {
		l.Logger.Errorf("Failed to reset trade password attempts for user %d: %v", uid, err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "failed to reset attempts", nil)
	}

	// 7. 记录审计日志
	now := time.Now()
	ip := middleware.GetClientIP(l.ctx)
	ua := middleware.GetUserAgent(l.ctx)
	detailsBytes, _ := json.Marshal(map[string]interface{}{
		"reason":            strings.TrimSpace(in.Reason),
		"previous_attempts": previousAttempts,
		"user_status":       u.Status,
	})

	if err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		auditRepo := repository.NewAdminAuditLogRepository(tx)
		return auditRepo.CreateLog(l.ctx, &model.AdminAuditLogModel{
			AdminID:     current.ID,
			Action:      "user.unlock_trade_password",
			TargetType:  "user",
			TargetID:    uidStr,
			Description: fmt.Sprintf("解锁用户交易密码: %s (之前错误次数: %d)", uidStr, previousAttempts),
			Details:     detailsBytes,
			IP:          ip,
			UserAgent:   ua,
		})
	}); err != nil {
		// 审计日志失败不影响主流程，仅记录错误
		l.Logger.Errorf("Failed to create audit log: %v", err)
	}

	// 8. 返回成功响应
	return &pb.UnlockTradePasswordResponse{
		Success: true,
		Message: "交易密码已解锁",
		Data: &pb.UnlockTradePasswordData{
			Uid:              uidStr,
			PreviousAttempts: int32(previousAttempts),
			UnlockedAt:       formatTime(now),
			UnlockedBy:       current.Username,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}

// getTradePasswordAttempts 获取当前的交易密码错误次数
func (l *UnlockTradePasswordLogic) getTradePasswordAttempts(userID int64) int {
	if l.svcCtx.RedisClient == nil {
		return 0
	}

	attemptsKey := fmt.Sprintf("%s%d", TradePasswordAttemptsKeyPrefix, userID)
	val, err := l.svcCtx.RedisClient.Get(l.ctx, attemptsKey).Result()
	if err != nil {
		if err == redis.Nil {
			// 键不存在，说明没有错误记录
			return 0
		}
		l.Logger.Errorf("Failed to get trade password attempts from Redis: user_id=%d, err=%v", userID, err)
		return 0
	}

	attempts, err := strconv.Atoi(val)
	if err != nil {
		l.Logger.Errorf("Failed to parse trade password attempts: user_id=%d, val=%s, err=%v", userID, val, err)
		return 0
	}

	return attempts
}

// resetTradePasswordAttemptsInRedis 从 Redis 中删除交易密码错误次数记录
func (l *UnlockTradePasswordLogic) resetTradePasswordAttemptsInRedis(userID int64) error {
	if l.svcCtx.RedisClient == nil {
		l.Logger.Infof("RedisClient not configured, skip resetting trade password attempts for user %d", userID)
		return nil
	}

	attemptsKey := fmt.Sprintf("%s%d", TradePasswordAttemptsKeyPrefix, userID)

	// 删除 Redis 键
	result, err := l.svcCtx.RedisClient.Del(l.ctx, attemptsKey).Result()
	if err != nil {
		return fmt.Errorf("failed to delete trade password attempts from redis: %w", err)
	}

	if result == 0 {
		// 键不存在（用户可能没有交易密码错误记录）
		l.Logger.Infof("Trade password attempts key not found in Redis (no attempts recorded): user_id=%d", userID)
	} else {
		l.Logger.Infof("✓ Trade password attempts deleted from Redis: user_id=%d, key=%s", userID, attemptsKey)
	}

	return nil
}
