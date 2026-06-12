package logic

import (
	"context"
	"encoding/json"
	"errors"
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
	"gorm.io/gorm/clause"
)

type UnblacklistWeb3AddressLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUnblacklistWeb3AddressLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UnblacklistWeb3AddressLogic {
	return &UnblacklistWeb3AddressLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UnblacklistWeb3AddressLogic) UnblacklistWeb3Address(in *pb.UnblacklistWeb3AddressRequest) (*pb.UnblacklistWeb3AddressResponse, error) {
	if in == nil || strings.TrimSpace(in.DeviceId) == "" || strings.TrimSpace(in.Address) == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "device_id/address required", map[string]string{
			"device_id": "required",
			"address":   "required",
		})
	}
	reason := strings.TrimSpace(in.Reason)
	if reason == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "reason required", map[string]string{"reason": "required"})
	}
	if l.svcCtx.DB == nil || l.svcCtx.Web3UserRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}
	current, ok := admininterceptor.GetCurrentAdmin(l.ctx)
	if !ok || current == nil {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}

	user, err := l.svcCtx.Web3UserRepo.FindByDeviceID(l.ctx, in.DeviceId)
	if err != nil || user == nil {
		return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "DEVICE_NOT_FOUND", "device not found", nil)
	}

	now := time.Now()
	ip := middleware.GetClientIP(l.ctx)
	ua := middleware.GetUserAgent(l.ctx)
	actor := strings.TrimSpace(current.Username)
	if actor == "" {
		actor = "admin"
	}

	var updated *model.Web3UserAddressModel
	if err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		var addr model.Web3UserAddressModel
		if err := tx.WithContext(l.ctx).
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("web3_user_id = ? AND address = ? AND deleted_at IS NULL", user.ID, strings.TrimSpace(in.Address)).
			First(&addr).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errx.New(codes.NotFound, 404, errx.CodeNotFound, "ADDRESS_NOT_FOUND", "address not found", nil)
			}
			return err
		}
		if !addr.IsBlacklisted {
			return errx.New(codes.FailedPrecondition, 409, errx.CodeConflict, "ADDRESS_NOT_BLACKLISTED", "address is not blacklisted", nil)
		}

		fields := map[string]interface{}{
			"is_blacklisted": false,
			"removed_at":     &now,
			"removed_by":     actor,
			"remove_reason":  reason,
			"updated_at":     &now,
		}
		if in.EnableAddress {
			fields["enabled"] = true
		}
		if err := tx.WithContext(l.ctx).
			Model(&model.Web3UserAddressModel{}).
			Where("id = ? AND deleted_at IS NULL", addr.ID).
			Updates(fields).Error; err != nil {
			return err
		}

		var out model.Web3UserAddressModel
		if err := tx.WithContext(l.ctx).
			Where("id = ? AND deleted_at IS NULL", addr.ID).
			First(&out).Error; err != nil {
			return err
		}
		updated = &out

		auditRepo := repository.NewAdminAuditLogRepository(tx)
		detailsBytes, _ := json.Marshal(map[string]interface{}{
			"device_id":      user.DeviceID,
			"address":        maskAddress(addr.Address),
			"network":        addr.Network,
			"enable_address": in.EnableAddress,
			"notify_user":    in.NotifyUser,
			"reason":         reason,
		})
		_ = auditRepo.CreateLog(l.ctx, &model.AdminAuditLogModel{
			AdminID:     current.ID,
			Action:      "web3.address.blacklist.remove",
			TargetType:  "web3_user_address",
			TargetID:    maskAddress(addr.Address),
			Description: "Web3 地址移出黑名单: " + maskAddress(addr.Address),
			Details:     detailsBytes,
			IP:          ip,
			UserAgent:   ua,
		})
		return nil
	}); err != nil {
		if _, ok := status.FromError(err); ok {
			return nil, err
		}
		l.Logger.Errorf("unblacklist web3 address failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "unblacklist failed", nil)
	}

	return &pb.UnblacklistWeb3AddressResponse{
		Success: true,
		Message: resp.Msg(l.ctx, "ADDRESS_UNBLACKLISTED"),
		Data: &pb.UnblacklistWeb3AddressData{
			Result: toPBWeb3BlacklistInfo(user.DeviceID, updated),
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
