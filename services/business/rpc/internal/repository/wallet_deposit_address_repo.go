package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"internalwallet/services/business/rpc/internal/model"

	"gorm.io/gorm"
)

type WalletDepositAddressRepository interface {
	WithTx(tx *gorm.DB) WalletDepositAddressRepository
	GetDB() *gorm.DB

	FindActiveByUserAndChain(ctx context.Context, userID int64, chainCode string) (*model.WalletDepositAddressModel, error)
	FindActiveByAddress(ctx context.Context, address string) (*model.WalletDepositAddressModel, error)
	FindActiveByAddressAndChain(ctx context.Context, address string, chainCode string) (*model.WalletDepositAddressModel, error)
	ListActiveByUser(ctx context.Context, userID int64) ([]*model.WalletDepositAddressModel, error)
}

type walletDepositAddressRepo struct {
	db *gorm.DB
}

func NewWalletDepositAddressRepository(db *gorm.DB) WalletDepositAddressRepository {
	return &walletDepositAddressRepo{db: db}
}

func (r *walletDepositAddressRepo) WithTx(tx *gorm.DB) WalletDepositAddressRepository {
	return &walletDepositAddressRepo{db: tx}
}

func (r *walletDepositAddressRepo) GetDB() *gorm.DB { return r.db }

func (r *walletDepositAddressRepo) FindActiveByUserAndChain(ctx context.Context, userID int64, chainCode string) (*model.WalletDepositAddressModel, error) {
	chainCode = strings.ToUpper(strings.TrimSpace(chainCode))
	if userID <= 0 || chainCode == "" {
		return nil, fmt.Errorf("deposit address not found")
	}

	var m model.WalletDepositAddressModel
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND chain_code = ? AND status = ? AND deleted_at IS NULL", userID, chainCode, "active").
		Order("is_default DESC, id DESC").
		First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("deposit address not found")
	}
	return &m, err
}

func (r *walletDepositAddressRepo) FindActiveByAddress(ctx context.Context, address string) (*model.WalletDepositAddressModel, error) {
	address = strings.TrimSpace(address)
	if address == "" {
		return nil, fmt.Errorf("deposit address not found")
	}

	var m model.WalletDepositAddressModel
	q := r.db.WithContext(ctx).
		Model(&model.WalletDepositAddressModel{}).
		Where("status = ? AND deleted_at IS NULL", "active")

	// EVM 地址大小写不敏感，TRON(Base58) 地址大小写敏感
	if strings.HasPrefix(strings.ToLower(address), "0x") {
		q = q.Where("LOWER(address) = LOWER(?)", address)
	} else {
		q = q.Where("address = ?", address)
	}

	err := q.Order("id DESC").First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("deposit address not found")
	}
	return &m, err
}

func (r *walletDepositAddressRepo) FindActiveByAddressAndChain(ctx context.Context, address string, chainCode string) (*model.WalletDepositAddressModel, error) {
	address = strings.TrimSpace(address)
	chainCode = strings.ToUpper(strings.TrimSpace(chainCode))
	if address == "" || chainCode == "" {
		return nil, fmt.Errorf("deposit address not found")
	}

	var m model.WalletDepositAddressModel
	q := r.db.WithContext(ctx).
		Model(&model.WalletDepositAddressModel{}).
		Where("chain_code = ? AND status = ? AND deleted_at IS NULL", chainCode, "active")

	// EVM 地址大小写不敏感，TRON(Base58) 地址大小写敏感
	if strings.HasPrefix(strings.ToLower(address), "0x") {
		q = q.Where("LOWER(address) = LOWER(?)", address)
	} else {
		q = q.Where("address = ?", address)
	}

	err := q.Order("id DESC").First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("deposit address not found")
	}
	return &m, err
}

func (r *walletDepositAddressRepo) ListActiveByUser(ctx context.Context, userID int64) ([]*model.WalletDepositAddressModel, error) {
	if userID <= 0 {
		return nil, nil
	}
	var items []*model.WalletDepositAddressModel
	err := r.db.WithContext(ctx).
		Model(&model.WalletDepositAddressModel{}).
		Where("user_id = ? AND status = ? AND deleted_at IS NULL", userID, "active").
		Order("is_default DESC, id DESC").
		Find(&items).Error
	return items, err
}
