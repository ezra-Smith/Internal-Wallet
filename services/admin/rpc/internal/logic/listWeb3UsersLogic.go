package logic

import (
	"context"
	"fmt"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	"internalwallet/services/admin/rpc/internal/repository"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
)

type ListWeb3UsersLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListWeb3UsersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListWeb3UsersLogic {
	return &ListWeb3UsersLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListWeb3UsersLogic) ListWeb3Users(in *pb.ListWeb3UsersRequest) (*pb.ListWeb3UsersResponse, error) {
	if in == nil {
		in = &pb.ListWeb3UsersRequest{}
	}
	if l.svcCtx.DB == nil || l.svcCtx.Web3UserRepo == nil || l.svcCtx.Web3UserAddressRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	addressStatus := strings.TrimSpace(in.AddressStatus)
	if addressStatus != "" && addressStatus != "normal" && addressStatus != "blacklisted" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_ADDRESS_STATUS", "invalid address_status", map[string]string{"address_status": "invalid"})
	}

	sortBy := strings.TrimSpace(in.SortBy)
	if sortBy != "" && sortBy != "created_at" && sortBy != "last_active_at" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_SORT_BY", "invalid sort_by", map[string]string{"sort_by": "invalid"})
	}
	sortOrder := strings.TrimSpace(in.SortOrder)
	if sortOrder != "" && !strings.EqualFold(sortOrder, "asc") && !strings.EqualFold(sortOrder, "desc") {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_SORT_ORDER", "invalid sort_order", map[string]string{"sort_order": "invalid"})
	}

	createdFrom, createdTo, err := parseDateFromTo(in.CreatedFrom, in.CreatedTo)
	if err != nil {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_DATE", "invalid date", map[string]string{"created_from": "invalid"})
	}

	var twoFactorEnabled *bool
	if in.TwoFactorEnabled != nil {
		v := in.TwoFactorEnabled.Value
		twoFactorEnabled = &v
	}
	var biometricEnabled *bool
	if in.BiometricEnabled != nil {
		v := in.BiometricEnabled.Value
		biometricEnabled = &v
	}

	f := repository.Web3UserListFilter{
		AddressStatus:    addressStatus,
		TwoFactorEnabled: twoFactorEnabled,
		BiometricEnabled: biometricEnabled,
		Network:          strings.TrimSpace(in.Network),
		Keyword:          strings.TrimSpace(in.Keyword),
		CreatedFrom:      createdFrom,
		CreatedTo:        createdTo,
		SortBy:           sortBy,
		SortOrder:        sortOrder,
	}

	items, total, err := l.svcCtx.Web3UserRepo.List(l.ctx, in.Page, in.PageSize, f)
	if err != nil {
		l.Logger.Errorf("list web3 users failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	userIDs := make([]int64, 0, len(items))
	for _, it := range items {
		if it == nil {
			continue
		}
		userIDs = append(userIDs, it.ID)
	}
	aggMap, err := l.svcCtx.Web3UserAddressRepo.AggregateByUserIDs(l.ctx, userIDs)
	if err != nil {
		l.Logger.Errorf("aggregate web3 user addresses failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	summaryCounts, err := l.svcCtx.Web3UserRepo.Summary(l.ctx, f)
	if err != nil {
		l.Logger.Errorf("web3 users summary failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	twoFactorRate := "0%"
	if summaryCounts.TotalDevices > 0 {
		r := decimal.NewFromInt(summaryCounts.TwoFactorEnabled).
			Div(decimal.NewFromInt(summaryCounts.TotalDevices)).
			Mul(decimal.NewFromInt(100))
		twoFactorRate = r.StringFixed(1) + "%"
	}

	respItems := make([]*pb.Web3UserListItem, 0, len(items))
	for _, it := range items {
		if it == nil {
			continue
		}
		agg := aggMap[it.ID]
		walletCount := int32(agg.WalletCount)
		if agg.WalletCount < 0 {
			walletCount = 0
		}
		if agg.WalletCount > int64(^uint32(0)>>1) {
			walletCount = int32(^uint32(0) >> 1)
		}
		respItems = append(respItems, &pb.Web3UserListItem{
			Id:       fmt.Sprintf("web3-%d", it.ID),
			DeviceId: it.DeviceID,
			DeviceInfo: &pb.Web3DeviceInfo{
				Platform:    it.Platform,
				OsVersion:   it.OsVersion,
				AppVersion:  it.AppVersion,
				DeviceModel: it.DeviceModel,
			},
			WalletCount:           walletCount,
			Networks:              parseCSV(agg.Networks),
			PrimaryAddress:        maskAddress(agg.PrimaryAddress),
			TotalBalanceUsd:       "0",
			TwoFactorEnabled:      it.TwoFactorEnabled,
			BiometricEnabled:      it.BiometricEnabled,
			HasBlacklistedAddress: agg.HasBlacklisted > 0,
			LastActiveAt:          formatTimePtr(it.LastActiveAt),
			CreatedAt:             formatTimePtr(it.CreatedAt),
		})
	}

	p := calcPagination(in.Page, in.PageSize, total)
	return &pb.ListWeb3UsersResponse{
		Success: true,
		Message: fmt.Sprintf("ok (%d)", total),
		Data: &pb.ListWeb3UsersData{
			Users: respItems,
			Summary: &pb.Web3UserListSummary{
				TotalDevices:         summaryCounts.TotalDevices,
				TotalAddresses:       summaryCounts.TotalAddresses,
				BlacklistedAddresses: summaryCounts.BlacklistedAddresses,
				TwoFactorRate:        twoFactorRate,
			},
		},
		Pagination: p,
		RequestId:  resp.RequestID(l.ctx),
		Timestamp:  resp.Timestamp(),
	}, nil
}
