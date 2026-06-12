package repository

import (
	"context"
	commonRepo "internalwallet/common/repository"
	"internalwallet/services/business/rpc/internal/model"

	"gorm.io/gorm"
)

type UserSecuritySettingsRepository interface {
	commonRepo.BaseRepository[model.UserSecuritySettingsModel]
	CreateDefaultSecurity(ctx context.Context, userID int64, emailBound bool, phoneBound bool) error
	GetByUserID(ctx context.Context, userID int64) (*model.UserSecuritySettingsModel, error)
	UpdateEmailBoundAndMask(ctx context.Context, userID int64, email string) error
	UpdatePhoneBoundAndMask(ctx context.Context, userID int64, phone string) error
	SetSecurityHasTradePassword(ctx context.Context, userID int64, has bool) error
	SetGoogleAuthEnabled(ctx context.Context, userID int64, enabled bool) error
	SetGoogleAuthBound(ctx context.Context, userID int64, bound bool) error
}

type userSecuritySettingsRepo struct {
	commonRepo.BaseRepository[model.UserSecuritySettingsModel]
}

func NewUserSecuritySettingsRepository(db *gorm.DB) UserSecuritySettingsRepository {
	return &userSecuritySettingsRepo{
		BaseRepository: commonRepo.NewBaseRepository[model.UserSecuritySettingsModel](db),
	}
}

func (r *userSecuritySettingsRepo) CreateDefaultSecurity(ctx context.Context, userID int64, emailBound bool, phoneBound bool) error {
	return r.GetDB().WithContext(context.Background()).Create(&model.UserSecuritySettingsModel{
		UserId:     userID,
		EmailBound: emailBound,
		PhoneBound: phoneBound,
	}).Error
}

func (r *userSecuritySettingsRepo) GetByUserID(ctx context.Context, userID int64) (*model.UserSecuritySettingsModel, error) {
	var s model.UserSecuritySettingsModel
	err := r.GetDB().WithContext(context.Background()).Where("user_id = ?", userID).First(&s).Error
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *userSecuritySettingsRepo) UpdateEmailBoundAndMask(ctx context.Context, userID int64, email string) error {
	mask := func(email string) string {
		at := -1
		for i := 0; i < len(email); i++ {
			if email[i] == '@' {
				at = i
				break
			}
		}
		if at <= 1 {
			return email
		}
		return email[:1] + "***" + email[at-1:]
	}
	return r.GetDB().WithContext(ctx).Model(&model.UserSecuritySettingsModel{}).
		Where("user_id = ?", userID).
		Updates(map[string]interface{}{"email_bound": true, "email_masked": mask(email)}).Error
}

func (r *userSecuritySettingsRepo) UpdatePhoneBoundAndMask(ctx context.Context, userID int64, phone string) error {
	mask := func(s string) string {
		if len(s) <= 4 {
			return s
		}
		return s[:3] + "****" + s[len(s)-2:]
	}
	return r.GetDB().WithContext(ctx).Model(&model.UserSecuritySettingsModel{}).
		Where("user_id = ?", userID).
		Updates(map[string]interface{}{"phone_bound": true, "phone_masked": mask(phone)}).Error
}

func (r *userSecuritySettingsRepo) SetSecurityHasTradePassword(ctx context.Context, userID int64, has bool) error {
	return r.GetDB().WithContext(context.Background()).Model(&model.UserSecuritySettingsModel{}).
		Where("user_id = ?", userID).
		Updates(map[string]interface{}{"has_trade_password": has}).Error
}

func (r *userSecuritySettingsRepo) SetGoogleAuthEnabled(ctx context.Context, userID int64, enabled bool) error {
	return r.GetDB().WithContext(context.Background()).Model(&model.UserSecuritySettingsModel{}).
		Where("user_id = ?", userID).
		Updates(map[string]interface{}{"google_auth_enabled": enabled}).Error
}

func (r *userSecuritySettingsRepo) SetGoogleAuthBound(ctx context.Context, userID int64, bound bool) error {
	return r.GetDB().WithContext(context.Background()).Model(&model.UserSecuritySettingsModel{}).
		Where("user_id = ?", userID).
		Updates(map[string]interface{}{"google_auth_bound": bound}).Error
}
