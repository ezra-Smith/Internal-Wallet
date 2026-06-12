package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	commonRepo "internalwallet/common/repository"
	"internalwallet/services/admin/rpc/internal/model"

	"gorm.io/gorm"
)

type UserWhitelistSettingsRepository interface {
	commonRepo.BaseRepository[model.UserWhitelistSettingsModel]

	GetByUserID(ctx context.Context, userID int64) (*model.UserWhitelistSettingsModel, error)
	UpsertByUserID(ctx context.Context, userID int64, bypassWithdrawAudit bool, reason string, operatorAdminID int64) (*model.UserWhitelistSettingsModel, error)
}

type userWhitelistSettingsRepo struct {
	commonRepo.BaseRepository[model.UserWhitelistSettingsModel]
}

func NewUserWhitelistSettingsRepository(db *gorm.DB) UserWhitelistSettingsRepository {
	return &userWhitelistSettingsRepo{
		BaseRepository: commonRepo.NewBaseRepository[model.UserWhitelistSettingsModel](db),
	}
}

func (r *userWhitelistSettingsRepo) GetByUserID(ctx context.Context, userID int64) (*model.UserWhitelistSettingsModel, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("invalid user_id")
	}
	var s model.UserWhitelistSettingsModel
	err := r.GetDB().WithContext(ctx).
		Model(&model.UserWhitelistSettingsModel{}).
		Where("user_id = ?", userID).
		First(&s).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &s, err
}

func (r *userWhitelistSettingsRepo) UpsertByUserID(ctx context.Context, userID int64, bypassWithdrawAudit bool, reason string, operatorAdminID int64) (*model.UserWhitelistSettingsModel, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("invalid user_id")
	}
	reason = strings.TrimSpace(reason)
	source := "admin"
	operatorUserID := int64(0)

	// Prefer update when exists.
	var existing model.UserWhitelistSettingsModel
	err := r.GetDB().WithContext(ctx).
		Model(&model.UserWhitelistSettingsModel{}).
		Where("user_id = ?", userID).
		First(&existing).Error
	if err == nil {
		existing.BypassWithdrawAudit = bypassWithdrawAudit
		existing.Reason = reason
		existing.OperatorAdminID = operatorAdminID
		existing.Source = source
		existing.OperatorUserID = operatorUserID
		if uErr := r.GetDB().WithContext(ctx).
			Model(&model.UserWhitelistSettingsModel{}).
			Where("user_id = ?", userID).
			Updates(map[string]interface{}{
				"bypass_withdraw_audit": bypassWithdrawAudit,
				"reason":                reason,
				"operator_admin_id":     operatorAdminID,
				"source":                source,
				"operator_user_id":      operatorUserID,
			}).Error; uErr != nil {
			return nil, uErr
		}
		return &existing, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	// Create new row.
	entity := &model.UserWhitelistSettingsModel{
		UserID:              userID,
		BypassWithdrawAudit: bypassWithdrawAudit,
		Reason:              reason,
		OperatorAdminID:     operatorAdminID,
		Source:              source,
		OperatorUserID:      operatorUserID,
	}
	if cErr := r.GetDB().WithContext(ctx).Create(entity).Error; cErr != nil {
		return nil, cErr
	}
	return entity, nil
}
