package repository

import (
	"context"
	"time"

	commonRepo "internalwallet/common/repository"
	"internalwallet/common/utils"
	"internalwallet/services/business/rpc/internal/model"

	"gorm.io/gorm"
)

type PlatformBindingRepository interface {
	commonRepo.BaseRepository[model.PlatformBindingModel]
	CreateBinding(ctx context.Context, userID int64, providerID string, accountID string) (*model.PlatformBindingModel, error)
	UnlinkBinding(ctx context.Context, userID int64, bindingID string, reason string) error
	SetPrimaryBinding(ctx context.Context, userID int64, bindingID string) error
	ListBindingsByUser(ctx context.Context, userID int64) ([]model.PlatformBindingModel, error)
	ListProviderAccountsByUser(ctx context.Context, userID int64, providerID string) ([]model.PlatformBindingModel, error)
}

type platformBindingRepo struct {
	commonRepo.BaseRepository[model.PlatformBindingModel]
}

func NewPlatformBindingRepository(db *gorm.DB) PlatformBindingRepository {
	return &platformBindingRepo{
		BaseRepository: commonRepo.NewBaseRepository[model.PlatformBindingModel](db),
	}
}

func (r *platformBindingRepo) CreateBinding(ctx context.Context, userID int64, providerID string, accountID string) (*model.PlatformBindingModel, error) {
	m := &model.PlatformBindingModel{
		BindingId:   utils.GenerateIDString(),
		UserId:      userID,
		PlatformUid: providerID + ":" + accountID,
		Status:      1,
		BoundAt:     time.Now(),
	}
	err := r.GetDB().WithContext(context.Background()).Create(m).Error
	return m, err
}

func (r *platformBindingRepo) UnlinkBinding(ctx context.Context, userID int64, bindingID string, reason string) error {
	return r.GetDB().WithContext(context.Background()).Model(&model.PlatformBindingModel{}).
		Where("binding_id = ? AND user_id = ? AND status <> 0", bindingID, userID).
		Updates(map[string]interface{}{
			"status":         0,
			"unbound_at":     time.Now(),
			"unbound_reason": reason,
		}).Error
}

func (r *platformBindingRepo) SetPrimaryBinding(ctx context.Context, userID int64, bindingID string) error {
	if err := r.GetDB().WithContext(context.Background()).Model(&model.PlatformBindingModel{}).
		Where("user_id = ? AND status = 2", userID).
		Update("status", 1).Error; err != nil {
		return err
	}
	return r.GetDB().WithContext(context.Background()).Model(&model.PlatformBindingModel{}).
		Where("binding_id = ? AND user_id = ? AND status <> 0", bindingID, userID).
		Update("status", 2).Error
}

func (r *platformBindingRepo) ListBindingsByUser(ctx context.Context, userID int64) ([]model.PlatformBindingModel, error) {
	var rows []model.PlatformBindingModel
	err := r.GetDB().WithContext(context.Background()).
		Model(&model.PlatformBindingModel{}).
		Where("user_id = ? AND status <> 0", userID).
		Order("bound_at DESC").
		Find(&rows).Error
	return rows, err
}

func (r *platformBindingRepo) ListProviderAccountsByUser(ctx context.Context, userID int64, providerID string) ([]model.PlatformBindingModel, error) {
	var rows []model.PlatformBindingModel
	prefix := providerID + ":"
	err := r.GetDB().WithContext(context.Background()).
		Model(&model.PlatformBindingModel{}).
		Where("user_id = ? AND status <> 0 AND platform_uid LIKE ?", userID, prefix+"%").
		Order("bound_at DESC").
		Find(&rows).Error
	return rows, err
}
