package logic

import (
	"context"
	"errors"
	"fmt"

	"internalwallet/common/constants"
	"internalwallet/common/errcode"
	"internalwallet/common/utils"
	"internalwallet/proto/pb"
	"internalwallet/services/signer/rpc/internal/models"
	"internalwallet/services/signer/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type ExportPublicKeyBatchLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewExportPublicKeyBatchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ExportPublicKeyBatchLogic {
	return &ExportPublicKeyBatchLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ExportPublicKeyBatchLogic) ExportPublicKeyBatch(in *pb.ExportPublicKeyBatchRequest) (*pb.ExportPublicKeyBatchResponse, error) {
	// 1. 参数验证
	if err := l.validateParams(in); err != nil {
		return &pb.ExportPublicKeyBatchResponse{
			Code:    int32(errcode.SignerInvalidParams),
			Message: err.Error(),
		}, nil
	}

	// 2. 查询Seed
	var seed models.MasterSeed
	err := l.svcCtx.DB.Where("seed_id = ?", in.SeedId).First(&seed).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &pb.ExportPublicKeyBatchResponse{
				Code:    int32(errcode.SignerSeedNotFound),
				Message: "seed not found",
			}, nil
		}
		l.Logger.Errorf("Failed to query seed: %v", err)
		return &pb.ExportPublicKeyBatchResponse{
			Code:    int32(errcode.SignerDatabaseError),
			Message: "database error",
		}, nil
	}

	// 3. 检查Seed状态
	if seed.Status != constants.SeedStatusActive {
		return &pb.ExportPublicKeyBatchResponse{
			Code:    int32(errcode.SignerSeedInactive),
			Message: "seed is not active",
		}, nil
	}

	// 4. 检查链是否支持
	chainSupported := false
	for _, supportedChain := range seed.SupportedChains {
		if supportedChain == in.Chain {
			chainSupported = true
			break
		}
	}
	if !chainSupported {
		return &pb.ExportPublicKeyBatchResponse{
			Code:    int32(errcode.SignerChainNotSupport),
			Message: "chain not supported by this seed",
		}, nil
	}

	// 5. 解密Seed
	seedBytes, ok := l.svcCtx.WalletRuntime.GetSeedCopy(seed.SeedID)
	if !ok {
		return &pb.ExportPublicKeyBatchResponse{
			Code:    int32(errcode.SignerWalletLocked),
			Message: "wallet is locked",
		}, nil
	}

	// 6. 验证Seed哈希
	if !utils.VerifySeedHash(seedBytes, seed.SeedHash) {
		l.Logger.Errorf("Seed hash verification failed")
		return &pb.ExportPublicKeyBatchResponse{
			Code:    int32(errcode.SignerSeedHashMismatch),
			Message: "seed integrity check failed",
		}, nil
	}

	// 7. 创建HD钱包
	wallet, err := NewHDWalletFromSeed(seedBytes, in.Chain)
	if err != nil {
		l.Logger.Errorf("Failed to create HD wallet: %v", err)
		return &pb.ExportPublicKeyBatchResponse{
			Code:    int32(errcode.SignerInternalError),
			Message: "failed to create wallet",
		}, nil
	}

	// 8. 获取coin_type
	coinType, ok := constants.GetCoinType(in.Chain)
	if !ok {
		return &pb.ExportPublicKeyBatchResponse{
			Code:    int32(errcode.SignerChainNotSupport),
			Message: "chain not supported",
		}, nil
	}

	// 9. 批量派生公钥
	publicKeys := make([]*pb.PubkeyInfo, 0, in.Count)
	for i := int32(0); i < in.Count; i++ {
		addressIndex := in.StartIndex + i

		// 构建BIP44路径: m/44'/coin_type'/account'/change/address_index
		path := BuildBIP44Path(coinType, uint32(in.Account), uint32(in.Change), uint32(addressIndex))

		uncompressed, compressed, err := wallet.GetPublicKey(path)
		if err != nil {
			l.Logger.Errorf("Failed to derive public key at index %d: %v", addressIndex, err)
			continue
		}

		publicKeys = append(publicKeys, &pb.PubkeyInfo{
			Index:            addressIndex,
			DerivationPath:   path,
			PublicKey:        uncompressed,
			CompressedPubkey: compressed,
		})
	}

	// 10. 记录导出日志
	firstPath := ""
	if len(publicKeys) > 0 {
		firstPath = publicKeys[0].DerivationPath
	}
	go l.logExport(seed.ID, in.Chain, firstPath, in.Requester, true, int(in.Count))

	l.Logger.Infof("✓ Exported %d public keys for %s on %s", len(publicKeys), in.SeedId, in.Chain)

	return &pb.ExportPublicKeyBatchResponse{
		Code:       0,
		Message:    "success",
		SeedId:     in.SeedId,
		Chain:      in.Chain,
		StartIndex: in.StartIndex,
		Count:      int32(len(publicKeys)),
		PublicKeys: publicKeys,
	}, nil
}

func (l *ExportPublicKeyBatchLogic) validateParams(in *pb.ExportPublicKeyBatchRequest) error {
	if in.SeedId == "" {
		return errors.New("seed_id is required")
	}
	if in.Chain == "" {
		return errors.New("chain is required")
	}
	if !constants.IsChainSupported(in.Chain) {
		return errors.New("unsupported chain")
	}
	if in.StartIndex < 0 {
		return errors.New("start_index must be non-negative")
	}
	if in.Count <= 0 {
		return errors.New("count must be positive")
	}
	if in.Count > int32(constants.MaxBatchExportSize) {
		return fmt.Errorf("count exceeds maximum batch size: %d", constants.MaxBatchExportSize)
	}
	if in.Change != 0 && in.Change != 1 {
		return errors.New("change must be 0 or 1")
	}
	return nil
}

func (l *ExportPublicKeyBatchLogic) logExport(masterSeedID int64, chain, path, requester string, isBatch bool, count int) {
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
		l.Logger.Errorf("Failed to log batch export: %v", err)
	}
}
