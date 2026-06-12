package logic

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
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
)

type ExportBlacklistAddressesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewExportBlacklistAddressesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ExportBlacklistAddressesLogic {
	return &ExportBlacklistAddressesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ExportBlacklistAddressesLogic) ExportBlacklistAddresses(in *pb.ExportBlacklistAddressesRequest) (*pb.ExportBlacklistAddressesResponse, error) {
	if in == nil {
		in = &pb.ExportBlacklistAddressesRequest{}
	}
	if l.svcCtx == nil || l.svcCtx.BlacklistAddressRepo == nil || l.svcCtx.DB == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}
	current, ok := admininterceptor.GetCurrentAdmin(l.ctx)
	if !ok || current == nil {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}

	format := strings.ToLower(strings.TrimSpace(in.Format))
	if format == "" {
		format = "csv"
	}
	if format != "csv" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_FORMAT", "invalid format", map[string]string{"format": "invalid"})
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

	const maxExportRows int64 = 5000
	const pageSize int32 = 200
	var (
		total int64
		list  []*model.BlacklistAddressModel
		page  int32 = 1
	)
	for {
		batch, t, err := l.svcCtx.BlacklistAddressRepo.List(l.ctx, page, pageSize, f)
		if err != nil {
			l.Logger.Errorf("export blacklist addresses query failed: %v", err)
			return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "query failed", nil)
		}
		if page == 1 {
			total = t
			if total > maxExportRows {
				return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "TOO_MANY_RECORDS", "too many records to export, please narrow filters", map[string]string{"filters": "too broad"})
			}
		}
		if len(batch) == 0 {
			break
		}
		list = append(list, batch...)
		if int64(len(list)) >= total {
			break
		}
		if int64(len(list)) >= maxExportRows {
			break
		}
		page++
	}

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{
		"id",
		"address",
		"network",
		"risk_level",
		"source",
		"monitor_status",
		"hit_count",
		"last_hit_at",
		"added_at",
		"added_by",
		"reason",
	})
	for _, m := range list {
		if m == nil {
			continue
		}
		_ = w.Write([]string{
			fmt.Sprintf("%d", m.ID),
			strings.TrimSpace(m.Address),
			strings.TrimSpace(m.Network),
			strings.TrimSpace(m.RiskLevel),
			strings.TrimSpace(m.Source),
			strings.TrimSpace(m.MonitorStatus),
			fmt.Sprintf("%d", m.HitCount),
			formatTimePtr(m.LastHitAt),
			formatTimePtr(m.CreatedAt),
			strings.TrimSpace(m.CreatedBy),
			derefString(m.Reason),
		})
	}
	w.Flush()
	if err := w.Error(); err != nil {
		l.Logger.Errorf("export blacklist addresses csv failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "export failed", nil)
	}

	fileName := fmt.Sprintf("blacklist_addresses_%s.%s", time.Now().UTC().Format("20060102_150405"), format)

	// Best-effort audit log
	ip := middleware.GetClientIP(l.ctx)
	ua := middleware.GetUserAgent(l.ctx)
	detailsBytes, _ := json.Marshal(map[string]interface{}{
		"format":         format,
		"rows":           total,
		"keyword":        f.Keyword,
		"risk_level":     f.RiskLevel,
		"network":        f.Network,
		"monitor_status": f.MonitorStatus,
	})
	_ = repository.NewAdminAuditLogRepository(l.svcCtx.DB).CreateLog(l.ctx, &model.AdminAuditLogModel{
		AdminID:     current.ID,
		Action:      "blacklist.address.export",
		TargetType:  "blacklist_address",
		TargetID:    "",
		Description: "导出黑名单地址列表",
		Details:     detailsBytes,
		IP:          ip,
		UserAgent:   ua,
	})

	return &pb.ExportBlacklistAddressesResponse{
		Success: true,
		Message: "ok",
		Data: &pb.ExportBlacklistAddressesData{
			Format:   format,
			FileName: fileName,
			Content:  buf.String(),
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
