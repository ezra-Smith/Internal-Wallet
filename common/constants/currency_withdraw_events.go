package constants

// Currency withdrawal order audit events (currency_withdraw_order_events.event_type).
const (
	WithdrawEventOrderCreated       = "order.created"
	WithdrawEventOrderUpdated       = "audit.order.updated"
	WithdrawEventFeeCalculated      = "fee.calculated"
	WithdrawEventAuditRuleMatched   = "audit.rule_matched"
	WithdrawEventAuditWhitelistHit  = "audit.whitelist_hit"
	WithdrawEventAuditWhitelistMiss = "audit.whitelist_miss"
	WithdrawEventAuditApproved      = "audit.approved"
	WithdrawEventAuditRejected      = "audit.rejected"
	WithdrawEventAssetFrozen        = "asset.frozen"
	WithdrawEventAssetUnfrozen      = "asset.unfrozen"
	WithdrawEventAssetDeducted      = "asset.deducted"
	WithdrawEventFeeCollected       = "fee.collected"
	WithdrawEventTransferInitiated  = "transfer.initiated"
	WithdrawEventTransferCompleted  = "transfer.completed"
	WithdrawEventTransferFailed     = "transfer.failed"
	WithdrawEventOrderCancelled     = "order.cancelled"
)

// Currency withdrawal order audit actors (currency_withdraw_order_events.actor_type).
const (
	WithdrawActorSystem = "system"
	WithdrawActorUser   = "user"
	WithdrawActorAdmin  = "admin"
)

// Fee rule sources (currency_withdraw_orders.fee_rule_source).
const (
	WithdrawFeeRuleSourceGlobal = "global"
	WithdrawFeeRuleSourceAsset  = "asset"
)
