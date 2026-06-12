package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"internalwallet/common/middleware"
	"internalwallet/common/mq"
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

type UpdateDepositAddressLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateDepositAddressLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateDepositAddressLogic {
	return &UpdateDepositAddressLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateDepositAddressLogic) UpdateDepositAddress(in *pb.UpdateDepositAddressRequest) (*pb.UpdateDepositAddressResponse, error) {
	if in == nil || in.Id <= 0 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid id", map[string]string{"id": "required"})
	}
	if l.svcCtx.DB == nil || l.svcCtx.UserRepo == nil || l.svcCtx.WalletDepositAddressRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	current, ok := admininterceptor.GetCurrentAdmin(l.ctx)
	if !ok || current == nil {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}

	existing, err := l.svcCtx.WalletDepositAddressRepo.FindByID(l.ctx, in.Id)
	if err != nil || existing == nil {
		return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "NOT_FOUND", "deposit address not found", nil)
	}

	newUserID := existing.UserID
	if in.UserId != nil {
		if in.UserId.Value <= 0 {
			return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid user_id", map[string]string{"user_id": "invalid"})
		}
		if _, err := l.svcCtx.UserRepo.FindByID(l.ctx, in.UserId.Value); err != nil {
			return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "USER_NOT_FOUND", "user not found", nil)
		}
		newUserID = in.UserId.Value
	}

	newStatus := existing.Status
	if in.Status != nil {
		v := strings.TrimSpace(in.Status.Value)
		if !validateDepositAddressStatus(v) {
			return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_STATUS", "invalid status", map[string]string{"status": "invalid"})
		}
		newStatus = v
	}

	// Enforce "single active per user+chain" on update.
	if newStatus == "active" {
		var count int64
		if err := l.svcCtx.DB.WithContext(l.ctx).
			Model(&model.WalletDepositAddressModel{}).
			Where("deleted_at IS NULL AND status = ? AND user_id = ? AND chain_code = ? AND id <> ?", "active", newUserID, existing.ChainCode, in.Id).
			Count(&count).Error; err != nil {
			l.Logger.Errorf("check active address uniqueness failed: %v", err)
			return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
		}
		if count > 0 {
			return nil, errx.New(codes.AlreadyExists, 409, errx.CodeConflict, "ACTIVE_ADDRESS_EXISTS", "active address already exists for user+chain", map[string]string{
				"user_id":    "exists",
				"chain_code": "exists",
			})
		}
	}

	now := time.Now()
	fields := map[string]interface{}{
		"updated_at": &now,
	}
	changes := map[string]interface{}{}

	if in.UserId != nil {
		fields["user_id"] = newUserID
		changes["user_id"] = newUserID
	}
	if in.Status != nil {
		fields["status"] = newStatus
		changes["status"] = newStatus
	}
	if in.Memo != nil {
		fields["memo"] = strPtrOrNilTrim(in.Memo.Value)
		changes["memo"] = strings.TrimSpace(in.Memo.Value)
	}
	if len(changes) == 0 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "no fields to update", map[string]string{"fields": "empty"})
	}

	ip := middleware.GetClientIP(l.ctx)
	ua := middleware.GetUserAgent(l.ctx)

	var updated *model.WalletDepositAddressModel
	if err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		addrRepo := l.svcCtx.WalletDepositAddressRepo.WithTx(tx)
		auditRepo := repository.NewAdminAuditLogRepository(tx)

		if err := addrRepo.UpdateFields(l.ctx, in.Id, fields); err != nil {
			return err
		}
		m, err := addrRepo.FindByID(l.ctx, in.Id)
		if err != nil {
			return err
		}
		updated = m

		detailsBytes, _ := json.Marshal(map[string]interface{}{
			"address":    maskAddress(existing.Address),
			"chain_code": existing.ChainCode,
			"changes":    changes,
		})
		_ = auditRepo.CreateLog(l.ctx, &model.AdminAuditLogModel{
			AdminID:     current.ID,
			Action:      "wallet.deposit_address.update",
			TargetType:  "wallet_deposit_address",
			TargetID:    maskAddress(existing.Address),
			Description: "更新充值地址: " + maskAddress(existing.Address),
			Details:     detailsBytes,
			IP:          ip,
			UserAgent:   ua,
		})
		return nil
	}); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			return nil, errx.New(codes.AlreadyExists, 409, errx.CodeConflict, "CONFLICT", "duplicate active user+token", nil)
		}
		l.Logger.Errorf("update deposit address failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "update deposit address failed", nil)
	}

	prevStatus := strings.TrimSpace(existing.Status)
	nextStatus := strings.TrimSpace(updated.Status)
	if prevStatus != nextStatus {
		switch {
		case prevStatus != "active" && nextStatus == "active":
			l.svcCtx.PublishAddressMonitorEvent(l.ctx, mq.AddressMonitorEvent{
				Action:  mq.AddressMonitorActionUpsert,
				Source:  mq.AddressMonitorSourceDeposit,
				Chain:   strings.TrimSpace(updated.ChainCode),
				Address: strings.TrimSpace(updated.Address),
				Reason:  "admin.deposit_address.status_active",
				Metadata: map[string]string{
					"deposit_address_id": fmt.Sprintf("%d", updated.ID),
					"user_id":            fmt.Sprintf("%d", updated.UserID),
				},
			})
		case prevStatus == "active" && nextStatus != "active":
			l.svcCtx.PublishAddressMonitorEvent(l.ctx, mq.AddressMonitorEvent{
				Action:  mq.AddressMonitorActionRemove,
				Source:  mq.AddressMonitorSourceDeposit,
				Chain:   strings.TrimSpace(updated.ChainCode),
				Address: strings.TrimSpace(updated.Address),
				Reason:  "admin.deposit_address.status_inactive",
				Metadata: map[string]string{
					"deposit_address_id": fmt.Sprintf("%d", updated.ID),
					"user_id":            fmt.Sprintf("%d", updated.UserID),
				},
			})
		}
	}

	return &pb.UpdateDepositAddressResponse{
		Success: true,
		Message: "ok",
		Data: &pb.UpdateDepositAddressData{
			Address: toPBDepositAddressItem(updated),
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
