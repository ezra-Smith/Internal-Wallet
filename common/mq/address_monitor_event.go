package mq

import "time"

// AddressMonitorAction defines how consumers should update in-memory monitoring state.
type AddressMonitorAction string

const (
	// AddressMonitorActionUpsert indicates the address should be monitored (active).
	AddressMonitorActionUpsert AddressMonitorAction = "upsert"
	// AddressMonitorActionRemove indicates the address should be removed from monitoring (inactive/deleted).
	AddressMonitorActionRemove AddressMonitorAction = "remove"
)

// AddressMonitorSource indicates which upstream system owns the address.
// Keep values stable and ASCII-only (used as Kafka message keys).
type AddressMonitorSource string

const (
	AddressMonitorSourceDeposit AddressMonitorSource = "deposit"
	AddressMonitorSourceCompany AddressMonitorSource = "company"
	AddressMonitorSourceVault   AddressMonitorSource = "vault"
	AddressMonitorSourceWeb3    AddressMonitorSource = "web3"
	AddressMonitorSourceManual  AddressMonitorSource = "manual"
	AddressMonitorSourceUnknown AddressMonitorSource = "unknown"
)

// AddressMonitorEvent is published by Admin/Signer/Business when a monitorable address changes.
// Chainsync consumes it for near-real-time incremental updates, while periodic full refresh
// remains the source-of-truth reconciliation mechanism.
type AddressMonitorEvent struct {
	Version   int32                `json:"version"`
	EventID   string               `json:"event_id"`
	Action    AddressMonitorAction `json:"action"`
	Source    AddressMonitorSource `json:"source"`
	Chain     string               `json:"chain"`   // canonical: ETH/BSC/TRON (case-insensitive)
	Address   string               `json:"address"` // raw address string from upstream
	CreatedAt time.Time            `json:"created_at"`
	Reason    string               `json:"reason,omitempty"`
	Metadata  map[string]string    `json:"metadata,omitempty"`
}
