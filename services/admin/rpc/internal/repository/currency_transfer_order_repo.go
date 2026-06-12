package repository

import (
	"context"
	"strings"

	"internalwallet/services/admin/rpc/internal/model"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// CurrencyTransferOrderSummary 内部转账统计汇总
type CurrencyTransferOrderSummary struct {
	Total       int64  // 总数
	Pending     int64  // 待处理数
	Completed   int64  // 已完成数
	Failed      int64  // 失败数
	Cancelled   int64  // 已取消数
	Rejected    int64  // 已拒绝数（管理员审核拒绝）
	TotalAmount string // 累计金额（仅统计 completed 状态）
}

type CurrencyTransferOrderRepository interface {
	WithTx(tx *gorm.DB) CurrencyTransferOrderRepository
	GetDB() *gorm.DB

	Create(ctx context.Context, order *model.CurrencyTransferOrderModel) error
	GetByID(ctx context.Context, id int64) (*model.CurrencyTransferOrderModel, error)
	Update(ctx context.Context, order *model.CurrencyTransferOrderModel) error
	List(ctx context.Context, filter CurrencyTransferOrderListFilter) ([]*model.CurrencyTransferOrderModel, int64, error)
	GetSummary(ctx context.Context) (*CurrencyTransferOrderSummary, error)
}

type CurrencyTransferOrderListFilter struct {
	FromUserID  int64
	ToUserID    int64
	AssetCode   string
	Status      []string // pending|processing|completed|failed|cancelled
	Strategy    []string // auto|manual_auto
	CreatedFrom string   // YYYY-MM-DD
	CreatedTo   string   // YYYY-MM-DD
	MinAmount   *string
	MaxAmount   *string
	SortBy      string // created_at|amount|status
	SortOrder   string // asc|desc
	Page        int32
	PageSize    int32
}

type currencyTransferOrderRepo struct {
	db *gorm.DB
}

func NewCurrencyTransferOrderRepository(db *gorm.DB) CurrencyTransferOrderRepository {
	return &currencyTransferOrderRepo{db: db}
}

func (r *currencyTransferOrderRepo) WithTx(tx *gorm.DB) CurrencyTransferOrderRepository {
	return &currencyTransferOrderRepo{db: tx}
}

func (r *currencyTransferOrderRepo) GetDB() *gorm.DB {
	return r.db
}

func (r *currencyTransferOrderRepo) Create(ctx context.Context, order *model.CurrencyTransferOrderModel) error {
	return r.db.WithContext(ctx).Create(order).Error
}

func (r *currencyTransferOrderRepo) GetByID(ctx context.Context, id int64) (*model.CurrencyTransferOrderModel, error) {
	var order model.CurrencyTransferOrderModel
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&order).Error
	if err != nil {
		return nil, err
	}
	return &order, nil
}

func (r *currencyTransferOrderRepo) Update(ctx context.Context, order *model.CurrencyTransferOrderModel) error {
	return r.db.WithContext(ctx).Save(order).Error
}

func (r *currencyTransferOrderRepo) List(ctx context.Context, filter CurrencyTransferOrderListFilter) ([]*model.CurrencyTransferOrderModel, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.CurrencyTransferOrderModel{})

	// Filter by from_user_id
	if filter.FromUserID > 0 {
		query = query.Where("from_user_id = ?", filter.FromUserID)
	}

	// Filter by to_user_id
	if filter.ToUserID > 0 {
		query = query.Where("to_user_id = ?", filter.ToUserID)
	}

	// Filter by asset_code
	if assetCode := strings.ToUpper(strings.TrimSpace(filter.AssetCode)); assetCode != "" {
		query = query.Where("asset_code = ?", assetCode)
	}

	// Filter by status
	if len(filter.Status) > 0 {
		query = query.Where("status IN ?", filter.Status)
	}

	// Filter by strategy
	if len(filter.Strategy) > 0 {
		query = query.Where("strategy IN ?", filter.Strategy)
	}

	// Filter by created_from
	if filter.CreatedFrom != "" {
		query = query.Where("DATE(created_at) >= ?", filter.CreatedFrom)
	}

	// Filter by created_to
	if filter.CreatedTo != "" {
		query = query.Where("DATE(created_at) <= ?", filter.CreatedTo)
	}

	// Filter by min_amount
	if filter.MinAmount != nil && *filter.MinAmount != "" {
		query = query.Where("amount >= ?", *filter.MinAmount)
	}

	// Filter by max_amount
	if filter.MaxAmount != nil && *filter.MaxAmount != "" {
		query = query.Where("amount <= ?", *filter.MaxAmount)
	}

	// Count total
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Pagination
	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	offset := (page - 1) * pageSize

	// Sort
	sortBy := strings.ToLower(strings.TrimSpace(filter.SortBy))
	sortOrder := strings.ToUpper(strings.TrimSpace(filter.SortOrder))
	if sortOrder != "ASC" && sortOrder != "DESC" {
		sortOrder = "DESC"
	}
	switch sortBy {
	case "amount":
		query = query.Order("amount " + sortOrder)
	case "status":
		query = query.Order("status " + sortOrder)
	case "created_at":
		query = query.Order("created_at " + sortOrder)
	default:
		query = query.Order("created_at DESC")
	}

	var items []*model.CurrencyTransferOrderModel
	err := query.Offset(int(offset)).Limit(int(pageSize)).Find(&items).Error
	if err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// GetSummary 获取内部转账统计汇总（全局统计，不受筛选条件影响）
func (r *currencyTransferOrderRepo) GetSummary(ctx context.Context) (*CurrencyTransferOrderSummary, error) {
	summary := &CurrencyTransferOrderSummary{
		TotalAmount: "0",
	}

	// 总数
	if err := r.db.WithContext(ctx).Model(&model.CurrencyTransferOrderModel{}).Count(&summary.Total).Error; err != nil {
		return nil, err
	}

	// 各状态数量
	type statusCount struct {
		Status string
		Count  int64
	}
	var statusCounts []statusCount
	if err := r.db.WithContext(ctx).
		Model(&model.CurrencyTransferOrderModel{}).
		Select("status, COUNT(*) as count").
		Group("status").
		Scan(&statusCounts).Error; err != nil {
		return nil, err
	}

	for _, sc := range statusCounts {
		switch strings.ToLower(sc.Status) {
		case "pending":
			summary.Pending = sc.Count
		case "completed":
			summary.Completed = sc.Count
		case "failed":
			summary.Failed = sc.Count
		case "cancelled":
			summary.Cancelled = sc.Count
		case "rejected":
			summary.Rejected = sc.Count
		}
	}

	// 累计金额（仅统计 completed 状态）
	type totalAmountResult struct {
		TotalAmount string
	}
	var result totalAmountResult
	if err := r.db.WithContext(ctx).
		Model(&model.CurrencyTransferOrderModel{}).
		Where("status = ?", "completed").
		Select("COALESCE(SUM(CAST(amount AS DECIMAL(65,30))), 0) as total_amount").
		Scan(&result).Error; err != nil {
		return nil, err
	}
	if result.TotalAmount != "" {
		// 格式化金额，保留 2 位小数
		if d, err := decimal.NewFromString(result.TotalAmount); err == nil {
			summary.TotalAmount = d.StringFixed(2)
		} else {
			summary.TotalAmount = result.TotalAmount
		}
	}

	return summary, nil
}
