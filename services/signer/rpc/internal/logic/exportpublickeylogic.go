package logic

import (
	"context"
	"errors"

	"internalwallet/common/constants"
	"internalwallet/common/errcode"
	"internalwallet/common/utils"
	"internalwallet/proto/pb"
	"internalwallet/services/signer/rpc/internal/models"
	"internalwallet/services/signer/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type ExportPublicKeyLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewExportPublicKeyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ExportPublicKeyLogic {
	return &ExportPublicKeyLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ExportPublicKeyLogic) ExportPublicKey(in *pb.ExportPublicKeyRequest) (*pb.ExportPublicKeyResponse, error) {
	// 1. 参数验证
	if err := l.validateParams(in); err != nil {
		return &pb.ExportPublicKeyResponse{
			Code:    int32(errcode.SignerInvalidParams),
			Message: err.Error(),
		}, nil
	}

	// 2. 验证派生路径格式
	if err := ValidatePath(in.DerivationPath); err != nil {
		return &pb.ExportPublicKeyResponse{
			Code:    int32(errcode.SignerInvalidPath),
			Message: "invalid derivation path: " + err.Error(),
		}, nil
	}

	// 3. 查询Seed
	var seed models.MasterSeed
	err := l.svcCtx.DB.Where("seed_id = ?", in.SeedId).First(&seed).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &pb.ExportPublicKeyResponse{
				Code:    int32(errcode.SignerSeedNotFound),
				Message: "seed not found",
			}, nil
		}
		l.Logger.Errorf("Failed to query seed: %v", err)
		return &pb.ExportPublicKeyResponse{
			Code:    int32(errcode.SignerDatabaseError),
			Message: "database error",
		}, nil
	}

	// 4. 检查Seed状态
	if seed.Status != constants.SeedStatusActive {
		return &pb.ExportPublicKeyResponse{
			Code:    int32(errcode.SignerSeedInactive),
			Message: "seed is not active",
		}, nil
	}

	// 5. 检查链是否支持
	chainSupported := false
	for _, supportedChain := range seed.SupportedChains {
		if supportedChain == in.Chain {
			chainSupported = true
			break
		}
	}
	if !chainSupported {
		return &pb.ExportPublicKeyResponse{
			Code:    int32(errcode.SignerChainNotSupport),
			Message: "chain not supported by this seed",
		}, nil
	}

	// 6. 解密Seed
	seedBytes, ok := l.svcCtx.WalletRuntime.GetSeedCopy(seed.SeedID)
	if !ok {
		return &pb.ExportPublicKeyResponse{
			Code:    int32(errcode.SignerWalletLocked),
			Message: "wallet is locked",
		}, nil
	}

	// 7. 验证Seed哈希
	if !utils.VerifySeedHash(seedBytes, seed.SeedHash) {
		l.Logger.Errorf("Seed hash verification failed")
		return &pb.ExportPublicKeyResponse{
			Code:    int32(errcode.SignerSeedHashMismatch),
			Message: "seed integrity check failed",
		}, nil
	}

	// 8. 创建HD钱包并派生公钥
	wallet, err := NewHDWalletFromSeed(seedBytes, in.Chain)
	if err != nil {
		l.Logger.Errorf("Failed to create HD wallet: %v", err)
		return &pb.ExportPublicKeyResponse{
			Code:    int32(errcode.SignerInternalError),
			Message: "failed to create wallet",
		}, nil
	}

	uncompressed, compressed, err := wallet.GetPublicKey(in.DerivationPath)
	if err != nil {
		l.Logger.Errorf("Failed to derive public key: %v", err)
		return &pb.ExportPublicKeyResponse{
			Code:    int32(errcode.SignerDerivationFail),
			Message: "failed to derive public key",
		}, nil
	}

	// 9. 记录导出日志
	go l.logExport(seed.ID, in.Chain, in.DerivationPath, in.Requester, false, 1)

	return &pb.ExportPublicKeyResponse{
		Code:             0,
		Message:          "success",
		SeedId:           in.SeedId,
		Chain:            in.Chain,
		DerivationPath:   in.DerivationPath,
		PublicKey:        uncompressed,
		CompressedPubkey: compressed,
	}, nil
}

func (l *ExportPublicKeyLogic) validateParams(in *pb.ExportPublicKeyRequest) error {
	if in.SeedId == "" {
		return errors.New("seed_id is required")
	}
	if in.Chain == "" {
		return errors.New("chain is required")
	}
	if in.DerivationPath == "" {
		return errors.New("derivation_path is required")
	}
	if !constants.IsChainSupported(in.Chain) {
		return errors.New("unsupported chain")
	}
	return nil
}

func (l *ExportPublicKeyLogic) logExport(masterSeedID int64, chain, path, requester string, isBatch bool, count int) {
	exportLog := &models.PubkeyExportLog{
		MasterSeedID:   masterSeedID,
		Chain:          chain,
		DerivationPath: path,
		IsBatch:        isBatch,
		BatchCount:     count,
		// CreatedAt 和 UpdatedAt 由 GORM autoCreateTime/autoUpdateTime 自动管理
	}
	if requester != "" {
		exportLog.Requester = &requester
	}

	err := l.svcCtx.DB.Create(exportLog).Error
	if err != nil {
		l.Logger.Errorf("Failed to log export: %v", err)
	}
}
