package repository

import (
	"context"
	"fmt"
	"strings"

	"internalwallet/services/accounting/rpc/internal/model"

	"gorm.io/gorm"
)

type AccountTypeRepository interface {
	WithTx(tx *gorm.DB) AccountTypeRepository
	GetDB() *gorm.DB

	Create(ctx context.Context, m *model.AcctAccountTypeModel) error
	Update(ctx context.Context, code string, fields map[string]interface{}) error
	SoftDelete(ctx context.Context, code string) error
	FindByCode(ctx context.Context, code string, includeDeleted bool) (*model.AcctAccountTypeModel, error)
	List(ctx context.Context, includeDeleted bool) ([]model.AcctAccountTypeModel, error)

	ListAssets(ctx context.Context, accountTypeCode string) ([]string, error)
	ReplaceAssets(ctx context.Context, accountTypeCode string, assetCodes []string) error
}

type accountTypeRepo struct {
	db *gorm.DB
}

func NewAccountTypeRepository(db *gorm.DB) AccountTypeRepository    { return &accountTypeRepo{db: db} }
func (r *accountTypeRepo) WithTx(tx *gorm.DB) AccountTypeRepository { return &accountTypeRepo{db: tx} }
func (r *accountTypeRepo) GetDB() *gorm.DB                          { return r.db }

func (r *accountTypeRepo) Create(ctx context.Context, m *model.AcctAccountTypeModel) error {
	if m == nil {
		return fmt.Errorf("invalid account type")
	}
	m.Code = strings.ToUpper(strings.TrimSpace(m.Code))
	m.Name = strings.TrimSpace(m.Name)
	m.Description = strings.TrimSpace(m.Description)
	m.NormalSide = strings.ToLower(strings.TrimSpace(m.NormalSide))
	if m.Code == "" || (m.NormalSide != "debit" && m.NormalSide != "credit") {
		return fmt.Errorf("invalid account type")
	}
	if len(m.Description) > 255 {
		return fmt.Errorf("description too long")
	}
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *accountTypeRepo) Update(ctx context.Context, code string, fields map[string]interface{}) error {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return fmt.Errorf("code required")
	}
	if len(fields) == 0 {
		return nil
	}
	res := r.db.WithContext(ctx).
		Model(&model.AcctAccountTypeModel{}).
		Where("code = ?", code).
		Updates(fields)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("account type not found")
	}
	return nil
}

func (r *accountTypeRepo) SoftDelete(ctx context.Context, code string) error {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return fmt.Errorf("code required")
	}
	res := r.db.WithContext(ctx).
		Where("code = ?", code).
		Delete(&model.AcctAccountTypeModel{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("account type not found")
	}
	return nil
}

func (r *accountTypeRepo) FindByCode(ctx context.Context, code string, includeDeleted bool) (*model.AcctAccountTypeModel, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var m model.AcctAccountTypeModel
	q := r.db.WithContext(ctx).Model(&model.AcctAccountTypeModel{})
	if includeDeleted {
		q = q.Unscoped()
	}
	q = q.Where("code = ?", code)
	if err := q.First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *accountTypeRepo) List(ctx context.Context, includeDeleted bool) ([]model.AcctAccountTypeModel, error) {
	var items []model.AcctAccountTypeModel
	q := r.db.WithContext(ctx).Model(&model.AcctAccountTypeModel{})
	if includeDeleted {
		q = q.Unscoped()
	}
	if err := q.Order("code ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *accountTypeRepo) ListAssets(ctx context.Context, accountTypeCode string) ([]string, error) {
	accountTypeCode = strings.ToUpper(strings.TrimSpace(accountTypeCode))
	if accountTypeCode == "" {
		return nil, nil
	}
	var rows []model.AcctAccountTypeAssetModel
	err := r.db.WithContext(ctx).
		Model(&model.AcctAccountTypeAssetModel{}).
		Where("account_type_code = ?", accountTypeCode).
		Order("asset_code ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.AssetCode)
	}
	return out, nil
}

func (r *accountTypeRepo) ReplaceAssets(ctx context.Context, accountTypeCode string, assetCodes []string) error {
	accountTypeCode = strings.ToUpper(strings.TrimSpace(accountTypeCode))
	if accountTypeCode == "" {
		return fmt.Errorf("account_type_code required")
	}
	// Normalize and de-duplicate.
	seen := map[string]struct{}{}
	normalized := make([]string, 0, len(assetCodes))
	for _, a := range assetCodes {
		a = strings.ToUpper(strings.TrimSpace(a))
		if a == "" {
			continue
		}
		if _, ok := seen[a]; ok {
			continue
		}
		seen[a] = struct{}{}
		normalized = append(normalized, a)
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Ensure account type exists (even if assets empty).
		if err := tx.Model(&model.AcctAccountTypeModel{}).Where("code = ?", accountTypeCode).First(&model.AcctAccountTypeModel{}).Error; err != nil {
			return err
		}

		if err := tx.Unscoped().Where("account_type_code = ?", accountTypeCode).Delete(&model.AcctAccountTypeAssetModel{}).Error; err != nil {
			return err
		}
		if len(normalized) == 0 {
			return nil
		}
		rows := make([]*model.AcctAccountTypeAssetModel, 0, len(normalized))
		for _, a := range normalized {
			rows = append(rows, &model.AcctAccountTypeAssetModel{
				AccountTypeCode: accountTypeCode,
				AssetCode:       a,
			})
		}
		return tx.CreateInBatches(rows, 200).Error
	})
}
