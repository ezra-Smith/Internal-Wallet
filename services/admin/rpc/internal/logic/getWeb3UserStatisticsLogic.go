package logic

import (
	"context"
	"fmt"
	"strings"
	"time"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
)

type GetWeb3UserStatisticsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetWeb3UserStatisticsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetWeb3UserStatisticsLogic {
	return &GetWeb3UserStatisticsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetWeb3UserStatisticsLogic) GetWeb3UserStatistics(in *pb.GetWeb3UserStatisticsRequest) (*pb.GetWeb3UserStatisticsResponse, error) {
	if in == nil {
		in = &pb.GetWeb3UserStatisticsRequest{}
	}
	if l.svcCtx.DB == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	period := strings.TrimSpace(in.Period)
	days := 30
	switch period {
	case "", "30d":
		days = 30
	case "7d":
		days = 7
	case "90d":
		days = 90
	default:
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PERIOD", "invalid period", map[string]string{"period": "invalid"})
	}

	now := time.Now().UTC()
	start := now.AddDate(0, 0, -days)
	start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)

	var totalDevices int64
	if err := l.svcCtx.DB.WithContext(l.ctx).
		Model(&model.Web3UserModel{}).
		Where("deleted_at IS NULL").
		Count(&totalDevices).Error; err != nil {
		l.Logger.Errorf("count web3 devices failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	var activeDevices30d int64
	activeFrom := now.AddDate(0, 0, -30)
	if err := l.svcCtx.DB.WithContext(l.ctx).
		Model(&model.Web3UserModel{}).
		Where("deleted_at IS NULL AND last_active_at IS NOT NULL AND last_active_at >= ?", activeFrom).
		Count(&activeDevices30d).Error; err != nil {
		l.Logger.Errorf("count active web3 devices failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	var totalAddresses int64
	if err := l.svcCtx.DB.WithContext(l.ctx).
		Model(&model.Web3UserAddressModel{}).
		Where("deleted_at IS NULL").
		Count(&totalAddresses).Error; err != nil {
		l.Logger.Errorf("count web3 addresses failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	var blacklistedAddresses int64
	if err := l.svcCtx.DB.WithContext(l.ctx).
		Model(&model.Web3UserAddressModel{}).
		Where("deleted_at IS NULL AND is_blacklisted = 1").
		Count(&blacklistedAddresses).Error; err != nil {
		l.Logger.Errorf("count blacklisted addresses failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	// by_network counts
	type byNetworkRow struct {
		Network string `gorm:"column:network"`
		Cnt     int64  `gorm:"column:cnt"`
	}
	var byNetworkRows []byNetworkRow
	if err := l.svcCtx.DB.WithContext(l.ctx).
		Table("web3_user_addresses").
		Select("network, COUNT(*) AS cnt").
		Where("deleted_at IS NULL").
		Group("network").
		Order("cnt DESC").
		Find(&byNetworkRows).Error; err != nil {
		l.Logger.Errorf("group by network failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}
	byNetwork := make([]*pb.Web3StatisticsByNetwork, 0, len(byNetworkRows))
	for _, r := range byNetworkRows {
		percentage := "0%"
		if totalAddresses > 0 {
			p := decimal.NewFromInt(r.Cnt).Div(decimal.NewFromInt(totalAddresses)).Mul(decimal.NewFromInt(100))
			percentage = p.StringFixed(0) + "%"
		}
		byNetwork = append(byNetwork, &pb.Web3StatisticsByNetwork{
			Network:      r.Network,
			AddressCount: r.Cnt,
			BalanceUsd:   "0",
			Percentage:   percentage,
		})
	}

	// by_platform counts
	type byPlatformRow struct {
		Platform string `gorm:"column:platform"`
		Cnt      int64  `gorm:"column:cnt"`
	}
	var byPlatformRows []byPlatformRow
	if err := l.svcCtx.DB.WithContext(l.ctx).
		Table("web3_users").
		Select("platform, COUNT(*) AS cnt").
		Where("deleted_at IS NULL").
		Group("platform").
		Order("cnt DESC").
		Find(&byPlatformRows).Error; err != nil {
		l.Logger.Errorf("group by platform failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}
	byPlatform := make([]*pb.Web3StatisticsByPlatform, 0, len(byPlatformRows))
	for _, r := range byPlatformRows {
		percentage := "0%"
		if totalDevices > 0 {
			p := decimal.NewFromInt(r.Cnt).Div(decimal.NewFromInt(totalDevices)).Mul(decimal.NewFromInt(100))
			percentage = p.StringFixed(1) + "%"
		}
		byPlatform = append(byPlatform, &pb.Web3StatisticsByPlatform{
			Platform:    r.Platform,
			DeviceCount: r.Cnt,
			Percentage:  percentage,
		})
	}

	// security stats
	var twoFactorEnabled int64
	_ = l.svcCtx.DB.WithContext(l.ctx).
		Model(&model.Web3UserModel{}).
		Where("deleted_at IS NULL AND two_factor_enabled = 1").
		Count(&twoFactorEnabled).Error
	var biometricEnabled int64
	_ = l.svcCtx.DB.WithContext(l.ctx).
		Model(&model.Web3UserModel{}).
		Where("deleted_at IS NULL AND biometric_enabled = 1").
		Count(&biometricEnabled).Error

	twoFactorRate := "0%"
	biometricRate := "0%"
	if totalDevices > 0 {
		twoFactorRate = decimal.NewFromInt(twoFactorEnabled).Div(decimal.NewFromInt(totalDevices)).Mul(decimal.NewFromInt(100)).StringFixed(1) + "%"
		biometricRate = decimal.NewFromInt(biometricEnabled).Div(decimal.NewFromInt(totalDevices)).Mul(decimal.NewFromInt(100)).StringFixed(1) + "%"
	}

	// growth trend (new devices per day)
	type growthRow struct {
		Date string `gorm:"column:dt"`
		Cnt  int64  `gorm:"column:cnt"`
	}
	var rows []growthRow
	if err := l.svcCtx.DB.WithContext(l.ctx).
		Table("web3_users").
		Select("DATE(created_at) AS dt, COUNT(*) AS cnt").
		Where("deleted_at IS NULL AND created_at >= ?", start).
		Group("dt").
		Order("dt ASC").
		Find(&rows).Error; err != nil {
		l.Logger.Errorf("growth trend query failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}
	newByDate := map[string]int64{}
	for _, r := range rows {
		newByDate[r.Date] = r.Cnt
	}

	var baseTotal int64
	_ = l.svcCtx.DB.WithContext(l.ctx).
		Model(&model.Web3UserModel{}).
		Where("deleted_at IS NULL AND created_at < ?", start).
		Count(&baseTotal).Error

	growthTrend := make([]*pb.Web3StatisticsGrowthTrend, 0, days+1)
	running := baseTotal
	for d := 0; d <= days; d++ {
		day := start.AddDate(0, 0, d)
		key := day.Format("2006-01-02")
		n := newByDate[key]
		running += n
		growthTrend = append(growthTrend, &pb.Web3StatisticsGrowthTrend{
			Date:         key,
			NewDevices:   n,
			TotalDevices: running,
		})
	}

	return &pb.GetWeb3UserStatisticsResponse{
		Success: true,
		Message: fmt.Sprintf("ok (%s)", periodOrDefault(period)),
		Data: &pb.GetWeb3UserStatisticsData{
			Overview: &pb.Web3StatisticsOverview{
				TotalDevices:         totalDevices,
				ActiveDevices_30D:    activeDevices30d,
				TotalAddresses:       totalAddresses,
				BlacklistedAddresses: blacklistedAddresses,
				TotalBalanceUsd:      "0",
			},
			ByNetwork:  byNetwork,
			ByPlatform: byPlatform,
			SecurityStats: &pb.Web3StatisticsSecurity{
				TwoFactorEnabled: twoFactorEnabled,
				TwoFactorRate:    twoFactorRate,
				BiometricEnabled: biometricEnabled,
				BiometricRate:    biometricRate,
			},
			GrowthTrend: growthTrend,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}

func periodOrDefault(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return "30d"
	}
	return p
}
