package logic

import (
	"context"
	"fmt"
	"time"

	"internalwallet/common/utils"
	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/repository"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListVaultAddressesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListVaultAddressesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListVaultAddressesLogic {
	return &ListVaultAddressesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListVaultAddressesLogic) ListVaultAddresses(in *pb.ListVaultAddressesRequest) (*pb.ListVaultAddressesResponse, error) {
	// 1. 参数验证和默认值
	page := in.Page
	if page <= 0 {
		page = 1
	}
	pageSize := in.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	// 2. 构建筛选条件
	filter := &repository.VaultAddressFilter{
		NetworkID:   in.NetworkId,
		AddressType: in.AddressType,
		Status:      in.Status,
	}

	// 3. 查询地址列表
	addresses, total, err := l.svcCtx.VaultAddressRepo.List(l.ctx, filter, page, pageSize)
	if err != nil {
		logx.Errorf("Failed to list vault addresses: %v", err)
		return &pb.ListVaultAddressesResponse{
			Success:   false,
			Message:   "Failed to list addresses",
			RequestId: resp.RequestID(l.ctx),
			Timestamp: time.Now().Format(time.RFC3339),
		}, err
	}

	// 4. 查询关联的网络信息和余额信息
	networks, err := l.svcCtx.VaultNetworkRepo.List(l.ctx, "")
	if err != nil {
		logx.Errorf("Failed to list networks: %v", err)
	}
	networkMap := make(map[int64]string)
	for _, n := range networks {
		networkMap[n.ID] = n.Network
	}

	// 5. 查询管理员信息 (创建人)
	adminMap := make(map[int64]string)
	// 这里简化处理，实际应该批量查询管理员信息
	// 先跳过，后续优化

	// 6. 组装返回数据
	items := make([]*pb.VaultAddressItem, 0, len(addresses))
	for _, addr := range addresses {
		// 查询该地址的总余额和币种数量
		totalBalanceUSD, err := l.svcCtx.VaultAddressBalanceRepo.GetTotalBalanceUSD(l.ctx, addr.ID)
		if err != nil {
			logx.Errorf("Failed to get total balance for address %d: %v", addr.ID, err)
			totalBalanceUSD = 0
		}

		currenciesCount, err := l.svcCtx.VaultAddressBalanceRepo.GetCurrenciesCount(l.ctx, addr.ID)
		if err != nil {
			logx.Errorf("Failed to get currencies count for address %d: %v", addr.ID, err)
			currenciesCount = 0
		}

		// 查询该地址的最后同步时间
		balances, err := l.svcCtx.VaultAddressBalanceRepo.FindByAddressID(l.ctx, addr.ID)
		var lastSyncedAt string
		if err == nil && len(balances) > 0 {
			// 找最新的同步时间
			var latest *time.Time
			for _, b := range balances {
				if b.LastSyncedAt != nil {
					if latest == nil || b.LastSyncedAt.After(*latest) {
						latest = b.LastSyncedAt
					}
				}
			}
			if latest != nil {
				lastSyncedAt = latest.Format(time.RFC3339)
			}
		}

		// 获取创建人名称
		createdByName := ""
		if name, ok := adminMap[addr.CreatedBy]; ok {
			createdByName = name
		} else if addr.CreatedBy > 0 {
			// 查询管理员信息
			admin, err := l.svcCtx.AdminUserRepo.FindByID(l.ctx, addr.CreatedBy)
			if err == nil && admin != nil {
				createdByName = admin.Username
				adminMap[addr.CreatedBy] = createdByName
			}
		}

		item := &pb.VaultAddressItem{
			Id:              addr.ID,
			NetworkId:       addr.NetworkID,
			Network:         networkMap[addr.NetworkID],
			Address:         addr.Address,
			AddressType:     addr.AddressType,
			Label:           utils.StringPtrToString(addr.Label),
			Status:          addr.Status,
			IsActiveWallet:  addr.IsActiveWallet == 1,
			TotalBalanceUsd: centsToUSDString(totalBalanceUSD),
			CurrenciesCount: currenciesCount,
			LastSyncedAt:    lastSyncedAt,
			CreatedAt:       formatTimePtrFriendly(addr.CreatedAt),
			CreatedByName:   createdByName,
		}
		items = append(items, item)
	}

	// 7. 分页信息
	totalPages := int32((total + int64(pageSize) - 1) / int64(pageSize))
	pagination := &pb.Pagination{
		Page:       page,
		PageSize:   pageSize,
		Total:      total,
		TotalPages: totalPages,
		HasNext:    totalPages > 0 && page < totalPages,
		HasPrev:    page > 1 && totalPages > 0,
	}

	// 8. 返回结果
	return &pb.ListVaultAddressesResponse{
		Success: true,
		Message: fmt.Sprintf("Found %d addresses", total),
		Data: &pb.ListVaultAddressesData{
			Addresses:  items,
			Pagination: pagination,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: time.Now().Format(time.RFC3339),
	}, nil
}
