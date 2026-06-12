package logic

import (
	"context"
	"fmt"
	"sync"
	"time"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
)

type GetDashboardTrendsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetDashboardTrendsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetDashboardTrendsLogic {
	return &GetDashboardTrendsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetDashboardTrendsLogic) GetDashboardTrends(in *pb.GetDashboardTrendsRequest) (*pb.GetDashboardTrendsResponse, error) {
	if in == nil {
		in = &pb.GetDashboardTrendsRequest{}
	}
	if l.svcCtx.DB == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	// 默认参数
	months := in.FundFlowMonths
	if months <= 0 {
		months = 6
	}
	days := in.TransferDays
	if days <= 0 {
		days = 7
	}

	// 并发查询
	var (
		fundFlowTrend      *pb.FundFlowTrend
		transferBatchTrend *pb.TransferBatchTrend
		approvalDist       *pb.WithdrawalApprovalDistribution
		vaultTopCurrencies []*pb.VaultCurrencyBalance

		wg     sync.WaitGroup
		mu     sync.Mutex
		errors []error
	)

	// 1. 资金流水趋势
	wg.Add(1)
	go func() {
		defer wg.Done()
		trend, err := l.fetchFundFlowTrend(months)
		if err != nil {
			mu.Lock()
			errors = append(errors, fmt.Errorf("fetch fund flow trend: %w", err))
			mu.Unlock()
			return
		}
		mu.Lock()
		fundFlowTrend = trend
		mu.Unlock()
	}()

	// 2. 转账批次趋势
	wg.Add(1)
	go func() {
		defer wg.Done()
		trend, err := l.fetchTransferBatchTrend(days)
		if err != nil {
			mu.Lock()
			errors = append(errors, fmt.Errorf("fetch transfer batch trend: %w", err))
			mu.Unlock()
			return
		}
		mu.Lock()
		transferBatchTrend = trend
		mu.Unlock()
	}()

	// 3. 审批分布
	wg.Add(1)
	go func() {
		defer wg.Done()
		dist, err := l.fetchApprovalDistribution()
		if err != nil {
			mu.Lock()
			errors = append(errors, fmt.Errorf("fetch approval distribution: %w", err))
			mu.Unlock()
			return
		}
		mu.Lock()
		approvalDist = dist
		mu.Unlock()
	}()

	// 4. Vault币种Top 5
	wg.Add(1)
	go func() {
		defer wg.Done()
		top, err := l.fetchVaultTopCurrencies()
		if err != nil {
			mu.Lock()
			errors = append(errors, fmt.Errorf("fetch vault top currencies: %w", err))
			mu.Unlock()
			return
		}
		mu.Lock()
		vaultTopCurrencies = top
		mu.Unlock()
	}()

	wg.Wait()

	if len(errors) > 0 {
		l.Logger.Errorf("dashboard trends errors: %v", errors)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "failed to fetch trend data", nil)
	}

	return &pb.GetDashboardTrendsResponse{
		Success: true,
		Message: "ok",
		Data: &pb.DashboardTrendsData{
			FundFlowTrend:        fundFlowTrend,
			TransferBatchTrend:   transferBatchTrend,
			ApprovalDistribution: approvalDist,
			VaultTopCurrencies:   vaultTopCurrencies,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}

// fetchFundFlowTrend 获取资金流水趋势（按月）
func (l *GetDashboardTrendsLogic) fetchFundFlowTrend(months int32) (*pb.FundFlowTrend, error) {
	type monthRow struct {
		Month  string `gorm:"column:month"`
		Amount string `gorm:"column:amount"`
	}

	// 充值
	var deposits []monthRow
	if err := l.svcCtx.DB.WithContext(l.ctx).
		Raw(`
			SELECT 
				DATE_FORMAT(created_at, '%Y-%m') AS month,
				COALESCE(SUM(amount), 0) AS amount
			FROM wallet_deposits
			WHERE deleted_at IS NULL
			  AND created_at >= DATE_SUB(NOW(), INTERVAL ? MONTH)
			  AND status = 'completed'
			GROUP BY month
			ORDER BY month
		`, months).
		Scan(&deposits).Error; err != nil {
		l.Logger.Errorf("fetch deposits trend failed: %v", err)
		deposits = []monthRow{}
	}

	// 提现
	var withdrawals []monthRow
	if err := l.svcCtx.DB.WithContext(l.ctx).
		Raw(`
			SELECT 
				DATE_FORMAT(created_at, '%Y-%m') AS month,
				COALESCE(SUM(amount), 0) AS amount
			FROM currency_withdraw_orders
			WHERE deleted_at IS NULL
			  AND created_at >= DATE_SUB(NOW(), INTERVAL ? MONTH)
			  AND status = 'completed'
			GROUP BY month
			ORDER BY month
		`, months).
		Scan(&withdrawals).Error; err != nil {
		l.Logger.Errorf("fetch withdrawals trend failed: %v", err)
		withdrawals = []monthRow{}
	}

	// 转账
	var transfers []monthRow
	if err := l.svcCtx.DB.WithContext(l.ctx).
		Raw(`
			SELECT 
				DATE_FORMAT(created_at, '%Y-%m') AS month,
				COALESCE(SUM(total_amount_usd), 0) AS amount
			FROM transfer_batches
			WHERE created_at >= DATE_SUB(NOW(), INTERVAL ? MONTH)
			  AND status = 'completed'
			GROUP BY month
			ORDER BY month
		`, months).
		Scan(&transfers).Error; err != nil {
		l.Logger.Errorf("fetch transfers trend failed: %v", err)
		transfers = []monthRow{}
	}

	// 合并数据
	dataMap := make(map[string]*pb.FundFlowDataPoint)

	// 生成所有月份
	now := time.Now()
	for i := int32(0); i < months; i++ {
		month := now.AddDate(0, -int(i), 0).Format("2006-01")
		dataMap[month] = &pb.FundFlowDataPoint{
			Period:           month,
			DepositAmount:    "0",
			WithdrawalAmount: "0",
			TransferAmount:   "0",
			TotalAmount:      "0",
		}
	}

	// 填充充值数据
	for _, d := range deposits {
		if dp, ok := dataMap[d.Month]; ok {
			dp.DepositAmount = d.Amount
		}
	}

	// 填充提现数据
	for _, w := range withdrawals {
		if dp, ok := dataMap[w.Month]; ok {
			dp.WithdrawalAmount = w.Amount
		}
	}

	// 填充转账数据
	for _, t := range transfers {
		if dp, ok := dataMap[t.Month]; ok {
			dp.TransferAmount = t.Amount
		}
	}

	// 转换为数组
	dataPoints := make([]*pb.FundFlowDataPoint, 0, len(dataMap))
	for _, dp := range dataMap {
		dataPoints = append(dataPoints, dp)
	}

	return &pb.FundFlowTrend{
		DataPoints: dataPoints,
	}, nil
}

// fetchTransferBatchTrend 获取转账批次趋势（按日）
func (l *GetDashboardTrendsLogic) fetchTransferBatchTrend(days int32) (*pb.TransferBatchTrend, error) {
	type dayRow struct {
		Date      string `gorm:"column:date"`
		Total     int32  `gorm:"column:total"`
		Completed int32  `gorm:"column:completed"`
		Failed    int32  `gorm:"column:failed"`
	}

	var rows []dayRow
	if err := l.svcCtx.DB.WithContext(l.ctx).
		Raw(`
			SELECT 
				DATE(created_at) AS date,
				COUNT(*) AS total,
				SUM(CASE WHEN status='completed' THEN 1 ELSE 0 END) AS completed,
				SUM(CASE WHEN status='failed' OR status='partial_failed' THEN 1 ELSE 0 END) AS failed
			FROM transfer_batches
			WHERE created_at >= DATE_SUB(NOW(), INTERVAL ? DAY)
			GROUP BY date
			ORDER BY date
		`, days).
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	dataPoints := make([]*pb.TransferBatchDataPoint, 0, len(rows))
	for _, row := range rows {
		successRate := 0.0
		if row.Total > 0 {
			successRate = float64(row.Completed) / float64(row.Total) * 100
		}

		dataPoints = append(dataPoints, &pb.TransferBatchDataPoint{
			Date:             row.Date,
			TotalBatches:     row.Total,
			CompletedBatches: row.Completed,
			FailedBatches:    row.Failed,
			SuccessRate:      successRate,
		})
	}

	return &pb.TransferBatchTrend{
		DataPoints: dataPoints,
	}, nil
}

// fetchApprovalDistribution 获取审批分布
func (l *GetDashboardTrendsLogic) fetchApprovalDistribution() (*pb.WithdrawalApprovalDistribution, error) {
	type statusRow struct {
		Status string `gorm:"column:status"`
		Count  int32  `gorm:"column:count"`
	}

	var rows []statusRow
	if err := l.svcCtx.DB.WithContext(l.ctx).
		Raw("SELECT status, COUNT(*) AS count FROM currency_withdraw_orders WHERE deleted_at IS NULL GROUP BY status").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	dist := &pb.WithdrawalApprovalDistribution{
		Pending:   0,
		Approved:  0,
		Rejected:  0,
		Cancelled: 0,
	}

	for _, row := range rows {
		switch row.Status {
		case "pending":
			dist.Pending += row.Count
		case "completed":
			dist.Approved = row.Count
		case "failed":
			dist.Rejected = row.Count
		case "cancelled":
			dist.Cancelled = row.Count
		}
	}

	return dist, nil
}

// fetchVaultTopCurrencies 获取Vault币种余额Top 5
func (l *GetDashboardTrendsLogic) fetchVaultTopCurrencies() ([]*pb.VaultCurrencyBalance, error) {
	type currencyRow struct {
		Currency      string `gorm:"column:currency"`
		Balance       string `gorm:"column:balance"`
		BalanceUSD    string `gorm:"column:balance_usd"`
		BalanceUSDRaw int64  `gorm:"column:balance_usd_raw"`
	}

	var rows []currencyRow
	if err := l.svcCtx.DB.WithContext(l.ctx).
		Raw(`
			SELECT 
				currency,
				balance,
				balance_usd,
				balance_usd_raw
			FROM vault_balances
			WHERE deleted_at IS NULL
			ORDER BY balance_usd_raw DESC
			LIMIT 5
		`).
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	// 计算总余额
	var totalUSDRaw int64
	for _, row := range rows {
		totalUSDRaw += row.BalanceUSDRaw
	}

	if totalUSDRaw == 0 {
		totalUSDRaw = 1 // 避免除0
	}

	result := make([]*pb.VaultCurrencyBalance, 0, len(rows))
	for i, row := range rows {
		percentage := float64(row.BalanceUSDRaw) / float64(totalUSDRaw) * 100

		result = append(result, &pb.VaultCurrencyBalance{
			Rank:       int32(i + 1),
			Currency:   row.Currency,
			Balance:    row.Balance,
			BalanceUsd: row.BalanceUSD,
			Percentage: percentage,
		})
	}

	return result, nil
}
