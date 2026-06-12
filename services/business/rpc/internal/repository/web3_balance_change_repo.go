package repository

import (
	"context"
	"strings"
	"time"

	"internalwallet/services/business/rpc/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Web3BalanceChangeListFilter list filters.
type Web3BalanceChangeListFilter struct {
	ChainCode string
	AssetCode string
	TxType    string
	Direction string
	Status    string
	FromTime  *time.Time
	ToTime    *time.Time
	SortBy    string // block_time/created_at
	SortOrder string // asc/desc
}

type Web3BalanceChangeRepository interface {
	WithTx(tx *gorm.DB) Web3BalanceChangeRepository
	GetDB() *gorm.DB

	Upsert(ctx context.Context, change *model.Web3BalanceChangeModel) error
	FindByID(ctx context.Context, id int64) (*model.Web3BalanceChangeModel, error)
	FindByOldID(ctx context.Context, oldID int64) (*model.Web3BalanceChangeModel, error)
	FindByTxHash(ctx context.Context, txHash string) (*model.Web3BalanceChangeModel, error)
	FindBroadcastPlaceholder(ctx context.Context, txHash string, userAddress string) (*model.Web3BalanceChangeModel, error)
	SoftDeleteBroadcastPlaceholder(ctx context.Context, txHash string, userAddress string) error
	ListPendingBroadcastPlaceholders(ctx context.Context, limit int, recheckBefore time.Time, createdAfter time.Time) ([]*model.Web3BalanceChangeModel, error)

	ListByAddress(ctx context.Context, address string, page, pageSize int32, filter Web3BalanceChangeListFilter) ([]*model.Web3BalanceChangeModel, int64, error)
}

type web3BalanceChangeRepo struct {
	db *gorm.DB
}

func NewWeb3BalanceChangeRepository(db *gorm.DB) Web3BalanceChangeRepository {
	return &web3BalanceChangeRepo{db: db}
}

func (r *web3BalanceChangeRepo) WithTx(tx *gorm.DB) Web3BalanceChangeRepository {
	return &web3BalanceChangeRepo{db: tx}
}

func (r *web3BalanceChangeRepo) GetDB() *gorm.DB {
	return r.db
}

func (r *web3BalanceChangeRepo) Upsert(ctx context.Context, change *model.Web3BalanceChangeModel) error {
	if change == nil {
		return nil
	}

	// Normalize key fields to ensure deterministic uniqueness.
	change.TxHash = strings.TrimSpace(change.TxHash)
	change.UserAddress = strings.TrimSpace(change.UserAddress)
	change.ChainCode = strings.ToUpper(strings.TrimSpace(change.ChainCode))
	change.AssetCode = strings.ToUpper(strings.TrimSpace(change.AssetCode))
	change.Direction = strings.TrimSpace(change.Direction)
	change.TxType = strings.TrimSpace(change.TxType)
	change.Status = strings.TrimSpace(change.Status)

	now := time.Now().Local()

	assignments := map[string]interface{}{
		"web3_user_id":   change.Web3UserID,
		"chain_code":     change.ChainCode,
		"chain_id":       change.ChainID,
		"block_number":   change.BlockNumber,
		"block_time":     change.BlockTime,
		"tx_type":        change.TxType,
		"direction":      change.Direction,
		"asset_code":     change.AssetCode,
		"amount":         change.Amount,
		"amount_raw":     change.AmountRaw,
		"token_address":  change.TokenAddress,
		"token_decimals": change.TokenDecimals,
		"from_address":   change.FromAddress,
		"to_address":     change.ToAddress,
		"status":         change.Status,
		"confirmations":  change.Confirmations,
		"fee":            change.Fee,
		"fee_asset":      change.FeeAsset,
		"raw_data":       change.RawData,
		"deleted_at":     gorm.Expr("NULL"),
		"updated_at":     now,
	}
	// Only write old_id when we actually have a previous placeholder id to link.
	// This prevents accidental NULL overwrites on subsequent upserts that don't carry old_id.
	if change.OldID != nil {
		assignments["old_id"] = *change.OldID
	}

	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "tx_hash"},
			{Name: "event_index"},
			{Name: "user_address"},
		},
		DoUpdates: clause.Assignments(assignments),
	}).Create(change).Error
}

func (r *web3BalanceChangeRepo) FindByID(ctx context.Context, id int64) (*model.Web3BalanceChangeModel, error) {
	if id <= 0 {
		return nil, gorm.ErrRecordNotFound
	}

	var item model.Web3BalanceChangeModel
	err := r.db.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", id).
		First(&item).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *web3BalanceChangeRepo) FindByOldID(ctx context.Context, oldID int64) (*model.Web3BalanceChangeModel, error) {
	if oldID <= 0 {
		return nil, gorm.ErrRecordNotFound
	}

	var item model.Web3BalanceChangeModel
	err := r.db.WithContext(ctx).
		Where("old_id = ? AND deleted_at IS NULL", oldID).
		Order("created_at DESC").
		First(&item).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *web3BalanceChangeRepo) FindByTxHash(ctx context.Context, txHash string) (*model.Web3BalanceChangeModel, error) {
	txHash = strings.TrimSpace(txHash)
	if txHash == "" {
		return nil, gorm.ErrRecordNotFound
	}

	var item model.Web3BalanceChangeModel
	err := r.db.WithContext(ctx).
		Where("tx_hash = ? AND deleted_at IS NULL", txHash).
		Order("created_at DESC").
		First(&item).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *web3BalanceChangeRepo) FindBroadcastPlaceholder(ctx context.Context, txHash string, userAddress string) (*model.Web3BalanceChangeModel, error) {
	txHash = strings.TrimSpace(txHash)
	userAddress = strings.TrimSpace(userAddress)
	if txHash == "" || userAddress == "" {
		return nil, gorm.ErrRecordNotFound
	}

	var item model.Web3BalanceChangeModel
	err := r.db.WithContext(ctx).
		Where("tx_hash = ? AND user_address = ? AND event_index = ? AND deleted_at IS NULL", txHash, userAddress, model.Web3BalanceChangeBroadcastEventIndex).
		First(&item).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *web3BalanceChangeRepo) SoftDeleteBroadcastPlaceholder(ctx context.Context, txHash string, userAddress string) error {
	txHash = strings.TrimSpace(txHash)
	userAddress = strings.TrimSpace(userAddress)
	if txHash == "" || userAddress == "" {
		return nil
	}

	now := time.Now().Local()
	return r.db.WithContext(ctx).
		Model(&model.Web3BalanceChangeModel{}).
		Where("tx_hash = ? AND user_address = ? AND event_index = ? AND deleted_at IS NULL", txHash, userAddress, model.Web3BalanceChangeBroadcastEventIndex).
		Update("deleted_at", now).Error
}

func (r *web3BalanceChangeRepo) ListPendingBroadcastPlaceholders(ctx context.Context, limit int, recheckBefore time.Time, createdAfter time.Time) ([]*model.Web3BalanceChangeModel, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}

	q := r.db.WithContext(ctx).
		Model(&model.Web3BalanceChangeModel{}).
		Where("event_index = ? AND status = ? AND deleted_at IS NULL",
			model.Web3BalanceChangeBroadcastEventIndex,
			model.Web3TxStatusPending,
		)

	if !recheckBefore.IsZero() {
		q = q.Where("updated_at <= ?", recheckBefore)
	}
	if !createdAfter.IsZero() {
		q = q.Where("created_at >= ?", createdAfter)
	}

	var items []*model.Web3BalanceChangeModel
	err := q.Order("updated_at ASC").Limit(limit).Find(&items).Error
	return items, err
}

func (r *web3BalanceChangeRepo) ListByAddress(ctx context.Context, address string, page, pageSize int32, filter Web3BalanceChangeListFilter) ([]*model.Web3BalanceChangeModel, int64, error) {
	address = strings.TrimSpace(address)
	if address == "" {
		return nil, 0, nil
	}

	query := r.db.WithContext(ctx).Model(&model.Web3BalanceChangeModel{}).
		Where("user_address = ? AND deleted_at IS NULL", address)

	query = applyWeb3BalanceChangeFilter(query, filter)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	orderClause := getWeb3BalanceChangeOrderClause(filter)
	offset := int((page - 1) * pageSize)

	var items []*model.Web3BalanceChangeModel
	err := query.Order(orderClause).Offset(offset).Limit(int(pageSize)).Find(&items).Error
	return items, total, err
}

func applyWeb3BalanceChangeFilter(q *gorm.DB, f Web3BalanceChangeListFilter) *gorm.DB {
	if q == nil {
		return q
	}

	chainCode := strings.ToUpper(strings.TrimSpace(f.ChainCode))
	if chainCode != "" {
		q = q.Where("chain_code = ?", chainCode)
	}
	assetCode := strings.ToUpper(strings.TrimSpace(f.AssetCode))
	if assetCode != "" {
		q = q.Where("asset_code = ?", assetCode)
	}

	// Normalize tx_type (DB stores lower-case values: receive/send/swap/contract_call/approve).
	txType := strings.ToLower(strings.TrimSpace(f.TxType))
	if txType != "" {
		q = q.Where("tx_type = ?", txType)
	}

	// Business rule:
	// Only native-asset rows are allowed to show tx_type=contract_call.
	//
	// - If chain_code is specified, allow contract_call only for that chain's native asset:
	//   ETH -> ETH, BSC -> BNB, TRON -> TRX.
	// - If chain_code is not specified, allow contract_call only for native assets across supported chains.
	allowedSwapAssetCodes := getAllowedSwapAssetCodes(chainCode)
	if txType == "" {
		// When listing "all tx types", suppress contract_call records for non-native assets.
		if len(allowedSwapAssetCodes) > 0 {
			q = q.Where("(tx_type <> ? OR asset_code IN ?)", model.Web3TxTypeContractCall, allowedSwapAssetCodes)
		} else {
			// Unknown chain_code: safest is to hide all contract_call records.
			q = q.Where("tx_type <> ?", model.Web3TxTypeContractCall)
		}
	} else if txType == model.Web3TxTypeContractCall {
		// When explicitly querying contract_call, hard-limit to native assets only.
		if len(allowedSwapAssetCodes) > 0 {
			q = q.Where("asset_code IN ?", allowedSwapAssetCodes)
		} else {
			// Unknown chain_code: no contract_call should be returned.
			q = q.Where("1 = 0")
		}
	}

	if s := strings.TrimSpace(f.Direction); s != "" {
		q = q.Where("direction = ?", s)
	}
	if s := strings.TrimSpace(f.Status); s != "" {
		q = q.Where("status = ?", s)
	}
	if f.FromTime != nil && !f.FromTime.IsZero() {
		q = q.Where("block_time >= ?", *f.FromTime)
	}
	if f.ToTime != nil && !f.ToTime.IsZero() {
		q = q.Where("block_time <= ?", *f.ToTime)
	}
	return q
}

func getAllowedSwapAssetCodes(chainCode string) []string {
	chainCode = strings.ToUpper(strings.TrimSpace(chainCode))
	if chainCode == "" {
		// When chain is not specified, only allow native assets across the supported chains.
		// NOTE: keep BSC's native as BNB (asset_code), not chain_code.
		return []string{"ETH", "BNB", "TRX"}
	}

	switch chainCode {
	case "ETH", "ETHEREUM":
		return []string{"ETH"}
	case "BSC", "BNB", "BNB SMART CHAIN", "BINANCE", "BINANCE SMART CHAIN":
		return []string{"BNB"}
	case "TRON", "TRX":
		return []string{"TRX"}
	default:
		return nil
	}
}

func getWeb3BalanceChangeOrderClause(f Web3BalanceChangeListFilter) string {
	orderCol := "block_time"
	if strings.EqualFold(strings.TrimSpace(f.SortBy), "created_at") {
		orderCol = "created_at"
	}

	orderDir := "DESC"
	if strings.EqualFold(strings.TrimSpace(f.SortOrder), "asc") {
		orderDir = "ASC"
	}
	return orderCol + " " + orderDir
}
