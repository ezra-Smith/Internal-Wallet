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

type BatchCreateBlacklistAddressesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewBatchCreateBlacklistAddressesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BatchCreateBlacklistAddressesLogic {
	return &BatchCreateBlacklistAddressesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *BatchCreateBlacklistAddressesLogic) BatchCreateBlacklistAddresses(in *pb.BatchCreateBlacklistAddressesRequest) (*pb.BatchCreateBlacklistAddressesResponse, error) {
	if in == nil {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid request", nil)
	}
	if len(in.Addresses) == 0 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "addresses required", map[string]string{"addresses": "required"})
	}
	if l.svcCtx == nil || l.svcCtx.DB == nil || l.svcCtx.BlacklistAddressRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}
	current, ok := admininterceptor.GetCurrentAdmin(l.ctx)
	if !ok || current == nil {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}

	now := time.Now()
	actor := strings.TrimSpace(current.Username)
	if actor == "" {
		actor = "admin"
	}

	type candidate struct {
		Address       string
		Network       string
		RiskLevel     string
		Source        string
		Reason        string
		MonitorStatus string
	}

	ignoreExisting := in.IgnoreExisting
	total := int32(len(in.Addresses))
	failedResults := make([]*pb.BatchCreateBlacklistAddressResult, 0)
	candidates := make([]candidate, 0, len(in.Addresses))

	seen := make(map[string]struct{}, len(in.Addresses))
	for _, a := range in.Addresses {
		if a == nil {
			failedResults = append(failedResults, &pb.BatchCreateBlacklistAddressResult{
				Status: "failed",
				Error:  "invalid row",
			})
			continue
		}

		address := strings.TrimSpace(a.Address)
		network := normalizeLower(a.Network)
		riskLevel := normalizeLower(a.RiskLevel)
		source := normalizeLower(a.Source)
		reason := strings.TrimSpace(a.Reason)
		monitorStatus := normalizeLower(a.MonitorStatus)
		if monitorStatus == "" {
			monitorStatus = "active"
		}

		// Per-row validation (strict, blocks entire import).
		if address == "" {
			failedResults = append(failedResults, &pb.BatchCreateBlacklistAddressResult{
				Address: address,
				Network: network,
				Status:  "failed",
				Error:   "address required",
			})
			continue
		}
		if network == "" {
			failedResults = append(failedResults, &pb.BatchCreateBlacklistAddressResult{
				Address: address,
				Network: network,
				Status:  "failed",
				Error:   "network required",
			})
			continue
		}
		if !validateBlacklistNetwork(network) {
			failedResults = append(failedResults, &pb.BatchCreateBlacklistAddressResult{
				Address: address,
				Network: network,
				Status:  "failed",
				Error:   "invalid network",
			})
			continue
		}
		if riskLevel == "" || !validateRiskLevel(riskLevel) {
			failedResults = append(failedResults, &pb.BatchCreateBlacklistAddressResult{
				Address: address,
				Network: network,
				Status:  "failed",
				Error:   "invalid risk_level",
			})
			continue
		}
		if source == "" || !validateBlacklistSource(source) {
			failedResults = append(failedResults, &pb.BatchCreateBlacklistAddressResult{
				Address: address,
				Network: network,
				Status:  "failed",
				Error:   "invalid source",
			})
			continue
		}
		if reason == "" {
			failedResults = append(failedResults, &pb.BatchCreateBlacklistAddressResult{
				Address: address,
				Network: network,
				Status:  "failed",
				Error:   "reason required",
			})
			continue
		}
		if !validateBlacklistMonitorStatus(monitorStatus) {
			failedResults = append(failedResults, &pb.BatchCreateBlacklistAddressResult{
				Address: address,
				Network: network,
				Status:  "failed",
				Error:   "invalid monitor_status",
			})
			continue
		}

		key := network + "#" + strings.ToLower(address)
		if _, ok := seen[key]; ok {
			failedResults = append(failedResults, &pb.BatchCreateBlacklistAddressResult{
				Address: address,
				Network: network,
				Status:  "failed",
				Error:   "duplicate in request",
			})
			continue
		}
		seen[key] = struct{}{}

		candidates = append(candidates, candidate{
			Address:       address,
			Network:       network,
			RiskLevel:     riskLevel,
			Source:        source,
			Reason:        reason,
			MonitorStatus: monitorStatus,
		})
	}

	// Any validation error blocks the whole operation.
	if len(failedResults) > 0 {
		failed := int32(len(failedResults))
		return &pb.BatchCreateBlacklistAddressesResponse{
			Success: false,
			Message: "validation failed",
			Data: &pb.BatchCreateBlacklistAddressesData{
				Total:     total,
				Succeeded: 0,
				Skipped:   0,
				Failed:    failed,
				Results:   failedResults,
			},
			RequestId: resp.RequestID(l.ctx),
			Timestamp: resp.Timestamp(),
		}, nil
	}

	repo := l.svcCtx.BlacklistAddressRepo
	db := repo.GetDB().WithContext(l.ctx)

	// Pre-check existing active rows to enforce "all-or-nothing".
	existing := make(map[string]struct{})
	byNetwork := make(map[string][]string)
	for _, c := range candidates {
		byNetwork[c.Network] = append(byNetwork[c.Network], c.Address)
	}
	for network, addrs := range byNetwork {
		// De-dup addresses per network.
		uniq := make(map[string]struct{}, len(addrs))
		list := make([]string, 0, len(addrs))
		for _, a := range addrs {
			k := strings.ToLower(strings.TrimSpace(a))
			if k == "" {
				continue
			}
			if _, ok := uniq[k]; ok {
				continue
			}
			uniq[k] = struct{}{}
			list = append(list, a)
		}
		if len(list) == 0 {
			continue
		}
		var rows []*model.BlacklistAddressModel
		if err := db.Model(&model.BlacklistAddressModel{}).
			Select("network, address").
			Where("deleted_at IS NULL").
			Where("network = ?", network).
			Where("address IN ?", list).
			Find(&rows).Error; err != nil {
			return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "query failed", nil)
		}
		for _, r := range rows {
			if r == nil {
				continue
			}
			existing[normalizeLower(r.Network)+"#"+strings.ToLower(strings.TrimSpace(r.Address))] = struct{}{}
		}
	}

	toInsert := make([]candidate, 0, len(candidates))
	results := make([]*pb.BatchCreateBlacklistAddressResult, 0, len(candidates))
	var skipped int32
	for _, c := range candidates {
		key := c.Network + "#" + strings.ToLower(c.Address)
		if _, ok := existing[key]; ok {
			if ignoreExisting {
				skipped++
				results = append(results, &pb.BatchCreateBlacklistAddressResult{
					Address: c.Address,
					Network: c.Network,
					Status:  "skipped",
					Error:   "",
				})
				continue
			}
			failedResults = append(failedResults, &pb.BatchCreateBlacklistAddressResult{
				Address: c.Address,
				Network: c.Network,
				Status:  "failed",
				Error:   "address already exists",
			})
			continue
		}
		toInsert = append(toInsert, c)
	}

	if len(failedResults) > 0 {
		failed := int32(len(failedResults))
		return &pb.BatchCreateBlacklistAddressesResponse{
			Success: false,
			Message: "validation failed",
			Data: &pb.BatchCreateBlacklistAddressesData{
				Total:     total,
				Succeeded: 0,
				Skipped:   0,
				Failed:    failed,
				Results:   failedResults,
			},
			RequestId: resp.RequestID(l.ctx),
			Timestamp: resp.Timestamp(),
		}, nil
	}

	// Insert all new rows in one transaction (all-or-nothing).
	if err := l.svcCtx.DB.WithContext(l.ctx).Transaction(func(tx *gorm.DB) error {
		txRepo := repo.WithTx(tx)
		for _, c := range toInsert {
			reasonPtr := c.Reason
			m := &model.BlacklistAddressModel{
				Address:          c.Address,
				Network:          c.Network,
				RiskLevel:        c.RiskLevel,
				Source:           c.Source,
				Reason:           &reasonPtr,
				MonitorStatus:    c.MonitorStatus,
				HitCount:         0,
				CreatedByAdminID: current.ID,
				CreatedBy:        actor,
				CreatedAt:        &now,
				UpdatedAt:        &now,
			}
			if err := txRepo.Create(l.ctx, m); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		if isMySQLDuplicate(err) {
			return &pb.BatchCreateBlacklistAddressesResponse{
				Success: false,
				Message: "address already exists",
				Data: &pb.BatchCreateBlacklistAddressesData{
					Total:     total,
					Succeeded: 0,
					Skipped:   0,
					Failed:    total,
					Results: []*pb.BatchCreateBlacklistAddressResult{
						{Status: "failed", Error: "address already exists"},
					},
				},
				RequestId: resp.RequestID(l.ctx),
				Timestamp: resp.Timestamp(),
			}, nil
		}
		l.Logger.Errorf("batch create blacklist addresses failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "create failed", nil)
	}

	succeeded := int32(len(toInsert))
	for _, c := range toInsert {
		results = append(results, &pb.BatchCreateBlacklistAddressResult{
			Address: c.Address,
			Network: c.Network,
			Status:  "success",
			Error:   "",
		})
	}

	failed := int32(0)

	// Best-effort audit log
	ip := middleware.GetClientIP(l.ctx)
	ua := middleware.GetUserAgent(l.ctx)
	detailsBytes, _ := json.Marshal(map[string]interface{}{
		"total":           total,
		"succeeded":       succeeded,
		"skipped":         skipped,
		"failed":          failed,
		"ignore_existing": ignoreExisting,
	})
	_ = repository.NewAdminAuditLogRepository(l.svcCtx.DB).CreateLog(l.ctx, &model.AdminAuditLogModel{
		AdminID:     current.ID,
		Action:      "blacklist.address.batch_create",
		TargetType:  "blacklist_address",
		TargetID:    "",
		Description: "批量导入黑名单地址",
		Details:     detailsBytes,
		IP:          ip,
		UserAgent:   ua,
	})

	return &pb.BatchCreateBlacklistAddressesResponse{
		Success: true,
		Message: "ok",
		Data: &pb.BatchCreateBlacklistAddressesData{
			Total:     total,
			Succeeded: succeeded,
			Skipped:   skipped,
			Failed:    failed,
			Results:   results,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
