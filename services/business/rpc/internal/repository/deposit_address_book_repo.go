package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"internalwallet/services/business/rpc/internal/model"

	"gorm.io/gorm"
)

var ErrDepositAddressNotFound = errors.New("deposit address not found")

type DepositAddressBookRepository interface {
	WithTx(tx *gorm.DB) DepositAddressBookRepository
	GetDB() *gorm.DB

	CountByUserAndChain(ctx context.Context, userID int64, chainCode string) (int64, error)
	CountActiveByUserAndChain(ctx context.Context, userID int64, chainCode string) (int64, error)
	GetMaxDerivationChange(ctx context.Context, userID int64, chainCode string) (int, error)
	FindActiveByID(ctx context.Context, userID int64, id int64) (*model.WalletDepositAddressModel, error)
	ListActiveByUser(ctx context.Context, userID int64, chainCode string, page, pageSize int32) ([]*model.WalletDepositAddressModel, int64, error)
	Create(ctx context.Context, m *model.WalletDepositAddressModel) error
	UpdateLabelRemark(ctx context.Context, userID int64, id int64, label *string, remark *string) error
	SoftDelete(ctx context.Context, userID int64, id int64) error
	SetDefaultByID(ctx context.Context, userID int64, chainCode string, id int64) error
}

type depositAddressBookRepo struct {
	db *gorm.DB
}

func NewDepositAddressBookRepository(db *gorm.DB) DepositAddressBookRepository {
	return &depositAddressBookRepo{db: db}
}

func (r *depositAddressBookRepo) WithTx(tx *gorm.DB) DepositAddressBookRepository {
	return &depositAddressBookRepo{db: tx}
}

func (r *depositAddressBookRepo) GetDB() *gorm.DB { return r.db }

func (r *depositAddressBookRepo) CountByUserAndChain(ctx context.Context, userID int64, chainCode string) (int64, error) {
	chainCode = strings.ToUpper(strings.TrimSpace(chainCode))
	if userID <= 0 || chainCode == "" {
		return 0, nil
	}
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.WalletDepositAddressModel{}).
		Where("user_id = ? AND chain_code = ?", userID, chainCode).
		Count(&count).Error
	return count, err
}

func (r *depositAddressBookRepo) CountActiveByUserAndChain(ctx context.Context, userID int64, chainCode string) (int64, error) {
	chainCode = strings.ToUpper(strings.TrimSpace(chainCode))
	if userID <= 0 || chainCode == "" {
		return 0, nil
	}
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.WalletDepositAddressModel{}).
		Where("user_id = ? AND chain_code = ? AND status = ? AND deleted_at IS NULL", userID, chainCode, "active").
		Count(&count).Error
	return count, err
}

func (r *depositAddressBookRepo) GetMaxDerivationChange(ctx context.Context, userID int64, chainCode string) (int, error) {
	chainCode = strings.ToUpper(strings.TrimSpace(chainCode))
	if userID <= 0 || chainCode == "" {
		return 0, nil
	}
	var maxChange int
	if err := r.db.WithContext(ctx).
		Model(&model.WalletDepositAddressModel{}).
		Select("COALESCE(MAX(derivation_change), 0)").
		Where("user_id = ? AND chain_code = ?", userID, chainCode).
		Scan(&maxChange).Error; err != nil {
		return 0, err
	}
	return maxChange, nil
}

func (r *depositAddressBookRepo) FindActiveByID(ctx context.Context, userID int64, id int64) (*model.WalletDepositAddressModel, error) {
	if userID <= 0 || id <= 0 {
		return nil, fmt.Errorf("%w", ErrDepositAddressNotFound)
	}
	var m model.WalletDepositAddressModel
	err := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ? AND status = ? AND deleted_at IS NULL", id, userID, "active").
		First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("%w", ErrDepositAddressNotFound)
	}
	return &m, err
}

func (r *depositAddressBookRepo) ListActiveByUser(ctx context.Context, userID int64, chainCode string, page, pageSize int32) ([]*model.WalletDepositAddressModel, int64, error) {
	chainCode = strings.ToUpper(strings.TrimSpace(chainCode))
	if userID <= 0 {
		return nil, 0, nil
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

	q := r.db.WithContext(ctx).Model(&model.WalletDepositAddressModel{}).
		Where("user_id = ? AND status = ? AND deleted_at IS NULL", userID, "active")
	if chainCode != "" {
		q = q.Where("chain_code = ?", chainCode)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []*model.WalletDepositAddressModel
	offset := int((page - 1) * pageSize)
	err := q.Order("is_default DESC, id DESC").Offset(offset).Limit(int(pageSize)).Find(&items).Error
	return items, total, err
}

func (r *depositAddressBookRepo) Create(ctx context.Context, m *model.WalletDepositAddressModel) error {
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *depositAddressBookRepo) UpdateLabelRemark(ctx context.Context, userID int64, id int64, label *string, remark *string) error {
	if userID <= 0 || id <= 0 {
		return fmt.Errorf("%w", ErrDepositAddressNotFound)
	}
	now := time.Now()
	fields := map[string]interface{}{
		"label":      label,
		"remark":     remark,
		"updated_at": &now,
	}
	res := r.db.WithContext(ctx).
		Model(&model.WalletDepositAddressModel{}).
		Where("id = ? AND user_id = ? AND status = ? AND deleted_at IS NULL", id, userID, "active").
		Updates(fields)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("%w", ErrDepositAddressNotFound)
	}
	return nil
}

func (r *depositAddressBookRepo) SoftDelete(ctx context.Context, userID int64, id int64) error {
	if userID <= 0 || id <= 0 {
		return fmt.Errorf("%w", ErrDepositAddressNotFound)
	}
	now := time.Now()
	res := r.db.WithContext(ctx).
		Model(&model.WalletDepositAddressModel{}).
		Where("id = ? AND user_id = ? AND status = ? AND deleted_at IS NULL", id, userID, "active").
		Updates(map[string]interface{}{
			"status":     "inactive",
			"is_default": 0,
			"deleted_at": &now,
			"updated_at": &now,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("%w", ErrDepositAddressNotFound)
	}
	return nil
}

func (r *depositAddressBookRepo) SetDefaultByID(ctx context.Context, userID int64, chainCode string, id int64) error {
	chainCode = strings.ToUpper(strings.TrimSpace(chainCode))
	if userID <= 0 || chainCode == "" || id <= 0 {
		return fmt.Errorf("%w", ErrDepositAddressNotFound)
	}
	now := time.Now()

	// NOTE: cannot rely on a single multi-row UPDATE with CASE here.
	// The table has a UNIQUE constraint on generated column `default_user_chain`.
	// A multi-row UPDATE may trigger a transient duplicate default state and fail.
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1) Clear defaults for this user+chain.
		if err := tx.Model(&model.WalletDepositAddressModel{}).
			Where("user_id = ? AND chain_code = ? AND status = ? AND deleted_at IS NULL", userID, chainCode, "active").
			Updates(map[string]interface{}{
				"is_default": 0,
				"updated_at": &now,
			}).Error; err != nil {
			return err
		}

		// 2) Set the target row as default.
		res := tx.Model(&model.WalletDepositAddressModel{}).
			Where("id = ? AND user_id = ? AND chain_code = ? AND status = ? AND deleted_at IS NULL", id, userID, chainCode, "active").
			Updates(map[string]interface{}{
				"is_default": 1,
				"updated_at": &now,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return fmt.Errorf("%w", ErrDepositAddressNotFound)
		}
		return nil
	})
}
