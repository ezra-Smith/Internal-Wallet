package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

// CurrencyChainSettingsRow is a minimal view for `currency_chain_settings`.
// NOTE: This table is owned by the currency-management domain (admin), but is used here
// as the source of truth for token metadata needed by consolidation (contract + decimals + switches).
type CurrencyChainSettingsRow struct {
	AssetCode       string  `gorm:"column:asset_code"`
	ChainCode       string  `gorm:"column:chain_code"`
	ContractAddress *string `gorm:"column:contract_address"`
	TokenDecimals   *int32  `gorm:"column:token_decimals"`
}

func (CurrencyChainSettingsRow) TableName() string { return "currency_chain_settings" }

type ConsolidationFingerprint struct {
	EnabledCount int64
	MaxUpdatedAt *time.Time
	Checksum     uint64
}

type CurrencyChainSettingsRepository interface {
	ListEnabledForConsolidation(ctx context.Context, chainCodes []string) ([]CurrencyChainSettingsRow, error)
	GetConsolidationFingerprint(ctx context.Context, chainCodes []string) (ConsolidationFingerprint, error)
}

type currencyChainSettingsRepo struct {
	db *gorm.DB
}

func NewCurrencyChainSettingsRepository(db *gorm.DB) CurrencyChainSettingsRepository {
	return &currencyChainSettingsRepo{db: db}
}

func (r *currencyChainSettingsRepo) ListEnabledForConsolidation(ctx context.Context, chainCodes []string) ([]CurrencyChainSettingsRow, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("db not configured")
	}
	normalizedChains := make([]string, 0, len(chainCodes))
	seen := map[string]struct{}{}
	for _, c := range chainCodes {
		cc := strings.ToUpper(strings.TrimSpace(c))
		if cc == "" {
			continue
		}
		if _, ok := seen[cc]; ok {
			continue
		}
		seen[cc] = struct{}{}
		normalizedChains = append(normalizedChains, cc)
	}
	if len(normalizedChains) == 0 {
		return []CurrencyChainSettingsRow{}, nil
	}

	q := r.db.WithContext(ctx).
		Model(&CurrencyChainSettingsRow{}).
		Select("asset_code, chain_code, contract_address, token_decimals").
		Where("deleted_at IS NULL AND status = 1 AND consolidation_enabled = 1").
		Where("chain_code IN ?", normalizedChains).
		Order("chain_code ASC, asset_code ASC")

	var rows []CurrencyChainSettingsRow
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *currencyChainSettingsRepo) GetConsolidationFingerprint(ctx context.Context, chainCodes []string) (ConsolidationFingerprint, error) {
	if r == nil || r.db == nil {
		return ConsolidationFingerprint{}, fmt.Errorf("db not configured")
	}

	normalizedChains := make([]string, 0, len(chainCodes))
	seen := map[string]struct{}{}
	for _, c := range chainCodes {
		cc := strings.ToUpper(strings.TrimSpace(c))
		if cc == "" {
			continue
		}
		if _, ok := seen[cc]; ok {
			continue
		}
		seen[cc] = struct{}{}
		normalizedChains = append(normalizedChains, cc)
	}
	if len(normalizedChains) == 0 {
		return ConsolidationFingerprint{}, nil
	}

	var out struct {
		EnabledCount int64      `gorm:"column:enabled_count"`
		MaxUpdatedAt *time.Time `gorm:"column:max_updated_at"`
		Checksum     uint64     `gorm:"column:checksum"`
	}
	err := r.db.WithContext(ctx).
		Table("currency_chain_settings").
		Select(`
			COUNT(1) AS enabled_count,
			MAX(updated_at) AS max_updated_at,
			CAST(COALESCE(SUM(CAST(CRC32(CONCAT_WS('#',
				asset_code,
				chain_code,
				IFNULL(contract_address, ''),
				IFNULL(token_decimals, -1)
			)) AS UNSIGNED)), 0) AS UNSIGNED) AS checksum
		`).
		Where("deleted_at IS NULL AND status = 1 AND consolidation_enabled = 1").
		Where("chain_code IN ?", normalizedChains).
		Scan(&out).Error
	if err != nil {
		return ConsolidationFingerprint{}, err
	}
	return ConsolidationFingerprint{
		EnabledCount: out.EnabledCount,
		MaxUpdatedAt: out.MaxUpdatedAt,
		Checksum:     out.Checksum,
	}, nil
}
