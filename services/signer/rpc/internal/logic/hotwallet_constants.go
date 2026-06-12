package logic

import "time"

const (
	defaultHotWalletSeedID = "hot_wallet_main"

	hotWalletWordCount = 24
	hotWalletPageSize  = 4
)

const (
	walletStateUninitialized = "uninitialized"
	walletStatePendingBackup = "pending_backup"
	walletStateLocked        = "locked"
	walletStateUnlocked      = "unlocked"
	walletStateUnknown       = "unknown"
)

const (
	hotWalletInitSessionTTL      = 15 * time.Minute
	hotWalletChallengeSessionTTL = 5 * time.Minute
	hotWalletChallengeMaxTry     = 3
)
