package repository

import (
	"context"
	"strings"
	"time"

	"internalwallet/services/accounting/rpc/internal/model"

	"gorm.io/gorm"
)

type LedgerTxListFilter struct {
	TxID           int64
	OpType         string
	BizRef         string
	IdempotencyKey string
	AssetCode      string
	UserID         int64
	CreatedFrom    *time.Time
	CreatedTo      *time.Time
}

type LedgerRepository interface {
	WithTx(tx *gorm.DB) LedgerRepository
	GetDB() *gorm.DB

	FindTxByIdempotencyKey(ctx context.Context, key string) (*model.AcctLedgerTxModel, error)
	FindTxByID(ctx context.Context, txID int64) (*model.AcctLedgerTxModel, error)
	ListTx(ctx context.Context, page, pageSize int32, f LedgerTxListFilter) ([]model.AcctLedgerTxModel, int64, error)
	ListPostingsByTxID(ctx context.Context, txID int64) ([]model.AcctLedgerPostingModel, error)
}

type ledgerRepo struct {
	db *gorm.DB
}

func NewLedgerRepository(db *gorm.DB) LedgerRepository    { return &ledgerRepo{db: db} }
func (r *ledgerRepo) WithTx(tx *gorm.DB) LedgerRepository { return &ledgerRepo{db: tx} }
func (r *ledgerRepo) GetDB() *gorm.DB                     { return r.db }

func (r *ledgerRepo) FindTxByIdempotencyKey(ctx context.Context, key string) (*model.AcctLedgerTxModel, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var m model.AcctLedgerTxModel
	if err := r.db.WithContext(ctx).Model(&model.AcctLedgerTxModel{}).Where("idempotency_key = ?", key).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *ledgerRepo) FindTxByID(ctx context.Context, txID int64) (*model.AcctLedgerTxModel, error) {
	if txID <= 0 {
		return nil, gorm.ErrRecordNotFound
	}
	var m model.AcctLedgerTxModel
	if err := r.db.WithContext(ctx).Model(&model.AcctLedgerTxModel{}).Where("id = ?", txID).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *ledgerRepo) ListTx(ctx context.Context, page, pageSize int32, f LedgerTxListFilter) ([]model.AcctLedgerTxModel, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}

	q := r.db.WithContext(ctx).Model(&model.AcctLedgerTxModel{})
	if f.TxID > 0 {
		q = q.Where("id = ?", f.TxID)
	}
	if strings.TrimSpace(f.OpType) != "" {
		q = q.Where("op_type = ?", strings.TrimSpace(f.OpType))
	}
	if strings.TrimSpace(f.BizRef) != "" {
		q = q.Where("biz_ref LIKE ?", "%"+strings.TrimSpace(f.BizRef)+"%")
	}
	if strings.TrimSpace(f.IdempotencyKey) != "" {
		q = q.Where("idempotency_key LIKE ?", "%"+strings.TrimSpace(f.IdempotencyKey)+"%")
	}
	if strings.TrimSpace(f.AssetCode) != "" || f.UserID > 0 {
		sub := r.db.WithContext(ctx).
			Table("acct_ledger_postings p").
			Select("DISTINCT p.tx_id").
			Joins("JOIN acct_accounts a ON a.id = p.account_id AND a.deleted_at IS NULL")
		sub = sub.Where("p.deleted_at IS NULL")
		if f.UserID > 0 {
			sub = sub.Where("a.owner_type = 'user' AND a.owner_id = ?", f.UserID)
		}
		if strings.TrimSpace(f.AssetCode) != "" {
			sub = sub.Where("p.asset_code = ?", strings.TrimSpace(f.AssetCode))
		}
		q = q.Where("id IN (?)", sub)
	}
	if f.CreatedFrom != nil && !f.CreatedFrom.IsZero() {
		q = q.Where("created_at >= ?", *f.CreatedFrom)
	}
	if f.CreatedTo != nil && !f.CreatedTo.IsZero() {
		q = q.Where("created_at <= ?", *f.CreatedTo)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []model.AcctLedgerTxModel
	offset := int((page - 1) * pageSize)
	err := q.Order("created_at DESC, id DESC").Offset(offset).Limit(int(pageSize)).Find(&rows).Error
	return rows, total, err
}

func (r *ledgerRepo) ListPostingsByTxID(ctx context.Context, txID int64) ([]model.AcctLedgerPostingModel, error) {
	if txID <= 0 {
		return nil, nil
	}
	var rows []model.AcctLedgerPostingModel
	err := r.db.WithContext(ctx).
		Model(&model.AcctLedgerPostingModel{}).
		Where("tx_id = ?", txID).
		Order("seq ASC").
		Find(&rows).Error
	return rows, err
}
