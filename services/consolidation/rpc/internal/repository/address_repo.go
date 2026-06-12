package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

type DepositAddressRow struct {
	ID        int64  `json:"id"`
	UserID    int64  `json:"user_id"`
	ChainCode string `json:"chain_code"`
	Address   string `json:"address"`
}

type AddressRepository interface {
	ListConsolidatableDepositAddresses(ctx context.Context, chains []string, afterID int64, limit int) ([]DepositAddressRow, error)
}

type addressRepo struct {
	db *gorm.DB
}

func NewAddressRepository(db *gorm.DB) AddressRepository {
	return &addressRepo{db: db}
}

func (r *addressRepo) ListConsolidatableDepositAddresses(ctx context.Context, chains []string, afterID int64, limit int) ([]DepositAddressRow, error) {
	if r.db == nil {
		return nil, fmt.Errorf("db not configured")
	}
	if limit <= 0 {
		limit = 1000
	}

	// NOTE:
	// - wallet_user_chain_addresses is the source of truth for deposit addresses.
	// - address_generation_logs ensures the address is signable via Signer (seed/path lookup).
	// - company_wallets and vault_addresses are excluded defensively.
	//
	// IMPORTANT (MariaDB collation):
	// Some tables use utf8mb4_general_ci while others use utf8mb4_unicode_ci.
	// Cross-table equality joins can fail with:
	//   Error 1267 (HY000): Illegal mix of collations ...
	// We force a consistent collation on the general_ci side for join predicates.
	sql := `
SELECT
  w.id AS id,
  w.user_id AS user_id,
  w.chain_code AS chain_code,
  w.address AS address
FROM wallet_user_chain_addresses w
JOIN address_generation_logs a
  ON a.address COLLATE utf8mb4_unicode_ci = w.address
  AND a.chain COLLATE utf8mb4_unicode_ci = w.chain_code
  AND a.deleted_at IS NULL
LEFT JOIN company_wallets c
  ON c.address COLLATE utf8mb4_unicode_ci = w.address
  AND c.chain COLLATE utf8mb4_unicode_ci = w.chain_code
  AND c.deleted_at IS NULL
LEFT JOIN vault_addresses v
  ON v.address = w.address
  AND v.deleted_at IS NULL
WHERE w.deleted_at IS NULL
  AND w.status = 'active'
  AND w.chain_code IN (?)
  AND w.id > ?
  AND c.id IS NULL
  AND v.id IS NULL
ORDER BY w.id ASC
LIMIT ?`

	var rows []DepositAddressRow
	if err := r.db.WithContext(ctx).Raw(sql, chains, afterID, limit).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}
