package logic

import (
	"context"
	"errors"
	"time"

	"internalwallet/common/constants"
	"internalwallet/common/errcode"
	"internalwallet/common/utils"
	"internalwallet/proto/pb"
	"internalwallet/services/signer/rpc/internal/models"
	"internalwallet/services/signer/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type SignMessageLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSignMessageLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SignMessageLogic {
	return &SignMessageLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SignMessageLogic) SignMessage(in *pb.SignMessageRequest) (*pb.SignMessageResponse, error) {
	// 1. 参数验证
	if err := l.validateParams(in); err != nil {
		return &pb.SignMessageResponse{
			Code:    int32(errcode.SignerInvalidParams),
			Message: err.Error(),
		}, nil
	}

	// 2. 查询Seed
	var seed models.MasterSeed
	err := l.svcCtx.DB.Where("seed_id = ?", in.SeedId).First(&seed).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &pb.SignMessageResponse{
				Code:    int32(errcode.SignerSeedNotFound),
				Message: "seed not found",
			}, nil
		}
		l.Logger.Errorf("Failed to query seed: %v", err)
		return &pb.SignMessageResponse{
			Code:    int32(errcode.SignerDatabaseError),
			Message: "database error",
		}, nil
	}

	// 3. 检查Seed状态
	if seed.Status != constants.SeedStatusActive {
		return &pb.SignMessageResponse{
			Code:    int32(errcode.SignerSeedInactive),
			Message: "seed is not active",
		}, nil
	}

	// 4. 解密Seed
	seedBytes, ok := l.svcCtx.WalletRuntime.GetSeedCopy(seed.SeedID)
	if !ok {
		return &pb.SignMessageResponse{
			Code:    int32(errcode.SignerWalletLocked),
			Message: "wallet is locked",
		}, nil
	}

	// 5. 验证Seed哈希
	if !utils.VerifySeedHash(seedBytes, seed.SeedHash) {
		l.Logger.Errorf("Seed hash verification failed")
		return &pb.SignMessageResponse{
			Code:    int32(errcode.SignerSeedHashMismatch),
			Message: "seed integrity check failed",
		}, nil
	}

	// 6. 创建HD钱包并签名消息
	wallet, err := NewHDWalletFromSeed(seedBytes, in.Chain)
	if err != nil {
		l.Logger.Errorf("Failed to create HD wallet: %v", err)
		return &pb.SignMessageResponse{
			Code:    int32(errcode.SignerInternalError),
			Message: "failed to create wallet",
		}, nil
	}

	signature, err := wallet.SignMessage(in.DerivationPath, in.Message)
	if err != nil {
		l.Logger.Errorf("Failed to sign message: %v", err)
		return &pb.SignMessageResponse{
			Code:    int32(errcode.SignerSignatureFailed),
			Message: "failed to sign message",
		}, nil
	}

	l.Logger.Infof("✓ Message signed successfully: request_id=%s", in.RequestId)

	return &pb.SignMessageResponse{
		Code:      0,
		Message:   "success",
		RequestId: in.RequestId,
		Signature: signature,
		SignedAt:  time.Now().Format(time.RFC3339),
	}, nil
}

func (l *SignMessageLogic) validateParams(in *pb.SignMessageRequest) error {
	if in.RequestId == "" {
		return errors.New("request_id is required")
	}
	if in.SeedId == "" {
		return errors.New("seed_id is required")
	}
	if in.Chain == "" {
		return errors.New("chain is required")
	}
	if in.DerivationPath == "" {
		return errors.New("derivation_path is required")
	}
	if in.Message == "" {
		return errors.New("message is required")
	}
	if !constants.IsChainSupported(in.Chain) {
		return errors.New("unsupported chain")
	}
	if err := ValidatePath(in.DerivationPath); err != nil {
		return errors.New("invalid derivation path: " + err.Error())
	}
	return nil
}
