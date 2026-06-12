package repository

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// EnsureConsolidationSchema fails fast if the consolidation_tasks table does not match the assumptions made by code.
// This prevents unsafe multi-instance execution when critical migrations (e.g., 63/64/65) are missing.
func EnsureConsolidationSchema(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("db not configured")
	}

	requiredCols := []string{
		"signed_tx",       // 63-consolidation-signed-tx.sql
		"version",         // 64-consolidation-address-lock.sql
		"bump_count",      // 65-consolidation-evm-bump.sql
		"active_task_key", // 60+64
	}
	for _, col := range requiredCols {
		var cnt int64
		if err := db.Raw(
			"SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'consolidation_tasks' AND COLUMN_NAME = ?",
			col,
		).Scan(&cnt).Error; err != nil {
			return fmt.Errorf("schema check failed for consolidation_tasks.%s: %w", col, err)
		}
		if cnt == 0 {
			return fmt.Errorf("missing consolidation_tasks.%s (apply DB migrations for consolidation service)", col)
		}
	}

	var idxCnt int64
	if err := db.Raw(
		"SELECT COUNT(*) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'consolidation_tasks' AND INDEX_NAME = 'uk_consolidation_tasks_active_task_key'",
	).Scan(&idxCnt).Error; err != nil {
		return fmt.Errorf("schema check failed for uk_consolidation_tasks_active_task_key: %w", err)
	}
	if idxCnt == 0 {
		return fmt.Errorf("missing unique index uk_consolidation_tasks_active_task_key (apply migration 64-consolidation-address-lock.sql)")
	}

	type genInfo struct {
		Expr  string `gorm:"column:expr"`
		Extra string `gorm:"column:extra"`
	}
	var gi genInfo
	if err := db.Raw(
		"SELECT GENERATION_EXPRESSION AS expr, EXTRA AS extra FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'consolidation_tasks' AND COLUMN_NAME = 'active_task_key'",
	).Scan(&gi).Error; err != nil {
		return fmt.Errorf("schema check failed for consolidation_tasks.active_task_key: %w", err)
	}
	expr := strings.TrimSpace(gi.Expr)
	if expr == "" || !strings.Contains(strings.ToLower(gi.Extra), "generated") {
		return fmt.Errorf("consolidation_tasks.active_task_key is not a generated column (apply migration 64-consolidation-address-lock.sql)")
	}
	// Must be per-address (chain#from_address), not the old per-(chain,asset,token,from) key.
	if strings.Contains(expr, "asset_symbol") || strings.Contains(expr, "token_contract") {
		return fmt.Errorf("consolidation_tasks.active_task_key uses legacy expression (apply migration 64-consolidation-address-lock.sql)")
	}
	// Must keep the address lock active for uncertain/blocked states, including Timeout (6).
	if !strings.Contains(expr, "status") || !strings.Contains(expr, "6") {
		return fmt.Errorf("consolidation_tasks.active_task_key does not include Timeout in its active set (apply migration 64-consolidation-address-lock.sql)")
	}
	return nil
}
