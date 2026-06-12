package logic

import (
	"context"
	"fmt"

	"internalwallet/services/signer/rpc/internal/repository"
	"internalwallet/services/signer/rpc/internal/svc"
)

type hotWalletStateSnapshot struct {
	SeedID            string
	State             string
	BackupConfirmed   bool
	UnlockedAtRFC3339 string
}

func getHotWalletState(ctx context.Context, svcCtx *svc.ServiceContext, seedID string) (*hotWalletStateSnapshot, error) {
	seedID = normalizeSeedID(seedID)

	if svcCtx == nil || svcCtx.WalletRuntime == nil {
		return nil, fmt.Errorf("signer runtime not ready")
	}
	if svcCtx.WalletMasterMnemonicRepo == nil {
		// DB not ready (or init failed). Caller should surface a meaningful error instead of panicking.
		return nil, fmt.Errorf("signer db not ready (wallet master mnemonic repo is nil)")
	}

	row, err := svcCtx.WalletMasterMnemonicRepo.FindBySeedID(ctx, seedID)
	if err != nil {
		if err == repository.ErrWalletMasterMnemonicNotFound {
			return &hotWalletStateSnapshot{
				SeedID: seedID,
				State:  walletStateUninitialized,
			}, nil
		}
		return nil, err
	}

	backupConfirmed := row.BackupConfirmedAt != nil
	if !backupConfirmed {
		return &hotWalletStateSnapshot{
			SeedID:          seedID,
			State:           walletStatePendingBackup,
			BackupConfirmed: false,
		}, nil
	}

	if svcCtx.WalletRuntime.IsSeedUnlocked(seedID) {
		unlockedAt := svcCtx.WalletRuntime.UnlockedAt()
		s := ""
		if unlockedAt != nil {
			s = unlockedAt.Format("2006-01-02T15:04:05Z07:00")
		}
		return &hotWalletStateSnapshot{
			SeedID:            seedID,
			State:             walletStateUnlocked,
			BackupConfirmed:   true,
			UnlockedAtRFC3339: s,
		}, nil
	}

	return &hotWalletStateSnapshot{
		SeedID:          seedID,
		State:           walletStateLocked,
		BackupConfirmed: true,
	}, nil
}
