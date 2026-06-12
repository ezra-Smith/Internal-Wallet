package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	commonRepo "internalwallet/common/repository"
	"internalwallet/services/business/rpc/internal/model"

	"gorm.io/gorm"
)

type UserWithdrawAuditWhitelistRuleRepository interface {
	commonRepo.BaseRepository[model.UserWithdrawAuditWhitelistRuleModel]

	GetEnabledMatch(ctx context.Context, userID int64, assetCode, chainCode, address string) (*model.UserWithdrawAuditWhitelistRuleModel, error)
	HasEnabledByUserSource(ctx context.Context, userID int64, source string) (bool, error)
	ListActiveByUserSource(ctx context.Context, userID int64, source string) ([]*model.UserWithdrawAuditWhitelistRuleModel, error)
	SoftDeleteByUserAssetChainSource(ctx context.Context, userID int64, assetCode, chainCode, source string) error
	SoftDeleteByUserChainSource(ctx context.Context, userID int64, chainCode, source string) error
	SoftDeleteByUserSource(ctx context.Context, userID int64, source string) error
}

type userWithdrawAuditWhitelistRuleRepo struct {
	commonRepo.BaseRepository[model.UserWithdrawAuditWhitelistRuleModel]
}

func NewUserWithdrawAuditWhitelistRuleRepository(db *gorm.DB) UserWithdrawAuditWhitelistRuleRepository {
	return &userWithdrawAuditWhitelistRuleRepo{
		BaseRepository: commonRepo.NewBaseRepository[model.UserWithdrawAuditWhitelistRuleModel](db),
	}
}

func (r *userWithdrawAuditWhitelistRuleRepo) GetEnabledMatch(ctx context.Context, userID int64, assetCode, chainCode, address string) (*model.UserWithdrawAuditWhitelistRuleModel, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("invalid user_id")
	}
	assetCode = strings.ToUpper(strings.TrimSpace(assetCode))
	chainCode = strings.ToUpper(strings.TrimSpace(chainCode))
	address = strings.TrimSpace(address)
	if assetCode == "" || chainCode == "" || address == "" {
		return nil, fmt.Errorf("invalid params")
	}

	var row model.UserWithdrawAuditWhitelistRuleModel
	err := r.GetDB().WithContext(ctx).
		Model(&model.UserWithdrawAuditWhitelistRuleModel{}).
		Where("enabled = 1 AND user_id = ? AND asset_code = ? AND chain_code = ? AND address = ?", userID, assetCode, chainCode, address).
		First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

func (r *userWithdrawAuditWhitelistRuleRepo) HasEnabledByUserSource(ctx context.Context, userID int64, source string) (bool, error) {
	if userID <= 0 {
		return false, nil
	}
	source = strings.ToLower(strings.TrimSpace(source))
	if source == "" {
		source = "admin"
	}

	var count int64
	err := r.GetDB().WithContext(ctx).
		Model(&model.UserWithdrawAuditWhitelistRuleModel{}).
		Where("deleted_at IS NULL AND enabled = 1 AND user_id = ? AND source = ?", userID, source).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *userWithdrawAuditWhitelistRuleRepo) ListActiveByUserSource(ctx context.Context, userID int64, source string) ([]*model.UserWithdrawAuditWhitelistRuleModel, error) {
	if userID <= 0 {
		return []*model.UserWithdrawAuditWhitelistRuleModel{}, nil
	}
	source = strings.ToLower(strings.TrimSpace(source))
	if source == "" {
		source = "admin"
	}

	var rows []*model.UserWithdrawAuditWhitelistRuleModel
	err := r.GetDB().WithContext(ctx).
		Model(&model.UserWithdrawAuditWhitelistRuleModel{}).
		Where("deleted_at IS NULL AND user_id = ? AND source = ?", userID, source).
		Order("updated_at DESC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *userWithdrawAuditWhitelistRuleRepo) SoftDeleteByUserAssetChainSource(ctx context.Context, userID int64, assetCode, chainCode, source string) error {
	if userID <= 0 {
		return fmt.Errorf("invalid user_id")
	}
	assetCode = strings.ToUpper(strings.TrimSpace(assetCode))
	chainCode = strings.ToUpper(strings.TrimSpace(chainCode))
	source = strings.ToLower(strings.TrimSpace(source))
	if assetCode == "" || chainCode == "" || source == "" {
		return fmt.Errorf("invalid params")
	}
	return r.GetDB().WithContext(ctx).
		Where("user_id = ? AND asset_code = ? AND chain_code = ? AND source = ?", userID, assetCode, chainCode, source).
		Delete(&model.UserWithdrawAuditWhitelistRuleModel{}).Error
}

func (r *userWithdrawAuditWhitelistRuleRepo) SoftDeleteByUserChainSource(ctx context.Context, userID int64, chainCode, source string) error {
	if userID <= 0 {
		return fmt.Errorf("invalid user_id")
	}
	chainCode = strings.ToUpper(strings.TrimSpace(chainCode))
	source = strings.ToLower(strings.TrimSpace(source))
	if chainCode == "" || source == "" {
		return fmt.Errorf("invalid params")
	}
	return r.GetDB().WithContext(ctx).
		Where("user_id = ? AND chain_code = ? AND source = ?", userID, chainCode, source).
		Delete(&model.UserWithdrawAuditWhitelistRuleModel{}).Error
}

func (r *userWithdrawAuditWhitelistRuleRepo) SoftDeleteByUserSource(ctx context.Context, userID int64, source string) error {
	if userID <= 0 {
		return fmt.Errorf("invalid user_id")
	}
	source = strings.ToLower(strings.TrimSpace(source))
	if source == "" {
		return fmt.Errorf("invalid params")
	}
	return r.GetDB().WithContext(ctx).
		Where("user_id = ? AND source = ?", userID, source).
		Delete(&model.UserWithdrawAuditWhitelistRuleModel{}).Error
}
