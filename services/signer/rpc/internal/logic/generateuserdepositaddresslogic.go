package logic

import (
	"context"
	"strings"
	"time"

	"internalwallet/common/errcode"
	"internalwallet/common/utils"
	"internalwallet/proto/pb"
	"internalwallet/services/signer/rpc/internal/models"
	"internalwallet/services/signer/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GenerateUserDepositAddressLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGenerateUserDepositAddressLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GenerateUserDepositAddressLogic {
	return &GenerateUserDepositAddressLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GenerateUserDepositAddress 为用户生成单个充值地址（支持 change_index，用于同链多地址）
func (l *GenerateUserDepositAddressLogic) GenerateUserDepositAddress(in *pb.GenerateUserDepositAddressRequest) (*pb.GenerateUserDepositAddressResponse, error) {
	if in == nil || strings.TrimSpace(in.SeedId) == "" || in.UserId <= 0 || strings.TrimSpace(in.Chain) == "" || in.ChangeIndex < 0 {
		return &pb.GenerateUserDepositAddressResponse{
			Code:    400,
			Message: "invalid params",
		}, nil
	}

	chain := strings.ToUpper(strings.TrimSpace(in.Chain))

	// 验证链是否支持
	if err := ValidateChain(chain); err != nil {
		return &pb.GenerateUserDepositAddressResponse{
			Code:    400,
			Message: "unsupported chain",
		}, nil
	}

	// 1) Load seed
	seedRecord, err := l.svcCtx.MasterSeedRepo.FindBySeedID(l.ctx, strings.TrimSpace(in.SeedId))
	if err != nil || seedRecord == nil {
		return &pb.GenerateUserDepositAddressResponse{
			Code:    404,
			Message: "seed not found",
		}, nil
	}
	if seedRecord.Status != 1 {
		return &pb.GenerateUserDepositAddressResponse{
			Code:    403,
			Message: "seed inactive",
		}, nil
	}

	// 2) Build derivation path (change=subIndex, address_index=user_id)
	derivationPath, err := BuildDerivationPathWithChange(chain, in.UserId, in.ChangeIndex)
	if err != nil {
		return &pb.GenerateUserDepositAddressResponse{
			Code:    400,
			Message: "invalid derivation path",
		}, nil
	}

	// 3) Idempotency: if already generated, return it.
	if existing, err := l.svcCtx.AddressGenerationLogRepo.FindByUserIDChainAndPath(l.ctx, in.UserId, chain, derivationPath); err == nil && existing != nil {
		return &pb.GenerateUserDepositAddressResponse{
			Code:      200,
			Message:   "success",
			UserId:    in.UserId,
			Chain:     existing.Chain,
			Address:   existing.Address,
			CreatedAt: existing.CreatedAt.Format("2006-01-02 15:04:05"),
		}, nil
	}

	// 4) signer 必须先解锁（seed 仅驻留内存）
	seedBytes, ok := l.svcCtx.WalletRuntime.GetSeedCopy(seedRecord.SeedID)
	if !ok {
		return &pb.GenerateUserDepositAddressResponse{
			Code:    int32(errcode.SignerWalletLocked),
			Message: "wallet is locked",
		}, nil
	}
	if !utils.VerifySeedHash(seedBytes, seedRecord.SeedHash) {
		return &pb.GenerateUserDepositAddressResponse{
			Code:    500,
			Message: "seed verify failed",
		}, nil
	}

	hdWallet, err := NewHDWalletFromSeed(seedBytes, chain)
	if err != nil {
		return &pb.GenerateUserDepositAddressResponse{
			Code:    500,
			Message: "hd wallet create failed",
		}, nil
	}

	uncompressed, compressed, err := hdWallet.GetPublicKey(derivationPath)
	if err != nil {
		return &pb.GenerateUserDepositAddressResponse{
			Code:    500,
			Message: "derive pubkey failed",
		}, nil
	}

	address, err := PublicKeyToAddress(chain, uncompressed)
	if err != nil {
		return &pb.GenerateUserDepositAddressResponse{
			Code:    500,
			Message: "address generation failed",
		}, nil
	}

	generatedBy := "system"
	if strings.TrimSpace(in.Requester) != "" {
		generatedBy = strings.TrimSpace(in.Requester)
	}

	addrLog := &models.AddressGenerationLog{
		UserID:           in.UserId,
		Chain:            chain,
		Address:          address,
		DerivationPath:   derivationPath,
		AddressIndex:     int(in.UserId),
		PublicKey:        uncompressed,
		CompressedPubkey: compressed,
		MasterSeedID:     &seedRecord.ID,
		GeneratedBy:      generatedBy,
	}

	if err := l.svcCtx.AddressGenerationLogRepo.Create(l.ctx, addrLog); err != nil {
		// Duplicate insert could happen in concurrent calls; try load and return.
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			if existing, err2 := l.svcCtx.AddressGenerationLogRepo.FindByUserIDChainAndPath(l.ctx, in.UserId, chain, derivationPath); err2 == nil && existing != nil {
				return &pb.GenerateUserDepositAddressResponse{
					Code:      200,
					Message:   "success",
					UserId:    in.UserId,
					Chain:     existing.Chain,
					Address:   existing.Address,
					CreatedAt: existing.CreatedAt.Format("2006-01-02 15:04:05"),
				}, nil
			}
		}

		l.Errorf("Create address generation log failed: %v", err)
		return &pb.GenerateUserDepositAddressResponse{
			Code:    500,
			Message: "save address log failed",
		}, nil
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	return &pb.GenerateUserDepositAddressResponse{
		Code:      200,
		Message:   "success",
		UserId:    in.UserId,
		Chain:     chain,
		Address:   address,
		CreatedAt: now,
	}, nil
}
