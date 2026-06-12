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

type ListDepositAddressesWithBalanceLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListDepositAddressesWithBalanceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListDepositAddressesWithBalanceLogic {
	return &ListDepositAddressesWithBalanceLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListDepositAddressesWithBalance 查询充值地址列表（带余额信息，用于管理后台）
func (l *ListDepositAddressesWithBalanceLogic) ListDepositAddressesWithBalance(in *pb.ListDepositAddressesWithBalanceRequest) (*pb.ListDepositAddressesWithBalanceResponse, error) {
	if in == nil {
		in = &pb.ListDepositAddressesWithBalanceRequest{}
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

	// 使用LEFT JOIN查询地址+余额（管理后台需要）
	items, total, err := l.svcCtx.WalletDepositAddressRepo.ListWithBalance(l.ctx, in.Page, in.PageSize, in.UserId, in.AssetCode, in.ChainCode, status, in.Address, sortBy, sortOrder)
	if err != nil {
		l.Logger.Errorf("list deposit addresses with balance failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	respItems := make([]*pb.WalletDepositAddressWithBalanceItem, 0, len(items))
	for _, it := range items {
		respItems = append(respItems, toPBDepositAddressWithBalanceItem(it))
	}
	p := calcPagination(in.Page, in.PageSize, total)

	return &pb.ListDepositAddressesWithBalanceResponse{
		Success: true,
		Message: fmt.Sprintf("ok (%d)", total),
		Data: &pb.ListDepositAddressesWithBalanceData{
			Addresses: respItems,
		},
		Pagination: p,
		RequestId:  resp.RequestID(l.ctx),
		Timestamp:  resp.Timestamp(),
	}, nil
}
