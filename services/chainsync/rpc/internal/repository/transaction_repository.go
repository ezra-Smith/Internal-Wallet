package repository

import (
	"context"
	"errors"
	"gorm.io/gorm"
	"internalwallet/services/chainsync/rpc/internal/model"
)

// TransactionRepository 交易仓库接口
type TransactionRepository interface {
	// 基础CRUD
	Create(ctx context.Context, tx *model.TransactionGorm) error
	GetByID(ctx context.Context, id int64) (*model.TransactionGorm, error)
	GetByHash(ctx context.Context, chain, hash string) (*model.TransactionGorm, error)
	Update(ctx context.Context, tx *model.TransactionGorm) error
	Delete(ctx context.Context, id int64) error

	// 查询方法
	GetTransactionsByAddress(ctx context.Context, chain, address string, limit, offset int) ([]*model.TransactionGorm, error)
	GetTransactionsByBlock(ctx context.Context, chain string, blockNumber uint64, limit, offset int) ([]*model.TransactionGorm, error)
	GetTransactionsByStatus(ctx context.Context, chain, status string, limit, offset int) ([]*model.TransactionGorm, error)
	GetTransactionsByRange(ctx context.Context, chain string, startBlock, endBlock uint64, limit, offset int) ([]*model.TransactionGorm, error)

	// 统计方法
	CountByAddress(ctx context.Context, chain, address string) (int64, error)
	CountByBlock(ctx context.Context, chain string, blockNumber uint64) (int64, error)
	CountByStatus(ctx context.Context, chain, status string) (int64, error)
}

// transactionRepository 交易仓库实现
type transactionRepository struct {
	db *gorm.DB
}

// NewTransactionRepository 创建交易仓库
func NewTransactionRepository(db *gorm.DB) TransactionRepository {
	return &transactionRepository{db: db}
}

// Create 创建交易记录
func (r *transactionRepository) Create(ctx context.Context, tx *model.TransactionGorm) error {
	return r.db.WithContext(ctx).Create(tx).Error
}

// GetByID 根据ID获取交易
func (r *transactionRepository) GetByID(ctx context.Context, id int64) (*model.TransactionGorm, error) {
	var transaction model.TransactionGorm
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&transaction).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, err
	}
	return &transaction, nil
}

// GetByHash 根据哈希获取交易
func (r *transactionRepository) GetByHash(ctx context.Context, chain, hash string) (*model.TransactionGorm, error) {
	var transaction model.TransactionGorm
	err := r.db.WithContext(ctx).Where("chain = ? AND tx_hash = ?", chain, hash).First(&transaction).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, err
	}
	return &transaction, nil
}

// Update 更新交易记录
func (r *transactionRepository) Update(ctx context.Context, tx *model.TransactionGorm) error {
	return r.db.WithContext(ctx).Model(tx).Where("id = ?", tx.ID).Updates(tx).Error
}

// Delete 软删除交易记录
func (r *transactionRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&model.TransactionGorm{}).Error
}

// GetTransactionsByAddress 根据地址获取交易
func (r *transactionRepository) GetTransactionsByAddress(ctx context.Context, chain, address string, limit, offset int) ([]*model.TransactionGorm, error) {
	var transactions []*model.TransactionGorm
	query := r.db.WithContext(ctx).Where("(from_address = ? OR to_address = ?) AND chain = ?", address, address, chain).
		Order("block_number DESC, transaction_index DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	err := query.Find(&transactions).Error
	return transactions, err
}

// GetTransactionsByBlock 根据区块获取交易
func (r *transactionRepository) GetTransactionsByBlock(ctx context.Context, chain string, blockNumber uint64, limit, offset int) ([]*model.TransactionGorm, error) {
	var transactions []*model.TransactionGorm
	query := r.db.WithContext(ctx).Where("chain = ? AND block_number = ?", chain, blockNumber).
		Order("transaction_index ASC")

	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	err := query.Find(&transactions).Error
	return transactions, err
}

// GetTransactionsByStatus 根据状态获取交易
func (r *transactionRepository) GetTransactionsByStatus(ctx context.Context, chain, status string, limit, offset int) ([]*model.TransactionGorm, error) {
	var transactions []*model.TransactionGorm
	query := r.db.WithContext(ctx).Where("chain = ? AND status = ?", chain, status).
		Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	err := query.Find(&transactions).Error
	return transactions, err
}

// GetTransactionsByRange 根据区块范围获取交易
func (r *transactionRepository) GetTransactionsByRange(ctx context.Context, chain string, startBlock, endBlock uint64, limit, offset int) ([]*model.TransactionGorm, error) {
	var transactions []*model.TransactionGorm
	query := r.db.WithContext(ctx).Where("chain = ? AND block_number BETWEEN ? AND ?", chain, startBlock, endBlock).
		Order("block_number ASC, transaction_index ASC")

	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	err := query.Find(&transactions).Error
	return transactions, err
}

// CountByAddress 统计指定地址的交易数量
func (r *transactionRepository) CountByAddress(ctx context.Context, chain, address string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.TransactionGorm{}).
		Where("(from_address = ? OR to_address = ?) AND chain = ?", address, address, chain).
		Count(&count).Error
	return count, err
}

// CountByBlock 统计指定区块的交易数量
func (r *transactionRepository) CountByBlock(ctx context.Context, chain string, blockNumber uint64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.TransactionGorm{}).
		Where("chain = ? AND block_number = ?", chain, blockNumber).
		Count(&count).Error
	return count, err
}

// CountByStatus 统计指定状态的交易数量
func (r *transactionRepository) CountByStatus(ctx context.Context, chain, status string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.TransactionGorm{}).
		Where("chain = ? AND status = ?", chain, status).
		Count(&count).Error
	return count, err
}
