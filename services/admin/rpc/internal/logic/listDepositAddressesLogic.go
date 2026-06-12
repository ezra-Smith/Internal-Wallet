package logic

import (
	"context"
	"fmt"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
)

type ListDepositAddressesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListDepositAddressesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListDepositAddressesLogic {
	return &ListDepositAddressesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListDepositAddressesLogic) ListDepositAddresses(in *pb.ListDepositAddressesRequest) (*pb.ListDepositAddressesResponse, error) {
	if in == nil {
		in = &pb.ListDepositAddressesRequest{}
	}
	if l.svcCtx.DB == nil || l.svcCtx.WalletDepositAddressRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	status := strings.TrimSpace(in.Status)
	if status != "" && !validateDepositAddressStatus(status) {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_STATUS", "invalid status", map[string]string{"status": "invalid"})
	}
	sortBy := strings.TrimSpace(in.SortBy)
	if sortBy != "" && sortBy != "created_at" && sortBy != "id" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_SORT_BY", "invalid sort_by", map[string]string{"sort_by": "invalid"})
	}
	sortOrder := strings.TrimSpace(in.SortOrder)
	if sortOrder != "" && !strings.EqualFold(sortOrder, "asc") && !strings.EqualFold(sortOrder, "desc") {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_SORT_ORDER", "invalid sort_order", map[string]string{"sort_order": "invalid"})
	}

	// 查询地址列表（不带余额，轻量级查询）
	items, total, err := l.svcCtx.WalletDepositAddressRepo.List(l.ctx, in.Page, in.PageSize, in.UserId, in.ChainCode, status, in.Address, sortBy, sortOrder)
	if err != nil {
		l.Logger.Errorf("list deposit addresses failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	respItems := make([]*pb.WalletDepositAddressItem, 0, len(items))
	for _, it := range items {
		respItems = append(respItems, toPBDepositAddressItem(it))
	}
	p := calcPagination(in.Page, in.PageSize, total)

	return &pb.ListDepositAddressesResponse{
		Success: true,
		Message: fmt.Sprintf("ok (%d)", total),
		Data: &pb.ListDepositAddressesData{
			Addresses: respItems,
		},
		Pagination: p,
		RequestId:  resp.RequestID(l.ctx),
		Timestamp:  resp.Timestamp(),
	}, nil
}
