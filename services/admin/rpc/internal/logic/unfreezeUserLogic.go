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

type UnfreezeUserLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUnfreezeUserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UnfreezeUserLogic {
	return &UnfreezeUserLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UnfreezeUserLogic) UnfreezeUser(in *pb.UnfreezeUserRequest) (*pb.UnfreezeUserResponse, error) {
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
		return nil, errx.New(codes.FailedPrecondition, 409, errx.CodeConflict, "USER_ALREADY_TERMINATED", "用户已终止，无法解冻", nil)
	}

	now := time.Now()
	ip := middleware.GetClientIP(l.ctx)
	ua := middleware.GetUserAgent(l.ctx)

	// 判断是否需要执行解冻操作
	needUnfreeze := u.Status == 2 // 只有状态为2（冻结）时才需要解冻
	action := "user.unfreeze"
	description := "解冻用户: " + uidStr

	if !needUnfreeze {
		// 用户未被冻结，只是重置交易密码错误次数
		action = "user.reset_trade_password_attempts"
		description = "重置用户交易密码错误次数: " + uidStr
		l.Logger.Infof("User %d is not frozen (status=%d), will only reset trade password attempts", uid, u.Status)
	}

	detailsBytes, _ := json.Marshal(map[string]interface{}{
		"reason":          strings.TrimSpace(in.Reason),
		"unfreeze_assets": in.UnfreezeAssets,
		"notify_user":     in.NotifyUser,
		"user_status":     u.Status,
		"need_unfreeze":   needUnfreeze,
	})

	// 只有需要解冻时才执行数据库操作
	if needUnfreeze {
		if err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
			userRepo := repository.NewUserRepository(tx)
			auditRepo := repository.NewAdminAuditLogRepository(tx)

			if err := userRepo.UpdateStatus(l.ctx, uid, 1); err != nil {
				return err
			}
			_ = auditRepo.CreateLog(l.ctx, &model.AdminAuditLogModel{
				AdminID:     current.ID,
				Action:      action,
				TargetType:  "user",
				TargetID:    uidStr,
				Description: description,
				Details:     detailsBytes,
				IP:          ip,
				UserAgent:   ua,
			})
			return nil
		}); err != nil {
			l.Logger.Errorf("unfreeze user failed: %v", err)
			return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
		}

		// 清除 Redis 中的冻结标记，让用户可以立即使用
		if err := l.removeUserFrozenFlag(uid); err != nil {
			// 删除标记失败不影响解冻流程（标记会在 30 天后自动过期）
			l.Logger.Errorf("failed to remove user frozen flag: user_id=%d, err=%v", uid, err)
		} else {
			l.Logger.Infof("✓ User frozen flag removed from Redis: user_id=%d", uid)
		}
	} else {
		// 用户未被冻结，只记录审计日志
		if err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
			auditRepo := repository.NewAdminAuditLogRepository(tx)
			return auditRepo.CreateLog(l.ctx, &model.AdminAuditLogModel{
				AdminID:     current.ID,
				Action:      action,
				TargetType:  "user",
				TargetID:    uidStr,
				Description: description,
				Details:     detailsBytes,
				IP:          ip,
				UserAgent:   ua,
			})
		}); err != nil {
			// 审计日志失败不影响主流程
			l.Logger.Errorf("failed to create audit log: %v", err)
		}
	}

	// 重置交易密码错误次数（不管用户是否被冻结，都要重置）
	if err := l.resetTradePasswordAttempts(uid); err != nil {
		// 重置失败不影响主流程（错误次数会在 24 小时后自动过期）
		l.Logger.Errorf("failed to reset trade password attempts: user_id=%d, err=%v", uid, err)
	} else {
		l.Logger.Infof("✓ Trade password attempts reset successfully: user_id=%d", uid)
	}

	// Best-effort: unfreeze user assets (move from locked to available bucket).
	// 只有在需要解冻且请求要求解冻资产时才执行
	if needUnfreeze && in.UnfreezeAssets && l.svcCtx.AccountingRpc != nil {
		balances, err := l.svcCtx.AccountingRpc.GetUserBalances(l.ctx, &pb.GetUserBalancesRequest{
			UserId: uid,
		})
		if err == nil && balances != nil {
			for _, bal := range balances.Items {
				if bal == nil || bal.Locked == "0" || bal.Locked == "" {
					continue
				}
				// Unfreeze each asset's locked balance.
				unfreezeResp, _ := l.svcCtx.AccountingRpc.UnfreezeUserAssets(l.ctx, &pb.UnfreezeUserAssetsRequest{
					IdempotencyKey: "user:unfreeze:" + uidStr + ":" + bal.AssetCode + ":" + strconv.FormatInt(now.Unix(), 10),
					BizRef:         "user_unfreeze:" + uidStr,
					UserId:         uid,
					AssetCode:      bal.AssetCode,
					AmountDecimal:  bal.Locked,
					Reason:         strings.TrimSpace(in.Reason),
				})
				if unfreezeResp != nil && !unfreezeResp.Success {
					l.Logger.Errorf("unfreeze assets failed for user %d asset %s: %s", uid, bal.AssetCode, unfreezeResp.Message)
				}
			}
		} else if err != nil {
			l.Logger.Errorf("get user balances failed for unfreezing assets: %v", err)
		}
	}

	// 根据操作类型返回不同的消息
	responseMessage := resp.Msg(l.ctx, "USER_UNFROZEN")
	if !needUnfreeze {
		responseMessage = "交易密码错误次数已重置"
	}

	// 获取当前状态（如果执行了解冻，状态应该是 active；否则保持原状态）
	currentStatus := "active"
	if !needUnfreeze {
		if u.Status == 1 {
			currentStatus = "active"
		} else {
			currentStatus = fmt.Sprintf("status_%d", u.Status)
		}
	}

	return &pb.UnfreezeUserResponse{
		Success: true,
		Message: responseMessage,
		Data: &pb.UnfreezeUserData{
			Uid:        uidStr,
			Status:     currentStatus,
			UnfrozenAt: formatTime(now),
			UnfrozenBy: current.Username,
			Reason:     strings.TrimSpace(in.Reason),
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}

// removeUserFrozenFlag 清除 Redis 中的用户冻结标记
// 解冻用户后，需要删除 Redis 标记，让用户可以立即使用其 token
func (l *UnfreezeUserLogic) removeUserFrozenFlag(userID int64) error {
	if l.svcCtx.RedisClient == nil {
		l.Logger.Infof("RedisClient not configured, skip removing frozen flag for user %d", userID)
		return nil
	}

	key := fmt.Sprintf("user:frozen:%d", userID)

	// 删除 Redis 键
	result, err := l.svcCtx.RedisClient.Del(l.ctx, key).Result()
	if err != nil {
		return fmt.Errorf("failed to delete frozen flag from redis: %w", err)
	}

	if result == 0 {
		// 键不存在（可能已经过期或者从未设置）
		l.Logger.Infof("Frozen flag not found in Redis (already expired or never set): user_id=%d", userID)
	} else {
		l.Logger.Infof("Frozen flag deleted from Redis: user_id=%d, key=%s", userID, key)
	}

	return nil
}

// resetTradePasswordAttempts 重置交易密码错误次数
// 解冻用户时调用，清除交易密码锁定状态，让用户可以重新输入交易密码
func (l *UnfreezeUserLogic) resetTradePasswordAttempts(userID int64) error {
	if l.svcCtx.RedisClient == nil {
		l.Logger.Infof("RedisClient not configured, skip resetting trade password attempts for user %d", userID)
		return nil
	}

	attemptsKey := fmt.Sprintf("%s%d", TradePasswordAttemptsKeyPrefix, userID)

	// 删除 Redis 键
	result, err := l.svcCtx.RedisClient.Del(l.ctx, attemptsKey).Result()
	if err != nil {
		return fmt.Errorf("failed to reset trade password attempts from redis: %w", err)
	}

	if result == 0 {
		// 键不存在（用户可能没有交易密码错误记录）
		l.Logger.Infof("Trade password attempts key not found in Redis (no attempts recorded): user_id=%d", userID)
	} else {
		l.Logger.Infof("✓ Trade password attempts reset: user_id=%d, key=%s", userID, attemptsKey)
	}

	return nil
}
