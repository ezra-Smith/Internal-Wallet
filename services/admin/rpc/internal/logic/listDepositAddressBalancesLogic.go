package logic

import (
	"context"
	"fmt"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	"internalwallet/services/admin/rpc/internal/repository"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
)

type ListDepositAddressBalancesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListDepositAddressBalancesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListDepositAddressBalancesLogic {
	return &ListDepositAddressBalancesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListDepositAddressBalancesLogic) ListDepositAddressBalances(in *pb.ListDepositAddressBalancesRequest) (*pb.ListDepositAddressBalancesResponse, error) {
	if in == nil {
		in = &pb.ListDepositAddressBalancesRequest{}
	}
	if l.svcCtx.DB == nil || l.svcCtx.WalletDepositAddressBalanceRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	// 构建筛选条件
	filter := &repository.DepositAddressBalanceFilter{
		UserID:    in.UserId,
		AssetCode: in.AssetCode,
		ChainCode: in.ChainCode,
		SortBy:    in.SortBy,
		SortOrder: in.SortOrder,
	}

	// 处理可选参数
	if in.NeedsSweep != nil {
		filter.NeedsSweep = in.NeedsSweep
	}
	if in.MinBalanceUsdRaw != nil {
		filter.MinBalanceUSDRaw = in.MinBalanceUsdRaw
	}
	if in.MaxBalanceUsdRaw != nil {
		filter.MaxBalanceUSDRaw = in.MaxBalanceUsdRaw
	}
	if in.SyncStatus != "" {
		filter.SyncStatus = in.SyncStatus
	}

	// 查询余额列表
	items, total, err := l.svcCtx.WalletDepositAddressBalanceRepo.List(l.ctx, filter, in.Page, in.PageSize)
	if err != nil {
		l.Logger.Errorf("list deposit address balances failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	// 查询关联的充值地址信息（用于获取address字段）
	addressMap := make(map[int64]string)
	for _, item := range items {
		if _, exists := addressMap[item.DepositAddressID]; !exists {
			addr, err := l.svcCtx.WalletDepositAddressRepo.FindByID(l.ctx, item.DepositAddressID)
			if err == nil && addr != nil {
				addressMap[item.DepositAddressID] = addr.Address
			}
		}
	}

	// 转换为 protobuf 格式
	respItems := make([]*pb.DepositAddressBalanceItem, 0, len(items))
	for _, it := range items {
		respItems = append(respItems, toPBDepositAddressBalanceItem(it, addressMap[it.DepositAddressID]))
	}

	// 分页信息
	p := calcPagination(in.Page, in.PageSize, total)

	return &pb.ListDepositAddressBalancesResponse{
		Success: true,
		Message: fmt.Sprintf("ok (%d)", total),
		Data: &pb.ListDepositAddressBalancesData{
			Balances: respItems,
		},
		Pagination: p,
		RequestId:  resp.RequestID(l.ctx),
		Timestamp:  resp.Timestamp(),
	}, nil
}
