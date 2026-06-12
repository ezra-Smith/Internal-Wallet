package repository

import (
	"context"
	"strings"
	"time"

	"internalwallet/services/business/rpc/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Web3TransactionListFilter 交易列表筛选条件
type Web3TransactionListFilter struct {
	Address   string     // 用户地址筛选
	Network   string     // 网络筛选
	AssetCode string     // 资产代码筛选
	TxType    string     // 交易类型筛选
	Direction string     // 交易方向筛选
	Status    string     // 状态筛选
	FromTime  *time.Time // 开始时间
	ToTime    *time.Time // 结束时间
	SortBy    string     // 排序字段（block_time/created_at）
	SortOrder string     // 排序方向（asc/desc）
}

type Web3TransactionRepository interface {
	WithTx(tx *gorm.DB) Web3TransactionRepository
	GetDB() *gorm.DB

	// 创建交易记录
	Create(ctx context.Context, tx *model.Web3TransactionModel) error
	// 幂等 upsert（按 tx_hash 唯一键）
	Upsert(ctx context.Context, tx *model.Web3TransactionModel) error
	// 批量创建
	BatchCreate(ctx context.Context, txs []*model.Web3TransactionModel) error
	// 根据交易哈希查询
	FindByTxHash(ctx context.Context, txHash string) (*model.Web3TransactionModel, error)
	// 根据用户ID查询列表
	ListByUserID(ctx context.Context, userID int64, page, pageSize int32, filter Web3TransactionListFilter) ([]*model.Web3TransactionModel, int64, error)
	// 根据地址查询列表
	ListByAddress(ctx context.Context, address string, page, pageSize int32, filter Web3TransactionListFilter) ([]*model.Web3TransactionModel, int64, error)
	// 更新交易状态和确认数
	UpdateStatus(ctx context.Context, txHash string, status string, confirmations int32) error
	// 查询待确认的交易（用于定时更新）
	ListPendingTransactions(ctx context.Context, limit int32) ([]*model.Web3TransactionModel, error)
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

func (r *web3TransactionRepo) Create(ctx context.Context, tx *model.Web3TransactionModel) error {
	return r.db.WithContext(ctx).Create(tx).Error
}

func (r *web3TransactionRepo) Upsert(ctx context.Context, tx *model.Web3TransactionModel) error {
	if tx == nil {
		return nil
	}

	// Normalize key fields for deterministic uniqueness and safer querying.
	tx.TxHash = strings.TrimSpace(tx.TxHash)
	tx.UserAddress = strings.TrimSpace(tx.UserAddress)
	tx.Network = strings.ToUpper(strings.TrimSpace(tx.Network))
	tx.TxType = strings.TrimSpace(tx.TxType)
	tx.Direction = strings.TrimSpace(tx.Direction)
	tx.AssetCode = strings.ToUpper(strings.TrimSpace(tx.AssetCode))
	tx.Status = strings.TrimSpace(tx.Status)

	now := time.Now()
	if tx.CreatedAt == nil {
		tx.CreatedAt = &now
	}
	tx.UpdatedAt = &now

	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "tx_hash"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"web3_user_id":  tx.Web3UserID,
			"user_address":  tx.UserAddress,
			"network":       tx.Network,
			"chain_id":      tx.ChainID,
			"block_number":  tx.BlockNumber,
			"block_time":    tx.BlockTime,
			"tx_type":       tx.TxType,
			"direction":     tx.Direction,
			"asset_code":    tx.AssetCode,
			"amount":        tx.Amount,
			"amount_usd":    tx.AmountUSD,
			"from_address":  tx.FromAddress,
			"to_address":    tx.ToAddress,
			"status":        tx.Status,
			"confirmations": tx.Confirmations,
			"fee":           tx.Fee,
			"fee_asset":     tx.FeeAsset,
			"raw_data":      tx.RawData,
			"updated_at":    now,
		}),
	}).Create(tx).Error
}

func (r *web3TransactionRepo) BatchCreate(ctx context.Context, txs []*model.Web3TransactionModel) error {
	if len(txs) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(txs).Error
}

func (r *web3TransactionRepo) FindByTxHash(ctx context.Context, txHash string) (*model.Web3TransactionModel, error) {
	txHash = strings.TrimSpace(txHash)
	if txHash == "" {
		return nil, gorm.ErrRecordNotFound
	}

	var tx model.Web3TransactionModel
	err := r.db.WithContext(ctx).
		Where("tx_hash = ?", txHash).
		First(&tx).Error

	if err != nil {
		return nil, err
	}
	return &tx, nil
}

func (r *web3TransactionRepo) ListByUserID(ctx context.Context, userID int64, page, pageSize int32, filter Web3TransactionListFilter) ([]*model.Web3TransactionModel, int64, error) {
	if userID <= 0 {
		return nil, 0, nil
	}

	query := r.db.WithContext(ctx).Model(&model.Web3TransactionModel{}).Where("web3_user_id = ?", userID)
	query = applyWeb3TransactionFilter(query, filter)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	// 排序
	orderClause := getWeb3TransactionOrderClause(filter)
	offset := int((page - 1) * pageSize)

	var items []*model.Web3TransactionModel
	err := query.Order(orderClause).Offset(offset).Limit(int(pageSize)).Find(&items).Error
	return items, total, err
}

func (r *web3TransactionRepo) ListByAddress(ctx context.Context, address string, page, pageSize int32, filter Web3TransactionListFilter) ([]*model.Web3TransactionModel, int64, error) {
	address = strings.TrimSpace(address)
	if address == "" {
		return nil, 0, nil
	}

	query := r.db.WithContext(ctx).Model(&model.Web3TransactionModel{}).Where("user_address = ?", address)
	query = applyWeb3TransactionFilter(query, filter)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	// 排序
	orderClause := getWeb3TransactionOrderClause(filter)
	offset := int((page - 1) * pageSize)

	var items []*model.Web3TransactionModel
	err := query.Order(orderClause).Offset(offset).Limit(int(pageSize)).Find(&items).Error
	return items, total, err
}

func (r *web3TransactionRepo) UpdateStatus(ctx context.Context, txHash string, status string, confirmations int32) error {
	txHash = strings.TrimSpace(txHash)
	if txHash == "" {
		return gorm.ErrRecordNotFound
	}

	return r.db.WithContext(ctx).
		Model(&model.Web3TransactionModel{}).
		Where("tx_hash = ?", txHash).
		Updates(map[string]interface{}{
			"status":        status,
			"confirmations": confirmations,
			"updated_at":    time.Now(),
		}).Error
}

func (r *web3TransactionRepo) ListPendingTransactions(ctx context.Context, limit int32) ([]*model.Web3TransactionModel, error) {
	if limit <= 0 {
		limit = 100
	}

	var items []*model.Web3TransactionModel
	err := r.db.WithContext(ctx).
		Where("status = ?", model.Web3TxStatusPending).
		Order("created_at ASC").
		Limit(int(limit)).
		Find(&items).Error

	return items, err
}

// applyWeb3TransactionFilter 应用筛选条件
func applyWeb3TransactionFilter(q *gorm.DB, f Web3TransactionListFilter) *gorm.DB {
	if f.Address != "" {
		q = q.Where("user_address = ?", f.Address)
	}
	if f.Network != "" {
		q = q.Where("network = ?", f.Network)
	}
	if f.AssetCode != "" {
		q = q.Where("asset_code = ?", f.AssetCode)
	}
	if f.TxType != "" {
		q = q.Where("tx_type = ?", f.TxType)
	}
	if f.Direction != "" {
		q = q.Where("direction = ?", f.Direction)
	}
	if f.Status != "" {
		q = q.Where("status = ?", f.Status)
	}
	if f.FromTime != nil && !f.FromTime.IsZero() {
		q = q.Where("block_time >= ?", f.FromTime)
	}
	if f.ToTime != nil && !f.ToTime.IsZero() {
		q = q.Where("block_time <= ?", f.ToTime)
	}
	return q
}

// getWeb3TransactionOrderClause 获取排序子句
func getWeb3TransactionOrderClause(f Web3TransactionListFilter) string {
	orderCol := "block_time"
	if f.SortBy == "created_at" {
		orderCol = "created_at"
	}

	orderDir := "DESC"
	if strings.EqualFold(strings.TrimSpace(f.SortOrder), "asc") {
		orderDir = "ASC"
	}

	return orderCol + " " + orderDir
}
