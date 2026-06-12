package repository

import (
	"context"

	"internalwallet/services/accounting/rpc/internal/model"

	"gorm.io/gorm"
)

type BalanceRepository interface {
	WithTx(tx *gorm.DB) BalanceRepository
	GetDB() *gorm.DB

	ListUserLiabilityBalances(ctx context.Context, userID int64) ([]model.AcctBalanceModel, error)
}

type balanceRepo struct {
	db *gorm.DB
}

func NewBalanceRepository(db *gorm.DB) BalanceRepository    { return &balanceRepo{db: db} }
func (r *balanceRepo) WithTx(tx *gorm.DB) BalanceRepository { return &balanceRepo{db: tx} }
func (r *balanceRepo) GetDB() *gorm.DB                      { return r.db }

func (r *balanceRepo) ListUserLiabilityBalances(ctx context.Context, userID int64) ([]model.AcctBalanceModel, error) {
	if userID <= 0 {
		return nil, nil
	}
	var rows []model.AcctBalanceModel
	err := r.db.WithContext(ctx).
		Table("acct_balances b").
		Select("b.*").
		Joins("JOIN acct_accounts a ON a.id = b.account_id").
		Where("a.owner_type = 'user' AND a.owner_id = ? AND a.account_type_code = 'USER_LIABILITY' AND a.deleted_at IS NULL AND b.deleted_at IS NULL", userID).
		Order("b.asset_code ASC, b.bucket ASC").
		Find(&rows).Error
	return rows, err
}
