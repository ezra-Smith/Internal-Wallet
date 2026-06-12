package logic

import (
	"context"
	"encoding/json"
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
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

type UpdateBlacklistAddressMonitorStatusLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateBlacklistAddressMonitorStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateBlacklistAddressMonitorStatusLogic {
	return &UpdateBlacklistAddressMonitorStatusLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateBlacklistAddressMonitorStatusLogic) UpdateBlacklistAddressMonitorStatus(in *pb.UpdateBlacklistAddressMonitorStatusRequest) (*pb.UpdateBlacklistAddressMonitorStatusResponse, error) {
	if in == nil || in.Id <= 0 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "id required", map[string]string{"id": "required"})
	}
	if l.svcCtx == nil || l.svcCtx.DB == nil || l.svcCtx.BlacklistAddressRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}
	current, ok := admininterceptor.GetCurrentAdmin(l.ctx)
	if !ok || current == nil {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}

	newStatus := normalizeLower(in.MonitorStatus)
	if newStatus == "" || !validateBlacklistMonitorStatus(newStatus) {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_MONITOR_STATUS", "invalid monitor_status", map[string]string{"monitor_status": "invalid"})
	}

	now := time.Now()
	ip := middleware.GetClientIP(l.ctx)
	ua := middleware.GetUserAgent(l.ctx)

	var before, after *model.BlacklistAddressModel
	if err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		repo := l.svcCtx.BlacklistAddressRepo.WithTx(tx)
		auditRepo := repository.NewAdminAuditLogRepository(tx)

		m, err := repo.FindByID(l.ctx, in.Id)
		if err != nil || m == nil {
			return errx.New(codes.NotFound, 404, errx.CodeNotFound, "NOT_FOUND", "blacklist address not found", nil)
		}
		before = m

		if err := repo.UpdateMonitorStatus(l.ctx, in.Id, newStatus, now); err != nil {
			return err
		}
		updated, err := repo.FindByID(l.ctx, in.Id)
		if err != nil || updated == nil {
			return errx.New(codes.NotFound, 404, errx.CodeNotFound, "NOT_FOUND", "blacklist address not found", nil)
		}
		after = updated

		detailsBytes, _ := json.Marshal(map[string]interface{}{
			"id":             in.Id,
			"address":        maskAddress(after.Address),
			"network":        after.Network,
			"old_status":     strings.TrimSpace(before.MonitorStatus),
			"monitor_status": newStatus,
		})
		_ = auditRepo.CreateLog(l.ctx, &model.AdminAuditLogModel{
			AdminID:     current.ID,
			Action:      "blacklist.address.monitor.update",
			TargetType:  "blacklist_address",
			TargetID:    maskAddress(after.Address),
			Description: "更新黑名单监测状态: " + maskAddress(after.Address),
			Details:     detailsBytes,
			IP:          ip,
			UserAgent:   ua,
		})
		return nil
	}); err != nil {
		if _, ok := status.FromError(err); ok {
			return nil, err
		}
		l.Logger.Errorf("update blacklist monitor status failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "update failed", nil)
	}

	return &pb.UpdateBlacklistAddressMonitorStatusResponse{
		Success: true,
		Message: "ok",
		Data: &pb.UpdateBlacklistAddressMonitorStatusData{
			Address: toPBBlacklistAddressItem(after),
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
