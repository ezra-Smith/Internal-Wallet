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

type CreateBlacklistAddressLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateBlacklistAddressLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateBlacklistAddressLogic {
	return &CreateBlacklistAddressLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateBlacklistAddressLogic) CreateBlacklistAddress(in *pb.CreateBlacklistAddressRequest) (*pb.CreateBlacklistAddressResponse, error) {
	if in == nil || strings.TrimSpace(in.Address) == "" || strings.TrimSpace(in.Network) == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "address/network required", map[string]string{
			"address": "required",
			"network": "required",
		})
	}
	if l.svcCtx == nil || l.svcCtx.DB == nil || l.svcCtx.BlacklistAddressRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}
	current, ok := admininterceptor.GetCurrentAdmin(l.ctx)
	if !ok || current == nil {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}

	address := strings.TrimSpace(in.Address)
	network := normalizeLower(in.Network)
	if !validateBlacklistNetwork(network) {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_NETWORK", "invalid network", map[string]string{"network": "invalid"})
	}

	riskLevel := normalizeLower(in.RiskLevel)
	if riskLevel == "" || !validateRiskLevel(riskLevel) {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_RISK_LEVEL", "invalid risk_level", map[string]string{"risk_level": "invalid"})
	}

	source := normalizeLower(in.Source)
	if source == "" || !validateBlacklistSource(source) {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_SOURCE", "invalid source", map[string]string{"source": "invalid"})
	}

	reason := strings.TrimSpace(in.Reason)
	if reason == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "reason required", map[string]string{"reason": "required"})
	}

	monitorStatus := normalizeLower(in.MonitorStatus)
	if monitorStatus == "" {
		monitorStatus = "active"
	}
	if !validateBlacklistMonitorStatus(monitorStatus) {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_MONITOR_STATUS", "invalid monitor_status", map[string]string{"monitor_status": "invalid"})
	}

	now := time.Now()
	ip := middleware.GetClientIP(l.ctx)
	ua := middleware.GetUserAgent(l.ctx)
	actor := strings.TrimSpace(current.Username)
	if actor == "" {
		actor = "admin"
	}

	var created *model.BlacklistAddressModel
	if err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		repo := l.svcCtx.BlacklistAddressRepo.WithTx(tx)
		auditRepo := repository.NewAdminAuditLogRepository(tx)

		reasonPtr := reason
		m := &model.BlacklistAddressModel{
			Address:          address,
			Network:          network,
			RiskLevel:        riskLevel,
			Source:           source,
			Reason:           &reasonPtr,
			MonitorStatus:    monitorStatus,
			HitCount:         0,
			CreatedByAdminID: current.ID,
			CreatedBy:        actor,
			CreatedAt:        &now,
			UpdatedAt:        &now,
		}
		if err := repo.Create(l.ctx, m); err != nil {
			return err
		}
		created = m

		detailsBytes, _ := json.Marshal(map[string]interface{}{
			"address":        maskAddress(address),
			"network":        network,
			"risk_level":     riskLevel,
			"source":         source,
			"monitor_status": monitorStatus,
			"reason":         reason,
		})
		_ = auditRepo.CreateLog(l.ctx, &model.AdminAuditLogModel{
			AdminID:     current.ID,
			Action:      "blacklist.address.create",
			TargetType:  "blacklist_address",
			TargetID:    maskAddress(address),
			Description: "创建黑名单地址: " + maskAddress(address),
			Details:     detailsBytes,
			IP:          ip,
			UserAgent:   ua,
		})
		return nil
	}); err != nil {
		if isMySQLDuplicate(err) {
			return nil, errx.New(codes.AlreadyExists, 409, errx.CodeConflict, "ADDRESS_EXISTS", "address already exists", map[string]string{"address": "exists"})
		}
		l.Logger.Errorf("create blacklist address failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "create failed", nil)
	}

	return &pb.CreateBlacklistAddressResponse{
		Success: true,
		Message: "ok",
		Data: &pb.CreateBlacklistAddressData{
			Address: toPBBlacklistAddressItem(created),
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
