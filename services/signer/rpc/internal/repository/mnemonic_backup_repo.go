package repository

import (
	"errors"
	commonRepo "internalwallet/common/repository"
	"time"

	"gorm.io/gorm"

	"internalwallet/services/signer/rpc/internal/models"
)

// MnemonicBackupRepository 助记词备份仓储接口
type MnemonicBackupRepository interface {
	commonRepo.BaseRepository[models.MnemonicBackup]
	// GetByUserIDAndSeedID 根据用户ID和种子ID获取备份记录
	GetByUserIDAndSeedID(userID int64, seedID string) (*models.MnemonicBackup, error)

	// GetByUserID 根据用户ID获取备份记录（用户可能有多个种子）
	GetByUserID(userID int64) ([]*models.MnemonicBackup, error)

	// UpdateBackupConfirmed 更新备份确认状态
	UpdateBackupConfirmed(id int64) error

	// IncrementChallengeCount 增加挑战次数
	IncrementChallengeCount(id int64) error

	// IncrementSuccessfulVerifies 增加成功验证次数
	IncrementSuccessfulVerifies(id int64) error

	// IncrementFailedVerifies 增加失败验证次数
	IncrementFailedVerifies(id int64) error

	// UpdateLastChallengedAt 更新最后挑战时间
	UpdateLastChallengedAt(id int64) error
}

// mnemonicBackupRepository 助记词备份仓储实现
type mnemonicBackupRepository struct {
	commonRepo.BaseRepository[models.MnemonicBackup]
}

// NewMnemonicBackupRepository 创建助记词备份仓储实例
func NewMnemonicBackupRepository(db *gorm.DB) MnemonicBackupRepository {
	return &mnemonicBackupRepository{
		BaseRepository: commonRepo.NewBaseRepository[models.MnemonicBackup](db),
	}
}

// GetByUserIDAndSeedID 根据用户ID和种子ID获取备份记录
func (r *mnemonicBackupRepository) GetByUserIDAndSeedID(userID int64, seedID string) (*models.MnemonicBackup, error) {
	var backup models.MnemonicBackup
	err := r.GetDB().Where("user_id = ? AND seed_id = ? AND status = 1", userID, seedID).First(&backup).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &backup, nil
}

// GetByUserID 根据用户ID获取备份记录（用户可能有多个种子）
func (r *mnemonicBackupRepository) GetByUserID(userID int64) ([]*models.MnemonicBackup, error) {
	var backups []*models.MnemonicBackup
	err := r.GetDB().Where("user_id = ? AND status = 1", userID).
		Order("created_at DESC").
		Find(&backups).Error
	if err != nil {
		return nil, err
	}
	return backups, nil
}

// UpdateBackupConfirmed 更新备份确认状态
func (r *mnemonicBackupRepository) UpdateBackupConfirmed(id int64) error {
	return r.GetDB().Model(&models.MnemonicBackup{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"backup_confirmed":    true,
			"backup_confirmed_at": time.Now().Local(),
		}).Error
}

// IncrementChallengeCount 增加挑战次数
func (r *mnemonicBackupRepository) IncrementChallengeCount(id int64) error {
	return r.GetDB().Model(&models.MnemonicBackup{}).
		Where("id = ?", id).
		UpdateColumn("challenge_count", gorm.Expr("challenge_count + 1")).Error
}

// IncrementSuccessfulVerifies 增加成功验证次数
func (r *mnemonicBackupRepository) IncrementSuccessfulVerifies(id int64) error {
	return r.GetDB().Model(&models.MnemonicBackup{}).
		Where("id = ?", id).
		UpdateColumn("successful_verifies", gorm.Expr("successful_verifies + 1")).Error
}

// IncrementFailedVerifies 增加失败验证次数
func (r *mnemonicBackupRepository) IncrementFailedVerifies(id int64) error {
	return r.GetDB().Model(&models.MnemonicBackup{}).
		Where("id = ?", id).
		UpdateColumn("failed_verifies", gorm.Expr("failed_verifies + 1")).Error
}

// UpdateLastChallengedAt 更新最后挑战时间
func (r *mnemonicBackupRepository) UpdateLastChallengedAt(id int64) error {
	return r.GetDB().Model(&models.MnemonicBackup{}).
		Where("id = ?", id).
		Update("last_challenged_at", time.Now().Local()).Error
}
