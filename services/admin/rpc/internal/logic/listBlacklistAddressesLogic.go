package logic

import (
	"context"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	"internalwallet/services/admin/rpc/internal/repository"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
)

type ListBlacklistAddressesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListBlacklistAddressesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListBlacklistAddressesLogic {
	return &ListBlacklistAddressesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ==================== Blacklist (Sensitive Addresses) ====================
func (l *ListBlacklistAddressesLogic) ListBlacklistAddresses(in *pb.ListBlacklistAddressesRequest) (*pb.ListBlacklistAddressesResponse, error) {
	if in == nil {
		in = &pb.ListBlacklistAddressesRequest{}
	}
	if l.svcCtx == nil || l.svcCtx.BlacklistAddressRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	f := repository.BlacklistAddressFilter{
		Keyword:       strings.TrimSpace(in.Keyword),
		RiskLevel:     normalizeAllFilter(normalizeLower(in.RiskLevel)),
		Network:       normalizeAllFilter(normalizeLower(in.Network)),
		MonitorStatus: normalizeAllFilter(normalizeLower(in.MonitorStatus)),
	}
	if f.RiskLevel != "" && !validateRiskLevel(f.RiskLevel) {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_RISK_LEVEL", "invalid risk_level", map[string]string{"risk_level": "invalid"})
	}
	if f.Network != "" && !validateBlacklistNetwork(f.Network) {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_NETWORK", "invalid network", map[string]string{"network": "invalid"})
	}
	if f.MonitorStatus != "" && !validateBlacklistMonitorStatus(f.MonitorStatus) {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_MONITOR_STATUS", "invalid monitor_status", map[string]string{"monitor_status": "invalid"})
	}

	list, total, err := l.svcCtx.BlacklistAddressRepo.List(l.ctx, in.Page, in.PageSize, f)
	if err != nil {
		l.Logger.Errorf("list blacklist addresses failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "query failed", nil)
	}
	stats, err := l.svcCtx.BlacklistAddressRepo.Stats(l.ctx, f)
	if err != nil {
		l.Logger.Errorf("blacklist stats failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "query failed", nil)
	}

	items := make([]*pb.BlacklistAddressItem, 0, len(list))
	for _, m := range list {
		items = append(items, toPBBlacklistAddressItem(m))
	}

	ratio := 0.0
	if stats.Total > 0 {
		ratio = float64(stats.HighRisk) * 100.0 / float64(stats.Total)
	}

	return &pb.ListBlacklistAddressesResponse{
		Success: true,
		Message: "ok",
		Data: &pb.ListBlacklistAddressesData{
			Addresses: items,
			Stats: &pb.BlacklistAddressStats{
				Total:         stats.Total,
				HighRisk:      stats.HighRisk,
				HighRiskRatio: ratio,
				Monitoring:    stats.Monitoring,
				TotalHits:     stats.TotalHits,
			},
		},
		Pagination: calcPagination(in.Page, in.PageSize, total),
		RequestId:  resp.RequestID(l.ctx),
		Timestamp:  resp.Timestamp(),
	}, nil
}
