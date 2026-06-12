package repository

import (
	"context"
	"errors"
	commonRepo "internalwallet/common/repository"
	"strings"

	"gorm.io/gorm"
	"internalwallet/services/signer/rpc/internal/models"
)

// AddressGenerationLogRepository 地址生成日志数据访问接口
// 只增不改，用于审计和追溯
type AddressGenerationLogRepository interface {
	// 继承基础方法（自动获得所有通用 CRUD 方法）
	commonRepo.BaseRepository[models.AddressGenerationLog]

	// 业务查询
	FindByUserIDAndChain(ctx context.Context, userID int64, chain string) (*models.AddressGenerationLog, error)
	FindByUserIDChainAndPath(ctx context.Context, userID int64, chain string, derivationPath string) (*models.AddressGenerationLog, error)
	FindByAddress(ctx context.Context, address string) (*models.AddressGenerationLog, error)
	FindByAddressAndChain(ctx context.Context, address string, chain string) (*models.AddressGenerationLog, error)
	FindByUserID(ctx context.Context, userID int64) ([]*models.AddressGenerationLog, error)
	FindByUserIDWithFilter(ctx context.Context, userID int64, chains []string) ([]*models.AddressGenerationLog, error)
	ListByChain(ctx context.Context, chain string, page, pageSize int) ([]*models.AddressGenerationLog, int64, error)
	ListWithFilter(ctx context.Context, chains []string, page, pageSize int) ([]*models.AddressGenerationLog, int64, error)

	// 地址管理
	ExistsByUserIDAndChain(ctx context.Context, userID int64, chain string) (bool, error)
	GetMaxAddressIndex(ctx context.Context, chain string) (int, error)

	// 统计查询
	CountByChain(ctx context.Context, chain string) (int64, error)
	CountByGeneratedBy(ctx context.Context, generatedBy string) (int64, error)
}

type addressGenerationLogRepo struct {
	commonRepo.BaseRepository[models.AddressGenerationLog] // 组合基础 Repository
}

// NewAddressGenerationLogRepository 创建 AddressGenerationLog Repository
func NewAddressGenerationLogRepository(db *gorm.DB) AddressGenerationLogRepository {
	return &addressGenerationLogRepo{
		BaseRepository: commonRepo.NewBaseRepository[models.AddressGenerationLog](db),
	}
}

// FindByUserIDAndChain 根据用户ID和链查询地址
func (r *addressGenerationLogRepo) FindByUserIDAndChain(ctx context.Context, userID int64, chain string) (*models.AddressGenerationLog, error) {
	var address models.AddressGenerationLog
	err := r.GetDB().WithContext(ctx).
		Where("user_id = ? AND chain = ?", userID, chain).
		First(&address).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errors.New("address generation log not found")
	}
	return &address, err
}

// FindByUserIDChainAndPath 根据用户ID、链和派生路径查询
func (r *addressGenerationLogRepo) FindByUserIDChainAndPath(ctx context.Context, userID int64, chain string, derivationPath string) (*models.AddressGenerationLog, error) {
	var log models.AddressGenerationLog
	err := r.GetDB().WithContext(ctx).
		Where("user_id = ? AND chain = ? AND derivation_path = ?", userID, chain, derivationPath).
		First(&log).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errors.New("address generation log not found")
	}
	return &log, err
}

// FindByAddress 根据地址查询
func (r *addressGenerationLogRepo) FindByAddress(ctx context.Context, address string) (*models.AddressGenerationLog, error) {
	var log models.AddressGenerationLog
	err := r.GetDB().WithContext(ctx).
		Where("address = ?", address).
		First(&log).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errors.New("address generation log not found")
	}
	return &log, err
}

// FindByAddressAndChain 根据地址和链类型查询（性能更好）
func (r *addressGenerationLogRepo) FindByAddressAndChain(ctx context.Context, address string, chain string) (*models.AddressGenerationLog, error) {
	var log models.AddressGenerationLog
	q := r.GetDB().WithContext(ctx).
		Model(&models.AddressGenerationLog{}).
		Where("chain = ?", chain)

	// EVM 地址大小写不敏感，TRON(Base58) 地址大小写敏感
	if strings.HasPrefix(strings.ToLower(address), "0x") {
		q = q.Where("LOWER(address) = LOWER(?)", address)
	} else {
		q = q.Where("address = ?", address)
	}

	err := q.First(&log).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errors.New("address generation log not found")
	}
	return &log, err
}

// FindByUserID 查询用户的所有生成地址
func (r *addressGenerationLogRepo) FindByUserID(ctx context.Context, userID int64) ([]*models.AddressGenerationLog, error) {
	var logs []*models.AddressGenerationLog
	err := r.GetDB().WithContext(ctx).
		Where("user_id = ?", userID).
		Order("chain ASC, created_at DESC").
		Find(&logs).Error
	return logs, err
}

// FindByUserIDWithFilter 根据用户ID查询地址（支持链筛选）
func (r *addressGenerationLogRepo) FindByUserIDWithFilter(ctx context.Context, userID int64, chains []string) ([]*models.AddressGenerationLog, error) {
	var logs []*models.AddressGenerationLog
	query := r.GetDB().WithContext(ctx).
		Where("user_id = ?", userID)

	// 如果指定了链，添加链筛选
	if len(chains) > 0 {
		query = query.Where("chain IN ?", chains)
	}

	err := query.Order("chain ASC, created_at DESC").Find(&logs).Error
	return logs, err
}

// ListByChain 分页查询指定链的地址生成日志
func (r *addressGenerationLogRepo) ListByChain(ctx context.Context, chain string, page, pageSize int) ([]*models.AddressGenerationLog, int64, error) {
	var logs []*models.AddressGenerationLog
	var total int64

	query := r.GetDB().WithContext(ctx).
		Model(&models.AddressGenerationLog{}).
		Where("chain = ?", chain)

	// 计算总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}

	offset := (page - 1) * pageSize
	err := query.Offset(offset).
		Limit(pageSize).
		Order("created_at DESC").
		Find(&logs).Error

	return logs, total, err
}

// ListWithFilter 分页查询地址生成日志（支持多链筛选）
func (r *addressGenerationLogRepo) ListWithFilter(ctx context.Context, chains []string, page, pageSize int) ([]*models.AddressGenerationLog, int64, error) {
	var logs []*models.AddressGenerationLog
	var total int64

	query := r.GetDB().WithContext(ctx).
		Model(&models.AddressGenerationLog{})

	// 如果指定了链，添加链筛选
	if len(chains) > 0 {
		query = query.Where("chain IN ?", chains)
	}

	// 计算总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页参数处理
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20 // 默认20
	}
	if pageSize > 100 {
		pageSize = 100 // 最大100
	}

	offset := (page - 1) * pageSize
	err := query.Offset(offset).
		Limit(pageSize).
		Order("created_at DESC").
		Find(&logs).Error

	return logs, total, err
}

// ExistsByUserIDAndChain 检查用户在指定链上是否已有地址
func (r *addressGenerationLogRepo) ExistsByUserIDAndChain(ctx context.Context, userID int64, chain string) (bool, error) {
	var count int64
	err := r.GetDB().WithContext(ctx).
		Model(&models.AddressGenerationLog{}).
		Where("user_id = ? AND chain = ?", userID, chain).
		Count(&count).Error
	return count > 0, err
}

// GetMaxAddressIndex 获取指定链的最大地址索引
func (r *addressGenerationLogRepo) GetMaxAddressIndex(ctx context.Context, chain string) (int, error) {
	var result struct {
		MaxIndex int
	}
	err := r.GetDB().WithContext(ctx).
		Model(&models.AddressGenerationLog{}).
		Select("COALESCE(MAX(address_index), 0) as max_index").
		Where("chain = ?", chain).
		Scan(&result).Error
	return result.MaxIndex, err
}

// CountByChain 统计指定链的地址数量
func (r *addressGenerationLogRepo) CountByChain(ctx context.Context, chain string) (int64, error) {
	var count int64
	err := r.GetDB().WithContext(ctx).
		Model(&models.AddressGenerationLog{}).
		Where("chain = ?", chain).
		Count(&count).Error
	return count, err
}

// CountByGeneratedBy 统计指定来源生成的地址数量
func (r *addressGenerationLogRepo) CountByGeneratedBy(ctx context.Context, generatedBy string) (int64, error) {
	var count int64
	err := r.GetDB().WithContext(ctx).
		Model(&models.AddressGenerationLog{}).
		Where("generated_by = ?", generatedBy).
		Count(&count).Error
	return count, err
}
