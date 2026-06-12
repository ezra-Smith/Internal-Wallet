package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"internalwallet/services/swap/rpc/internal/model"

	"gorm.io/gorm"
)

type SwapTransactionListFilter struct {
	ChainID     int64
	Provider    string
	Status      string
	ProjectName string
	StartDate   string
	EndDate     string
}

type SwapTransactionStatsSummary struct {
	TotalSwaps       int64
	TotalVolumeUSD   string
	TotalFeeUSD      string
	AvgSwapAmountUSD string
	SuccessRate      string
	UniqueWallets    int64
}

type SwapTransactionStatsByChain struct {
	ChainID   int64
	ChainName string
	SwapCount int64
	VolumeUSD string
}

type SwapTransactionStatsByProvider struct {
	Provider    string
	SwapCount   int64
	VolumeUSD   string
	SuccessRate string
}

type SwapTransactionStatsTrend struct {
	Date      string
	SwapCount int64
	VolumeUSD string
}

type SwapTransactionRepository interface {
	WithTx(tx *gorm.DB) SwapTransactionRepository
	GetDB() *gorm.DB

	Create(ctx context.Context, m *model.SwapSvcTransactionModel) error
	FindByID(ctx context.Context, id int64) (*model.SwapSvcTransactionModel, error)
	FindByTxHash(ctx context.Context, chainID int64, txHash string) (*model.SwapSvcTransactionModel, error)
	ListByWallet(ctx context.Context, walletAddress string, page, pageSize int32, f SwapTransactionListFilter) ([]*model.SwapSvcTransactionModel, int64, error)
	ListWithFilters(ctx context.Context, projectName string, page, pageSize int32, f SwapTransactionListFilter) ([]*model.SwapSvcTransactionModel, int64, error)
	UpdateFields(ctx context.Context, id int64, fields map[string]any) error

	// Statistics methods
	GetStatsSummary(ctx context.Context, projectName, startDate, endDate string) (*SwapTransactionStatsSummary, error)
	GetStatsByChain(ctx context.Context, projectName, startDate, endDate string) ([]*SwapTransactionStatsByChain, error)
	GetStatsByProvider(ctx context.Context, projectName, startDate, endDate string) ([]*SwapTransactionStatsByProvider, error)
	GetStatsTrend(ctx context.Context, projectName, startDate, endDate string) ([]*SwapTransactionStatsTrend, error)
}

type swapTransactionRepo struct {
	db *gorm.DB
}

func NewSwapTransactionRepository(db *gorm.DB) SwapTransactionRepository {
	return &swapTransactionRepo{db: db}
}

func (r *swapTransactionRepo) WithTx(tx *gorm.DB) SwapTransactionRepository {
	return &swapTransactionRepo{db: tx}
}
func (r *swapTransactionRepo) GetDB() *gorm.DB { return r.db }

func (r *swapTransactionRepo) Create(ctx context.Context, m *model.SwapSvcTransactionModel) error {
	if m == nil {
		return fmt.Errorf("nil model")
	}
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *swapTransactionRepo) FindByID(ctx context.Context, id int64) (*model.SwapSvcTransactionModel, error) {
	if id <= 0 {
		return nil, fmt.Errorf("not found")
	}
	var m model.SwapSvcTransactionModel
	err := r.db.WithContext(ctx).
		Where("id = ?", id).
		First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("not found")
	}
	return &m, err
}

func (r *swapTransactionRepo) FindByTxHash(ctx context.Context, chainID int64, txHash string) (*model.SwapSvcTransactionModel, error) {
	if chainID <= 0 {
		return nil, fmt.Errorf("not found")
	}
	txHash = strings.TrimSpace(txHash)
	if txHash == "" {
		return nil, fmt.Errorf("not found")
	}
	var m model.SwapSvcTransactionModel
	err := r.db.WithContext(ctx).
		Where("chain_id = ? AND tx_hash = ?", chainID, txHash).
		First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("not found")
	}
	return &m, err
}

func (r *swapTransactionRepo) ListByWallet(ctx context.Context, walletAddress string, page, pageSize int32, f SwapTransactionListFilter) ([]*model.SwapSvcTransactionModel, int64, error) {
	walletAddress = strings.TrimSpace(walletAddress)
	if walletAddress == "" {
		return nil, 0, fmt.Errorf("wallet_address required")
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}

	query := r.db.WithContext(ctx).Model(&model.SwapSvcTransactionModel{}).
		Where("wallet_address = ?", walletAddress)
	if f.ChainID > 0 {
		query = query.Where("chain_id = ?", f.ChainID)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := int((page - 1) * pageSize)
	var items []*model.SwapSvcTransactionModel
	err := query.Order("created_at DESC").Offset(offset).Limit(int(pageSize)).Find(&items).Error
	return items, total, err
}

func (r *swapTransactionRepo) ListWithFilters(ctx context.Context, projectName string, page, pageSize int32, f SwapTransactionListFilter) ([]*model.SwapSvcTransactionModel, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}

	query := r.db.WithContext(ctx).Model(&model.SwapSvcTransactionModel{})

	if projectName != "" {
		query = query.Where("project_name = ?", projectName)
	}
	if f.ChainID > 0 {
		query = query.Where("chain_id = ?", f.ChainID)
	}
	if f.Provider != "" {
		query = query.Where("provider = ?", f.Provider)
	}
	if f.Status != "" {
		query = query.Where("status = ?", f.Status)
	}
	if f.StartDate != "" {
		query = query.Where("DATE(created_at) >= ?", f.StartDate)
	}
	if f.EndDate != "" {
		query = query.Where("DATE(created_at) <= ?", f.EndDate)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := int((page - 1) * pageSize)
	var items []*model.SwapSvcTransactionModel
	err := query.Order("created_at DESC").Offset(offset).Limit(int(pageSize)).Find(&items).Error
	return items, total, err
}

func (r *swapTransactionRepo) UpdateFields(ctx context.Context, id int64, fields map[string]any) error {
	if id <= 0 {
		return fmt.Errorf("invalid id: %d", id)
	}
	if fields == nil {
		fields = map[string]any{}
	}
	fields["updated_at"] = time.Now().Local()

	res := r.db.WithContext(ctx).
		Model(&model.SwapSvcTransactionModel{}).
		Where("id = ?", id).
		Updates(fields)

	if res.Error != nil {
		return fmt.Errorf("update failed: %w (id=%d, fields=%+v)", res.Error, id, fields)
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("no rows affected, record not found or no changes (id=%d)", id)
	}
	return nil
}

// Statistics methods

func (r *swapTransactionRepo) GetStatsSummary(ctx context.Context, projectName, startDate, endDate string) (*SwapTransactionStatsSummary, error) {
	query := r.db.WithContext(ctx).Model(&model.SwapSvcTransactionModel{})

	if projectName != "" {
		query = query.Where("project_name = ?", projectName)
	}
	if startDate != "" {
		query = query.Where("DATE(created_at) >= ?", startDate)
	}
	if endDate != "" {
		query = query.Where("DATE(created_at) <= ?", endDate)
	}

	var summary SwapTransactionStatsSummary
	err := query.Select(`
		COUNT(*) as total_swaps,
		COUNT(DISTINCT wallet_address) as unique_wallets,
		COALESCE(SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END) * 100.0 / NULLIF(COUNT(*), 0), 0) as success_rate
	`).Scan(&summary).Error

	if err != nil {
		return nil, err
	}

	summary.TotalVolumeUSD = "0"
	summary.TotalFeeUSD = "0"
	summary.AvgSwapAmountUSD = "0"

	return &summary, nil
}

func (r *swapTransactionRepo) GetStatsByChain(ctx context.Context, projectName, startDate, endDate string) ([]*SwapTransactionStatsByChain, error) {
	query := r.db.WithContext(ctx).Model(&model.SwapSvcTransactionModel{})

	if projectName != "" {
		query = query.Where("project_name = ?", projectName)
	}
	if startDate != "" {
		query = query.Where("DATE(created_at) >= ?", startDate)
	}
	if endDate != "" {
		query = query.Where("DATE(created_at) <= ?", endDate)
	}

	var stats []*SwapTransactionStatsByChain
	err := query.Select(`
		chain_id,
		chain_id as chain_name,
		COUNT(*) as swap_count,
		'0' as volume_usd
	`).
		Group("chain_id").
		Order("swap_count DESC").
		Scan(&stats).Error

	return stats, err
}

func (r *swapTransactionRepo) GetStatsByProvider(ctx context.Context, projectName, startDate, endDate string) ([]*SwapTransactionStatsByProvider, error) {
	query := r.db.WithContext(ctx).Model(&model.SwapSvcTransactionModel{})

	if projectName != "" {
		query = query.Where("project_name = ?", projectName)
	}
	if startDate != "" {
		query = query.Where("DATE(created_at) >= ?", startDate)
	}
	if endDate != "" {
		query = query.Where("DATE(created_at) <= ?", endDate)
	}

	var stats []*SwapTransactionStatsByProvider
	err := query.Select(`
		provider,
		COUNT(*) as swap_count,
		'0' as volume_usd,
		COALESCE(SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END) * 100.0 / NULLIF(COUNT(*), 0), 0) as success_rate
	`).
		Group("provider").
		Order("swap_count DESC").
		Scan(&stats).Error

	return stats, err
}

func (r *swapTransactionRepo) GetStatsTrend(ctx context.Context, projectName, startDate, endDate string) ([]*SwapTransactionStatsTrend, error) {
	query := r.db.WithContext(ctx).Model(&model.SwapSvcTransactionModel{})

	if projectName != "" {
		query = query.Where("project_name = ?", projectName)
	}
	if startDate != "" {
		query = query.Where("DATE(created_at) >= ?", startDate)
	}
	if endDate != "" {
		query = query.Where("DATE(created_at) <= ?", endDate)
	}

	var stats []*SwapTransactionStatsTrend
	err := query.Select(`
		DATE(created_at) as date,
		COUNT(*) as swap_count,
		'0' as volume_usd
	`).
		Group("DATE(created_at)").
		Order("date ASC").
		Scan(&stats).Error

	return stats, err
}
