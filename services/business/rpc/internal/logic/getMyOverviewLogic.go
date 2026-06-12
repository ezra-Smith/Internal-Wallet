package logic

import (
	"context"
	"strconv"
	"strings"

	"internalwallet/common/middleware"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetMyOverviewLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetMyOverviewLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetMyOverviewLogic {
	return &GetMyOverviewLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetMyOverviewLogic) GetMyOverview(in *pb.GetMyOverviewReq) (*pb.GetMyOverviewResp, error) {
	uidStr := middleware.GetUserID(l.ctx)
	uidStr = strings.TrimSpace(uidStr)
	if uidStr == "" {
		l.Logger.Error("GetMyOverview: missing user id in context")
		return nil, errx.Unauthorized("unauthorized")
	}
	id, err := strconv.ParseInt(uidStr, 10, 64)
	if err != nil || id <= 0 {
		l.Logger.Errorf("GetMyOverview: invalid user id format: %q, err=%v", uidStr, err)
		return nil, errx.InvalidParam("invalid user id")
	}

	var googleAuthEnabled bool
	var googleAuthBound bool
	if l.svcCtx.UserAccountRepository == nil {
		l.Logger.Error("GetMyOverview: UserAccountRepository not initialized (likely DB/GORM config missing)")
		return nil, errx.ServiceNotAvailable("db")
	}
	profile, err := l.svcCtx.UserAccountRepository.GetByUID(l.ctx, uidStr)
	if profile == nil {
		if err != nil {
			l.Logger.Errorf("GetMyOverview: get user profile failed (uid=%s): %v", uidStr, err)
		}
		return nil, errx.UserNotFound()
	}

	if l.svcCtx.UserSecuritySettingsRepository != nil {
		ss, ssErr := l.svcCtx.UserSecuritySettingsRepository.GetByUserID(l.ctx, id)
		if ssErr != nil {
			l.Logger.Errorf("GetMyOverview: get security settings failed (uid=%d): %v", id, ssErr)
		}
		if ss != nil {
			googleAuthEnabled = ss.GoogleAuthEnabled
			googleAuthBound = ss.GoogleAuthBound
		}
	}

	var unread int64
	if l.svcCtx.UserNotificationRepository != nil {
		u, uErr := l.svcCtx.UserNotificationRepository.CountUnreadByUser(l.ctx, id)
		if uErr != nil {
			l.Logger.Errorf("GetMyOverview: count unread notifications failed (uid=%d): %v", id, uErr)
		} else {
			unread = u
		}
	}

	var devices int64
	if l.svcCtx.UserDeviceRepository != nil {
		d, dErr := l.svcCtx.UserDeviceRepository.CountByUser(l.ctx, id)
		if dErr != nil {
			l.Logger.Errorf("GetMyOverview: count devices failed (uid=%d): %v", id, dErr)
		} else {
			devices = d
		}
	}

	// 查询关联钱包数量
	var linkedWalletCount int32
	var linkedWalletBound bool
	if l.svcCtx.PlatformBindingRepository != nil {
		bindings, bErr := l.svcCtx.PlatformBindingRepository.ListBindingsByUser(l.ctx, id)
		if bErr != nil {
			l.Logger.Errorf("GetMyOverview: list platform bindings failed (uid=%d): %v", id, bErr)
		} else {
			linkedWalletCount = int32(len(bindings))
			linkedWalletBound = linkedWalletCount > 0
		}
	}

	// 查询提现免审白名单绑定状态
	var withdrawWhitelistBound bool
	if l.svcCtx.UserWithdrawAuditWhitelistRuleRepository != nil {
		bound, wErr := l.svcCtx.UserWithdrawAuditWhitelistRuleRepository.HasEnabledByUserSource(l.ctx, id, "user")
		if wErr != nil {
			l.Logger.Errorf("GetMyOverview: check withdraw whitelist rules failed (uid=%d): %v", id, wErr)
		} else {
			withdrawWhitelistBound = bound
		}
	}

	return &pb.GetMyOverviewResp{
		Success:                true,
		Uid:                    strconv.FormatInt(id, 10),
		Nickname:               profile.Nickname,
		EmailBound:             profile.Email != "",
		PhoneBound:             profile.Phone != "",
		GoogleAuthEnabled:      googleAuthEnabled,
		HasTradePassword:       profile.HasTradePassword,
		UnreadCount:            int32(unread),
		DevicesCount:           int32(devices),
		Language:               profile.Language,
		ThemeMode:              profile.ThemeMode,
		PriceUnit:              profile.PriceUnit,
		AvatarUrl:              profile.Avatar,
		Email:                  profile.Email,
		Phone:                  profile.Phone,
		LinkedWalletBound:      linkedWalletBound,
		LinkedWalletCount:      linkedWalletCount,
		WithdrawWhitelistBound: withdrawWhitelistBound,
		GoogleAuthBound:        googleAuthBound,
	}, nil

}
