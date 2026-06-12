package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	commonRepo "internalwallet/common/repository"
	"internalwallet/services/business/rpc/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type MemberBiometricCredentialRepository interface {
	commonRepo.BaseRepository[model.MemberBiometricCredentialModel]

	Upsert(ctx context.Context, m *model.MemberBiometricCredentialModel) error
	GetActiveByUserKeyID(ctx context.Context, userID int64, keyID string) (*model.MemberBiometricCredentialModel, error)
	ListActiveByUser(ctx context.Context, userID int64) ([]*model.MemberBiometricCredentialModel, error)
	DisableAllByUser(ctx context.Context, userID int64) error
}

type memberBiometricCredentialRepo struct {
	commonRepo.BaseRepository[model.MemberBiometricCredentialModel]
}

func NewMemberBiometricCredentialRepository(db *gorm.DB) MemberBiometricCredentialRepository {
	return &memberBiometricCredentialRepo{
		BaseRepository: commonRepo.NewBaseRepository[model.MemberBiometricCredentialModel](db),
	}
}

func (r *memberBiometricCredentialRepo) Upsert(ctx context.Context, m *model.MemberBiometricCredentialModel) error {
	if m == nil {
		return fmt.Errorf("credential is nil")
	}
	if m.UserId <= 0 {
		return fmt.Errorf("invalid user_id")
	}
	m.KeyId = strings.TrimSpace(m.KeyId)
	if m.KeyId == "" {
		return fmt.Errorf("invalid key_id")
	}
	if len(m.PublicKey) == 0 {
		return fmt.Errorf("public_key required")
	}
	return r.GetDB().WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "user_id"}, {Name: "key_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"algorithm",
				"public_key_format",
				"public_key",
				"status",
				"platform",
				"device_id",
				"app_version",
				"deleted_at",
				"updated_at",
			}),
		}).
		Create(m).Error
}

func (r *memberBiometricCredentialRepo) GetActiveByUserKeyID(ctx context.Context, userID int64, keyID string) (*model.MemberBiometricCredentialModel, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("invalid user_id")
	}
	keyID = strings.TrimSpace(keyID)
	if keyID == "" {
		return nil, fmt.Errorf("invalid key_id")
	}
	var row model.MemberBiometricCredentialModel
	err := r.GetDB().WithContext(ctx).
		Model(&model.MemberBiometricCredentialModel{}).
		Where("user_id = ? AND key_id = ? AND status = 1", userID, keyID).
		First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

func (r *memberBiometricCredentialRepo) ListActiveByUser(ctx context.Context, userID int64) ([]*model.MemberBiometricCredentialModel, error) {
	if userID <= 0 {
		return []*model.MemberBiometricCredentialModel{}, nil
	}
	var rows []*model.MemberBiometricCredentialModel
	err := r.GetDB().WithContext(ctx).
		Model(&model.MemberBiometricCredentialModel{}).
		Where("user_id = ? AND status = 1", userID).
		Order("updated_at DESC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *memberBiometricCredentialRepo) DisableAllByUser(ctx context.Context, userID int64) error {
	if userID <= 0 {
		return fmt.Errorf("invalid user_id")
	}
	return r.GetDB().WithContext(ctx).
		Model(&model.MemberBiometricCredentialModel{}).
		Where("user_id = ? AND status = 1", userID).
		Updates(map[string]interface{}{"status": 0}).Error
}