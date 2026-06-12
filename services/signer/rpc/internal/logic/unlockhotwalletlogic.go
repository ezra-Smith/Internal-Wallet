package logic

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/tyler-smith/go-bip39"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"internalwallet/common/constants"
	"internalwallet/common/errcode"
	"internalwallet/common/utils"
	"internalwallet/proto/pb"
	"internalwallet/services/signer/rpc/internal/models"
	"internalwallet/services/signer/rpc/internal/repository"
	"internalwallet/services/signer/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type UnlockHotWalletLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUnlockHotWalletLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UnlockHotWalletLogic {
	return &UnlockHotWalletLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UnlockHotWalletLogic) UnlockHotWallet(in *pb.UnlockHotWalletRequest) (*pb.UnlockHotWalletResponse, error) {
	if in == nil {
		return &pb.UnlockHotWalletResponse{Code: int32(errcode.SignerInvalidParams), Message: "invalid params"}, nil
	}
	seedID := normalizeSeedID(in.SeedId)
	unlockPassword := in.UnlockPassword
	if strings.TrimSpace(unlockPassword) == "" {
		return &pb.UnlockHotWalletResponse{Code: int32(errcode.SignerInvalidParams), Message: "invalid params", SeedId: seedID}, nil
	}

	// 防并发解锁（分布式锁）
	unlockFn, lockErr := l.svcCtx.SignerCache.LockWithRetry(l.ctx, "hot_wallet_unlock:"+seedID, 20*time.Second, 2, 200*time.Millisecond)
	if lockErr != nil {
		return &pb.UnlockHotWalletResponse{Code: int32(errcode.SignerServiceUnavail), Message: "unlock busy, try again", SeedId: seedID}, nil
	}
	defer func() { _ = unlockFn() }()

	row, err := l.svcCtx.WalletMasterMnemonicRepo.FindBySeedID(l.ctx, seedID)
	if err != nil {
		if err == repository.ErrWalletMasterMnemonicNotFound {
			return &pb.UnlockHotWalletResponse{Code: int32(errcode.SignerWalletUninitialized), Message: "wallet not initialized", SeedId: seedID}, nil
		}
		l.Logger.Errorf("UnlockHotWallet: query mnemonic failed: %v", err)
		return &pb.UnlockHotWalletResponse{Code: int32(errcode.SignerDatabaseError), Message: "database error", SeedId: seedID}, nil
	}
	if row.BackupConfirmedAt == nil {
		return &pb.UnlockHotWalletResponse{Code: int32(errcode.SignerWalletPendingBackup), Message: "backup not confirmed", SeedId: seedID}, nil
	}

	if bcrypt.CompareHashAndPassword([]byte(row.UnlockPasswordBcrypt), []byte(unlockPassword)) != nil {
		return &pb.UnlockHotWalletResponse{Code: int32(errcode.SignerSeedInvalidPassword), Message: "invalid unlock_password", SeedId: seedID}, nil
	}

	plain, dErr := utils.DecryptSeed(row.MnemonicEncrypted, row.MnemonicSalt, unlockPassword)
	if dErr != nil {
		return &pb.UnlockHotWalletResponse{Code: int32(errcode.SignerWalletUnlockFailed), Message: "decrypt mnemonic failed", SeedId: seedID}, nil
	}
	mnemonic := strings.TrimSpace(string(plain))
	if len(splitMnemonicWords(mnemonic)) != hotWalletWordCount {
		return &pb.UnlockHotWalletResponse{Code: int32(errcode.SignerInternalError), Message: "invalid mnemonic", SeedId: seedID}, nil
	}

	seedBytes := bip39.NewSeed(mnemonic, unlockPassword)
	seedHash := utils.HashSeed(seedBytes)

	now := time.Now().Local()

	// 确保 master_seeds 中的 hash 一致（用于完整性校验）
	txErr := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		var ms models.MasterSeed
		err := tx.Where("deleted_at IS NULL").Where("seed_id = ?", seedID).Limit(1).Find(&ms).Error
		if err != nil {
			return err
		}
		if ms.ID != 0 {
			if strings.TrimSpace(ms.SeedHash) != "" && ms.SeedHash != seedHash {
				return errors.New("seed hash mismatch with master_seeds")
			}
			// 确保 status 激活
			if ms.Status != constants.SeedStatusActive {
				return tx.Model(&models.MasterSeed{}).
					Where("id = ?", ms.ID).
					Update("status", constants.SeedStatusActive).Error
			}
			return nil
		}

		encSeed, encErr := utils.EncryptSeed(seedBytes, unlockPassword)
		if encErr != nil {
			return encErr
		}
		newSeed := &models.MasterSeed{
			SeedID:          seedID,
			SeedName:        seedID,
			SeedEncrypted:   encSeed.Encrypted,
			SeedHash:        seedHash,
			EncryptionSalt:  encSeed.Salt,
			EncryptionIV:    encSeed.IV,
			SupportedChains: models.ChainList([]string{"ETH", "BSC", "TRON"}),
			Temperature:     constants.TemperatureHot,
			Status:          constants.SeedStatusActive,
			TeeEnabled:      false,
			KeyVersion:      1,
		}
		return tx.Create(newSeed).Error
	})
	if txErr != nil {
		if strings.Contains(strings.ToLower(txErr.Error()), "mismatch") {
			return &pb.UnlockHotWalletResponse{Code: int32(errcode.SignerSeedHashMismatch), Message: "seed mismatch", SeedId: seedID}, nil
		}
		l.Logger.Errorf("UnlockHotWallet: transaction failed: %v", txErr)
		return &pb.UnlockHotWalletResponse{Code: int32(errcode.SignerDatabaseError), Message: "database error", SeedId: seedID}, nil
	}

	// 写入运行态内存（解锁）
	// Ensure company hot wallet payout addresses exist for all supported chains.
	// This is required for auto-withdrawal to resolve from_address via company_wallets.
	if err := ensureCompanyHotPrimaryWallets(l.ctx, l.svcCtx, seedID, seedBytes); err != nil {
		if isCompanyWalletMisconfigured(err) {
			return &pb.UnlockHotWalletResponse{
				Code:     int32(errcode.SignerCompanyWalletMisconfigured),
				Message:  err.Error(),
				SeedId:   seedID,
				Unlocked: false,
			}, nil
		}
		l.Logger.Errorf("UnlockHotWallet: ensure company hot wallet failed: %v", err)
		return &pb.UnlockHotWalletResponse{
			Code:    int32(errcode.SignerDatabaseError),
			Message: "failed to ensure company hot wallet",
			SeedId:  seedID,
		}, nil
	}

	l.svcCtx.WalletRuntime.SetSeed(seedID, seedBytes, now)

	return &pb.UnlockHotWalletResponse{
		Code:       0,
		Message:    "success",
		SeedId:     seedID,
		Unlocked:   true,
		UnlockedAt: now.Format(time.RFC3339),
	}, nil
}
