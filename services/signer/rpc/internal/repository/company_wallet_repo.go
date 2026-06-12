package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	commonRepo "internalwallet/common/repository"
	"internalwallet/services/signer/rpc/internal/models"

	"gorm.io/gorm"
)

var (
	// ErrCompanyWalletNotFound 公司钱包地址不存在
	ErrCompanyWalletNotFound = errors.New("company wallet not found")

	// ErrCompanyWalletDuplicate 公司钱包地址重复
	ErrCompanyWalletDuplicate = errors.New("company wallet address already exists")
)

// CompanyWalletRepository 公司钱包数据访问接口
type CompanyWalletRepository interface {
	// 继承基础方法
	commonRepo.BaseRepository[models.CompanyWallet]

	// 查询方法
	FindByChainAndIndex(ctx context.Context, chain string, addressIndex int) (*models.CompanyWallet, error)
	FindByChainAndAddress(ctx context.Context, chain string, address string) (*models.CompanyWallet, error)
	FindByAddressType(ctx context.Context, addressType string, chain string) (*models.CompanyWallet, error)
	FindBySeedID(ctx context.Context, seedID string) ([]*models.CompanyWallet, error)
	FindByTemperature(ctx context.Context, temperature int8) ([]*models.CompanyWallet, error)
	ListByChain(ctx context.Context, chain string) ([]*models.CompanyWallet, error)
	ListActive(ctx context.Context) ([]*models.CompanyWallet, error)

	// 热钱包相关
	GetDefaultHotWallet(ctx context.Context, chain string) (*models.CompanyWallet, error)
	GetHotWalletPrimary(ctx context.Context, chain string) (*models.CompanyWallet, error)
	GetHotWalletBackup(ctx context.Context, chain string) (*models.CompanyWallet, error)
	ListHotWallets(ctx context.Context, chain string) ([]*models.CompanyWallet, error)

	// 冷钱包相关
	GetColdWalletPrimary(ctx context.Context, chain string) (*models.CompanyWallet, error)
	ListColdWallets(ctx context.Context, chain string) ([]*models.CompanyWallet, error)

	// 归集地址相关
	ListCollectionAddresses(ctx context.Context, chain string) ([]*models.CompanyWallet, error)

	// 更新方法
	UpdateStatus(ctx context.Context, id int64, status int8) error
	UpdateBalanceThreshold(ctx context.Context, id int64, threshold float64) error

	// 批量插入
	BatchCreate(ctx context.Context, wallets []*models.CompanyWallet) error

	// 检查地址是否存在
	ExistsByChainAndAddress(ctx context.Context, chain string, address string) (bool, error)

	// 多条件筛选查询（支持RPC接口）
	ListWithFilters(ctx context.Context, chain string, temperature int8, status int8, addressType string) ([]*models.CompanyWallet, error)
}

type companyWalletRepo struct {
	commonRepo.BaseRepository[models.CompanyWallet]
}

// NewCompanyWalletRepository 创建 CompanyWallet Repository
func NewCompanyWalletRepository(db *gorm.DB) CompanyWalletRepository {
	return &companyWalletRepo{
		BaseRepository: commonRepo.NewBaseRepository[models.CompanyWallet](db),
	}
}

// FindByChainAndIndex 根据链和索引查询
func (r *companyWalletRepo) FindByChainAndIndex(ctx context.Context, chain string, addressIndex int) (*models.CompanyWallet, error) {
	var wallet models.CompanyWallet
	err := r.GetDB().WithContext(ctx).
		Where("chain = ? AND address_index = ? AND status = 1", chain, addressIndex).
		First(&wallet).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrCompanyWalletNotFound
	}
	return &wallet, err
}

// FindByChainAndAddress 根据链和地址查询
func (r *companyWalletRepo) FindByChainAndAddress(ctx context.Context, chain string, address string) (*models.CompanyWallet, error) {
	var wallet models.CompanyWallet
	q := r.GetDB().WithContext(ctx).
		Model(&models.CompanyWallet{}).
		Where("chain = ? AND status = 1", chain)

	// EVM 地址大小写不敏感，TRON(Base58) 地址大小写敏感
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(address)), "0x") {
		q = q.Where("LOWER(address) = LOWER(?)", address)
	} else {
		q = q.Where("address = ?", address)
	}

	err := q.First(&wallet).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrCompanyWalletNotFound
	}
	return &wallet, err
}

// FindByAddressType 根据地址类型查询
func (r *companyWalletRepo) FindByAddressType(ctx context.Context, addressType string, chain string) (*models.CompanyWallet, error) {
	var wallet models.CompanyWallet
	err := r.GetDB().WithContext(ctx).
		Where("address_type = ? AND chain = ? AND status = 1", addressType, chain).
		First(&wallet).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrCompanyWalletNotFound
	}
	return &wallet, err
}

// FindBySeedID 根据种子ID查询
func (r *companyWalletRepo) FindBySeedID(ctx context.Context, seedID string) ([]*models.CompanyWallet, error) {
	var wallets []*models.CompanyWallet
	err := r.GetDB().WithContext(ctx).
		Where("seed_id = ? AND status = 1", seedID).
		Order("chain ASC, address_index ASC").
		Find(&wallets).Error
	return wallets, err
}

// FindByTemperature 根据温度查询（1=热, 2=冷）
func (r *companyWalletRepo) FindByTemperature(ctx context.Context, temperature int8) ([]*models.CompanyWallet, error) {
	var wallets []*models.CompanyWallet
	err := r.GetDB().WithContext(ctx).
		Where("temperature = ? AND status = 1", temperature).
		Order("chain ASC, address_index ASC").
		Find(&wallets).Error
	return wallets, err
}

// ListByChain 按链查询所有地址
func (r *companyWalletRepo) ListByChain(ctx context.Context, chain string) ([]*models.CompanyWallet, error) {
	var wallets []*models.CompanyWallet
	err := r.GetDB().WithContext(ctx).
		Where("chain = ? AND status = 1", chain).
		Order("address_index ASC").
		Find(&wallets).Error
	return wallets, err
}

// ListActive 查询所有启用的钱包
func (r *companyWalletRepo) ListActive(ctx context.Context) ([]*models.CompanyWallet, error) {
	var wallets []*models.CompanyWallet
	err := r.GetDB().WithContext(ctx).
		Where("status = 1").
		Order("chain ASC, address_index ASC").
		Find(&wallets).Error
	return wallets, err
}

// GetDefaultHotWallet 获取指定链的默认热钱包
// 如果有标记为 is_default=1 的热钱包，返回该钱包
// 否则返回该链的 hot_primary 地址作为兜底
func (r *companyWalletRepo) GetDefaultHotWallet(ctx context.Context, chain string) (*models.CompanyWallet, error) {
	var wallet models.CompanyWallet

	// 优先查询标记为默认的热钱包
	err := r.GetDB().WithContext(ctx).
		Where("chain = ? AND temperature = 1 AND is_default = 1 AND status = 1", chain).
		First(&wallet).Error

	if err == nil {
		return &wallet, nil
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("failed to query default hot wallet: %w", err)
	}

	// 如果没有标记为默认的，查询 hot_primary 作为兜底
	err = r.GetDB().WithContext(ctx).
		Where("chain = ? AND address_type = ? AND temperature = 1 AND status = 1", chain, "hot_primary").
		First(&wallet).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("no default hot wallet found for chain %s", chain)
	}

	return &wallet, err
}

// GetHotWalletPrimary 获取热钱包主地址
func (r *companyWalletRepo) GetHotWalletPrimary(ctx context.Context, chain string) (*models.CompanyWallet, error) {
	return r.FindByAddressType(ctx, "hot_primary", chain)
}

// GetHotWalletBackup 获取热钱包备用地址
func (r *companyWalletRepo) GetHotWalletBackup(ctx context.Context, chain string) (*models.CompanyWallet, error) {
	return r.FindByAddressType(ctx, "hot_backup", chain)
}

// ListHotWallets 获取所有热钱包地址
func (r *companyWalletRepo) ListHotWallets(ctx context.Context, chain string) ([]*models.CompanyWallet, error) {
	var wallets []*models.CompanyWallet
	err := r.GetDB().WithContext(ctx).
		Where("chain = ? AND temperature = 1 AND status = 1", chain).
		Order("address_index ASC").
		Find(&wallets).Error
	return wallets, err
}

// GetColdWalletPrimary 获取冷钱包主地址
func (r *companyWalletRepo) GetColdWalletPrimary(ctx context.Context, chain string) (*models.CompanyWallet, error) {
	return r.FindByAddressType(ctx, "cold_primary", chain)
}

// ListColdWallets 获取所有冷钱包地址
func (r *companyWalletRepo) ListColdWallets(ctx context.Context, chain string) ([]*models.CompanyWallet, error) {
	var wallets []*models.CompanyWallet
	err := r.GetDB().WithContext(ctx).
		Where("chain = ? AND temperature = 2 AND status = 1", chain).
		Order("address_index ASC").
		Find(&wallets).Error
	return wallets, err
}

// ListCollectionAddresses 获取归集地址
func (r *companyWalletRepo) ListCollectionAddresses(ctx context.Context, chain string) ([]*models.CompanyWallet, error) {
	var wallets []*models.CompanyWallet
	err := r.GetDB().WithContext(ctx).
		Where("chain = ? AND address_type LIKE ? AND status = 1", chain, "collection%").
		Order("address_index ASC").
		Find(&wallets).Error
	return wallets, err
}

// UpdateStatus 更新状态
func (r *companyWalletRepo) UpdateStatus(ctx context.Context, id int64, status int8) error {
	return r.GetDB().WithContext(ctx).
		Model(&models.CompanyWallet{}).
		Where("id = ?", id).
		Update("status", status).Error
}

// UpdateBalanceThreshold 更新余额阈值
func (r *companyWalletRepo) UpdateBalanceThreshold(ctx context.Context, id int64, threshold float64) error {
	return r.GetDB().WithContext(ctx).
		Model(&models.CompanyWallet{}).
		Where("id = ?", id).
		Update("balance_threshold", threshold).Error
}

// BatchCreate 批量创建
func (r *companyWalletRepo) BatchCreate(ctx context.Context, wallets []*models.CompanyWallet) error {
	if len(wallets) == 0 {
		return nil
	}
	return r.GetDB().WithContext(ctx).Create(&wallets).Error
}

// ExistsByChainAndAddress 检查地址是否已存在
func (r *companyWalletRepo) ExistsByChainAndAddress(ctx context.Context, chain string, address string) (bool, error) {
	var count int64
	q := r.GetDB().WithContext(ctx).
		Model(&models.CompanyWallet{}).
		Where("chain = ?", chain)

	// EVM 地址大小写不敏感，TRON(Base58) 地址大小写敏感
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(address)), "0x") {
		q = q.Where("LOWER(address) = LOWER(?)", address)
	} else {
		q = q.Where("address = ?", address)
	}

	err := q.Count(&count).Error

	if err != nil {
		return false, fmt.Errorf("failed to check address existence: %w", err)
	}
	return count > 0, nil
}

// ListWithFilters 多条件筛选查询
// chain: 链类型，空字符串表示不筛选
// temperature: 1=热钱包, 2=冷钱包, 0=不筛选
// status: 1=启用, 0=禁用, -1=所有
// addressType: 地址类型，空字符串表示不筛选
func (r *companyWalletRepo) ListWithFilters(ctx context.Context, chain string, temperature int8, status int8, addressType string) ([]*models.CompanyWallet, error) {
	query := r.GetDB().WithContext(ctx).Model(&models.CompanyWallet{})

	// 链类型筛选
	if chain != "" {
		query = query.Where("chain = ?", chain)
	}

	// 温度筛选
	if temperature > 0 {
		query = query.Where("temperature = ?", temperature)
	}

	// 状态筛选
	if status >= 0 {
		query = query.Where("status = ?", status)
	}

	// 地址类型筛选
	if addressType != "" {
		query = query.Where("address_type = ?", addressType)
	}

	var wallets []*models.CompanyWallet
	err := query.Order("chain ASC, address_index ASC").Find(&wallets).Error
	return wallets, err
}
