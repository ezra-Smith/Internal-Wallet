package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"internalwallet/services/signer/rpc/internal/models"

	"gorm.io/gorm"
)

var (
	ErrWalletMasterMnemonicNotFound = errors.New("wallet master mnemonic not found")
)

type WalletMasterMnemonicRepository interface {
	FindBySeedID(ctx context.Context, seedID string) (*models.WalletMasterMnemonic, error)
	ExistsBySeedID(ctx context.Context, seedID string) (bool, error)
	Create(ctx context.Context, m *models.WalletMasterMnemonic) error
	MarkBackupConfirmed(ctx context.Context, seedID string, confirmedAt time.Time) error
}

type walletMasterMnemonicRepository struct {
	db *gorm.DB
}

func NewWalletMasterMnemonicRepository(db *gorm.DB) WalletMasterMnemonicRepository {
	return &walletMasterMnemonicRepository{db: db}
}

func (r *walletMasterMnemonicRepository) FindBySeedID(ctx context.Context, seedID string) (*models.WalletMasterMnemonic, error) {
	seedID = strings.TrimSpace(seedID)
	if seedID == "" {
		return nil, ErrWalletMasterMnemonicNotFound
	}
	var out models.WalletMasterMnemonic
	err := r.db.WithContext(ctx).
		Where("deleted_at IS NULL").
		Where("seed_id = ?", seedID).
		Limit(1).
		Find(&out).Error
	if err != nil {
		return nil, err
	}
	if out.ID == 0 {
		return nil, ErrWalletMasterMnemonicNotFound
	}
	return &out, nil
}

func (r *walletMasterMnemonicRepository) ExistsBySeedID(ctx context.Context, seedID string) (bool, error) {
	seedID = strings.TrimSpace(seedID)
	if seedID == "" {
		return false, nil
	}
	var cnt int64
	err := r.db.WithContext(ctx).
		Model(&models.WalletMasterMnemonic{}).
		Where("deleted_at IS NULL").
		Where("seed_id = ?", seedID).
		Count(&cnt).Error
	return cnt > 0, err
}

func (r *walletMasterMnemonicRepository) Create(ctx context.Context, m *models.WalletMasterMnemonic) error {
	if m == nil {
		return errors.New("wallet master mnemonic is nil")
	}
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *walletMasterMnemonicRepository) MarkBackupConfirmed(ctx context.Context, seedID string, confirmedAt time.Time) error {
	seedID = strings.TrimSpace(seedID)
	if seedID == "" {
		return errors.New("seed_id is empty")
	}
	return r.db.WithContext(ctx).
		Model(&models.WalletMasterMnemonic{}).
		Where("deleted_at IS NULL").
		Where("seed_id = ?", seedID).
		Update("backup_confirmed_at", confirmedAt).Error
}
