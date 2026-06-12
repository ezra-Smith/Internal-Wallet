package repository

import (
	"context"

	commonRepo "internalwallet/common/repository"
	"internalwallet/services/admin/rpc/internal/model"

	"gorm.io/gorm"
)

type AdminBackupCodeRepository interface {
	commonRepo.BaseRepository[model.AdminBackupCodeModel]

	ReplaceAll(ctx context.Context, adminID int64, codeHashes []string, createdBy int64) error
}

type adminBackupCodeRepo struct {
	commonRepo.BaseRepository[model.AdminBackupCodeModel]
}

func NewAdminBackupCodeRepository(db *gorm.DB) AdminBackupCodeRepository {
	return &adminBackupCodeRepo{
		BaseRepository: commonRepo.NewBaseRepository[model.AdminBackupCodeModel](db),
	}
}

func (r *adminBackupCodeRepo) ReplaceAll(ctx context.Context, adminID int64, codeHashes []string, createdBy int64) error {
	return r.GetDB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Soft-delete previous codes (best-effort keep updated_by).
		if err := tx.Model(&model.AdminBackupCodeModel{}).
			Where("admin_id = ?", adminID).
			Update("updated_by", createdBy).Error; err != nil {
			return err
		}
		if err := tx.Where("admin_id = ?", adminID).Delete(&model.AdminBackupCodeModel{}).Error; err != nil {
			return err
		}

		if len(codeHashes) == 0 {
			return nil
		}

		items := make([]*model.AdminBackupCodeModel, 0, len(codeHashes))
		for _, h := range codeHashes {
			items = append(items, &model.AdminBackupCodeModel{
				AdminID:   adminID,
				CodeHash:  h,
				CreatedBy: createdBy,
				UpdatedBy: createdBy,
				UsedAt:    nil,
			})
		}
		return tx.Create(&items).Error
	})
}
