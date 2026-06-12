package logic

import (
	"context"
	"fmt"
	"sync"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
)

type GetDashboardOverviewLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetDashboardOverviewLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetDashboardOverviewLogic {
	return &GetDashboardOverviewLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetDashboardOverviewLogic) GetDashboardOverview(in *pb.GetDashboardOverviewRequest) (*pb.GetDashboardOverviewResponse, error) {
	if in == nil {
		in = &pb.GetDashboardOverviewRequest{}
	}
	if l.svcCtx.DB == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	// 并发查询各个统计项
	var (
		userStats      *pb.UserStatsOverview
		vaultBalance   *pb.VaultBalanceOverview
		transferStats  *pb.TransferOverview
		approvalStats  *pb.ApprovalOverview
		userStatusDist *pb.UserStatusDistribution
		currencyCount  int32

		wg     sync.WaitGroup
		mu     sync.Mutex
		errors []error
	)

	// 1. 用户统计
	wg.Add(1)
	go func() {
		defer wg.Done()
		stats, err := l.fetchUserStats()
		if err != nil {
			mu.Lock()
			errors = append(errors, fmt.Errorf("fetch user stats: %w", err))
			mu.Unlock()
			return
		}
		mu.Lock()
		userStats = stats
		mu.Unlock()
	}()

	// 2. 金库余额
	wg.Add(1)
	go func() {
		defer wg.Done()
		balance, err := l.fetchVaultBalance()
		if err != nil {
			mu.Lock()
			errors = append(errors, fmt.Errorf("fetch vault balance: %w", err))
			mu.Unlock()
			return
		}
		mu.Lock()
		vaultBalance = balance
		mu.Unlock()
	}()

	// 3. 转账统计
	wg.Add(1)
	go func() {
		defer wg.Done()
		stats, err := l.fetchTransferStats()
		if err != nil {
			mu.Lock()
			errors = append(errors, fmt.Errorf("fetch transfer stats: %w", err))
			mu.Unlock()
			return
		}
		mu.Lock()
		transferStats = stats
		mu.Unlock()
	}()

	// 4. 审批统计
	wg.Add(1)
	go func() {
		defer wg.Done()
		stats, err := l.fetchApprovalStats()
		if err != nil {
			mu.Lock()
			errors = append(errors, fmt.Errorf("fetch approval stats: %w", err))
			mu.Unlock()
			return
		}
		mu.Lock()
		approvalStats = stats
		mu.Unlock()
	}()

	// 5. 用户状态分布
	wg.Add(1)
	go func() {
		defer wg.Done()
		dist, err := l.fetchUserStatusDistribution()
		if err != nil {
			mu.Lock()
			errors = append(errors, fmt.Errorf("fetch user status distribution: %w", err))
			mu.Unlock()
			return
		}
		mu.Lock()
		userStatusDist = dist
		mu.Unlock()
	}()

	// 6. 支持币种数
	wg.Add(1)
	go func() {
		defer wg.Done()
		count, err := l.fetchSupportedCurrenciesCount()
		if err != nil {
			mu.Lock()
			errors = append(errors, fmt.Errorf("fetch currency count: %w", err))
			mu.Unlock()
			return
		}
		mu.Lock()
		currencyCount = count
		mu.Unlock()
	}()

	wg.Wait()

	if len(errors) > 0 {
		l.Logger.Errorf("dashboard overview errors: %v", errors)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "failed to fetch dashboard data", nil)
	}

	return &pb.GetDashboardOverviewResponse{
		Success: true,
		Message: "ok",
		Data: &pb.DashboardOverviewData{
			UserStats:           userStats,
			VaultBalance:        vaultBalance,
			TodayTransfer:       transferStats,
			ApprovalStats:       approvalStats,
			UserStatusDist:      userStatusDist,
			SupportedCurrencies: currencyCount,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}

// fetchUserStats 获取用户统计
func (l *GetDashboardOverviewLogic) fetchUserStats() (*pb.UserStatsOverview, error) {
	type countRow struct {
		Cnt int64 `gorm:"column:cnt"`
	}

	// 总用户数
	var totalUsers int64
	if err := l.svcCtx.DB.WithContext(l.ctx).
		Model(&model.UserModel{}).
		Where("deleted_at IS NULL").
		Count(&totalUsers).Error; err != nil {
		return nil, err
	}

	// 今日新增
	var todayNew countRow
	if err := l.svcCtx.DB.WithContext(l.ctx).
		Raw("SELECT COUNT(*) AS cnt FROM users WHERE deleted_at IS NULL AND DATE(created_at) = CURDATE()").
		Scan(&todayNew).Error; err != nil {
		return nil, err
	}

	// 昨日新增
	var yesterdayNew countRow
	if err := l.svcCtx.DB.WithContext(l.ctx).
		Raw("SELECT COUNT(*) AS cnt FROM users WHERE deleted_at IS NULL AND DATE(created_at) = CURDATE() - INTERVAL 1 DAY").
		Scan(&yesterdayNew).Error; err != nil {
		return nil, err
	}

	// 计算增长率
	growthRate := 0.0
	if yesterdayNew.Cnt > 0 {
		growthRate = float64(todayNew.Cnt-yesterdayNew.Cnt) / float64(yesterdayNew.Cnt) * 100
	} else if todayNew.Cnt > 0 {
		growthRate = 100.0
	}

	// 近7日活跃用户（以最近登录时间为准）
	var active7d countRow
	if err := l.svcCtx.DB.WithContext(l.ctx).
		Raw("SELECT COUNT(*) AS cnt FROM users WHERE deleted_at IS NULL AND last_login_time >= NOW() - INTERVAL 7 DAY").
		Scan(&active7d).Error; err != nil {
		return nil, err
	}

	return &pb.UserStatsOverview{
		TotalUsers:     totalUsers,
		TodayNewUsers:  todayNew.Cnt,
		GrowthRate:     growthRate,
		ActiveUsers_7D: active7d.Cnt,
	}, nil
}

// fetchVaultBalance 获取金库余额
func (l *GetDashboardOverviewLogic) fetchVaultBalance() (*pb.VaultBalanceOverview, error) {
	type sumRow struct {
		Total int64 `gorm:"column:total"`
	}

	var result sumRow
	if err := l.svcCtx.DB.WithContext(l.ctx).
		Raw("SELECT COALESCE(SUM(balance_usd_raw), 0) AS total FROM vault_balances WHERE deleted_at IS NULL").
		Scan(&result).Error; err != nil {
		return nil, err
	}

	// 转换为USD字符串（分转美元）
	totalUSD := float64(result.Total) / 100.0

	return &pb.VaultBalanceOverview{
		TotalBalanceUsd: fmt.Sprintf("%.2f", totalUSD),
		TodayChangeUsd:  "0", // 简化版：返回0
		ChangeRate:      0.0,
	}, nil
}

// fetchTransferStats 获取转账统计
func (l *GetDashboardOverviewLogic) fetchTransferStats() (*pb.TransferOverview, error) {
	type statsRow struct {
		TotalAmount string `gorm:"column:total_amount"`
		Count       int32  `gorm:"column:count"`
	}

	// 今日转账
	var today statsRow
	if err := l.svcCtx.DB.WithContext(l.ctx).
		Raw(`
			SELECT 
				COALESCE(SUM(total_amount_usd), 0) AS total_amount,
				COUNT(*) AS count
			FROM transfer_batches
			WHERE DATE(created_at) = CURDATE()
			  AND status IN ('completed', 'partial_failed')
		`).
		Scan(&today).Error; err != nil {
		return nil, err
	}

	// 昨日转账
	var yesterday statsRow
	if err := l.svcCtx.DB.WithContext(l.ctx).
		Raw(`
			SELECT 
				COALESCE(SUM(total_amount_usd), 0) AS total_amount,
				COUNT(*) AS count
			FROM transfer_batches
			WHERE DATE(created_at) = CURDATE() - INTERVAL 1 DAY
			  AND status IN ('completed', 'partial_failed')
		`).
		Scan(&yesterday).Error; err != nil {
		return nil, err
	}

	// 计算增长率
	growthRate := 0.0
	if yesterday.Count > 0 {
		growthRate = float64(today.Count-yesterday.Count) / float64(yesterday.Count) * 100
	} else if today.Count > 0 {
		growthRate = 100.0
	}

	return &pb.TransferOverview{
		TodayAmountUsd: today.TotalAmount,
		TodayCount:     today.Count,
		GrowthRate:     growthRate,
	}, nil
}

// fetchApprovalStats 获取审批统计
func (l *GetDashboardOverviewLogic) fetchApprovalStats() (*pb.ApprovalOverview, error) {
	type countRow struct {
		Cnt int32 `gorm:"column:cnt"`
	}

	// 待审批数（pending状态）
	var pending countRow
	if err := l.svcCtx.DB.WithContext(l.ctx).
		Raw("SELECT COUNT(*) AS cnt FROM currency_withdraw_orders WHERE status = 'pending' AND deleted_at IS NULL").
		Scan(&pending).Error; err != nil {
		return nil, err
	}

	// 今日新增待审批
	var todayNew countRow
	if err := l.svcCtx.DB.WithContext(l.ctx).
		Raw("SELECT COUNT(*) AS cnt FROM currency_withdraw_orders WHERE status = 'pending' AND DATE(created_at) = CURDATE() AND deleted_at IS NULL").
		Scan(&todayNew).Error; err != nil {
		return nil, err
	}

	// 今日提现笔数
	var todayWithdrawals countRow
	if err := l.svcCtx.DB.WithContext(l.ctx).
		Raw("SELECT COUNT(*) AS cnt FROM currency_withdraw_orders WHERE DATE(created_at) = CURDATE() AND deleted_at IS NULL").
		Scan(&todayWithdrawals).Error; err != nil {
		return nil, err
	}

	return &pb.ApprovalOverview{
		PendingCount:         pending.Cnt,
		TodayNewCount:        todayNew.Cnt,
		TodayWithdrawalCount: todayWithdrawals.Cnt,
	}, nil
}

// fetchUserStatusDistribution 获取用户状态分布
func (l *GetDashboardOverviewLogic) fetchUserStatusDistribution() (*pb.UserStatusDistribution, error) {
	type statusRow struct {
		Status int   `gorm:"column:status"`
		Count  int64 `gorm:"column:count"`
	}

	var rows []statusRow
	if err := l.svcCtx.DB.WithContext(l.ctx).
		Raw("SELECT status, COUNT(*) AS count FROM users WHERE deleted_at IS NULL GROUP BY status").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	var normal, frozen, terminated int64
	for _, row := range rows {
		switch row.Status {
		case 1: // 正常
			normal = row.Count
		case 2: // 冻结
			frozen = row.Count
		case 3: // 注销
			terminated = row.Count
		}
	}

	total := normal + frozen + terminated
	if total == 0 {
		total = 1 // 避免除0
	}

	return &pb.UserStatusDistribution{
		Normal:         normal,
		Frozen:         frozen,
		Terminated:     terminated,
		NormalRate:     float64(normal) / float64(total) * 100,
		FrozenRate:     float64(frozen) / float64(total) * 100,
		TerminatedRate: float64(terminated) / float64(total) * 100,
	}, nil
}

// fetchSupportedCurrenciesCount 获取支持币种数
func (l *GetDashboardOverviewLogic) fetchSupportedCurrenciesCount() (int32, error) {
	type countRow struct {
		Cnt int32 `gorm:"column:cnt"`
	}

	var result countRow
	// 从asset表统计（通过Accounting服务管理）
	if err := l.svcCtx.DB.WithContext(l.ctx).
		Raw("SELECT COUNT(DISTINCT asset_code) AS cnt FROM asset WHERE deleted_at IS NULL AND status = 'active'").
		Scan(&result).Error; err != nil {
		// 如果asset表不存在，尝试从currency_chain_settings统计
		if err := l.svcCtx.DB.WithContext(l.ctx).
			Raw("SELECT COUNT(DISTINCT asset_code) AS cnt FROM currency_chain_settings WHERE deleted_at IS NULL").
			Scan(&result).Error; err != nil {
			return 0, err
		}
	}

	return result.Cnt, nil
}
