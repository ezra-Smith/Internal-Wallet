package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"internalwallet/common/utils"
	"internalwallet/pkg/accounting"
	"internalwallet/services/accounting/rpc/internal/model"
)

const (
	OwnerTypeUser   = "user"
	OwnerTypeSystem = "system"

	AccountTypeUserLiability = "USER_LIABILITY"
	AccountTypeSysWalletHot  = "SYS_WALLET_HOT"
	AccountTypeSysFeeIncome  = "SYS_FEE_INCOME"
	AccountTypeSysAdjustExp  = "SYS_ADJUST_EXPENSE"
	AccountTypeSysAdjustInc  = "SYS_ADJUST_INCOME"
	AccountTypeSysCapital    = "SYS_CAPITAL"

	BalanceBucketAvailable = "available"
	BalanceBucketLocked    = "locked"
)

var (
	ErrInvalidRequest        = errors.New("invalid request")
	ErrIdempotencyConflict   = errors.New("idempotency conflict")
	ErrInsufficientFunds     = errors.New("insufficient funds")
	ErrAccountNotFound       = errors.New("account not found")
	ErrAssetNotFoundOrFrozen = errors.New("asset not available")
)

type Posting struct {
	AssetCode string
	AccountID int64
	Bucket    string
	DebitRaw  decimal.Decimal
	CreditRaw decimal.Decimal
}

type PostTxRequest struct {
	IdempotencyKey string
	OpType         string
	BizRef         string
	Postings       []Posting
}

type PostTxResult struct {
	TxID      int64
	Duplicate bool
}

type Engine struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Engine { return &Engine{db: db} }

func (e *Engine) EnsureUserAccountingSetup(ctx context.Context, userID int64) error {
	if e.db == nil {
		return fmt.Errorf("db not initialized")
	}
	if userID <= 0 {
		return fmt.Errorf("%w: user_id", ErrInvalidRequest)
	}

	return e.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1) Ensure user liability account exists.
		userAcc := &model.AcctAccountModel{
			ID:              utils.GenerateID(),
			OwnerType:       OwnerTypeUser,
			OwnerID:         userID,
			AccountTypeCode: AccountTypeUserLiability,
			ChainScope:      "",
		}
		if err := ensureAccount(tx, userAcc); err != nil {
			return err
		}

		// 2) Ensure USER_LIABILITY supports all enabled assets (best-effort).
		assets, err := listEnabledAssets(tx)
		if err != nil {
			return err
		}
		if err := ensureAccountTypeAssets(tx, AccountTypeUserLiability, assets); err != nil {
			return err
		}

		// 3) Ensure balance rows exist for all enabled assets (available+locked).
		for _, a := range assets {
			if err := insertIgnoreBalance(tx, userAcc.ID, a.Code, BalanceBucketAvailable); err != nil {
				return err
			}
			if err := insertIgnoreBalance(tx, userAcc.ID, a.Code, BalanceBucketLocked); err != nil {
				return err
			}
		}
		return nil
	})
}

func (e *Engine) EnsureSystemAccounts(ctx context.Context, chainCode string) error {
	if e.db == nil {
		return fmt.Errorf("db not initialized")
	}
	chainCode = accounting.NormalizeChainCode(chainCode)

	return e.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		assets, err := listEnabledAssets(tx)
		if err != nil {
			return err
		}

		// Always ensure global system accounts.
		sysAccounts := []*model.AcctAccountModel{
			{ID: utils.GenerateID(), OwnerType: OwnerTypeSystem, OwnerID: 0, AccountTypeCode: AccountTypeSysFeeIncome, ChainScope: ""},
			{ID: utils.GenerateID(), OwnerType: OwnerTypeSystem, OwnerID: 0, AccountTypeCode: AccountTypeSysAdjustExp, ChainScope: ""},
			{ID: utils.GenerateID(), OwnerType: OwnerTypeSystem, OwnerID: 0, AccountTypeCode: AccountTypeSysAdjustInc, ChainScope: ""},
			{ID: utils.GenerateID(), OwnerType: OwnerTypeSystem, OwnerID: 0, AccountTypeCode: AccountTypeSysCapital, ChainScope: ""},
		}
		for _, a := range sysAccounts {
			if err := ensureAccount(tx, a); err != nil {
				return err
			}
			if err := ensureAccountTypeAssets(tx, a.AccountTypeCode, assets); err != nil {
				return err
			}
			// System accounts use available bucket by convention.
			for _, asset := range assets {
				if err := insertIgnoreBalance(tx, a.ID, asset.Code, BalanceBucketAvailable); err != nil {
					return err
				}
			}
		}

		// Ensure chain-scoped hot wallet account if chain_code provided.
		if chainCode != "" {
			hot := &model.AcctAccountModel{
				ID:              utils.GenerateID(),
				OwnerType:       OwnerTypeSystem,
				OwnerID:         0,
				AccountTypeCode: AccountTypeSysWalletHot,
				ChainScope:      chainCode,
			}
			if err := ensureAccount(tx, hot); err != nil {
				return err
			}
			if err := ensureAccountTypeAssets(tx, AccountTypeSysWalletHot, assets); err != nil {
				return err
			}
			for _, asset := range assets {
				if err := insertIgnoreBalance(tx, hot.ID, asset.Code, BalanceBucketAvailable); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (e *Engine) PostTx(ctx context.Context, req PostTxRequest) (PostTxResult, error) {
	if e.db == nil {
		return PostTxResult{}, fmt.Errorf("db not initialized")
	}
	req.IdempotencyKey = strings.TrimSpace(req.IdempotencyKey)
	req.OpType = strings.TrimSpace(req.OpType)
	req.BizRef = strings.TrimSpace(req.BizRef)
	if req.IdempotencyKey == "" || len(req.IdempotencyKey) > 128 {
		return PostTxResult{}, fmt.Errorf("%w: idempotency_key", ErrInvalidRequest)
	}
	if req.OpType == "" || len(req.OpType) > 64 {
		return PostTxResult{}, fmt.Errorf("%w: op_type", ErrInvalidRequest)
	}
	if len(req.BizRef) > 128 {
		return PostTxResult{}, fmt.Errorf("%w: biz_ref", ErrInvalidRequest)
	}
	if len(req.Postings) == 0 {
		return PostTxResult{}, fmt.Errorf("%w: postings", ErrInvalidRequest)
	}

	normalized := make([]Posting, 0, len(req.Postings))
	for _, p := range req.Postings {
		p.AssetCode = accounting.NormalizeAssetCode(p.AssetCode)
		p.Bucket = strings.ToLower(strings.TrimSpace(p.Bucket))
		if p.AssetCode == "" || p.AccountID <= 0 || (p.Bucket != BalanceBucketAvailable && p.Bucket != BalanceBucketLocked) {
			return PostTxResult{}, fmt.Errorf("%w: posting fields", ErrInvalidRequest)
		}
		if p.DebitRaw.LessThan(decimal.Zero) || p.CreditRaw.LessThan(decimal.Zero) {
			return PostTxResult{}, fmt.Errorf("%w: negative amount", ErrInvalidRequest)
		}
		if (p.DebitRaw.Equal(decimal.Zero) && p.CreditRaw.Equal(decimal.Zero)) || (!p.DebitRaw.Equal(decimal.Zero) && !p.CreditRaw.Equal(decimal.Zero)) {
			return PostTxResult{}, fmt.Errorf("%w: posting must be one-sided", ErrInvalidRequest)
		}
		if err := accounting.ValidateDecimal65Int(p.DebitRaw); err != nil {
			return PostTxResult{}, fmt.Errorf("%w: debit_raw %v", ErrInvalidRequest, err)
		}
		if err := accounting.ValidateDecimal65Int(p.CreditRaw); err != nil {
			return PostTxResult{}, fmt.Errorf("%w: credit_raw %v", ErrInvalidRequest, err)
		}
		normalized = append(normalized, p)
	}
	req.Postings = normalized

	// Strict double-entry per asset: Σdebit == Σcredit.
	sumDebit := map[string]decimal.Decimal{}
	sumCredit := map[string]decimal.Decimal{}
	for _, p := range req.Postings {
		sumDebit[p.AssetCode] = sumDebit[p.AssetCode].Add(p.DebitRaw)
		sumCredit[p.AssetCode] = sumCredit[p.AssetCode].Add(p.CreditRaw)
	}
	for asset, d := range sumDebit {
		if !d.Equal(sumCredit[asset]) {
			return PostTxResult{}, fmt.Errorf("%w: unbalanced postings for %s", ErrInvalidRequest, asset)
		}
	}

	// Request hash based on derived ledger intent (op + biz_ref + postings).
	reqHash, err := computeRequestHash(req.OpType, req.BizRef, req.Postings)
	if err != nil {
		return PostTxResult{}, err
	}

	// Load account normal side and supported assets for validation.
	accountIDs := uniqueAccountIDs(req.Postings)
	accountSides, err := e.getAccountNormalSides(ctx, accountIDs)
	if err != nil {
		return PostTxResult{}, err
	}
	if len(accountSides) != len(accountIDs) {
		return PostTxResult{}, ErrAccountNotFound
	}
	if err := e.validateAccountAssetSupport(ctx, req.Postings); err != nil {
		return PostTxResult{}, err
	}

	// Aggregate per balance key deltas using normal-side balance semantics.
	deltas := map[balanceKey]decimal.Decimal{}
	for _, p := range req.Postings {
		side, ok := accountSides[p.AccountID]
		if !ok {
			return PostTxResult{}, ErrAccountNotFound
		}
		var delta decimal.Decimal
		switch side {
		case "debit":
			delta = p.DebitRaw.Sub(p.CreditRaw)
		case "credit":
			delta = p.CreditRaw.Sub(p.DebitRaw)
		default:
			return PostTxResult{}, fmt.Errorf("%w: invalid normal_side", ErrInvalidRequest)
		}
		k := balanceKey{AccountID: p.AccountID, AssetCode: p.AssetCode, Bucket: p.Bucket}
		deltas[k] = deltas[k].Add(delta)
	}

	keys := make([]balanceKey, 0, len(deltas))
	for k := range deltas {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].AccountID != keys[j].AccountID {
			return keys[i].AccountID < keys[j].AccountID
		}
		if keys[i].AssetCode != keys[j].AssetCode {
			return keys[i].AssetCode < keys[j].AssetCode
		}
		return keys[i].Bucket < keys[j].Bucket
	})

	var out PostTxResult
	err = e.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().Local()

		// 1) Insert tx header (idempotency key unique).
		txID := utils.GenerateID()
		header := &model.AcctLedgerTxModel{
			ID:             txID,
			OpType:         req.OpType,
			BizRef:         req.BizRef,
			IdempotencyKey: req.IdempotencyKey,
			RequestHash:    reqHash,
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		if err := tx.Create(header).Error; err != nil {
			if isDuplicateKey(err) {
				existing, err2 := findLedgerTxByIdempotencyKey(tx, req.IdempotencyKey)
				if err2 != nil {
					return err2
				}
				if existing.RequestHash != reqHash {
					return ErrIdempotencyConflict
				}
				out = PostTxResult{TxID: existing.ID, Duplicate: true}
				return nil
			}
			return err
		}

		// 2) Ensure balance rows exist (INSERT IGNORE) then apply deltas with no-underflow checks.
		for _, k := range keys {
			if err := insertIgnoreBalance(tx, k.AccountID, k.AssetCode, k.Bucket); err != nil {
				return err
			}
		}
		for _, k := range keys {
			delta := deltas[k]
			if delta.Equal(decimal.Zero) {
				continue
			}
			if err := applyBalanceDelta(tx, k, delta); err != nil {
				return err
			}
		}

		// 3) Insert postings.
		postings := make([]*model.AcctLedgerPostingModel, 0, len(req.Postings))
		for i, p := range req.Postings {
			postings = append(postings, &model.AcctLedgerPostingModel{
				TxID:      txID,
				Seq:       int32(i + 1),
				AssetCode: p.AssetCode,
				AccountID: p.AccountID,
				Bucket:    p.Bucket,
				DebitRaw:  p.DebitRaw.String(),
				CreditRaw: p.CreditRaw.String(),
				CreatedAt: now,
				UpdatedAt: now,
			})
		}
		if err := tx.CreateInBatches(postings, 200).Error; err != nil {
			return err
		}
		out = PostTxResult{TxID: txID, Duplicate: false}
		return nil
	})
	if err != nil {
		return PostTxResult{}, err
	}
	return out, nil
}

type balanceKey struct {
	AccountID int64
	AssetCode string
	Bucket    string
}

func uniqueAccountIDs(postings []Posting) []int64 {
	set := map[int64]struct{}{}
	for _, p := range postings {
		set[p.AccountID] = struct{}{}
	}
	out := make([]int64, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func computeRequestHash(opType, bizRef string, postings []Posting) (string, error) {
	type hPosting struct {
		AssetCode string `json:"asset_code"`
		AccountID int64  `json:"account_id"`
		Bucket    string `json:"bucket"`
		DebitRaw  string `json:"debit_raw"`
		CreditRaw string `json:"credit_raw"`
	}
	type payload struct {
		OpType   string     `json:"op_type"`
		BizRef   string     `json:"biz_ref"`
		Postings []hPosting `json:"postings"`
	}
	rows := make([]hPosting, 0, len(postings))
	for _, p := range postings {
		rows = append(rows, hPosting{
			AssetCode: p.AssetCode,
			AccountID: p.AccountID,
			Bucket:    p.Bucket,
			DebitRaw:  p.DebitRaw.String(),
			CreditRaw: p.CreditRaw.String(),
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].AssetCode != rows[j].AssetCode {
			return rows[i].AssetCode < rows[j].AssetCode
		}
		if rows[i].AccountID != rows[j].AccountID {
			return rows[i].AccountID < rows[j].AccountID
		}
		if rows[i].Bucket != rows[j].Bucket {
			return rows[i].Bucket < rows[j].Bucket
		}
		if rows[i].DebitRaw != rows[j].DebitRaw {
			return rows[i].DebitRaw < rows[j].DebitRaw
		}
		return rows[i].CreditRaw < rows[j].CreditRaw
	})
	b, err := json.Marshal(payload{OpType: opType, BizRef: bizRef, Postings: rows})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func isDuplicateKey(err error) bool {
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) {
		return mysqlErr.Number == 1062
	}
	// Fallback string check (best-effort).
	return strings.Contains(strings.ToLower(err.Error()), "duplicate")
}

func listEnabledAssets(tx *gorm.DB) ([]model.AssetModel, error) {
	var assets []model.AssetModel
	err := tx.Model(&model.AssetModel{}).Where("status = 1").Order("id ASC").Find(&assets).Error
	return assets, err
}

func ensureAccountTypeAssets(tx *gorm.DB, accountTypeCode string, assets []model.AssetModel) error {
	accountTypeCode = strings.ToUpper(strings.TrimSpace(accountTypeCode))
	if accountTypeCode == "" {
		return fmt.Errorf("%w: account_type_code", ErrInvalidRequest)
	}
	for _, a := range assets {
		code := accounting.NormalizeAssetCode(a.Code)
		if code == "" {
			continue
		}
		if err := tx.Exec(
			"INSERT IGNORE INTO acct_account_type_assets (account_type_code, asset_code, created_at, updated_at, deleted_at) VALUES (?, ?, NOW(), NOW(), NULL)",
			accountTypeCode, code,
		).Error; err != nil {
			return err
		}
	}
	return nil
}

func ensureAccount(tx *gorm.DB, m *model.AcctAccountModel) error {
	if m == nil || m.ID <= 0 {
		return fmt.Errorf("%w: account", ErrInvalidRequest)
	}
	m.OwnerType = strings.ToLower(strings.TrimSpace(m.OwnerType))
	m.AccountTypeCode = strings.ToUpper(strings.TrimSpace(m.AccountTypeCode))
	m.ChainScope = strings.ToUpper(strings.TrimSpace(m.ChainScope))

	var existing model.AcctAccountModel
	err := tx.Model(&model.AcctAccountModel{}).
		Where("owner_type = ? AND owner_id = ? AND account_type_code = ? AND chain_scope = ?",
			m.OwnerType, m.OwnerID, m.AccountTypeCode, m.ChainScope,
		).
		First(&existing).Error
	if err == nil {
		m.ID = existing.ID
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	if err := tx.Create(m).Error; err != nil {
		// Race fallback.
		var again model.AcctAccountModel
		if err2 := tx.Model(&model.AcctAccountModel{}).
			Where("owner_type = ? AND owner_id = ? AND account_type_code = ? AND chain_scope = ?",
				m.OwnerType, m.OwnerID, m.AccountTypeCode, m.ChainScope,
			).
			First(&again).Error; err2 == nil {
			m.ID = again.ID
			return nil
		}
		return err
	}
	return nil
}

func insertIgnoreBalance(tx *gorm.DB, accountID int64, assetCode string, bucket string) error {
	assetCode = accounting.NormalizeAssetCode(assetCode)
	bucket = strings.ToLower(strings.TrimSpace(bucket))
	if accountID <= 0 || assetCode == "" || (bucket != BalanceBucketAvailable && bucket != BalanceBucketLocked) {
		return fmt.Errorf("%w: balance key", ErrInvalidRequest)
	}
	return tx.Exec(
		"INSERT IGNORE INTO acct_balances (account_id, asset_code, bucket, balance_raw, created_at, updated_at, deleted_at) VALUES (?, ?, ?, 0, NOW(), NOW(), NULL)",
		accountID, assetCode, bucket,
	).Error
}

func applyBalanceDelta(tx *gorm.DB, k balanceKey, delta decimal.Decimal) error {
	if delta.Equal(decimal.Zero) {
		return nil
	}
	// Validate integer delta magnitude (DECIMAL(65,0)).
	if err := accounting.ValidateDecimal65Int(delta.Abs()); err != nil {
		return fmt.Errorf("%w: delta too large", ErrInvalidRequest)
	}
	res := tx.Exec(
		"UPDATE acct_balances SET balance_raw = balance_raw + ?, updated_at = NOW() "+
			"WHERE account_id = ? AND asset_code = ? AND bucket = ? AND deleted_at IS NULL AND balance_raw + ? >= 0",
		delta.String(), k.AccountID, k.AssetCode, k.Bucket, delta.String(),
	)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return ErrInsufficientFunds
	}
	return nil
}

func findLedgerTxByIdempotencyKey(tx *gorm.DB, key string) (*model.AcctLedgerTxModel, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var m model.AcctLedgerTxModel
	if err := tx.Model(&model.AcctLedgerTxModel{}).Where("idempotency_key = ?", key).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

func (e *Engine) getAccountNormalSides(ctx context.Context, accountIDs []int64) (map[int64]string, error) {
	if len(accountIDs) == 0 {
		return map[int64]string{}, nil
	}
	type row struct {
		ID         int64  `gorm:"column:id"`
		NormalSide string `gorm:"column:normal_side"`
	}
	var rows []row
	err := e.db.WithContext(ctx).
		Table("acct_accounts a").
		Select("a.id, t.normal_side").
		Joins("JOIN acct_account_types t ON t.code = a.account_type_code").
		Where("a.id IN ? AND a.deleted_at IS NULL AND t.deleted_at IS NULL", accountIDs).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[int64]string, len(rows))
	for _, r := range rows {
		out[r.ID] = strings.ToLower(strings.TrimSpace(r.NormalSide))
	}
	return out, nil
}

func (e *Engine) validateAccountAssetSupport(ctx context.Context, postings []Posting) error {
	type pair struct {
		AccountID int64
		AssetCode string
	}
	seen := map[pair]struct{}{}
	pairs := make([]pair, 0, len(postings))
	accountIDs := make([]int64, 0)
	assetCodes := make([]string, 0)
	seenAcc := map[int64]struct{}{}
	seenAsset := map[string]struct{}{}
	for _, p := range postings {
		k := pair{AccountID: p.AccountID, AssetCode: p.AssetCode}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		pairs = append(pairs, k)
		if _, ok := seenAcc[p.AccountID]; !ok {
			seenAcc[p.AccountID] = struct{}{}
			accountIDs = append(accountIDs, p.AccountID)
		}
		if _, ok := seenAsset[p.AssetCode]; !ok {
			seenAsset[p.AssetCode] = struct{}{}
			assetCodes = append(assetCodes, p.AssetCode)
		}
	}
	if len(pairs) == 0 {
		return nil
	}
	sort.Slice(accountIDs, func(i, j int) bool { return accountIDs[i] < accountIDs[j] })
	sort.Strings(assetCodes)

	// Validate via join: account -> type -> type_assets(asset_code).
	// Each (account_id, asset_code) must have a matching row in acct_account_type_assets.
	type row struct {
		AccountID int64  `gorm:"column:account_id"`
		AssetCode string `gorm:"column:asset_code"`
	}
	var rows []row
	err := e.db.WithContext(ctx).
		Table("acct_accounts a").
		Select("a.id AS account_id, ata.asset_code AS asset_code").
		Joins("JOIN acct_account_type_assets ata ON ata.account_type_code = a.account_type_code").
		Where("a.deleted_at IS NULL AND a.id IN ? AND ata.asset_code IN ? AND ata.deleted_at IS NULL", accountIDs, assetCodes).
		Scan(&rows).Error
	if err != nil {
		return err
	}
	okSet := map[pair]struct{}{}
	for _, r := range rows {
		okSet[pair{AccountID: r.AccountID, AssetCode: accounting.NormalizeAssetCode(r.AssetCode)}] = struct{}{}
	}
	for _, p := range pairs {
		if _, ok := okSet[p]; !ok {
			return fmt.Errorf("%w: account %d does not support asset %s", ErrInvalidRequest, p.AccountID, p.AssetCode)
		}
	}
	return nil
}
