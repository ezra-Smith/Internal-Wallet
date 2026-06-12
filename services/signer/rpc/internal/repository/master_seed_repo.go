package repository

import (
	"context"
	"errors"
	commonRepo "internalwallet/common/repository"
	"time"

	"gorm.io/gorm"
	"internalwallet/services/signer/rpc/internal/models"
)

var (
	// ErrMasterSeedNotFound indicates that the requested master seed does not exist.
	ErrMasterSeedNotFound = errors.New("seed not found")
)

// MasterSeedRepository MasterSeed数据访问接口
type MasterSeedRepository interface {
	// 继承基础方法（自动获得所有通用 CRUD 方法）
	commonRepo.BaseRepository[models.MasterSeed]

	// 业务查询
	FindBySeedID(ctx context.Context, seedID string) (*models.MasterSeed, error)
	List(ctx context.Context, status int) ([]*models.MasterSeed, error)
	ExistsBySeedID(ctx context.Context, seedID string) (bool, error)
	FindActiveSeedsByChain(ctx context.Context, chain string) ([]*models.MasterSeed, error)

	// 业务更新
	UpdateStatus(ctx context.Context, seedID string, status int) error
	IncrementSignatureCount(ctx context.Context, seedID string) error

	// 统计查询
	CountByStatus(ctx context.Context, status int) (int64, error)
	GetTotalSignatures(ctx context.Context, seedID string) (int64, error)
}

type masterSeedRepo struct {
	commonRepo.BaseRepository[models.MasterSeed] // 组合基础 Repository
}

// NewMasterSeedRepository 创建MasterSeed Repository
func NewMasterSeedRepository(db *gorm.DB) MasterSeedRepository {
	return &masterSeedRepo{BaseRepository: commonRepo.NewBaseRepository[models.MasterSeed](db)}
}

// FindBySeedID 根据SeedID查询
func (r *masterSeedRepo) FindBySeedID(ctx context.Context, seedID string) (*models.MasterSeed, error) {
	var seed models.MasterSeed
	err := r.GetDB().WithContext(ctx).
		Where("seed_id = ?", seedID).
		First(&seed).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrMasterSeedNotFound
	}
	return &seed, err
}

// UpdateStatus 更新Seed状态
func (r *masterSeedRepo) UpdateStatus(ctx context.Context, seedID string, status int) error {
	return r.GetDB().WithContext(ctx).
		Model(&models.MasterSeed{}).
		Where("seed_id = ?", seedID).
		Update("status", status).Error
}

// List 查询Seed列表
func (r *masterSeedRepo) List(ctx context.Context, status int) ([]*models.MasterSeed, error) {
	var seeds []*models.MasterSeed
	query := r.GetDB().WithContext(ctx).
		Order("created_at DESC")

	if status > 0 {
		query = query.Where("status = ?", status)
	}

	err := query.Find(&seeds).Error
	return seeds, err
}

// ExistsBySeedID 检查SeedID是否存在
func (r *masterSeedRepo) ExistsBySeedID(ctx context.Context, seedID string) (bool, error) {
	var count int64
	err := r.GetDB().WithContext(ctx).
		Model(&models.MasterSeed{}).
		Where("seed_id = ?", seedID).
		Count(&count).Error
	return count > 0, err
}

// IncrementSignatureCount 增加签名计数
func (r *masterSeedRepo) IncrementSignatureCount(ctx context.Context, seedID string) error {
	return r.GetDB().WithContext(ctx).
		Model(&models.MasterSeed{}).
		Where("seed_id = ?", seedID).
		Updates(map[string]interface{}{
			"total_signatures":  gorm.Expr("total_signatures + 1"),
			"last_signature_at": time.Now().Local(),
		}).Error
}

// FindActiveSeedsByChain 查询支持指定链的活跃Seed
func (r *masterSeedRepo) FindActiveSeedsByChain(ctx context.Context, chain string) ([]*models.MasterSeed, error) {
	var seeds []*models.MasterSeed
	err := r.GetDB().WithContext(ctx).
		Where("status = 1").
		Where("JSON_CONTAINS(supported_chains, ?)", `"`+chain+`"`).
		Order("created_at DESC").
		Find(&seeds).Error
	return seeds, err
}

// CountByStatus 统计指定状态的Seed数量
func (r *masterSeedRepo) CountByStatus(ctx context.Context, status int) (int64, error) {
	var count int64
	err := r.GetDB().WithContext(ctx).
		Model(&models.MasterSeed{}).
		Where("status = ?", status).
		Count(&count).Error
	return count, err
}

// GetTotalSignatures 获取Seed的总签名次数
func (r *masterSeedRepo) GetTotalSignatures(ctx context.Context, seedID string) (int64, error) {
	var seed models.MasterSeed
	err := r.GetDB().WithContext(ctx).
		Select("total_signatures").
		Where("seed_id = ?", seedID).
		First(&seed).Error
	if err != nil {
		return 0, err
	}
	return seed.TotalSignatures, nil
}
