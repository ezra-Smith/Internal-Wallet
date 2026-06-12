package repository

import (
	"context"
	"time"

	"internalwallet/services/admin/rpc/internal/model"

	"gorm.io/gorm"
)

// WalletDepositAddressBalanceChangeRepository 充值地址余额变动仓储接口
type WalletDepositAddressBalanceChangeRepository interface {
	WithTx(tx *gorm.DB) WalletDepositAddressBalanceChangeRepository
	GetDB() *gorm.DB

	// 基础操作
	Create(ctx context.Context, m *model.WalletDepositAddressBalanceChangeModel) error
	FindByID(ctx context.Context, id int64) (*model.WalletDepositAddressBalanceChangeModel, error)
	UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error

	// 查询操作
	FindByTxHash(ctx context.Context, txHash string) (*model.WalletDepositAddressBalanceChangeModel, error)
	FindBySweepTaskNo(ctx context.Context, taskNo string) ([]*model.WalletDepositAddressBalanceChangeModel, error)
	ListByDepositAddress(ctx context.Context, depositAddressID int64, offset, limit int32) ([]*model.WalletDepositAddressBalanceChangeModel, int64, error)
	ListByUser(ctx context.Context, userID int64, offset, limit int32) ([]*model.WalletDepositAddressBalanceChangeModel, int64, error)
	ListByAssetChain(ctx context.Context, assetCode, chainCode string, offset, limit int32) ([]*model.WalletDepositAddressBalanceChangeModel, int64, error)
	ListByChangeType(ctx context.Context, changeType string, offset, limit int32) ([]*model.WalletDepositAddressBalanceChangeModel, int64, error)
	ListByStatus(ctx context.Context, status string, offset, limit int32) ([]*model.WalletDepositAddressBalanceChangeModel, int64, error)

	// 范围查询
	ListByTimeRange(ctx context.Context, startTime, endTime time.Time, offset, limit int32) ([]*model.WalletDepositAddressBalanceChangeModel, int64, error)
	ListByBlockRange(ctx context.Context, fromBlock, toBlock int64, offset, limit int32) ([]*model.WalletDepositAddressBalanceChangeModel, int64, error)

	// 统计操作
	CountByDepositAddress(ctx context.Context, depositAddressID int64) (int64, error)
	CountByChangeType(ctx context.Context, changeType string) (int64, error)
	GetDailyStats(ctx context.Context, date time.Time, assetCode, chainCode string) (map[string]interface{}, error)

	// 高级查询
	List(ctx context.Context, filter *BalanceChangeFilter, offset, limit int32) ([]*model.WalletDepositAddressBalanceChangeModel, int64, error)
}

// BalanceChangeFilter 余额变动筛选条件
type BalanceChangeFilter struct {
	DepositAddressID int64
	UserID           int64
	AssetCode        string
	ChainCode        string
	ChangeType       string
	Status           string
	Source           string
	IsPermanent      *bool
	StartTime        *time.Time
	EndTime          *time.Time
	MinChangeAmount  *int64
	MaxChangeAmount  *int64
	SweepTaskNo      *string
	TxHash           *string
	SortBy           string // created_at/change_amount_raw/balance_raw
	SortOrder        string // asc/desc
}

type walletDepositAddressBalanceChangeRepo struct {
	db *gorm.DB
}

// NewWalletDepositAddressBalanceChangeRepository 创建仓储实例
func NewWalletDepositAddressBalanceChangeRepository(db *gorm.DB) WalletDepositAddressBalanceChangeRepository {
	return &walletDepositAddressBalanceChangeRepo{db: db}
}

func (r *walletDepositAddressBalanceChangeRepo) WithTx(tx *gorm.DB) WalletDepositAddressBalanceChangeRepository {
	return &walletDepositAddressBalanceChangeRepo{db: tx}
}

func (r *walletDepositAddressBalanceChangeRepo) GetDB() *gorm.DB {
	return r.db
}

// Create 创建新的变动记录
func (r *walletDepositAddressBalanceChangeRepo) Create(ctx context.Context, m *model.WalletDepositAddressBalanceChangeModel) error {
	return r.db.WithContext(ctx).Create(m).Error
}

// FindByID 按ID查询
func (r *walletDepositAddressBalanceChangeRepo) FindByID(ctx context.Context, id int64) (*model.WalletDepositAddressBalanceChangeModel, error) {
	var m model.WalletDepositAddressBalanceChangeModel
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&m).Error
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// UpdateFields 更新指定字段
func (r *walletDepositAddressBalanceChangeRepo) UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error {
	return r.db.WithContext(ctx).Model(&model.WalletDepositAddressBalanceChangeModel{}).Where("id = ?", id).Updates(fields).Error
}

// FindByTxHash 按交易哈希查询
func (r *walletDepositAddressBalanceChangeRepo) FindByTxHash(ctx context.Context, txHash string) (*model.WalletDepositAddressBalanceChangeModel, error) {
	var m model.WalletDepositAddressBalanceChangeModel
	err := r.db.WithContext(ctx).Where("tx_hash = ?", txHash).First(&m).Error
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// FindBySweepTaskNo 按归集任务编号查询
func (r *walletDepositAddressBalanceChangeRepo) FindBySweepTaskNo(ctx context.Context, taskNo string) ([]*model.WalletDepositAddressBalanceChangeModel, error) {
	var records []*model.WalletDepositAddressBalanceChangeModel
	err := r.db.WithContext(ctx).Where("sweep_task_no = ?", taskNo).Find(&records).Error
	return records, err
}

// ListByDepositAddress 查询指定充值地址的所有变动
func (r *walletDepositAddressBalanceChangeRepo) ListByDepositAddress(ctx context.Context, depositAddressID int64, offset, limit int32) ([]*model.WalletDepositAddressBalanceChangeModel, int64, error) {
	var records []*model.WalletDepositAddressBalanceChangeModel
	var total int64

	err := r.db.WithContext(ctx).
		Where("deposit_address_id = ? AND deleted_at IS NULL", depositAddressID).
		Order("created_at DESC").
		Offset(int(offset)).
		Limit(int(limit)).
		Find(&records).
		Count(&total).Error

	return records, total, err
}

// ListByUser 查询指定用户的所有充值地址变动
func (r *walletDepositAddressBalanceChangeRepo) ListByUser(ctx context.Context, userID int64, offset, limit int32) ([]*model.WalletDepositAddressBalanceChangeModel, int64, error) {
	var records []*model.WalletDepositAddressBalanceChangeModel
	var total int64

	err := r.db.WithContext(ctx).
		Where("user_id = ? AND deleted_at IS NULL", userID).
		Order("created_at DESC").
		Offset(int(offset)).
		Limit(int(limit)).
		Find(&records).
		Count(&total).Error

	return records, total, err
}

// ListByAssetChain 查询指定币种和链的变动
func (r *walletDepositAddressBalanceChangeRepo) ListByAssetChain(ctx context.Context, assetCode, chainCode string, offset, limit int32) ([]*model.WalletDepositAddressBalanceChangeModel, int64, error) {
	var records []*model.WalletDepositAddressBalanceChangeModel
	var total int64

	err := r.db.WithContext(ctx).
		Where("asset_code = ? AND chain_code = ? AND deleted_at IS NULL", assetCode, chainCode).
		Order("created_at DESC").
		Offset(int(offset)).
		Limit(int(limit)).
		Find(&records).
		Count(&total).Error

	return records, total, err
}

// ListByChangeType 按变动类型查询
func (r *walletDepositAddressBalanceChangeRepo) ListByChangeType(ctx context.Context, changeType string, offset, limit int32) ([]*model.WalletDepositAddressBalanceChangeModel, int64, error) {
	var records []*model.WalletDepositAddressBalanceChangeModel
	var total int64

	err := r.db.WithContext(ctx).
		Where("change_type = ? AND deleted_at IS NULL", changeType).
		Order("created_at DESC").
		Offset(int(offset)).
		Limit(int(limit)).
		Find(&records).
		Count(&total).Error

	return records, total, err
}

// ListByStatus 按状态查询
func (r *walletDepositAddressBalanceChangeRepo) ListByStatus(ctx context.Context, status string, offset, limit int32) ([]*model.WalletDepositAddressBalanceChangeModel, int64, error) {
	var records []*model.WalletDepositAddressBalanceChangeModel
	var total int64

	err := r.db.WithContext(ctx).
		Where("status = ? AND deleted_at IS NULL", status).
		Order("created_at DESC").
		Offset(int(offset)).
		Limit(int(limit)).
		Find(&records).
		Count(&total).Error

	return records, total, err
}

// ListByTimeRange 按时间范围查询
func (r *walletDepositAddressBalanceChangeRepo) ListByTimeRange(ctx context.Context, startTime, endTime time.Time, offset, limit int32) ([]*model.WalletDepositAddressBalanceChangeModel, int64, error) {
	var records []*model.WalletDepositAddressBalanceChangeModel
	var total int64

	err := r.db.WithContext(ctx).
		Where("created_at BETWEEN ? AND ? AND deleted_at IS NULL", startTime, endTime).
		Order("created_at DESC").
		Offset(int(offset)).
		Limit(int(limit)).
		Find(&records).
		Count(&total).Error

	return records, total, err
}

// ListByBlockRange 按块高度范围查询
func (r *walletDepositAddressBalanceChangeRepo) ListByBlockRange(ctx context.Context, fromBlock, toBlock int64, offset, limit int32) ([]*model.WalletDepositAddressBalanceChangeModel, int64, error) {
	var records []*model.WalletDepositAddressBalanceChangeModel
	var total int64

	err := r.db.WithContext(ctx).
		Where("block_number BETWEEN ? AND ? AND deleted_at IS NULL", fromBlock, toBlock).
		Order("block_number DESC").
		Offset(int(offset)).
		Limit(int(limit)).
		Find(&records).
		Count(&total).Error

	return records, total, err
}

// CountByDepositAddress 统计指定地址的变动数
func (r *walletDepositAddressBalanceChangeRepo) CountByDepositAddress(ctx context.Context, depositAddressID int64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.WalletDepositAddressBalanceChangeModel{}).
		Where("deposit_address_id = ? AND deleted_at IS NULL", depositAddressID).
		Count(&count).Error
	return count, err
}

// CountByChangeType 统计指定变动类型的数量
func (r *walletDepositAddressBalanceChangeRepo) CountByChangeType(ctx context.Context, changeType string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.WalletDepositAddressBalanceChangeModel{}).
		Where("change_type = ? AND deleted_at IS NULL", changeType).
		Count(&count).Error
	return count, err
}

// GetDailyStats 获取日统计数据
func (r *walletDepositAddressBalanceChangeRepo) GetDailyStats(ctx context.Context, date time.Time, assetCode, chainCode string) (map[string]interface{}, error) {
	var result []map[string]interface{}

	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	endOfDay := startOfDay.Add(24 * time.Hour)

	err := r.db.WithContext(ctx).
		Model(&model.WalletDepositAddressBalanceChangeModel{}).
		Where("asset_code = ? AND chain_code = ? AND created_at BETWEEN ? AND ? AND deleted_at IS NULL",
			assetCode, chainCode, startOfDay, endOfDay).
		Select(
			"change_type",
			"COUNT(*) as count",
			"SUM(change_amount_raw) as total_amount_raw",
			"SUM(change_amount_usd_raw) as total_amount_usd_raw",
			"COUNT(DISTINCT deposit_address_id) as unique_addresses",
		).
		Group("change_type").
		Scan(&result).Error

	if err != nil {
		return nil, err
	}

	stats := make(map[string]interface{})
	stats["date"] = date.Format("2006-01-02")
	stats["asset_code"] = assetCode
	stats["chain_code"] = chainCode
	stats["details"] = result

	return stats, nil
}

// List 通用查询接口
func (r *walletDepositAddressBalanceChangeRepo) List(ctx context.Context, filter *BalanceChangeFilter, offset, limit int32) ([]*model.WalletDepositAddressBalanceChangeModel, int64, error) {
	query := r.db.WithContext(ctx)

	// 应用筛选条件
	if filter.DepositAddressID > 0 {
		query = query.Where("deposit_address_id = ?", filter.DepositAddressID)
	}
	if filter.UserID > 0 {
		query = query.Where("user_id = ?", filter.UserID)
	}
	if filter.AssetCode != "" {
		query = query.Where("asset_code = ?", filter.AssetCode)
	}
	if filter.ChainCode != "" {
		query = query.Where("chain_code = ?", filter.ChainCode)
	}
	if filter.ChangeType != "" {
		query = query.Where("change_type = ?", filter.ChangeType)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.Source != "" {
		query = query.Where("source = ?", filter.Source)
	}
	if filter.IsPermanent != nil {
		query = query.Where("is_permanent = ?", *filter.IsPermanent)
	}
	if filter.StartTime != nil {
		query = query.Where("created_at >= ?", *filter.StartTime)
	}
	if filter.EndTime != nil {
		query = query.Where("created_at <= ?", *filter.EndTime)
	}
	if filter.MinChangeAmount != nil {
		query = query.Where("change_amount_raw >= ?", *filter.MinChangeAmount)
	}
	if filter.MaxChangeAmount != nil {
		query = query.Where("change_amount_raw <= ?", *filter.MaxChangeAmount)
	}
	if filter.SweepTaskNo != nil {
		query = query.Where("sweep_task_no = ?", *filter.SweepTaskNo)
	}
	if filter.TxHash != nil {
		query = query.Where("tx_hash = ?", *filter.TxHash)
	}

	// 加上软删除条件
	query = query.Where("deleted_at IS NULL")

	var records []*model.WalletDepositAddressBalanceChangeModel
	var total int64

	// 应用排序
	sortBy := "created_at"
	if filter.SortBy != "" {
		sortBy = filter.SortBy
	}
	sortOrder := "DESC"
	if filter.SortOrder == "asc" {
		sortOrder = "ASC"
	}

	err := query.
		Order(sortBy + " " + sortOrder).
		Offset(int(offset)).
		Limit(int(limit)).
		Find(&records).
		Count(&total).Error

	return records, total, err
}
