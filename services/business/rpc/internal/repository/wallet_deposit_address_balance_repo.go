package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"internalwallet/services/business/rpc/internal/model"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// WalletDepositAddressBalanceRepository 用户充值地址余额仓储接口
type WalletDepositAddressBalanceRepository interface {
	WithTx(tx *gorm.DB) WalletDepositAddressBalanceRepository
	GetDB() *gorm.DB

	// 基础操作
	Create(ctx context.Context, m *model.WalletDepositAddressBalanceModel) error
	FindByID(ctx context.Context, id int64) (*model.WalletDepositAddressBalanceModel, error)
	FindByDepositAddressAssetChain(ctx context.Context, depositAddressID int64, assetCode, chainCode string) (*model.WalletDepositAddressBalanceModel, error)
	UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error
	Upsert(ctx context.Context, m *model.WalletDepositAddressBalanceModel) error

	// 查询操作
	List(ctx context.Context, filter *DepositAddressBalanceFilter, page, pageSize int32) ([]*model.WalletDepositAddressBalanceModel, int64, error)
	FindByUserID(ctx context.Context, userID int64) ([]*model.WalletDepositAddressBalanceModel, error)
	FindByUserAssetChain(ctx context.Context, userID int64, assetCode, chainCode string) ([]*model.WalletDepositAddressBalanceModel, error)

	// 聚合统计
	GetTotalBalanceUSDByUser(ctx context.Context, userID int64) (int64, error)
	GetSweepCandidates(ctx context.Context, assetCode, chainCode string, limit int32) ([]*model.WalletDepositAddressBalanceModel, error)
	CountNeedsSweep(ctx context.Context, assetCode, chainCode string) (int64, error)

	// 归集相关
	MarkNeedsSweep(ctx context.Context, id int64, needsSweep bool) error
	UpdateSweepInfo(ctx context.Context, id int64, sweptAmountRaw int64, sweptAt time.Time) error
}

// DepositAddressBalanceFilter 充值地址余额筛选条件
type DepositAddressBalanceFilter struct {
	UserID           int64
	AssetCode        string
	ChainCode        string
	NeedsSweep       *bool   // nil=all, true=需要归集, false=不需要归集
	MinBalanceRaw    *string // 最小余额（原始值，DECIMAL(65,0)）
	MaxBalanceRaw    *string // 最大余额（原始值，DECIMAL(65,0)）
	MinBalanceUSDRaw *string // 最小USD余额（分，DECIMAL(65,0)）
	MaxBalanceUSDRaw *string // 最大USD余额（分，DECIMAL(65,0)）
	SyncStatus       string  // unknown/synced/syncing/error
	SortBy           string  // balance_raw/balance_usd_raw/last_synced_at/created_at
	SortOrder        string  // asc/desc
}

type walletDepositAddressBalanceRepo struct {
	db *gorm.DB
}

func NewWalletDepositAddressBalanceRepository(db *gorm.DB) WalletDepositAddressBalanceRepository {
	return &walletDepositAddressBalanceRepo{db: db}
}

func (r *walletDepositAddressBalanceRepo) WithTx(tx *gorm.DB) WalletDepositAddressBalanceRepository {
	return &walletDepositAddressBalanceRepo{db: tx}
}

func (r *walletDepositAddressBalanceRepo) GetDB() *gorm.DB {
	return r.db
}

func (r *walletDepositAddressBalanceRepo) Create(ctx context.Context, m *model.WalletDepositAddressBalanceModel) error {
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *walletDepositAddressBalanceRepo) FindByID(ctx context.Context, id int64) (*model.WalletDepositAddressBalanceModel, error) {
	var m model.WalletDepositAddressBalanceModel
	err := r.db.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", id).
		First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("deposit address balance not found")
	}
	return &m, err
}

func (r *walletDepositAddressBalanceRepo) FindByDepositAddressAssetChain(ctx context.Context, depositAddressID int64, assetCode, chainCode string) (*model.WalletDepositAddressBalanceModel, error) {
	assetCode = strings.ToUpper(strings.TrimSpace(assetCode))
	chainCode = strings.ToUpper(strings.TrimSpace(chainCode))
	if depositAddressID <= 0 || assetCode == "" || chainCode == "" {
		return nil, fmt.Errorf("deposit address balance not found")
	}
	var m model.WalletDepositAddressBalanceModel
	err := r.db.WithContext(ctx).
		Where("deposit_address_id = ? AND asset_code = ? AND chain_code = ? AND deleted_at IS NULL", depositAddressID, assetCode, chainCode).
		First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("deposit address balance not found")
	}
	return &m, err
}

func (r *walletDepositAddressBalanceRepo) UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error {
	res := r.db.WithContext(ctx).
		Model(&model.WalletDepositAddressBalanceModel{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(fields)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("deposit address balance not found")
	}
	return nil
}

// Upsert 创建或更新余额记录
func (r *walletDepositAddressBalanceRepo) Upsert(ctx context.Context, m *model.WalletDepositAddressBalanceModel) error {
	// 检查是否已存在
	existing, err := r.FindByDepositAddressAssetChain(ctx, m.DepositAddressID, m.AssetCode, m.ChainCode)
	if err != nil {
		// 不存在，创建新记录
		return r.Create(ctx, m)
	}

	// 已存在，更新余额
	now := time.Now()
	fields := map[string]interface{}{
		"balance":         m.Balance,
		"balance_raw":     m.BalanceRaw,
		"balance_usd":     m.BalanceUSD,
		"balance_usd_raw": m.BalanceUSDRaw,
		"last_synced_at":  m.LastSyncedAt,
		"sync_status":     m.SyncStatus,
		"sync_error":      m.SyncError,
		"updated_at":      &now,
	}

	// 检查是否需要归集（余额 > 阈值）
	// 使用 decimal 进行比较，因为 balance_raw 和 sweep_threshold_raw 都是 DECIMAL(65,0)（在 DB 中）和 string（在 Go model 中）
	balanceRaw, err1 := decimal.NewFromString(m.BalanceRaw)
	thresholdRaw, err2 := decimal.NewFromString(existing.SweepThresholdRaw)
	if err1 == nil && err2 == nil && balanceRaw.GreaterThan(thresholdRaw) {
		fields["needs_sweep"] = int8(1)
	} else {
		fields["needs_sweep"] = int8(0)
	}

	return r.UpdateFields(ctx, existing.ID, fields)
}

func (r *walletDepositAddressBalanceRepo) List(ctx context.Context, filter *DepositAddressBalanceFilter, page, pageSize int32) ([]*model.WalletDepositAddressBalanceModel, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	query := r.db.WithContext(ctx).Model(&model.WalletDepositAddressBalanceModel{}).Where("deleted_at IS NULL")

	if filter != nil {
		if filter.UserID > 0 {
			query = query.Where("user_id = ?", filter.UserID)
		}
		if strings.TrimSpace(filter.AssetCode) != "" {
			query = query.Where("asset_code = ?", strings.ToUpper(strings.TrimSpace(filter.AssetCode)))
		}
		if strings.TrimSpace(filter.ChainCode) != "" {
			query = query.Where("chain_code = ?", strings.ToUpper(strings.TrimSpace(filter.ChainCode)))
		}
		if filter.NeedsSweep != nil {
			if *filter.NeedsSweep {
				query = query.Where("needs_sweep = 1")
			} else {
				query = query.Where("needs_sweep = 0")
			}
		}
		if filter.MinBalanceRaw != nil {
			query = query.Where("balance_raw >= ?", *filter.MinBalanceRaw)
		}
		if filter.MaxBalanceRaw != nil {
			query = query.Where("balance_raw <= ?", *filter.MaxBalanceRaw)
		}
		if filter.MinBalanceUSDRaw != nil {
			query = query.Where("balance_usd_raw >= ?", *filter.MinBalanceUSDRaw)
		}
		if filter.MaxBalanceUSDRaw != nil {
			query = query.Where("balance_usd_raw <= ?", *filter.MaxBalanceUSDRaw)
		}
		if strings.TrimSpace(filter.SyncStatus) != "" {
			query = query.Where("sync_status = ?", strings.TrimSpace(filter.SyncStatus))
		}
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 排序
	orderCol := "created_at"
	if filter != nil && strings.TrimSpace(filter.SortBy) != "" {
		switch strings.TrimSpace(filter.SortBy) {
		case "balance_raw", "balance_usd_raw", "last_synced_at", "created_at":
			orderCol = strings.TrimSpace(filter.SortBy)
		}
	}
	orderDir := "DESC"
	if filter != nil && strings.EqualFold(strings.TrimSpace(filter.SortOrder), "asc") {
		orderDir = "ASC"
	}

	var items []*model.WalletDepositAddressBalanceModel
	offset := int((page - 1) * pageSize)
	err := query.Order(orderCol + " " + orderDir).Offset(offset).Limit(int(pageSize)).Find(&items).Error
	return items, total, err
}

func (r *walletDepositAddressBalanceRepo) FindByUserID(ctx context.Context, userID int64) ([]*model.WalletDepositAddressBalanceModel, error) {
	var items []*model.WalletDepositAddressBalanceModel
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND deleted_at IS NULL", userID).
		Order("balance_usd_raw DESC").
		Find(&items).Error
	return items, err
}

func (r *walletDepositAddressBalanceRepo) FindByUserAssetChain(ctx context.Context, userID int64, assetCode, chainCode string) ([]*model.WalletDepositAddressBalanceModel, error) {
	assetCode = strings.ToUpper(strings.TrimSpace(assetCode))
	chainCode = strings.ToUpper(strings.TrimSpace(chainCode))
	var items []*model.WalletDepositAddressBalanceModel
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND asset_code = ? AND chain_code = ? AND deleted_at IS NULL", userID, assetCode, chainCode).
		Order("balance_raw DESC").
		Find(&items).Error
	return items, err
}

func (r *walletDepositAddressBalanceRepo) GetTotalBalanceUSDByUser(ctx context.Context, userID int64) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).
		Model(&model.WalletDepositAddressBalanceModel{}).
		Where("user_id = ? AND deleted_at IS NULL", userID).
		Select("COALESCE(SUM(balance_usd_raw), 0)").
		Scan(&total).Error
	return total, err
}

func (r *walletDepositAddressBalanceRepo) GetSweepCandidates(ctx context.Context, assetCode, chainCode string, limit int32) ([]*model.WalletDepositAddressBalanceModel, error) {
	assetCode = strings.ToUpper(strings.TrimSpace(assetCode))
	chainCode = strings.ToUpper(strings.TrimSpace(chainCode))
	if limit <= 0 {
		limit = 50
	}
	if limit > 1000 {
		limit = 1000
	}

	var items []*model.WalletDepositAddressBalanceModel
	query := r.db.WithContext(ctx).
		Where("needs_sweep = 1 AND deleted_at IS NULL", assetCode, chainCode)

	if assetCode != "" {
		query = query.Where("asset_code = ?", assetCode)
	}
	if chainCode != "" {
		query = query.Where("chain_code = ?", chainCode)
	}

	err := query.
		Order("balance_raw DESC").
		Limit(int(limit)).
		Find(&items).Error
	return items, err
}

func (r *walletDepositAddressBalanceRepo) CountNeedsSweep(ctx context.Context, assetCode, chainCode string) (int64, error) {
	assetCode = strings.ToUpper(strings.TrimSpace(assetCode))
	chainCode = strings.ToUpper(strings.TrimSpace(chainCode))

	var count int64
	query := r.db.WithContext(ctx).
		Model(&model.WalletDepositAddressBalanceModel{}).
		Where("needs_sweep = 1 AND deleted_at IS NULL")

	if assetCode != "" {
		query = query.Where("asset_code = ?", assetCode)
	}
	if chainCode != "" {
		query = query.Where("chain_code = ?", chainCode)
	}

	err := query.Count(&count).Error
	return count, err
}

func (r *walletDepositAddressBalanceRepo) MarkNeedsSweep(ctx context.Context, id int64, needsSweep bool) error {
	val := int8(0)
	if needsSweep {
		val = 1
	}
	return r.UpdateFields(ctx, id, map[string]interface{}{
		"needs_sweep": val,
	})
}

func (r *walletDepositAddressBalanceRepo) UpdateSweepInfo(ctx context.Context, id int64, sweptAmountRaw int64, sweptAt time.Time) error {
	return r.UpdateFields(ctx, id, map[string]interface{}{
		"last_sweep_at":         &sweptAt,
		"last_sweep_amount_raw": sweptAmountRaw,
		"needs_sweep":           int8(0), // 归集后清除标记
	})
}
