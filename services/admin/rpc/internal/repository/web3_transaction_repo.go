package repository

import (
	"context"
	"strings"
	"time"

	"internalwallet/services/admin/rpc/internal/model"

	"gorm.io/gorm"
)

type Web3TransactionFilter struct {
	TxType    string // receive | send | swap | contract_call | approve
	Direction string // in | out
	AssetCode string // BTC | ETH | USDT etc.
	Status    string // pending | confirmed | failed
	Network   string // ethereum | bsc | polygon etc.
	DateFrom  *time.Time
	DateTo    *time.Time
	SortBy    string // block_time | created_at (default: block_time)
	SortOrder string // asc | desc (default: desc)
}

type Web3TransactionSummary struct {
	TotalTransactions  int64
	TotalDepositUSD    string
	TotalWithdrawalUSD string
	TotalSwapUSD       string
	FirstTransactionAt *time.Time
	LastTransactionAt  *time.Time
}

type Web3TransactionRepository interface {
	WithTx(tx *gorm.DB) Web3TransactionRepository
	GetDB() *gorm.DB

	// ListByUserAddress queries transactions for a specific user address
	ListByUserAddress(ctx context.Context, userAddress string, page, pageSize int32, f Web3TransactionFilter) ([]*model.Web3TransactionModel, int64, error)

	// ListByWeb3UserID queries all transactions for a Web3 user (all addresses)
	ListByWeb3UserID(ctx context.Context, userID int64, page, pageSize int32, f Web3TransactionFilter) ([]*model.Web3TransactionModel, int64, error)

	// GetUserTransactionSummary returns transaction statistics for a Web3 user
	GetUserTransactionSummary(ctx context.Context, userID int64) (Web3TransactionSummary, error)
}

type web3TransactionRepo struct {
	db *gorm.DB
}

func NewWeb3TransactionRepository(db *gorm.DB) Web3TransactionRepository {
	return &web3TransactionRepo{db: db}
}

func (r *web3TransactionRepo) WithTx(tx *gorm.DB) Web3TransactionRepository {
	return &web3TransactionRepo{db: tx}
}

func (r *web3TransactionRepo) GetDB() *gorm.DB {
	return r.db
}

func applyWeb3TransactionFilters(q *gorm.DB, f Web3TransactionFilter) *gorm.DB {
	if v := strings.TrimSpace(f.TxType); v != "" {
		q = q.Where("tx_type = ?", v)
	}
	if v := strings.TrimSpace(f.Direction); v != "" {
		q = q.Where("direction = ?", v)
	}
	if v := strings.TrimSpace(f.AssetCode); v != "" {
		q = q.Where("asset_code = ?", strings.ToUpper(v))
	}
	if v := strings.TrimSpace(f.Status); v != "" {
		q = q.Where("status = ?", v)
	}
	if v := strings.TrimSpace(f.Network); v != "" {
		q = q.Where("network = ?", strings.ToLower(v))
	}
	if f.DateFrom != nil && !f.DateFrom.IsZero() {
		q = q.Where("block_time >= ?", f.DateFrom)
	}
	if f.DateTo != nil && !f.DateTo.IsZero() {
		q = q.Where("block_time <= ?", f.DateTo)
	}
	return q
}

func (r *web3TransactionRepo) ListByUserAddress(ctx context.Context, userAddress string, page, pageSize int32, f Web3TransactionFilter) ([]*model.Web3TransactionModel, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	userAddress = strings.TrimSpace(userAddress)
	query := r.db.WithContext(ctx).Model(&model.Web3TransactionModel{}).
		Where("user_address = ?", userAddress)

	// Apply filters
	query = applyWeb3TransactionFilters(query, f)

	// Count
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Sort
	orderCol := "block_time"
	if strings.TrimSpace(f.SortBy) == "created_at" {
		orderCol = "created_at"
	}
	orderDir := "DESC"
	if strings.EqualFold(strings.TrimSpace(f.SortOrder), "asc") {
		orderDir = "ASC"
	}

	// Query
	var items []*model.Web3TransactionModel
	offset := int((page - 1) * pageSize)
	err := query.Order(orderCol + " " + orderDir).
		Offset(offset).Limit(int(pageSize)).
		Find(&items).Error

	return items, total, err
}

func (r *web3TransactionRepo) ListByWeb3UserID(ctx context.Context, userID int64, page, pageSize int32, f Web3TransactionFilter) ([]*model.Web3TransactionModel, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	query := r.db.WithContext(ctx).Model(&model.Web3TransactionModel{}).
		Where("web3_user_id = ?", userID)

	// Apply filters
	query = applyWeb3TransactionFilters(query, f)

	// Count
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Sort
	orderCol := "block_time"
	if strings.TrimSpace(f.SortBy) == "created_at" {
		orderCol = "created_at"
	}
	orderDir := "DESC"
	if strings.EqualFold(strings.TrimSpace(f.SortOrder), "asc") {
		orderDir = "ASC"
	}

	// Query
	var items []*model.Web3TransactionModel
	offset := int((page - 1) * pageSize)
	err := query.Order(orderCol + " " + orderDir).
		Offset(offset).Limit(int(pageSize)).
		Find(&items).Error

	return items, total, err
}

func (r *web3TransactionRepo) GetUserTransactionSummary(ctx context.Context, userID int64) (Web3TransactionSummary, error) {
	var summary Web3TransactionSummary
	query := r.db.WithContext(ctx).Model(&model.Web3TransactionModel{}).Where("web3_user_id = ?", userID)

	// Total transactions
	if err := query.Count(&summary.TotalTransactions).Error; err != nil {
		return summary, err
	}

	// Total deposit USD
	var totalDepositUSD string
	if err := r.db.WithContext(ctx).Model(&model.Web3TransactionModel{}).
		Select("COALESCE(SUM(CAST(amount_usd AS DECIMAL(65, 18))), 0)").
		Where("web3_user_id = ? AND direction = ? AND tx_type = ?", userID, "in", "receive").
		Scan(&totalDepositUSD).Error; err != nil {
		return summary, err
	}
	summary.TotalDepositUSD = totalDepositUSD

	// Total withdrawal USD
	var totalWithdrawalUSD string
	if err := r.db.WithContext(ctx).Model(&model.Web3TransactionModel{}).
		Select("COALESCE(SUM(CAST(amount_usd AS DECIMAL(65, 18))), 0)").
		Where("web3_user_id = ? AND direction = ? AND tx_type = ?", userID, "out", "send").
		Scan(&totalWithdrawalUSD).Error; err != nil {
		return summary, err
	}
	summary.TotalWithdrawalUSD = totalWithdrawalUSD

	// Total swap USD
	var totalSwapUSD string
	if err := r.db.WithContext(ctx).Model(&model.Web3TransactionModel{}).
		Select("COALESCE(SUM(CAST(amount_usd AS DECIMAL(65, 18))), 0)").
		Where("web3_user_id = ? AND tx_type = ?", userID, "swap").
		Scan(&totalSwapUSD).Error; err != nil {
		return summary, err
	}
	summary.TotalSwapUSD = totalSwapUSD

	// First and Last transaction time
	var firstTxTime, lastTxTime *time.Time
	if err := r.db.WithContext(ctx).Model(&model.Web3TransactionModel{}).
		Select("MIN(block_time)").Where("web3_user_id = ?", userID).Scan(&firstTxTime).Error; err != nil {
		return summary, err
	}
	if err := r.db.WithContext(ctx).Model(&model.Web3TransactionModel{}).
		Select("MAX(block_time)").Where("web3_user_id = ?", userID).Scan(&lastTxTime).Error; err != nil {
		return summary, err
	}
	summary.FirstTransactionAt = firstTxTime
	summary.LastTransactionAt = lastTxTime

	return summary, nil
}
