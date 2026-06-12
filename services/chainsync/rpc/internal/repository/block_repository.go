package repository

import (
	"context"
	"errors"
	"gorm.io/gorm"
	"internalwallet/services/chainsync/rpc/internal/model"
)

// BlockRepository 区块仓库接口
type BlockRepository interface {
	// 基础CRUD
	Create(ctx context.Context, block *model.BlockGorm) error
	GetByID(ctx context.Context, id int64) (*model.BlockGorm, error)
	GetByChainAndNumber(ctx context.Context, chainID int64, blockNumber uint64) (*model.BlockGorm, error)
	GetByHash(ctx context.Context, chainID int64, hash string) (*model.BlockGorm, error)
	Update(ctx context.Context, block *model.BlockGorm) error
	Delete(ctx context.Context, id int64) error

	// 查询方法
	GetLatestBlock(ctx context.Context, chainID int64) (*model.BlockGorm, error)
	GetBlocksByRange(ctx context.Context, chainID int64, startBlock, endBlock uint64, limit int) ([]*model.BlockGorm, error)
	GetBlocksByStatus(ctx context.Context, chainID int64, status string, limit int) ([]*model.BlockGorm, error)

	// 统计方法
	CountByChain(ctx context.Context, chainID int64) (int64, error)
	CountByStatus(ctx context.Context, chainID int64, status string) (int64, error)
}

// blockRepository 区块仓库实现
type blockRepository struct {
	db *gorm.DB
}

// NewBlockRepository 创建区块仓库
func NewBlockRepository(db *gorm.DB) BlockRepository {
	return &blockRepository{db: db}
}

// Create 创建区块记录
func (r *blockRepository) Create(ctx context.Context, block *model.BlockGorm) error {
	return r.db.WithContext(ctx).Create(block).Error
}

// GetByID 根据ID获取区块
func (r *blockRepository) GetByID(ctx context.Context, id int64) (*model.BlockGorm, error) {
	var block model.BlockGorm
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&block).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, err
	}
	return &block, nil
}

// GetByChainAndNumber 根据链和区块号获取区块
func (r *blockRepository) GetByChainAndNumber(ctx context.Context, chainID int64, blockNumber uint64) (*model.BlockGorm, error) {
	var block model.BlockGorm
	err := r.db.WithContext(ctx).Where("chain_id = ? AND block_number = ?", chainID, blockNumber).First(&block).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, err
	}
	return &block, nil
}

// GetByHash 根据哈希获取区块
func (r *blockRepository) GetByHash(ctx context.Context, chainID int64, hash string) (*model.BlockGorm, error) {
	var block model.BlockGorm
	err := r.db.WithContext(ctx).Where("chain_id = ? AND block_hash = ?", chainID, hash).First(&block).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, err
	}
	return &block, nil
}

// Update 更新区块记录
func (r *blockRepository) Update(ctx context.Context, block *model.BlockGorm) error {
	return r.db.WithContext(ctx).Model(block).Where("id = ?", block.ID).Updates(block).Error
}

// Delete 软删除区块记录
func (r *blockRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&model.BlockGorm{}).Error
}

// GetLatestBlock 获取最新区块
func (r *blockRepository) GetLatestBlock(ctx context.Context, chainID int64) (*model.BlockGorm, error) {
	var block model.BlockGorm
	err := r.db.WithContext(ctx).Where("chain_id = ?", chainID).Order("block_number DESC").First(&block).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, err
	}
	return &block, nil
}

// GetBlocksByRange 获取指定范围内的区块
func (r *blockRepository) GetBlocksByRange(ctx context.Context, chainID int64, startBlock, endBlock uint64, limit int) ([]*model.BlockGorm, error) {
	var blocks []*model.BlockGorm
	query := r.db.WithContext(ctx).Where("chain_id = ? AND block_number BETWEEN ? AND ?", chainID, startBlock, endBlock).Order("block_number ASC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Find(&blocks).Error
	return blocks, err
}

// GetBlocksByStatus 根据状态获取区块
func (r *blockRepository) GetBlocksByStatus(ctx context.Context, chainID int64, status string, limit int) ([]*model.BlockGorm, error) {
	var blocks []*model.BlockGorm
	query := r.db.WithContext(ctx).Where("chain_id = ? AND sync_status = ?", chainID, status).Order("block_number DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Find(&blocks).Error
	return blocks, err
}

// CountByChain 统计指定链的区块数量
func (r *blockRepository) CountByChain(ctx context.Context, chainID int64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.BlockGorm{}).Where("chain_id = ?", chainID).Count(&count).Error
	return count, err
}

// CountByStatus 统计指定链和状态的区块数量
func (r *blockRepository) CountByStatus(ctx context.Context, chainID int64, status string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.BlockGorm{}).Where("chain_id = ? AND sync_status = ?", chainID, status).Count(&count).Error
	return count, err
}
