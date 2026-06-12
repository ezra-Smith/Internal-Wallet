package repository

import (
	"context"
	"fmt"
	"strings"

	"internalwallet/services/admin/rpc/internal/model"

	"gorm.io/gorm"
)

type ChainRepository interface {
	WithTx(tx *gorm.DB) ChainRepository
	GetDB() *gorm.DB

	ListEnabled(ctx context.Context) ([]*model.ChainModel, error)
	ListAll(ctx context.Context) ([]*model.ChainModel, error)
	UpdateIconURLByCode(ctx context.Context, code string, iconURL string) error
}

type chainRepo struct{ db *gorm.DB }

func NewChainRepository(db *gorm.DB) ChainRepository    { return &chainRepo{db: db} }
func (r *chainRepo) WithTx(tx *gorm.DB) ChainRepository { return &chainRepo{db: tx} }
func (r *chainRepo) GetDB() *gorm.DB                    { return r.db }

func (r *chainRepo) ListEnabled(ctx context.Context) ([]*model.ChainModel, error) {
	var items []*model.ChainModel
	err := r.db.WithContext(ctx).
		Where("status = 1").
		Order("name ASC").
		Find(&items).Error
	return items, err
}

func (r *chainRepo) ListAll(ctx context.Context) ([]*model.ChainModel, error) {
	var items []*model.ChainModel
	err := r.db.WithContext(ctx).
		Order("name ASC").
		Find(&items).Error
	for _, it := range items {
		if it != nil {
			it.Name = strings.ToUpper(strings.TrimSpace(it.Name))
		}
	}
	return items, err
}

func (r *chainRepo) UpdateIconURLByCode(ctx context.Context, code string, iconURL string) error {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return fmt.Errorf("chain not found")
	}
	res := r.db.WithContext(ctx).
		Model(&model.ChainModel{}).
		Where("name = ?", code).
		Updates(map[string]interface{}{
			"icon_url": iconURL,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("chain not found")
	}
	return nil
}
