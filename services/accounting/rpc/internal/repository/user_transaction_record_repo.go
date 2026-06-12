package repository

import (
	"context"
	"strings"
	"time"

	"internalwallet/services/accounting/rpc/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type UserTransactionRecordListFilter struct {
	UserID    int64
	TxType    int32
	AssetCode string
	ChainCode string
	Status    string
	StartTime *time.Time
	EndTime   *time.Time
}

type UserTransactionRecordRepository interface {
	WithTx(tx *gorm.DB) UserTransactionRecordRepository
	GetDB() *gorm.DB

	FindByID(ctx context.Context, id int64) (*model.AcctUserTransactionRecordModel, error)
	List(ctx context.Context, page, pageSize int32, f UserTransactionRecordListFilter) ([]model.AcctUserTransactionRecordModel, int64, error)
	Upsert(ctx context.Context, m *model.AcctUserTransactionRecordModel) error
	UpdateMeta(ctx context.Context, id int64, fields map[string]any) error
}

type userTransactionRecordRepo struct {
	db *gorm.DB
}

func NewUserTransactionRecordRepository(db *gorm.DB) UserTransactionRecordRepository {
	return &userTransactionRecordRepo{db: db}
}

func (r *userTransactionRecordRepo) WithTx(tx *gorm.DB) UserTransactionRecordRepository {
	return &userTransactionRecordRepo{db: tx}
}
func (r *userTransactionRecordRepo) GetDB() *gorm.DB { return r.db }

func (r *userTransactionRecordRepo) FindByID(ctx context.Context, id int64) (*model.AcctUserTransactionRecordModel, error) {
	if id <= 0 {
		return nil, gorm.ErrRecordNotFound
	}
	var m model.AcctUserTransactionRecordModel
	if err := r.db.WithContext(ctx).Model(&model.AcctUserTransactionRecordModel{}).Where("id = ?", id).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *userTransactionRecordRepo) List(ctx context.Context, page, pageSize int32, f UserTransactionRecordListFilter) ([]model.AcctUserTransactionRecordModel, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}

	q := r.db.WithContext(ctx).Model(&model.AcctUserTransactionRecordModel{})
	if f.UserID > 0 {
		q = q.Where("user_id = ?", f.UserID)
	}
	if f.TxType > 0 {
		q = q.Where("tx_type = ?", f.TxType)
	}
	if strings.TrimSpace(f.AssetCode) != "" {
		q = q.Where("asset_code = ?", strings.TrimSpace(f.AssetCode))
	}
	if strings.TrimSpace(f.ChainCode) != "" {
		q = q.Where("chain_code = ?", strings.TrimSpace(f.ChainCode))
	}
	if strings.TrimSpace(f.Status) != "" {
		raw := strings.TrimSpace(f.Status)
		parts := strings.Split(raw, ",")
		statuses := make([]string, 0, len(parts))
		for _, p := range parts {
			if s := strings.TrimSpace(p); s != "" {
				statuses = append(statuses, s)
			}
		}
		if len(statuses) == 1 {
			q = q.Where("status = ?", statuses[0])
		} else if len(statuses) > 1 {
			q = q.Where("status IN ?", statuses)
		}
	}
	if f.StartTime != nil && !f.StartTime.IsZero() {
		q = q.Where("created_at >= ?", *f.StartTime)
	}
	if f.EndTime != nil && !f.EndTime.IsZero() {
		q = q.Where("created_at <= ?", *f.EndTime)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []model.AcctUserTransactionRecordModel
	offset := int((page - 1) * pageSize)
	err := q.Order("created_at DESC, id DESC").Offset(offset).Limit(int(pageSize)).Find(&rows).Error
	return rows, total, err
}

func (r *userTransactionRecordRepo) Upsert(ctx context.Context, m *model.AcctUserTransactionRecordModel) error {
	if m == nil || m.ID <= 0 {
		return gorm.ErrRecordNotFound
	}
	now := time.Now().Local()
	assignments := map[string]any{
		"updated_at": now,

		"user_id":              m.UserID,
		"tx_type":              m.TxType,
		"asset_code":           strings.TrimSpace(m.AssetCode),
		"chain_code":           strings.TrimSpace(m.ChainCode),
		"amount_decimal":       strings.TrimSpace(m.AmountDecimal),
		"fee_decimal":          strings.TrimSpace(m.FeeDecimal),
		"status":               strings.TrimSpace(m.Status),
		"memo":                 strings.TrimSpace(m.Memo),
		"from_address":         strings.TrimSpace(m.FromAddress),
		"to_address":           strings.TrimSpace(m.ToAddress),
		"tx_hash":              strings.TrimSpace(m.TxHash),
		"biz_ref":              strings.TrimSpace(m.BizRef),
		"idempotency_key":      strings.TrimSpace(m.IdempotencyKey),
		"freeze_ledger_tx_id":  m.FreezeLedgerTxID,
		"settle_ledger_tx_id":  m.SettleLedgerTxID,
		"confirm_ledger_tx_id": m.ConfirmLedgerTxID,
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.Assignments(assignments),
	}).Create(m).Error
}

func (r *userTransactionRecordRepo) UpdateMeta(ctx context.Context, id int64, fields map[string]any) error {
	if id <= 0 {
		return gorm.ErrRecordNotFound
	}
	if len(fields) == 0 {
		return nil
	}
	fields["updated_at"] = time.Now().Local()
	return r.db.WithContext(ctx).Model(&model.AcctUserTransactionRecordModel{}).Where("id = ?", id).Updates(fields).Error
}
