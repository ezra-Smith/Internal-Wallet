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

type BlacklistWeb3AddressLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewBlacklistWeb3AddressLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BlacklistWeb3AddressLogic {
	return &BlacklistWeb3AddressLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func validateRiskLevel(level string) bool {
	switch strings.TrimSpace(level) {
	case "high", "medium", "low":
		return true
	default:
		return false
	}
}

func (l *BlacklistWeb3AddressLogic) BlacklistWeb3Address(in *pb.BlacklistWeb3AddressRequest) (*pb.BlacklistWeb3AddressResponse, error) {
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
	riskLevel := strings.TrimSpace(in.RiskLevel)
	if riskLevel == "" || !validateRiskLevel(riskLevel) {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_RISK_LEVEL", "invalid risk_level", map[string]string{"risk_level": "invalid"})
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
		if addr.IsBlacklisted {
			return errx.New(codes.AlreadyExists, 409, errx.CodeConflict, "ADDRESS_ALREADY_BLACKLISTED", "address already blacklisted", nil)
		}

		fields := map[string]interface{}{
			"is_blacklisted":   true,
			"blacklist_reason": reason,
			"risk_level":       riskLevel,
			"blacklisted_at":   &now,
			"blacklisted_by":   actor,
			"removed_at":       nil,
			"removed_by":       nil,
			"remove_reason":    nil,
			"updated_at":       &now,
		}
		if in.DisableAddress {
			fields["enabled"] = false
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
			"device_id":       user.DeviceID,
			"address":         maskAddress(addr.Address),
			"network":         addr.Network,
			"risk_level":      riskLevel,
			"disable_address": in.DisableAddress,
			"notify_user":     in.NotifyUser,
			"reason":          reason,
		})
		_ = auditRepo.CreateLog(l.ctx, &model.AdminAuditLogModel{
			AdminID:     current.ID,
			Action:      "web3.address.blacklist.add",
			TargetType:  "web3_user_address",
			TargetID:    maskAddress(addr.Address),
			Description: "Web3 地址加入黑名单: " + maskAddress(addr.Address),
			Details:     detailsBytes,
			IP:          ip,
			UserAgent:   ua,
		})
		return nil
	}); err != nil {
		if _, ok := status.FromError(err); ok {
			return nil, err
		}
		l.Logger.Errorf("blacklist web3 address failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "blacklist failed", nil)
	}

	return &pb.BlacklistWeb3AddressResponse{
		Success: true,
		Message: resp.Msg(l.ctx, "ADDRESS_BLACKLISTED"),
		Data: &pb.BlacklistWeb3AddressData{
			Result: toPBWeb3BlacklistInfo(user.DeviceID, updated),
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}

func toPBWeb3BlacklistInfo(deviceID string, m *model.Web3UserAddressModel) *pb.Web3AddressBlacklistInfo {
	if m == nil {
		return nil
	}
	return &pb.Web3AddressBlacklistInfo{
		DeviceId:        deviceID,
		Address:         maskAddress(m.Address),
		Network:         m.Network,
		Enabled:         m.Enabled,
		IsBlacklisted:   m.IsBlacklisted,
		BlacklistReason: derefString(m.BlacklistReason),
		RiskLevel:       derefString(m.RiskLevel),
		BlacklistedAt:   formatTimePtr(m.BlacklistedAt),
		BlacklistedBy:   derefString(m.BlacklistedBy),
		RemovedAt:       formatTimePtr(m.RemovedAt),
		RemovedBy:       derefString(m.RemovedBy),
		RemoveReason:    derefString(m.RemoveReason),
	}
}
