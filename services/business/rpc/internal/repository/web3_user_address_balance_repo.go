package repository

import (
	"context"
	"errors"
	"fmt"
	"internalwallet/services/business/rpc/internal/model"
	"strings"

	"gorm.io/gorm"
)

// Web3UserAddressBalanceRepository Web3用户地址余额仓储接口
type Web3UserAddressBalanceRepository interface {
	WithTx(tx *gorm.DB) Web3UserAddressBalanceRepository
	GetDB() *gorm.DB

	// FindByAddress 根据地址查询所有余额（按USD估值排序）
	// network 参数可选，为空则查询所有网络，否则只查询指定网络
	FindByAddress(ctx context.Context, address string, network string) ([]*model.Web3UserAddressBalanceModel, error)

	// FindByAddressAndAsset 根据地址和币种查询单个余额
	FindByAddressAndAsset(ctx context.Context, address string, assetCode string, chainCode string) (*model.Web3UserAddressBalanceModel, error)

	// Create 创建余额记录
	Create(ctx context.Context, balance *model.Web3UserAddressBalanceModel) error

	// Update 更新余额记录
	Update(ctx context.Context, balance *model.Web3UserAddressBalanceModel) error

	// UpdateBalance 更新余额（只更新余额相关字段）
	UpdateBalance(ctx context.Context, id int64, balance string, balanceRaw int64, balanceUSD string, balanceUSDRaw int64) error

	// Upsert 创建或更新余额记录（根据 asset_code + chain_code + wallet_address 唯一键）
	Upsert(ctx context.Context, balance *model.Web3UserAddressBalanceModel) error
}

// Web3UserAssetSummary 用户资产汇总（价格和Logo从accounting/market服务获取）
type Web3UserAssetSummary struct {
	AssetCode       string `gorm:"column:asset_code"`
	TotalBalanceRaw int64  `gorm:"column:total_balance_raw"`
	TotalBalanceUSD int64  `gorm:"column:total_balance_usd_raw"`
}

type web3UserAddressBalanceRepo struct {
	db *gorm.DB
}

func NewWeb3UserAddressBalanceRepository(db *gorm.DB) Web3UserAddressBalanceRepository {
	return &web3UserAddressBalanceRepo{db: db}
}

func (r *web3UserAddressBalanceRepo) WithTx(tx *gorm.DB) Web3UserAddressBalanceRepository {
	return &web3UserAddressBalanceRepo{db: tx}
}

func (r *web3UserAddressBalanceRepo) GetDB() *gorm.DB {
	return r.db
}

// FindByAddress 根据地址查询余额（按USD估值排序，过滤垃圾币）
// network 参数可选，为空则查询所有网络，否则只查询指定网络
func (r *web3UserAddressBalanceRepo) FindByAddress(ctx context.Context, address string, network string) ([]*model.Web3UserAddressBalanceModel, error) {
	address = strings.TrimSpace(address)
	network = strings.TrimSpace(network)

	if address == "" {
		return nil, fmt.Errorf("address cannot be empty")
	}

	var items []*model.Web3UserAddressBalanceModel
	query := r.db.WithContext(ctx).
		Model(&model.Web3UserAddressBalanceModel{}).
		Where("wallet_address = ? AND deleted_at IS NULL AND is_spam = 0", address)

	// 如果指定了 network，添加网络过滤条件
	if network != "" {
		// 统一按 chain_code 过滤（network 字段历史上可能存在大小写/命名不一致）
		query = query.Where("chain_code = ?", strings.ToUpper(network))
	}

	// 注意：即使 balance_usd_raw=0 也必须返回，否则上层会用“0余额占位项”覆盖真实余额（尤其是稳定币）
	err := query.Order("balance_usd_raw DESC, balance_raw DESC").Find(&items).Error

	return items, err
}

// FindByAddressAndAsset 根据地址和币种查询单个余额
func (r *web3UserAddressBalanceRepo) FindByAddressAndAsset(ctx context.Context, address string, assetCode string, chainCode string) (*model.Web3UserAddressBalanceModel, error) {
	address = strings.TrimSpace(address)
	assetCode = strings.TrimSpace(assetCode)
	chainCode = strings.TrimSpace(chainCode)

	if address == "" || assetCode == "" || chainCode == "" {
		return nil, fmt.Errorf("address, asset_code and chain_code cannot be empty")
	}

	var m model.Web3UserAddressBalanceModel
	err := r.db.WithContext(ctx).
		Model(&model.Web3UserAddressBalanceModel{}).
		Where("wallet_address = ? AND asset_code = ? AND chain_code = ? AND deleted_at IS NULL", address, assetCode, chainCode).
		First(&m).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("balance not found")
	}

	return &m, err
}

// Create 创建余额记录
func (r *web3UserAddressBalanceRepo) Create(ctx context.Context, balance *model.Web3UserAddressBalanceModel) error {
	return r.db.WithContext(ctx).Create(balance).Error
}

// Update 更新余额记录
func (r *web3UserAddressBalanceRepo) Update(ctx context.Context, balance *model.Web3UserAddressBalanceModel) error {
	return r.db.WithContext(ctx).Save(balance).Error
}

// UpdateBalance 更新余额（只更新余额相关字段）
func (r *web3UserAddressBalanceRepo) UpdateBalance(ctx context.Context, id int64, balance string, balanceRaw int64, balanceUSD string, balanceUSDRaw int64) error {
	return r.db.WithContext(ctx).
		Model(&model.Web3UserAddressBalanceModel{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"balance":            balance,
			"balance_raw":        balanceRaw,
			"balance_usd":        balanceUSD,
			"balance_usd_raw":    balanceUSDRaw,
			"balance_updated_at": gorm.Expr("NOW()"),
		}).Error
}

// Upsert 创建或更新余额记录（根据 asset_code + chain_code + wallet_address 唯一键）
// 如果记录存在则更新余额，否则创建新记录
func (r *web3UserAddressBalanceRepo) Upsert(ctx context.Context, balance *model.Web3UserAddressBalanceModel) error {
	if balance.WalletAddress == "" || balance.AssetCode == "" || balance.ChainCode == "" {
		return fmt.Errorf("wallet_address, asset_code and chain_code are required")
	}

	// 先尝试查找现有记录
	var existing model.Web3UserAddressBalanceModel
	err := r.db.WithContext(ctx).
		Model(&model.Web3UserAddressBalanceModel{}).
		Where("wallet_address = ? AND asset_code = ? AND chain_code = ? AND deleted_at IS NULL",
			balance.WalletAddress, balance.AssetCode, balance.ChainCode).
		First(&existing).Error

	if err == nil {
		// 记录存在，更新余额相关字段
		return r.db.WithContext(ctx).
			Model(&model.Web3UserAddressBalanceModel{}).
			Where("id = ?", existing.ID).
			Updates(map[string]interface{}{
				"balance":            balance.Balance,
				"balance_raw":        balance.BalanceRaw,
				"balance_usd":        balance.BalanceUSD,
				"balance_usd_raw":    balance.BalanceUSDRaw,
				"token_decimals":     balance.TokenDecimals,
				"balance_updated_at": gorm.Expr("NOW()"),
				"last_synced_at":     gorm.Expr("NOW()"),
				"sync_status":        "synced",
			}).Error
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		// 记录不存在，创建新记录
		balance.SyncStatus = "synced"
		return r.db.WithContext(ctx).Create(balance).Error
	}

	return err
}
