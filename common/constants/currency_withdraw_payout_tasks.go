package constants

// Currency withdrawal payout task (currency_withdraw_payout_tasks).
const (
	WithdrawPayoutModeSystem = "system"
	WithdrawPayoutModeManual = "manual"
)

const (
	WithdrawPayoutStatePendingBroadcast     = "pending_broadcast"
	WithdrawPayoutStateAwaitingManualTxHash = "awaiting_manual_txhash"
	WithdrawPayoutStateConfirming           = "confirming"
	WithdrawPayoutStateSettlePending        = "settle_pending"
	WithdrawPayoutStateDone                 = "done"
	WithdrawPayoutStateFailed               = "failed"
)
