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

type StartHotWalletInitLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewStartHotWalletInitLogic(ctx context.Context, svcCtx *svc.ServiceContext) *StartHotWalletInitLogic {
	return &StartHotWalletInitLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *StartHotWalletInitLogic) StartHotWalletInit(in *pb.StartHotWalletInitRequest) (*pb.StartHotWalletInitResponse, error) {
	if in == nil {
		return &pb.StartHotWalletInitResponse{Code: int32(errcode.SignerInvalidParams), Message: "invalid params"}, nil
	}
	seedID := normalizeSeedID(in.SeedId)
	seedName := strings.TrimSpace(in.SeedName)
	unlockPassword := in.UnlockPassword

	if seedID == "" || seedName == "" || len(in.SupportedChains) == 0 || strings.TrimSpace(unlockPassword) == "" {
		return &pb.StartHotWalletInitResponse{Code: int32(errcode.SignerInvalidParams), Message: "invalid params"}, nil
	}
	// 额外保护：解锁密码最小长度（可按需调整）
	// IMPORTANT: EncryptSeed enforces >= 16 chars; keep consistent to avoid surfacing as "internal error".
	if len(unlockPassword) < 16 {
		return &pb.StartHotWalletInitResponse{Code: int32(errcode.SignerInvalidParams), Message: "invalid unlock_password: password too weak, minimum 16 characters"}, nil
	}

	// 防并发初始化（分布式锁）
	unlockFn, lockErr := l.svcCtx.SignerCache.LockWithRetry(l.ctx, "hot_wallet_init:"+seedID, 20*time.Second, 2, 200*time.Millisecond)
	if lockErr != nil {
		return &pb.StartHotWalletInitResponse{Code: int32(errcode.SignerServiceUnavail), Message: "init busy, try again"}, nil
	}
	defer func() { _ = unlockFn() }()

	// 已存在：允许“继续初始化”（pending_backup）或拒绝（已确认）
	existingMnemonic, err := l.svcCtx.WalletMasterMnemonicRepo.FindBySeedID(l.ctx, seedID)
	if err == nil && existingMnemonic != nil {
		if existingMnemonic.BackupConfirmedAt != nil {
			return &pb.StartHotWalletInitResponse{
				Code:    int32(errcode.SignerSeedAlreadyExist),
				Message: "wallet already initialized",
				SeedId:  seedID,
			}, nil
		}
		// pending_backup：校验密码一致后允许继续（生成新的 init_session）
		if bcrypt.CompareHashAndPassword([]byte(existingMnemonic.UnlockPasswordBcrypt), []byte(unlockPassword)) != nil {
			return &pb.StartHotWalletInitResponse{
				Code:    int32(errcode.SignerSeedInvalidPassword),
				Message: "invalid unlock_password",
				SeedId:  seedID,
			}, nil
		}
		return l.newInitSessionResponse(seedID, unlockPassword)
	}
	if err != nil && !errors.Is(err, repository.ErrWalletMasterMnemonicNotFound) {
		l.Logger.Errorf("StartHotWalletInit: query existing failed: %v", err)
		return &pb.StartHotWalletInitResponse{Code: int32(errcode.SignerDatabaseError), Message: "database error", SeedId: seedID}, nil
	}

	// 生成 24 词助记词
	entropy, gErr := utils.GenerateRandomSeed(32) // 256-bit entropy
	if gErr != nil {
		l.Logger.Errorf("StartHotWalletInit: entropy gen failed: %v", gErr)
		return &pb.StartHotWalletInitResponse{Code: int32(errcode.SignerInternalError), Message: "internal error", SeedId: seedID}, nil
	}
	mnemonic, mErr := bip39.NewMnemonic(entropy)
	if mErr != nil {
		l.Logger.Errorf("StartHotWalletInit: mnemonic gen failed: %v", mErr)
		return &pb.StartHotWalletInitResponse{Code: int32(errcode.SignerInternalError), Message: "internal error", SeedId: seedID}, nil
	}

	// 用 unlock_password 加密助记词（落库）
	encMnemonic, eErr := utils.EncryptSeed([]byte(mnemonic), unlockPassword)
	if eErr != nil {
		// Map password weakness to a user-actionable param error instead of generic internal error.
		if strings.Contains(strings.ToLower(eErr.Error()), "password too weak") {
			return &pb.StartHotWalletInitResponse{
				Code:    int32(errcode.SignerInvalidParams),
				Message: "invalid unlock_password: " + eErr.Error(),
				SeedId:  seedID,
			}, nil
		}
		l.Logger.Errorf("StartHotWalletInit: encrypt mnemonic failed: %v", eErr)
		return &pb.StartHotWalletInitResponse{Code: int32(errcode.SignerInternalError), Message: "internal error", SeedId: seedID}, nil
	}

	// bcrypt 保存解锁密码校验值
	pwHash, bErr := bcrypt.GenerateFromPassword([]byte(unlockPassword), bcrypt.DefaultCost)
	if bErr != nil {
		l.Logger.Errorf("StartHotWalletInit: bcrypt failed: %v", bErr)
		return &pb.StartHotWalletInitResponse{Code: int32(errcode.SignerInternalError), Message: "internal error", SeedId: seedID}, nil
	}

	// 以 unlock_password 作为 BIP39 passphrase（Extension Word）派生 seed，并写入 master_seeds（用于完整性/元数据）
	seedBytes := bip39.NewSeed(mnemonic, unlockPassword)
	seedHash := utils.HashSeed(seedBytes)
	encSeed, esErr := utils.EncryptSeed(seedBytes, unlockPassword)
	if esErr != nil {
		l.Logger.Errorf("StartHotWalletInit: encrypt seed failed: %v", esErr)
		return &pb.StartHotWalletInitResponse{Code: int32(errcode.SignerInternalError), Message: "internal error", SeedId: seedID}, nil
	}

	txErr := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		// 1) wallet_master_mnemonics
		wm := &models.WalletMasterMnemonic{
			SeedID:               seedID,
			MnemonicEncrypted:    encMnemonic.Encrypted,
			MnemonicSalt:         encMnemonic.Salt,
			MnemonicIV:           encMnemonic.IV,
			UnlockPasswordBcrypt: string(pwHash),
			BackupConfirmedAt:    nil,
			CreatedByAdminID:     in.AdminId,
			WordCount:            hotWalletWordCount,
			Version:              1,
		}
		if err := tx.Create(wm).Error; err != nil {
			return err
		}

		// 2) master_seeds：若存在且 hash 不一致则拒绝（避免覆盖旧钱包）
		var existing models.MasterSeed
		err := tx.Where("deleted_at IS NULL").Where("seed_id = ?", seedID).Limit(1).Find(&existing).Error
		if err != nil {
			return err
		}
		if existing.ID != 0 {
			if strings.TrimSpace(existing.SeedHash) != "" && existing.SeedHash != seedHash {
				return errors.New("existing master_seeds conflicts with new seed hash")
			}
			updates := map[string]interface{}{
				"seed_name":        seedName,
				"seed_encrypted":   encSeed.Encrypted,
				"seed_hash":        seedHash,
				"encryption_salt":  encSeed.Salt,
				"encryption_iv":    encSeed.IV,
				"supported_chains": models.ChainList(in.SupportedChains),
				"temperature":      constants.TemperatureHot,
				"status":           constants.SeedStatusActive,
				"key_version":      1,
			}
			return tx.Model(&models.MasterSeed{}).
				Where("id = ?", existing.ID).
				Updates(updates).Error
		}

		ms := &models.MasterSeed{
			SeedID:          seedID,
			SeedName:        seedName,
			SeedEncrypted:   encSeed.Encrypted,
			SeedHash:        seedHash,
			EncryptionSalt:  encSeed.Salt,
			EncryptionIV:    encSeed.IV,
			SupportedChains: models.ChainList(in.SupportedChains),
			Temperature:     constants.TemperatureHot,
			Status:          constants.SeedStatusActive,
			TeeEnabled:      false,
			KeyVersion:      1,
		}
		return tx.Create(ms).Error
	})
	if txErr != nil {
		// 冲突：master_seeds 已存在且与新 seed 不一致
		if strings.Contains(strings.ToLower(txErr.Error()), "conflicts") {
			return &pb.StartHotWalletInitResponse{
				Code:    int32(errcode.SignerSeedAlreadyExist),
				Message: "wallet already initialized (legacy)",
				SeedId:  seedID,
			}, nil
		}
		l.Logger.Errorf("StartHotWalletInit: transaction failed: %v", txErr)
		return &pb.StartHotWalletInitResponse{Code: int32(errcode.SignerDatabaseError), Message: "database error", SeedId: seedID}, nil
	}

	return l.newInitSessionResponse(seedID, unlockPassword)
}

func (l *StartHotWalletInitLogic) newInitSessionResponse(seedID string, unlockPassword string) (*pb.StartHotWalletInitResponse, error) {
	now := time.Now()
	expiresAt := now.Add(hotWalletInitSessionTTL)
	sess := hotWalletInitSession{
		InitSessionID:  "init_" + randHex(18),
		SeedID:         seedID,
		UnlockPassword: unlockPassword,
		CreatedAt:      now,
		ExpiresAt:      expiresAt,
	}
	if err := l.svcCtx.SignerCache.SetJSON(l.ctx, hotWalletInitSessionKeyPrefix+sess.InitSessionID, &sess, hotWalletInitSessionTTL); err != nil {
		l.Logger.Errorf("StartHotWalletInit: save session failed: %v", err)
		return &pb.StartHotWalletInitResponse{Code: int32(errcode.SignerInternalError), Message: "cache error", SeedId: seedID}, nil
	}

	totalPages := hotWalletWordCount / hotWalletPageSize
	return &pb.StartHotWalletInitResponse{
		Code:          0,
		Message:       "success",
		SeedId:        seedID,
		InitSessionId: sess.InitSessionID,
		WordCount:     hotWalletWordCount,
		PageSize:      hotWalletPageSize,
		TotalPages:    int32(totalPages),
		ExpiresAt:     expiresAt.Format(time.RFC3339),
	}, nil
}
