package logic

import (
	"context"
	"fmt"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListWeb3UserAddressesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListWeb3UserAddressesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListWeb3UserAddressesLogic {
	return &ListWeb3UserAddressesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListWeb3UserAddressesLogic) ListWeb3UserAddresses(in *pb.ListWeb3UserAddressesRequest) (*pb.ListWeb3UserAddressesResponse, error) {
	// 1. 参数验证和默认值
	page := in.Page
	if page <= 0 {
		page = 1
	}
	pageSize := in.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 500 {
		pageSize = 500
	}

	// 2. 查询地址列表
	addresses, total, err := l.svcCtx.Web3UserAddressRepo.List(l.ctx, page, pageSize, in.Network, in.Status)
	if err != nil {
		l.Logger.Errorf("Failed to list web3 user addresses: %v", err)
		return &pb.ListWeb3UserAddressesResponse{
			Success:   false,
			Message:   "Failed to list web3 user addresses",
			RequestId: resp.RequestID(l.ctx),
			Timestamp: resp.Timestamp(),
		}, err
	}

	// 3. 组装返回数据
	items := make([]*pb.Web3UserAddressItem, 0, len(addresses))
	for _, addr := range addresses {
		if addr == nil {
			continue
		}
		item := &pb.Web3UserAddressItem{
			UserId:        addr.Web3UserID,
			Network:       addr.Network,
			ChainId:       addr.ChainID,
			Address:       addr.Address,
			Enabled:       addr.Enabled,
			IsBlacklisted: addr.IsBlacklisted,
		}
		items = append(items, item)
	}

	// 4. 构建分页信息
	totalPages := int32(0)
	if pageSize > 0 {
		totalPages = int32((total + int64(pageSize) - 1) / int64(pageSize))
	}
	pagination := &pb.Pagination{
		Page:       page,
		PageSize:   pageSize,
		Total:      total,
		TotalPages: totalPages,
		HasNext:    page < totalPages,
		HasPrev:    page > 1,
	}

	return &pb.ListWeb3UserAddressesResponse{
		Success: true,
		Message: fmt.Sprintf("ok (%d addresses)", total),
		Data: &pb.ListWeb3UserAddressesData{
			Addresses:  items,
			Pagination: pagination,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
