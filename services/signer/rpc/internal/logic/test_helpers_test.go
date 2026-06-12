package logic

import (
	"context"
	"crypto/rand"
	"testing"
	"time"

	"internalwallet/common/utils"
	"internalwallet/services/signer/rpc/internal/config"
	"internalwallet/services/signer/rpc/internal/models"
	"internalwallet/services/signer/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/conf"
)

// requireSignerSvcCtx loads signer.yaml and returns a ServiceContext with a working DB.
// If DB is not reachable in current test environment, it will Skip().
func requireSignerSvcCtx(t *testing.T) *svc.ServiceContext {
	t.Helper()

	var c config.Config
	conf.MustLoad("../../etc/signer.yaml", &c)
	ctx := svc.NewServiceContext(c)

	if ctx == nil || ctx.DB == nil || ctx.MasterSeedRepo == nil {
		t.Skip("signer test db not configured (ctx.DB is nil)")
		return nil
	}

	sqlDB, err := ctx.DB.DB()
	if err != nil {
		t.Skip("signer test db not available (gorm DB() failed)")
		return nil
	}
	sqlDB.SetConnMaxLifetime(30 * time.Second)
	if err := sqlDB.Ping(); err != nil {
		t.Skip("signer test db not reachable (ping failed)")
		return nil
	}

	return ctx
}

// createSeedForTest creates a test master seed record WITHOUT mnemonic generation.
// It uses the server-side EncryptionPassword for encryption, so unlockSeedForTest can decrypt it.
func createSeedForTest(t *testing.T, ctx *svc.ServiceContext, seedID, seedName string, supportedChains []string, temperature int32) {
	t.Helper()
	if ctx == nil || ctx.MasterSeedRepo == nil || ctx.DB == nil {
		t.Skip("signer ctx not ready for create seed")
		return
	}
	if seedID == "" || seedName == "" || len(supportedChains) == 0 {
		t.Fatalf("invalid createSeedForTest params")
		return
	}

	// Idempotency for tests: if exists, do nothing.
	if ok, _ := ctx.MasterSeedRepo.ExistsBySeedID(context.Background(), seedID); ok {
		return
	}

	pw := ctx.Config.Security.EncryptionPassword
	if pw == "" {
		t.Skip("signer encryption password not configured for tests")
		return
	}

	seedBytes := make([]byte, 64)
	_, _ = rand.Read(seedBytes)

	enc, err := utils.EncryptSeed(seedBytes, pw)
	if err != nil {
		t.Fatalf("encrypt seed failed: %v", err)
		return
	}

	ms := &models.MasterSeed{
		SeedID:          seedID,
		SeedName:        seedName,
		SeedEncrypted:   enc.Encrypted,
		SeedHash:        enc.Hash,
		EncryptionSalt:  enc.Salt,
		EncryptionIV:    enc.IV,
		SupportedChains: models.ChainList(supportedChains),
		Temperature:     int(temperature),
		Status:          1,
		TeeEnabled:      false,
		KeyVersion:      1,
	}
	if err := ctx.MasterSeedRepo.Create(context.Background(), ms); err != nil {
		t.Fatalf("create seed failed: %v", err)
		return
	}
}

func unlockSeedForTest(t *testing.T, ctx *svc.ServiceContext, seedID string) {
	t.Helper()
	if ctx == nil || ctx.MasterSeedRepo == nil || ctx.WalletRuntime == nil {
		t.Skip("signer ctx not ready for unlock")
		return
	}
	seed, err := ctx.MasterSeedRepo.FindBySeedID(context.Background(), seedID)
	if err != nil || seed == nil {
		t.Fatalf("seed not found for unlock: seed_id=%s err=%v", seedID, err)
		return
	}
	pw := ctx.Config.Security.EncryptionPassword
	seedBytes, err := utils.DecryptSeed(seed.SeedEncrypted, seed.EncryptionSalt, pw)
	if err != nil {
		t.Fatalf("decrypt seed failed for unlock: %v", err)
		return
	}
	if !utils.VerifySeedHash(seedBytes, seed.SeedHash) {
		t.Fatalf("seed hash mismatch for unlock: seed_id=%s", seedID)
		return
	}
	ctx.WalletRuntime.SetSeed(seedID, seedBytes, time.Now())
}
