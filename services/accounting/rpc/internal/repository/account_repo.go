package repository

import (
	"context"
	"fmt"
	"strings"

	"internalwallet/services/accounting/rpc/internal/model"

	"gorm.io/gorm"
)

type AccountRepository interface {
	WithTx(tx *gorm.DB) AccountRepository
	GetDB() *gorm.DB

	FindByOwnerType(ctx context.Context, ownerType string, ownerID int64, accountTypeCode string, chainScope string) (*model.AcctAccountModel, error)
	FindByIDs(ctx context.Context, ids []int64) ([]model.AcctAccountModel, error)
	Ensure(ctx context.Context, m *model.AcctAccountModel) (*model.AcctAccountModel, error)
}

type accountRepo struct {
	db *gorm.DB
}

func NewAccountRepository(db *gorm.DB) AccountRepository { return &accountRepo{db: db} }
func (r *accountRepo) WithTx(tx *gorm.DB) AccountRepository {
	return &accountRepo{db: tx}
}
func (r *accountRepo) GetDB() *gorm.DB { return r.db }

func (r *accountRepo) FindByOwnerType(ctx context.Context, ownerType string, ownerID int64, accountTypeCode string, chainScope string) (*model.AcctAccountModel, error) {
	ownerType = strings.ToLower(strings.TrimSpace(ownerType))
	accountTypeCode = strings.ToUpper(strings.TrimSpace(accountTypeCode))
	chainScope = strings.ToUpper(strings.TrimSpace(chainScope))
	if ownerType == "" || accountTypeCode == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var m model.AcctAccountModel
	err := r.db.WithContext(ctx).
		Model(&model.AcctAccountModel{}).
		Where("owner_type = ? AND owner_id = ? AND account_type_code = ? AND chain_scope = ?",
			ownerType, ownerID, accountTypeCode, chainScope,
		).
		First(&m).Error
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *accountRepo) FindByIDs(ctx context.Context, ids []int64) ([]model.AcctAccountModel, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	seen := map[int64]struct{}{}
	normalized := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		normalized = append(normalized, id)
	}
	if len(normalized) == 0 {
		return nil, nil
	}
	var rows []model.AcctAccountModel
	err := r.db.WithContext(ctx).
		Model(&model.AcctAccountModel{}).
		Where("id IN ?", normalized).
		Find(&rows).Error
	return rows, err
}

func (r *accountRepo) Ensure(ctx context.Context, m *model.AcctAccountModel) (*model.AcctAccountModel, error) {
	if m == nil || m.ID <= 0 {
		return nil, fmt.Errorf("invalid account")
	}
	m.OwnerType = strings.ToLower(strings.TrimSpace(m.OwnerType))
	m.AccountTypeCode = strings.ToUpper(strings.TrimSpace(m.AccountTypeCode))
	m.ChainScope = strings.ToUpper(strings.TrimSpace(m.ChainScope))
	if m.OwnerType == "" || m.AccountTypeCode == "" {
		return nil, fmt.Errorf("invalid account")
	}
	// Fast path: existing.
	if got, err := r.FindByOwnerType(ctx, m.OwnerType, m.OwnerID, m.AccountTypeCode, m.ChainScope); err == nil && got != nil {
		return got, nil
	}
	// Best-effort create; on race/duplicate, fall back to re-read.
	if err := r.db.WithContext(ctx).Create(m).Error; err != nil {
		got, err2 := r.FindByOwnerType(ctx, m.OwnerType, m.OwnerID, m.AccountTypeCode, m.ChainScope)
		if err2 == nil && got != nil {
			return got, nil
		}
		return nil, err
	}
	return m, nil
}
