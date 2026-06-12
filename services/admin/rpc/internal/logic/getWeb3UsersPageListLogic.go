package logic

import (
	"context"
	"fmt"
	"strings"
	"time"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	"internalwallet/services/admin/rpc/internal/repository"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
)

type GetWeb3UsersPageListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetWeb3UsersPageListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetWeb3UsersPageListLogic {
	return &GetWeb3UsersPageListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetWeb3UsersPageListLogic) GetWeb3UsersPageList(in *pb.GetWeb3UsersPageListRequest) (*pb.GetWeb3UsersPageListResponse, error) {
	if l.svcCtx.DB == nil || l.svcCtx.Web3UserRepo == nil || l.svcCtx.Web3UserAddressRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

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

	// Build filter from request
	filter := repository.Web3UserListFilter{
		DeviceID:       strings.TrimSpace(in.DeviceId),
		Platform:       strings.TrimSpace(in.Platform),
		OsVersion:      strings.TrimSpace(in.OsVersion),
		AppVersion:     strings.TrimSpace(in.AppVersion),
		DeviceModel:    strings.TrimSpace(in.DeviceModel),
		DeviceName:     strings.TrimSpace(in.DeviceName),
		Locale:         strings.TrimSpace(in.Locale),
		Network:        strings.TrimSpace(in.Network),
		AddressStatus:  strings.TrimSpace(in.AddressStatus),
		PrimaryAddress: strings.TrimSpace(in.PrimaryAddress),
		Keyword:        strings.TrimSpace(in.Keyword),
		SortBy:         strings.TrimSpace(in.SortBy),
		SortOrder:      strings.TrimSpace(in.SortOrder),
	}

	// Boolean filters (using wrapper to distinguish between false and unset)
	if in.TwoFactorEnabled != nil {
		val := in.TwoFactorEnabled.Value
		filter.TwoFactorEnabled = &val
	}
	if in.BiometricEnabled != nil {
		val := in.BiometricEnabled.Value
		filter.BiometricEnabled = &val
	}
	if in.HasTradePassword != nil {
		val := in.HasTradePassword.Value
		filter.HasTradePassword = &val
	}

	// Wallet count range
	if in.WalletCountMin > 0 {
		filter.WalletCountMin = &in.WalletCountMin
	}
	if in.WalletCountMax > 0 {
		filter.WalletCountMax = &in.WalletCountMax
	}

	// Time range filters
	if createdFrom := strings.TrimSpace(in.CreatedFrom); createdFrom != "" {
		if t, err := time.Parse("2006-01-02", createdFrom); err == nil {
			filter.CreatedFrom = &t
		}
	}
	if createdTo := strings.TrimSpace(in.CreatedTo); createdTo != "" {
		if t, err := time.Parse("2006-01-02", createdTo); err == nil {
			// Set to end of day
			t = t.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
			filter.CreatedTo = &t
		}
	}
	if lastActiveFrom := strings.TrimSpace(in.LastActiveFrom); lastActiveFrom != "" {
		if t, err := time.Parse("2006-01-02", lastActiveFrom); err == nil {
			filter.LastActiveFrom = &t
		}
	}
	if lastActiveTo := strings.TrimSpace(in.LastActiveTo); lastActiveTo != "" {
		if t, err := time.Parse("2006-01-02", lastActiveTo); err == nil {
			t = t.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
			filter.LastActiveTo = &t
		}
	}

	// Query users with filter
	users, total, err := l.svcCtx.Web3UserRepo.ListWithFilter(l.ctx, page, pageSize, filter)
	if err != nil {
		l.Logger.Errorf("list web3 users failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "query failed", nil)
	}

	if len(users) == 0 {
		return &pb.GetWeb3UsersPageListResponse{
			Success: true,
			Message: "ok",
			Data: &pb.GetWeb3UsersPageListData{
				Items:      []*pb.Web3UserPageListItem{},
				Pagination: calcPagination(page, pageSize, 0),
			},
			RequestId: resp.RequestID(l.ctx),
			Timestamp: resp.Timestamp(),
		}, nil
	}

	// Collect user IDs for aggregation
	userIDs := make([]int64, len(users))
	for i, u := range users {
		userIDs[i] = u.ID
	}

	// Aggregate address info
	aggMap, err := l.svcCtx.Web3UserAddressRepo.AggregateByUserIDs(l.ctx, userIDs)
	if err != nil {
		l.Logger.Errorf("aggregate addresses failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "aggregate failed", nil)
	}

	// Build response items
	items := make([]*pb.Web3UserPageListItem, 0, len(users))
	for _, u := range users {
		if u == nil {
			continue
		}

		agg, exists := aggMap[u.ID]
		var walletCount int32
		var networks []string
		var primaryAddress string
		var isPrimaryBlacklisted bool

		if exists {
			walletCount = int32(agg.WalletCount)
			if agg.Networks != "" {
				networks = strings.Split(agg.Networks, ",")
			}
			primaryAddress = agg.PrimaryAddress
			isPrimaryBlacklisted = (agg.IsPrimaryBlacklisted == 1)
		}

		items = append(items, &pb.Web3UserPageListItem{
			Id:                   fmt.Sprintf("web3-%d", u.ID),
			DeviceId:             u.DeviceID,
			Platform:             u.Platform,
			OsVersion:            u.OsVersion,
			AppVersion:           u.AppVersion,
			WalletCount:          walletCount,
			Networks:             networks,
			PrimaryAddress:       primaryAddress,
			TwoFactorEnabled:     u.TwoFactorEnabled,
			BiometricEnabled:     u.BiometricEnabled,
			IsPrimaryBlacklisted: isPrimaryBlacklisted,
			LastActiveAt:         formatTimePtr(u.LastActiveAt),
			CreatedAt:            formatTimePtr(u.CreatedAt),
		})
	}

	return &pb.GetWeb3UsersPageListResponse{
		Success: true,
		Message: "ok",
		Data: &pb.GetWeb3UsersPageListData{
			Items:      items,
			Pagination: calcPagination(page, pageSize, total),
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
