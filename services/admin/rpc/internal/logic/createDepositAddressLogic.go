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
	"gorm.io/gorm"
)

type CreateDepositAddressLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateDepositAddressLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateDepositAddressLogic {
	return &CreateDepositAddressLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateDepositAddressLogic) CreateDepositAddress(in *pb.CreateDepositAddressRequest) (*pb.CreateDepositAddressResponse, error) {
	if in == nil || in.UserId <= 0 || strings.TrimSpace(in.ChainCode) == "" || strings.TrimSpace(in.Address) == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid params", map[string]string{
			"user_id":    "required",
			"chain_code": "required",
			"address":    "required",
		})
	}
	if l.svcCtx.DB == nil || l.svcCtx.UserRepo == nil || l.svcCtx.ChainRepo == nil || l.svcCtx.WalletDepositAddressRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	current, ok := admininterceptor.GetCurrentAdmin(l.ctx)
	if !ok || current == nil {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}

	// Validate user exists
	if _, err := l.svcCtx.UserRepo.FindByID(l.ctx, in.UserId); err != nil {
		return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "USER_NOT_FOUND", "user not found", nil)
	}

	chainCode := normalizeCode(in.ChainCode)
	chains, err := l.svcCtx.ChainRepo.ListEnabled(l.ctx)
	if err != nil {
		l.Logger.Errorf("list enabled chains failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}
	chainEnabled := false
	for _, c := range chains {
		if c == nil {
			continue
		}
		if normalizeCode(c.Name) == chainCode {
			chainEnabled = true
			break
		}
	}
	if !chainEnabled {
		return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "CHAIN_NOT_FOUND", "chain not found", map[string]string{"chain_code": "not found"})
	}

	address := strings.TrimSpace(in.Address)
	if err := validateChainAddress(chainCodeToChainType(chainCode), address); err != nil {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_ADDRESS", "invalid address", map[string]string{"address": "invalid"})
	}

	status := strings.TrimSpace(in.Status)
	if status == "" {
		status = "active"
	}
	if !validateDepositAddressStatus(status) {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_STATUS", "invalid status", map[string]string{"status": "invalid"})
	}

	// Prevent duplicate active address for same user+chain
	if status == "active" {
		if _, err := l.svcCtx.WalletDepositAddressRepo.FindActiveByUserChain(l.ctx, in.UserId, chainCode); err == nil {
			return nil, errx.New(codes.AlreadyExists, 409, errx.CodeConflict, "ACTIVE_ADDRESS_EXISTS", "active address already exists for user+token", map[string]string{
				"user_id":    "exists",
				"chain_code": "exists",
			})
		}
	}
	// Prevent duplicate address string (across users)
	if _, err := l.svcCtx.WalletDepositAddressRepo.FindByAddress(l.ctx, address); err == nil {
		return nil, errx.New(codes.AlreadyExists, 409, errx.CodeConflict, "ADDRESS_EXISTS", "address already exists", map[string]string{"address": "exists"})
	}
	now := time.Now()
	ip := middleware.GetClientIP(l.ctx)
	ua := middleware.GetUserAgent(l.ctx)

	var created *model.WalletDepositAddressModel
	if err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		addrRepo := l.svcCtx.WalletDepositAddressRepo.WithTx(tx)
		auditRepo := repository.NewAdminAuditLogRepository(tx)

		isDefault := false
		if status == "active" {
			isDefault = true
		}
		m := &model.WalletDepositAddressModel{
			UserID:    in.UserId,
			ChainCode: chainCode,
			Address:   address,
			Status:    status,
			Memo:      strPtrOrNilTrim(in.Memo),
			IsDefault: isDefault,
			// For admin-created user deposit addresses (chain-level), keep derivation_change at 0 by default.
			DerivationChange: 0,
			CreatedAt:        &now,
			UpdatedAt:        &now,
		}
		if err := addrRepo.Create(l.ctx, m); err != nil {
			return err
		}
		created = m

		detailsBytes, _ := json.Marshal(map[string]interface{}{
			"user_id":    in.UserId,
			"chain_code": chainCode,
			"address":    maskAddress(address),
			"status":     status,
		})
		_ = auditRepo.CreateLog(l.ctx, &model.AdminAuditLogModel{
			AdminID:     current.ID,
			Action:      "wallet.deposit_address.create",
			TargetType:  "wallet_deposit_address",
			TargetID:    maskAddress(address),
			Description: "创建充值地址: " + maskAddress(address),
			Details:     detailsBytes,
			IP:          ip,
			UserAgent:   ua,
		})
		return nil
	}); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			return nil, errx.New(codes.AlreadyExists, 409, errx.CodeConflict, "CONFLICT", "duplicate address", map[string]string{"address": "exists"})
		}
		l.Logger.Errorf("create deposit address failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "create deposit address failed", nil)
	}

	return &pb.CreateDepositAddressResponse{
		Success: true,
		Message: "ok",
		Data: &pb.CreateDepositAddressData{
			Address: toPBDepositAddressItem(created),
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
