package repository

import (
	"context"
	"strings"

	"gorm.io/gorm"
	commonRepo "internalwallet/common/repository"
	"internalwallet/services/business/rpc/internal/model"
)

type ChainRepository interface {
	commonRepo.BaseRepository[model.ChainModel]
	ListEnabledChains(ctx context.Context) ([]model.ChainModel, error)
	FindByName(ctx context.Context, name string) (*model.ChainModel, error)
}

type chainRepo struct {
	commonRepo.BaseRepository[model.ChainModel]
}

func NewChainRepository(db *gorm.DB) ChainRepository {
	return &chainRepo{
		BaseRepository: commonRepo.NewBaseRepository[model.ChainModel](db),
	}
}

func (r *chainRepo) ListEnabledChains(ctx context.Context) ([]model.ChainModel, error) {
	var rows []model.ChainModel
	err := r.GetDB().WithContext(ctx).Model(&model.ChainModel{}).Where("status = 1").Order("id ASC").Find(&rows).Error
	return rows, err
}

func (r *chainRepo) FindByName(ctx context.Context, name string) (*model.ChainModel, error) {
	name = strings.ToUpper(strings.TrimSpace(name))
	if name == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var m model.ChainModel
	err := r.GetDB().WithContext(ctx).Model(&model.ChainModel{}).Where("name = ?", name).First(&m).Error
	if err != nil {
		return nil, err
	}
	return &m, nil
}
