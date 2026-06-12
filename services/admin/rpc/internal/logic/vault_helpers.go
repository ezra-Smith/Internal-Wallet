package logic

import (
	"encoding/json"
	"strings"

	"github.com/shopspring/decimal"
)

const (
	vaultAdjustmentTypeIncrease = "increase"
	vaultAdjustmentTypeDecrease = "decrease"

	vaultAdjustmentStatusPending    = "pending_approval"
	vaultAdjustmentStatusApproved   = "approved"
	vaultAdjustmentStatusRejected   = "rejected"
	vaultAdjustmentStatusProcessing = "processing"
	vaultAdjustmentStatusCompleted  = "completed"
	vaultAdjustmentStatusFailed     = "failed"

	vaultCurrencyStatusNormal   = "normal"
	vaultCurrencyStatusLow      = "low"
	vaultCurrencyStatusCritical = "critical"

	vaultNetworkStatusHealthy  = "healthy"
	vaultNetworkStatusWarning  = "warning"
	vaultNetworkStatusCritical = "critical"

	// Vault Address Types
	vaultAddressTypeActive  = "active"
	vaultAddressTypeHot     = "hot"
	vaultAddressTypeCold    = "cold"
	vaultAddressTypeDeposit = "deposit"

	// Vault Address Status
	vaultAddressStatusActive   = "active"
	vaultAddressStatusInactive = "inactive"
	vaultAddressStatusDisabled = "disabled"
)

type vaultThresholdNotifications struct {
	EmailEnabled    bool     `json:"email_enabled"`
	EmailRecipients []string `json:"email_recipients,omitempty"`
	WebhookEnabled  bool     `json:"webhook_enabled"`
	WebhookURL      string   `json:"webhook_url,omitempty"`
}

func validateVaultAdjustmentType(t string) bool {
	switch strings.TrimSpace(t) {
	case vaultAdjustmentTypeIncrease, vaultAdjustmentTypeDecrease:
		return true
	default:
		return false
	}
}

func validateVaultAdjustmentSource(s string) bool {
	switch strings.TrimSpace(s) {
	case "cold_wallet", "exchange", "other":
		return true
	default:
		return false
	}
}

func validateVaultAdjustmentStatus(s string) bool {
	switch strings.TrimSpace(s) {
	case vaultAdjustmentStatusPending,
		vaultAdjustmentStatusApproved,
		vaultAdjustmentStatusRejected,
		vaultAdjustmentStatusProcessing,
		vaultAdjustmentStatusCompleted,
		vaultAdjustmentStatusFailed:
		return true
	default:
		return false
	}
}

func vaultAmountStatus(balanceRaw int64, lowRaw int64, criticalRaw int64) string {
	// If thresholds are not configured (0), treat as normal.
	if lowRaw <= 0 && criticalRaw <= 0 {
		return vaultCurrencyStatusNormal
	}
	if criticalRaw > 0 && balanceRaw < criticalRaw {
		return vaultCurrencyStatusCritical
	}
	if lowRaw > 0 && balanceRaw < lowRaw {
		return vaultCurrencyStatusLow
	}
	return vaultCurrencyStatusNormal
}

func centsToUSDString(cents int64) string {
	if cents <= 0 {
		return "0.00"
	}
	d := decimal.NewFromInt(cents).Div(decimal.NewFromInt(100))
	return d.StringFixed(2)
}

func rawToFixedAmountString(raw int64, precision int32) string {
	if raw == 0 {
		return decimal.Zero.StringFixed(precision)
	}
	return decimal.NewFromInt(raw).Shift(-precision).StringFixed(precision)
}

func parseVaultNotificationsJSON(b []byte) vaultThresholdNotifications {
	var cfg vaultThresholdNotifications
	if len(b) == 0 {
		return cfg
	}
	_ = json.Unmarshal(b, &cfg)
	return cfg
}

// Vault Address validation functions
func validateVaultAddressType(t string) bool {
	switch strings.TrimSpace(t) {
	case vaultAddressTypeActive, vaultAddressTypeHot, vaultAddressTypeCold, vaultAddressTypeDeposit:
		return true
	default:
		return false
	}
}

func validateVaultAddressStatus(s string) bool {
	switch strings.TrimSpace(s) {
	case vaultAddressStatusActive, vaultAddressStatusInactive, vaultAddressStatusDisabled:
		return true
	default:
		return false
	}
}
