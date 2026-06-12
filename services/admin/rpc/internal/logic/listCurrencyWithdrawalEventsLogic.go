package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	admininterceptor "internalwallet/services/admin/rpc/internal/interceptor"
	"internalwallet/services/admin/rpc/internal/repository"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/structpb"
)

type ListCurrencyWithdrawalEventsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListCurrencyWithdrawalEventsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListCurrencyWithdrawalEventsLogic {
	return &ListCurrencyWithdrawalEventsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListCurrencyWithdrawalEventsLogic) ListCurrencyWithdrawalEvents(in *pb.ListCurrencyWithdrawalEventsRequest) (*pb.ListCurrencyWithdrawalEventsResponse, error) {
	if in == nil || in.Id <= 0 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid id", map[string]string{"id": "required"})
	}
	if l.svcCtx.DB == nil || l.svcCtx.CurrencyWithdrawOrderEventRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}
	if _, ok := admininterceptor.GetCurrentAdmin(l.ctx); !ok {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}

	eventTypes := make([]string, 0, len(in.EventTypes))
	for _, v := range in.EventTypes {
		for _, p := range strings.Split(v, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				eventTypes = append(eventTypes, p)
			}
		}
	}
	actorTypes := make([]string, 0, len(in.ActorTypes))
	for _, v := range in.ActorTypes {
		for _, p := range strings.Split(v, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				actorTypes = append(actorTypes, p)
			}
		}
	}

	dateFrom, dateTo, err := parseDateFromTo(in.DateFrom, in.DateTo)
	if err != nil {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_DATE", "invalid date", map[string]string{"date_from": "invalid"})
	}

	f := repository.CurrencyWithdrawOrderEventListFilter{
		WithdrawOrderID: in.Id,
		EventTypes:      eventTypes,
		ActorTypes:      actorTypes,
		DateFrom:        dateFrom,
		DateTo:          dateTo,
	}
	items, total, err := l.svcCtx.CurrencyWithdrawOrderEventRepo.List(l.ctx, in.Page, in.PageSize, f)
	if err != nil {
		l.Logger.Errorf("list currency withdrawal events failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	out := make([]*pb.CurrencyWithdrawalEventItem, 0, len(items))
	for _, it := range items {
		if it == nil {
			continue
		}
		ip := ""
		if it.IP != nil {
			ip = strings.TrimSpace(*it.IP)
		}
		var details *structpb.Struct
		if len(it.Details) > 0 {
			var m map[string]interface{}
			if err := json.Unmarshal(it.Details, &m); err == nil {
				if s, err := structpb.NewStruct(m); err == nil {
					details = s
				}
			}
		}
		out = append(out, &pb.CurrencyWithdrawalEventItem{
			Id:              it.ID,
			WithdrawOrderId: it.WithdrawOrderID,
			EventType:       strings.TrimSpace(it.EventType),
			ActorType:       strings.TrimSpace(it.ActorType),
			ActorId:         it.ActorID,
			Ip:              ip,
			Summary:         strings.TrimSpace(it.Summary),
			Details:         details,
			CreatedAt:       formatTime(it.CreatedAt),
		})
	}
	p := calcPagination(in.Page, in.PageSize, total)
	return &pb.ListCurrencyWithdrawalEventsResponse{
		Success: true,
		Message: fmt.Sprintf("ok (%d)", total),
		Data: &pb.ListCurrencyWithdrawalEventsData{
			Events: out,
		},
		Pagination: p,
		RequestId:  resp.RequestID(l.ctx),
		Timestamp:  resp.Timestamp(),
	}, nil
}
