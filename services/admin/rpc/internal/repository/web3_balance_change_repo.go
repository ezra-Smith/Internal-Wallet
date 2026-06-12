package repository

import (
	"context"
	"strings"
	"time"

	"internalwallet/services/admin/rpc/internal/model"

	"gorm.io/gorm"
)

type Web3BalanceChangeFilter struct {
	TxType    string // receive | send | swap | contract_call | approve
	Direction string // in | out
	AssetCode string // BTC | ETH | USDT etc.
	Status    string // pending | confirmed | failed
	ChainCode string // ethereum | bsc | polygon etc.
	DateFrom  *time.Time
	DateTo    *time.Time
	SortBy    string // block_time | created_at (default: block_time)
	SortOrder string // asc | desc (default: desc)
}

type Web3BalanceChangeRepository interface {
	WithTx(tx *gorm.DB) Web3BalanceChangeRepository
	GetDB() *gorm.DB

	// ListByWeb3UserID queries all balance changes for a Web3 user (all addresses)
	ListByWeb3UserID(ctx context.Context, userID int64, page, pageSize int32, f Web3BalanceChangeFilter) ([]*model.Web3BalanceChangeModel, int64, error)
}

type web3BalanceChangeRepo struct {
	db *gorm.DB
}

func NewWeb3BalanceChangeRepository(db *gorm.DB) Web3BalanceChangeRepository {
	return &web3BalanceChangeRepo{db: db}
}

func (r *web3BalanceChangeRepo) WithTx(tx *gorm.DB) Web3BalanceChangeRepository {
	return &web3BalanceChangeRepo{db: tx}
}

func (r *web3BalanceChangeRepo) GetDB() *gorm.DB {
	return r.db
}

func applyWeb3BalanceChangeFilters(q *gorm.DB, f Web3BalanceChangeFilter) *gorm.DB {
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
	if v := strings.TrimSpace(f.ChainCode); v != "" {
		q = q.Where("chain_code = ?", strings.ToUpper(v))
	}
	if f.DateFrom != nil && !f.DateFrom.IsZero() {
		q = q.Where("block_time >= ?", f.DateFrom)
	}
	if f.DateTo != nil && !f.DateTo.IsZero() {
		q = q.Where("block_time <= ?", f.DateTo)
	}
	return q
}

func (r *web3BalanceChangeRepo) ListByWeb3UserID(ctx context.Context, userID int64, page, pageSize int32, f Web3BalanceChangeFilter) ([]*model.Web3BalanceChangeModel, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	query := r.db.WithContext(ctx).Model(&model.Web3BalanceChangeModel{}).
		Where("web3_user_id = ?", userID)

	// Apply filters
	query = applyWeb3BalanceChangeFilters(query, f)

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
	var items []*model.Web3BalanceChangeModel
	offset := int((page - 1) * pageSize)
	err := query.Order(orderCol + " " + orderDir).
		Offset(offset).Limit(int(pageSize)).
		Find(&items).Error

	return items, total, err
}
