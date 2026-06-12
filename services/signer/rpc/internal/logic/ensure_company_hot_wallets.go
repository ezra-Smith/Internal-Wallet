package logic

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"internalwallet/services/signer/rpc/internal/models"
	"internalwallet/services/signer/rpc/internal/svc"

	"gorm.io/gorm"
)

type companyWalletMisconfiguredError struct {
	Chain           string
	ExpectedAddress string
	ExpectedPath    string
	ExpectedSeedID  string

	ExistingID      int64
	ExistingType    string
	ExistingIndex   int
	ExistingAddress string
	ExistingPath    string
	ExistingSeedID  string
	ExistingTemp    int8
}

func (e *companyWalletMisconfiguredError) Error() string {
	return fmt.Sprintf(
		"company hot wallet misconfigured: chain=%s expected(seed_id=%s path=%s address=%s) existing(id=%d type=%s index=%d temp=%d seed_id=%s path=%s address=%s)",
		strings.TrimSpace(e.Chain),
		strings.TrimSpace(e.ExpectedSeedID),
		strings.TrimSpace(e.ExpectedPath),
		strings.TrimSpace(e.ExpectedAddress),
		e.ExistingID,
		strings.TrimSpace(e.ExistingType),
		e.ExistingIndex,
		e.ExistingTemp,
		strings.TrimSpace(e.ExistingSeedID),
		strings.TrimSpace(e.ExistingPath),
		strings.TrimSpace(e.ExistingAddress),
	)
}

func ensureCompanyHotPrimaryWallets(ctx context.Context, svcCtx *svc.ServiceContext, seedID string, seedBytes []byte) error {
	if svcCtx == nil || svcCtx.DB == nil {
		return fmt.Errorf("db not configured")
	}
	seedID = strings.TrimSpace(seedID)
	if seedID == "" || len(seedBytes) < 16 {
		return fmt.Errorf("invalid seed")
	}

	type derived struct {
		Chain   string
		Address string
		Path    string
	}

	chains := []string{"ETH", "BSC", "TRON"}
	derivedList := make([]derived, 0, len(chains))
	for _, chain := range chains {
		path, err := BuildCompanyWalletPath(chain, 1)
		if err != nil {
			return err
		}
		w, err := NewHDWalletFromSeed(seedBytes, chain)
		if err != nil {
			return err
		}
		uncompressed, _, err := w.GetPublicKey(path)
		if err != nil {
			return err
		}
		addr, err := PublicKeyToAddress(chain, uncompressed)
		if err != nil {
			return err
		}
		derivedList = append(derivedList, derived{
			Chain:   chain,
			Address: strings.TrimSpace(addr),
			Path:    strings.TrimSpace(path),
		})
	}

	return svcCtx.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, d := range derivedList {
			if err := ensureOneHotPrimary(ctx, tx, seedID, d.Chain, d.Address, d.Path); err != nil {
				return err
			}
			// Seed vault funds tables so admin /vault/funds has wallet rows to display and sync balances.
			if err := ensureVaultFundsForHotPrimary(ctx, tx, d.Chain, d.Address); err != nil {
				return err
			}
		}
		return nil
	})
}

func ensureOneHotPrimary(ctx context.Context, tx *gorm.DB, seedID string, chain string, expectedAddr string, expectedPath string) error {
	if tx == nil {
		return fmt.Errorf("tx is nil")
	}
	seedID = strings.TrimSpace(seedID)
	chain = strings.ToUpper(strings.TrimSpace(chain))
	expectedAddr = strings.TrimSpace(expectedAddr)
	expectedPath = strings.TrimSpace(expectedPath)
	if seedID == "" || chain == "" || expectedAddr == "" || expectedPath == "" {
		return fmt.Errorf("invalid ensure params")
	}

	// 1) locate existing hot_primary (any status) for this chain
	var existingPrimary models.CompanyWallet
	primaryErr := tx.WithContext(ctx).
		Model(&models.CompanyWallet{}).
		Where("deleted_at IS NULL").
		Where("chain = ? AND address_type = ?", chain, "hot_primary").
		Order("address_index ASC, id ASC").
		Limit(1).
		Find(&existingPrimary).Error
	if primaryErr != nil {
		return primaryErr
	}

	// 2) locate existing index=1 occupant (any type) for this chain (unique constraint)
	var existingIndex models.CompanyWallet
	indexErr := tx.WithContext(ctx).
		Model(&models.CompanyWallet{}).
		Where("deleted_at IS NULL").
		Where("chain = ? AND address_index = ?", chain, 1).
		Order("id ASC").
		Limit(1).
		Find(&existingIndex).Error
	if indexErr != nil {
		return indexErr
	}

	if existingPrimary.ID != 0 {
		// Must match expected derived data.
		if err := verifyExistingHotPrimary(&existingPrimary, chain, seedID, expectedAddr, expectedPath); err != nil {
			return err
		}
		if existingPrimary.AddressIndex != 1 {
			return misconfiguredFromExisting(&existingPrimary, chain, seedID, expectedAddr, expectedPath)
		}
		if existingIndex.ID != 0 && existingIndex.ID != existingPrimary.ID {
			// index=1 is taken by another row -> schema-level misconfiguration.
			return misconfiguredFromExisting(&existingIndex, chain, seedID, expectedAddr, expectedPath)
		}
		return normalizeExistingHotPrimary(ctx, tx, &existingPrimary, chain)
	}

	// No hot_primary record exists. Index=1 must be available.
	if existingIndex.ID != 0 {
		return misconfiguredFromExisting(&existingIndex, chain, seedID, expectedAddr, expectedPath)
	}

	isDefault := int8(0)
	var defaultCount int64
	if err := tx.WithContext(ctx).
		Model(&models.CompanyWallet{}).
		Where("deleted_at IS NULL").
		Where("chain = ? AND temperature = 1 AND is_default = 1", chain).
		Count(&defaultCount).Error; err != nil {
		return err
	}
	if defaultCount == 0 {
		isDefault = 1
	}

	w := &models.CompanyWallet{
		AddressIndex:   1,
		AddressType:    "hot_primary",
		Chain:          chain,
		Address:        expectedAddr,
		DerivationPath: expectedPath,
		SeedID:         seedID,
		Temperature:    1,
		Status:         1,
		IsDefault:      isDefault,
		Remark:         "auto-generated on unlock",
	}
	return tx.WithContext(ctx).Create(w).Error
}

type vaultNetworkMeta struct {
	ChainID   int64
	Network   string
	ChainType string
}

func vaultMetaForChain(chain string) (vaultNetworkMeta, bool) {
	switch strings.ToUpper(strings.TrimSpace(chain)) {
	case "ETH", "ETHEREUM":
		return vaultNetworkMeta{ChainID: 1, Network: "ETH", ChainType: "ethereum"}, true
	case "BSC", "BNB":
		return vaultNetworkMeta{ChainID: 56, Network: "BSC", ChainType: "bsc"}, true
	case "TRON", "TRX", "TRN":
		// Use existing init-db chain_id for tron in vault_networks.
		return vaultNetworkMeta{ChainID: 728126428, Network: "TRON", ChainType: "tron"}, true
	default:
		return vaultNetworkMeta{}, false
	}
}

type vaultNetworkRow struct {
	ID        int64   `gorm:"column:id"`
	ChainID   int64   `gorm:"column:chain_id"`
	Network   string  `gorm:"column:network"`
	ChainType string  `gorm:"column:chain_type"`
	VaultAddr *string `gorm:"column:vault_address"`
}

func ensureVaultFundsForHotPrimary(ctx context.Context, tx *gorm.DB, chain string, hotAddr string) error {
	meta, ok := vaultMetaForChain(chain)
	if !ok {
		return fmt.Errorf("unsupported chain for vault funds: %s", strings.TrimSpace(chain))
	}
	hotAddr = strings.TrimSpace(hotAddr)
	if hotAddr == "" {
		return fmt.Errorf("empty hot wallet address")
	}

	net, err := findOrCreateVaultNetwork(ctx, tx, meta)
	if err != nil {
		return err
	}
	if net == nil || net.ID <= 0 {
		return fmt.Errorf("vault_networks ensure returned empty network")
	}

	// Set vault_networks.vault_address for display (best-effort).
	if net.VaultAddr == nil || strings.TrimSpace(*net.VaultAddr) == "" {
		if err := tx.WithContext(ctx).
			Table("vault_networks").
			Where("id = ?", net.ID).
			Update("vault_address", hotAddr).Error; err != nil {
			return err
		}
	}

	return ensureVaultAddress(ctx, tx, net.ID, meta.Network, hotAddr)
}

func findOrCreateVaultNetwork(ctx context.Context, tx *gorm.DB, meta vaultNetworkMeta) (*vaultNetworkRow, error) {
	var row vaultNetworkRow
	if err := tx.WithContext(ctx).
		Table("vault_networks").
		Select("id, chain_id, network, chain_type, vault_address").
		Where("deleted_at IS NULL").
		Where("chain_type = ?", meta.ChainType).
		Order("id ASC").
		Limit(1).
		Find(&row).Error; err != nil {
		return nil, err
	}
	if row.ID == 0 {
		if err := tx.WithContext(ctx).
			Table("vault_networks").
			Select("id, chain_id, network, chain_type, vault_address").
			Where("deleted_at IS NULL").
			Where("UPPER(network) = ?", strings.ToUpper(meta.Network)).
			Order("id ASC").
			Limit(1).
			Find(&row).Error; err != nil {
			return nil, err
		}
	}
	if row.ID == 0 {
		if err := tx.WithContext(ctx).
			Table("vault_networks").
			Select("id, chain_id, network, chain_type, vault_address").
			Where("deleted_at IS NULL").
			Where("chain_id = ?", meta.ChainID).
			Order("id ASC").
			Limit(1).
			Find(&row).Error; err != nil {
			return nil, err
		}
	}
	if row.ID != 0 {
		return &row, nil
	}

	createErr := tx.WithContext(ctx).Table("vault_networks").Create(map[string]any{
		"chain_id":    meta.ChainID,
		"network":     meta.Network,
		"chain_type":  meta.ChainType,
		"status":      "active",
		"sync_status": "unknown",
	}).Error
	if createErr != nil {
		// If concurrent creation, re-select.
		if strings.Contains(strings.ToLower(createErr.Error()), "duplicate") {
			var again vaultNetworkRow
			if err := tx.WithContext(ctx).
				Table("vault_networks").
				Select("id, chain_id, network, chain_type, vault_address").
				Where("deleted_at IS NULL").
				Where("chain_type = ?", meta.ChainType).
				Order("id ASC").
				Limit(1).
				Find(&again).Error; err != nil {
				return nil, err
			}
			if again.ID != 0 {
				return &again, nil
			}
		}
		return nil, createErr
	}

	var created vaultNetworkRow
	if err := tx.WithContext(ctx).
		Table("vault_networks").
		Select("id, chain_id, network, chain_type, vault_address").
		Where("deleted_at IS NULL").
		Where("chain_type = ?", meta.ChainType).
		Order("id DESC").
		Limit(1).
		Find(&created).Error; err != nil {
		return nil, err
	}
	return &created, nil
}

type vaultAddressRow struct {
	ID             int64      `gorm:"column:id"`
	NetworkID      int64      `gorm:"column:network_id"`
	Address        string     `gorm:"column:address"`
	AddressType    string     `gorm:"column:address_type"`
	Label          *string    `gorm:"column:label"`
	Status         string     `gorm:"column:status"`
	IsActiveWallet int8       `gorm:"column:is_active_wallet"`
	DeletedAt      *time.Time `gorm:"column:deleted_at"`
}

func ensureVaultAddress(ctx context.Context, tx *gorm.DB, networkID int64, networkName string, address string) error {
	if networkID <= 0 {
		return fmt.Errorf("invalid network_id")
	}
	networkName = strings.TrimSpace(networkName)
	address = strings.TrimSpace(address)
	if address == "" {
		return fmt.Errorf("empty address")
	}

	var existing vaultAddressRow
	q := tx.WithContext(ctx).Table("vault_addresses").
		Select("id, network_id, address, address_type, label, status, is_active_wallet, deleted_at").
		Where("network_id = ?", networkID).
		Where("deleted_at IS NULL")
	if strings.HasPrefix(strings.ToLower(address), "0x") {
		q = q.Where("LOWER(address) = LOWER(?)", address)
	} else {
		q = q.Where("address = ?", address)
	}
	if err := q.Order("id ASC").Limit(1).Find(&existing).Error; err != nil {
		return err
	}

	label := fmt.Sprintf("系统热钱包 - %s - hot_primary", networkName)

	if existing.ID != 0 {
		updates := map[string]any{}
		if strings.TrimSpace(existing.Status) == "" || strings.EqualFold(existing.Status, "inactive") {
			updates["status"] = "active"
		}
		if strings.TrimSpace(existing.AddressType) == "" || strings.EqualFold(existing.AddressType, "active") {
			updates["address_type"] = "hot"
		}
		if existing.Label == nil || strings.TrimSpace(*existing.Label) == "" {
			updates["label"] = label
		}
		if len(updates) == 0 {
			return nil
		}
		return tx.WithContext(ctx).Table("vault_addresses").Where("id = ?", existing.ID).Updates(updates).Error
	}

	isActive := int8(0)
	var cnt int64
	if err := tx.WithContext(ctx).Table("vault_addresses").
		Where("network_id = ?", networkID).
		Where("deleted_at IS NULL").
		Where("is_active_wallet = 1").
		Count(&cnt).Error; err != nil {
		return err
	}
	if cnt == 0 {
		isActive = 1
	}

	insertErr := tx.WithContext(ctx).Table("vault_addresses").Create(map[string]any{
		"network_id":       networkID,
		"address":          address,
		"address_type":     "hot",
		"label":            label,
		"status":           "active",
		"is_active_wallet": isActive,
		"created_by":       int64(0),
	}).Error
	if insertErr != nil && strings.Contains(strings.ToLower(insertErr.Error()), "duplicate") {
		// Some environments may have different uniqueness constraints; treat as idempotent.
		return nil
	}
	return insertErr
}

func verifyExistingHotPrimary(existing *models.CompanyWallet, chain string, seedID string, expectedAddr string, expectedPath string) error {
	if existing == nil {
		return fmt.Errorf("existing is nil")
	}
	if strings.TrimSpace(existing.AddressType) != "hot_primary" {
		return misconfiguredFromExisting(existing, chain, seedID, expectedAddr, expectedPath)
	}
	if existing.Temperature != 1 {
		return misconfiguredFromExisting(existing, chain, seedID, expectedAddr, expectedPath)
	}
	if strings.TrimSpace(existing.SeedID) != strings.TrimSpace(seedID) {
		return misconfiguredFromExisting(existing, chain, seedID, expectedAddr, expectedPath)
	}
	if strings.TrimSpace(existing.DerivationPath) != strings.TrimSpace(expectedPath) {
		return misconfiguredFromExisting(existing, chain, seedID, expectedAddr, expectedPath)
	}
	if !companyWalletAddressEqual(chain, existing.Address, expectedAddr) {
		return misconfiguredFromExisting(existing, chain, seedID, expectedAddr, expectedPath)
	}
	return nil
}

func normalizeExistingHotPrimary(ctx context.Context, tx *gorm.DB, existing *models.CompanyWallet, chain string) error {
	if tx == nil || existing == nil {
		return fmt.Errorf("invalid normalize params")
	}
	updates := map[string]interface{}{}
	if existing.Status != 1 {
		updates["status"] = int8(1)
	}

	// Best-effort: mark as default if no other default exists (unique constraint uses deleted_at + is_default + temperature).
	if existing.IsDefault != 1 {
		var cnt int64
		if err := tx.WithContext(ctx).
			Model(&models.CompanyWallet{}).
			Where("deleted_at IS NULL").
			Where("chain = ? AND temperature = 1 AND is_default = 1 AND id <> ?", chain, existing.ID).
			Count(&cnt).Error; err != nil {
			return err
		}
		if cnt == 0 {
			updates["is_default"] = int8(1)
		}
	}

	if len(updates) == 0 {
		return nil
	}
	return tx.WithContext(ctx).Model(&models.CompanyWallet{}).Where("id = ?", existing.ID).Updates(updates).Error
}

func misconfiguredFromExisting(existing *models.CompanyWallet, chain string, seedID string, expectedAddr string, expectedPath string) error {
	if existing == nil {
		return &companyWalletMisconfiguredError{
			Chain:           chain,
			ExpectedAddress: expectedAddr,
			ExpectedPath:    expectedPath,
			ExpectedSeedID:  seedID,
		}
	}
	return &companyWalletMisconfiguredError{
		Chain:           chain,
		ExpectedAddress: expectedAddr,
		ExpectedPath:    expectedPath,
		ExpectedSeedID:  seedID,
		ExistingID:      existing.ID,
		ExistingType:    existing.AddressType,
		ExistingIndex:   existing.AddressIndex,
		ExistingAddress: existing.Address,
		ExistingPath:    existing.DerivationPath,
		ExistingSeedID:  existing.SeedID,
		ExistingTemp:    existing.Temperature,
	}
}

func companyWalletAddressEqual(chain string, a string, b string) bool {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if a == "" || b == "" {
		return false
	}
	// EVM addresses are case-insensitive.
	if strings.HasPrefix(strings.ToLower(a), "0x") || strings.HasPrefix(strings.ToLower(b), "0x") {
		return strings.EqualFold(a, b)
	}
	// TRON base58 addresses are case-sensitive.
	return a == b
}

func isCompanyWalletMisconfigured(err error) bool {
	var e *companyWalletMisconfiguredError
	return errors.As(err, &e)
}
