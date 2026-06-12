package logic

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/repository"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/structpb"
)

type GetAuditLogsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetAuditLogsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetAuditLogsLogic {
	return &GetAuditLogsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ==================== Audit Logs ====================
func (l *GetAuditLogsLogic) GetAuditLogs(in *pb.GetAuditLogsRequest) (*pb.GetAuditLogsResponse, error) {
	if in == nil {
		in = &pb.GetAuditLogsRequest{}
	}
	if l.svcCtx.AdminAuditLogRepo == nil || l.svcCtx.AdminUserRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "service not ready", nil)
	}

	var operatorID int64
	operator := strings.TrimSpace(in.Operator)
	if operator != "" {
		if id, err := resp.ParseAdminID(operator); err == nil {
			operatorID = id
		} else {
			// 尝试按 username 精确匹配
			var u model.AdminUserModel
			qErr := l.svcCtx.AdminUserRepo.GetDB().WithContext(l.ctx).
				Where("username = ?", operator).
				First(&u).Error
			if qErr == nil {
				operatorID = u.ID
			} else {
				// 操作人不存在：直接返回空结果
				p := calcPagination(in.Page, in.PageSize, 0)
				return &pb.GetAuditLogsResponse{
					Success: true,
					Message: "ok",
					Data: &pb.GetAuditLogsData{
						Logs: []*pb.AuditLogItem{},
					},
					Pagination: p,
					RequestId:  resp.RequestID(l.ctx),
					Timestamp:  resp.Timestamp(),
				}, nil
			}
		}
	}

	var dateFrom *time.Time
	var dateTo *time.Time
	if strings.TrimSpace(in.DateFrom) != "" {
		t, err := time.Parse(time.RFC3339, strings.TrimSpace(in.DateFrom))
		if err != nil {
			return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid date_from", map[string]string{"date_from": "invalid RFC3339"})
		}
		dateFrom = &t
	}
	if strings.TrimSpace(in.DateTo) != "" {
		t, err := time.Parse(time.RFC3339, strings.TrimSpace(in.DateTo))
		if err != nil {
			return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid date_to", map[string]string{"date_to": "invalid RFC3339"})
		}
		dateTo = &t
	}

	var successFilter *bool
	if in.Success != nil {
		v := in.Success.Value
		successFilter = &v
	}
	module := strings.TrimSpace(in.Module)
	keyword := strings.TrimSpace(in.Keyword)

	role := strings.TrimSpace(in.Role)
	ip := strings.TrimSpace(in.Ip)
	requestID := strings.TrimSpace(in.RequestId)
	httpMethod := strings.TrimSpace(in.HttpMethod)
	httpPath := strings.TrimSpace(in.HttpPath)
	rpcMethod := strings.TrimSpace(in.RpcMethod)

	var durationFrom *int32
	var durationTo *int32
	if in.DurationMsFrom > 0 {
		v := in.DurationMsFrom
		durationFrom = &v
	}
	if in.DurationMsTo > 0 {
		v := in.DurationMsTo
		durationTo = &v
	}
	if durationFrom != nil && durationTo != nil && *durationFrom > *durationTo {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid duration_ms range", map[string]string{"duration_ms": "from > to"})
	}

	logs, total, stats, err := l.svcCtx.AdminAuditLogRepo.List(l.ctx, in.Page, in.PageSize, repository.AuditLogListFilter{
		OperatorID: operatorID,
		Role:       role,
		Action:     strings.TrimSpace(in.Action),
		TargetType: strings.TrimSpace(in.TargetType),
		TargetID:   strings.TrimSpace(in.TargetId),
		Module:     module,

		IP:         ip,
		RequestID:  requestID,
		HttpMethod: httpMethod,
		HttpPath:   httpPath,
		RpcMethod:  rpcMethod,

		DurationMsFrom: durationFrom,
		DurationMsTo:   durationTo,

		Keyword: keyword,
		Success: successFilter,

		DateFrom: dateFrom,
		DateTo:   dateTo,
	})
	if err != nil {
		l.Logger.Errorf("list audit logs failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	// 批量加载 operator 信息
	ids := make([]int64, 0, len(logs))
	uniq := map[int64]struct{}{}
	for _, it := range logs {
		if it == nil || it.AdminID <= 0 {
			continue
		}
		if _, ok := uniq[it.AdminID]; ok {
			continue
		}
		uniq[it.AdminID] = struct{}{}
		ids = append(ids, it.AdminID)
	}

	type opRow struct {
		ID       int64
		Username string
		Name     string
		Role     string
	}
	opMap := map[int64]opRow{}
	if len(ids) > 0 {
		var rows []opRow
		_ = l.svcCtx.AdminUserRepo.GetDB().WithContext(l.ctx).
			Model(&model.AdminUserModel{}).
			Select("id, username, name, role").
			Where("id IN (?)", ids).
			Find(&rows).Error
		for _, r := range rows {
			opMap[r.ID] = r
		}
	}

	items := make([]*pb.AuditLogItem, 0, len(logs))
	for _, it := range logs {
		if it == nil {
			continue
		}

		op := opMap[it.AdminID]
		var details *structpb.Struct
		if len(it.Details) > 0 {
			var m map[string]interface{}
			if err := json.Unmarshal(it.Details, &m); err == nil {
				if s, err := structpb.NewStruct(m); err == nil {
					details = s
				}
			}
		}

		items = append(items, &pb.AuditLogItem{
			Id: "audit-" + strconv.FormatInt(it.ID, 10),
			Operator: &pb.OperatorInfo{
				AdminId:  resp.AdminIDString(it.AdminID),
				Username: op.Username,
				Name:     op.Name,
				Role:     op.Role,
			},
			Action:      it.Action,
			TargetType:  it.TargetType,
			TargetId:    it.TargetID,
			Description: it.Description,
			Details:     details,
			Ip:          it.IP,
			UserAgent:   it.UserAgent,
			CreatedAt:   formatTime(it.CreatedAt),
			Success:     it.Success,
			GrpcCode:    it.GrpcCode,
			ErrorMessage: func() string {
				if strings.TrimSpace(it.ErrorMessage) == "" {
					return ""
				}
				return it.ErrorMessage
			}(),
			RequestId:      it.RequestID,
			RpcMethod:      it.RpcMethod,
			HttpMethod:     it.HttpMethod,
			HttpPath:       it.HttpPath,
			DurationMs:     it.DurationMs,
			OperatorEmail:  it.OperatorEmail,
			Module:         it.Module,
			IdempotencyKey: it.IdempotencyKey,
		})
	}

	p := calcPagination(in.Page, in.PageSize, total)
	if stats == nil {
		stats = &repository.AuditLogStats{Total: total}
	}

	return &pb.GetAuditLogsResponse{
		Success: true,
		Message: "ok",
		Data: &pb.GetAuditLogsData{
			Logs: items,
			Stats: &pb.AuditLogStats{
				Total:   stats.Total,
				Success: stats.Success,
				Failed:  stats.Failed,
				Today:   stats.Today,
			},
		},
		Pagination: p,
		RequestId:  resp.RequestID(l.ctx),
		Timestamp:  resp.Timestamp(),
	}, nil
}
