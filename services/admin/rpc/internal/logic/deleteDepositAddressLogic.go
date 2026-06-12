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

type DeleteDepositAddressLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteDepositAddressLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteDepositAddressLogic {
	return &DeleteDepositAddressLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *DeleteDepositAddressLogic) DeleteDepositAddress(in *pb.DeleteDepositAddressRequest) (*pb.DeleteDepositAddressResponse, error) {
	if in == nil || in.Id <= 0 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid id", map[string]string{"id": "required"})
	}
	if l.svcCtx.DB == nil || l.svcCtx.WalletDepositAddressRepo == nil || l.svcCtx.WalletDepositRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	current, ok := admininterceptor.GetCurrentAdmin(l.ctx)
	if !ok || current == nil {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}

	addr, err := l.svcCtx.WalletDepositAddressRepo.FindByID(l.ctx, in.Id)
	if err != nil || addr == nil {
		return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "NOT_FOUND", "deposit address not found", nil)
	}

	pendingCount, err := l.svcCtx.WalletDepositRepo.CountByDepositAddressAndStatuses(l.ctx, addr.Address, []string{"pending", "confirmed"})
	if err != nil {
		l.Logger.Errorf("count pending deposits by address failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}
	if pendingCount > 0 {
		return nil, errx.New(codes.FailedPrecondition, 409, errx.CodeConflict, "ADDRESS_IN_USE", "deposit address has pending deposits", map[string]string{
			"address": "has_pending_deposits",
		})
	}

	now := time.Now()
	ip := middleware.GetClientIP(l.ctx)
	ua := middleware.GetUserAgent(l.ctx)

	if err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		addrRepo := l.svcCtx.WalletDepositAddressRepo.WithTx(tx)
		auditRepo := repository.NewAdminAuditLogRepository(tx)

		if err := addrRepo.SoftDelete(l.ctx, in.Id, now); err != nil {
			return err
		}

		detailsBytes, _ := json.Marshal(map[string]interface{}{
			"user_id":    addr.UserID,
			"chain_code": addr.ChainCode,
			"address":    maskAddress(addr.Address),
			"reason":     strings.TrimSpace(in.Reason),
		})
		_ = auditRepo.CreateLog(l.ctx, &model.AdminAuditLogModel{
			AdminID:     current.ID,
			Action:      "wallet.deposit_address.delete",
			TargetType:  "wallet_deposit_address",
			TargetID:    maskAddress(addr.Address),
			Description: "删除充值地址: " + maskAddress(addr.Address),
			Details:     detailsBytes,
			IP:          ip,
			UserAgent:   ua,
		})
		return nil
	}); err != nil {
		l.Logger.Errorf("delete deposit address failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "delete deposit address failed", nil)
	}

	l.svcCtx.PublishAddressMonitorEvent(l.ctx, mq.AddressMonitorEvent{
		Action:  mq.AddressMonitorActionRemove,
		Source:  mq.AddressMonitorSourceDeposit,
		Chain:   strings.TrimSpace(addr.ChainCode),
		Address: strings.TrimSpace(addr.Address),
		Reason:  "admin.deposit_address.deleted",
		Metadata: map[string]string{
			"deposit_address_id": fmt.Sprintf("%d", addr.ID),
			"user_id":            fmt.Sprintf("%d", addr.UserID),
		},
	})

	return &pb.DeleteDepositAddressResponse{
		Success:   true,
		Message:   "ok",
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
