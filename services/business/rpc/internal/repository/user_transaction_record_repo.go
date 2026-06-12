package repository

import (
	"context"
	"strings"

	"internalwallet/services/business/rpc/internal/model"

	"gorm.io/gorm"
)

type UserTransactionRecordRepository interface {
	WithTx(tx *gorm.DB) UserTransactionRecordRepository
	GetDB() *gorm.DB

	// 创建交易记录
	CreateRecord(ctx context.Context, tx *model.UserTransactionRecordModel) error
	// 根据交易哈希查询
	FindByTxHash(ctx context.Context, txHash string) (*model.UserTransactionRecordModel, error)
	// 根据用户ID查询列表
	ListByUserID(ctx context.Context, userID int64, page, pageSize int32) ([]*model.UserTransactionRecordModel, int64, error)
	// 根据用户ID和类型查询列表
	ListByUserIDAndType(ctx context.Context, userID int64, txType int32, page, pageSize int32) ([]*model.UserTransactionRecordModel, int64, error)
}

type userTransactionRecordRepo struct {
	db *gorm.DB
}

func NewUserTransactionRecordRepository(db *gorm.DB) UserTransactionRecordRepository {
	return &userTransactionRecordRepo{db: db}
}

func (r *userTransactionRecordRepo) WithTx(tx *gorm.DB) UserTransactionRecordRepository {
	return &userTransactionRecordRepo{db: tx}
}

func (r *userTransactionRecordRepo) GetDB() *gorm.DB {
	return r.db
}

func (r *userTransactionRecordRepo) CreateRecord(ctx context.Context, tx *model.UserTransactionRecordModel) error {
	// 设置兼容字段映射
	if tx.TxId != "" && tx.BizRef == "" {
		tx.BizRef = tx.TxId
	}
	if tx.Address != "" {
		if tx.Type == 1 { // deposit
			if tx.ToAddress == "" {
				tx.ToAddress = tx.Address
			}
		} else { // withdraw
			if tx.FromAddress == "" {
				tx.FromAddress = tx.Address
			}
		}
	}
	if tx.Network != "" && tx.ChainCode == "" {
		tx.ChainCode = tx.Network
	}
	if tx.Counterparty != "" {
		if tx.Type == 1 { // deposit
			if tx.FromAddress == "" {
				tx.FromAddress = tx.Counterparty
			}
		} else { // withdraw
			if tx.ToAddress == "" {
				tx.ToAddress = tx.Counterparty
			}
		}
	}

	return r.db.WithContext(ctx).Create(tx).Error
}

func (r *userTransactionRecordRepo) FindByTxHash(ctx context.Context, txHash string) (*model.UserTransactionRecordModel, error) {
	txHash = strings.TrimSpace(txHash)
	if txHash == "" {
		return nil, gorm.ErrRecordNotFound
	}

	var tx model.UserTransactionRecordModel
	err := r.db.WithContext(ctx).
		Where("tx_hash = ? AND deleted_at IS NULL", txHash).
		First(&tx).Error

	if err != nil {
		return nil, err
	}
	return &tx, nil
}

func (r *userTransactionRecordRepo) ListByUserID(ctx context.Context, userID int64, page, pageSize int32) ([]*model.UserTransactionRecordModel, int64, error) {
	if userID <= 0 {
		return nil, 0, nil
	}

	query := r.db.WithContext(ctx).Model(&model.UserTransactionRecordModel{}).
		Where("user_id = ? AND deleted_at IS NULL", userID)

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

	offset := int((page - 1) * pageSize)
	var items []*model.UserTransactionRecordModel
	err := query.Order("created_at DESC").Offset(offset).Limit(int(pageSize)).Find(&items).Error
	return items, total, err
}

func (r *userTransactionRecordRepo) ListByUserIDAndType(ctx context.Context, userID int64, txType int32, page, pageSize int32) ([]*model.UserTransactionRecordModel, int64, error) {
	if userID <= 0 {
		return nil, 0, nil
	}

	query := r.db.WithContext(ctx).Model(&model.UserTransactionRecordModel{}).
		Where("user_id = ? AND tx_type = ? AND deleted_at IS NULL", userID, txType)

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

	offset := int((page - 1) * pageSize)
	var items []*model.UserTransactionRecordModel
	err := query.Order("created_at DESC").Offset(offset).Limit(int(pageSize)).Find(&items).Error
	return items, total, err
}
