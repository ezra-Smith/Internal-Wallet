package repository

import (
	"context"
	"fmt"
	"strings"

	commonRepo "internalwallet/common/repository"
	"internalwallet/services/admin/rpc/internal/model"

	"gorm.io/gorm"
)

type UserWithdrawAuditWhitelistRuleRepository interface {
	commonRepo.BaseRepository[model.UserWithdrawAuditWhitelistRuleModel]

	ListByUserID(ctx context.Context, userID int64) ([]*model.UserWithdrawAuditWhitelistRuleModel, error)
	SyncByUserID(ctx context.Context, userID int64, rules []*model.UserWithdrawAuditWhitelistRuleModel) error
	HasAnyEnabledByUserID(ctx context.Context, userID int64) (bool, error)
}

type userWithdrawAuditWhitelistRuleRepo struct {
	commonRepo.BaseRepository[model.UserWithdrawAuditWhitelistRuleModel]
}

func NewUserWithdrawAuditWhitelistRuleRepository(db *gorm.DB) UserWithdrawAuditWhitelistRuleRepository {
	return &userWithdrawAuditWhitelistRuleRepo{
		BaseRepository: commonRepo.NewBaseRepository[model.UserWithdrawAuditWhitelistRuleModel](db),
	}
}

func (r *userWithdrawAuditWhitelistRuleRepo) ListByUserID(ctx context.Context, userID int64) ([]*model.UserWithdrawAuditWhitelistRuleModel, error) {
	if userID <= 0 {
		return []*model.UserWithdrawAuditWhitelistRuleModel{}, nil
	}
	var rows []*model.UserWithdrawAuditWhitelistRuleModel
	err := r.GetDB().WithContext(ctx).
		Model(&model.UserWithdrawAuditWhitelistRuleModel{}).
		Where("user_id = ?", userID).
		Order("updated_at DESC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *userWithdrawAuditWhitelistRuleRepo) SyncByUserID(ctx context.Context, userID int64, rules []*model.UserWithdrawAuditWhitelistRuleModel) error {
	if userID <= 0 {
		return fmt.Errorf("invalid user_id")
	}
	db := r.GetDB().WithContext(ctx)

	// Soft-delete existing admin rules first (idempotent).
	// User self-bound rules are read-only in admin and must be preserved.
	if err := db.
		Where("user_id = ? AND (source IS NULL OR source = '' OR source = 'admin')", userID).
		Delete(&model.UserWithdrawAuditWhitelistRuleModel{}).Error; err != nil {
		return err
	}

	// Then insert the provided rules (deleted_at is NULL by default for new rows).
	if len(rules) == 0 {
		return nil
	}
	for _, rr := range rules {
		if rr == nil {
			continue
		}
		rr.UserID = userID
		rr.AssetCode = strings.ToUpper(strings.TrimSpace(rr.AssetCode))
		rr.ChainCode = strings.ToUpper(strings.TrimSpace(rr.ChainCode))
		rr.Address = strings.TrimSpace(rr.Address)
		rr.Source = "admin"
		rr.OperatorUserID = 0
	}
	return db.CreateInBatches(rules, 200).Error
}

func (r *userWithdrawAuditWhitelistRuleRepo) HasAnyEnabledByUserID(ctx context.Context, userID int64) (bool, error) {
	if userID <= 0 {
		return false, nil
	}
	var n int64
	err := r.GetDB().WithContext(ctx).
		Model(&model.UserWithdrawAuditWhitelistRuleModel{}).
		Where("enabled = 1 AND user_id = ?", userID).
		Count(&n).Error
	if err != nil {
		return false, err
	}
	return n > 0, nil
}
