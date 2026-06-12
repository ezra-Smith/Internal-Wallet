package logic

import (
	"context"
	"internalwallet/common/utils"

	"internalwallet/common/errcode"
	"internalwallet/proto/pb"
	"internalwallet/services/signer/rpc/internal/models"
	"internalwallet/services/signer/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetAddressFromPathLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetAddressFromPathLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetAddressFromPathLogic {
	return &GetAddressFromPathLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetAddressFromPath 根据派生路径获取地址
func (l *GetAddressFromPathLogic) GetAddressFromPath(in *pb.GetAddressFromPathRequest) (*pb.GetAddressFromPathResponse, error) {
	// 1. 参数验证
	if in.SeedId == "" {
		return &pb.GetAddressFromPathResponse{
			Code:    400,
			Message: "seed_id不能为空",
		}, nil
	}

	if in.Chain == "" {
		return &pb.GetAddressFromPathResponse{
			Code:    400,
			Message: "chain不能为空",
		}, nil
	}

	if in.DerivationPath == "" {
		return &pb.GetAddressFromPathResponse{
			Code:    400,
			Message: "derivation_path不能为空",
		}, nil
	}

	// 验证派生路径格式
	if err := ValidatePath(in.DerivationPath); err != nil {
		return &pb.GetAddressFromPathResponse{
			Code:    400,
			Message: "派生路径格式错误: " + err.Error(),
		}, nil
	}

	// 验证链是否支持
	if err := ValidateChain(in.Chain); err != nil {
		return &pb.GetAddressFromPathResponse{
			Code:    400,
			Message: err.Error(),
		}, nil
	}

	// 2. 从数据库加载Seed
	var seedRecord models.MasterSeed
	err := l.svcCtx.DB.WithContext(l.ctx).
		Where("seed_id = ?", in.SeedId).
		First(&seedRecord).Error

	if err != nil {
		l.Errorf("查询Seed失败: %v", err)
		return &pb.GetAddressFromPathResponse{
			Code:    404,
			Message: "Seed不存在",
		}, nil
	}

	if seedRecord.Status != 1 {
		return &pb.GetAddressFromPathResponse{
			Code:    403,
			Message: "Seed未激活",
		}, nil
	}

	// 3. signer 必须先解锁（seed 仅驻留内存）
	seedBytes, ok := l.svcCtx.WalletRuntime.GetSeedCopy(in.SeedId)
	if !ok {
		return &pb.GetAddressFromPathResponse{
			Code:    int32(errcode.SignerWalletLocked),
			Message: "wallet is locked",
		}, nil
	}

	// 验证Seed哈希（可选）
	if !utils.VerifySeedHash(seedBytes, seedRecord.SeedHash) {
		l.Errorf("Seed哈希验证失败，可能数据已损坏")
		return &pb.GetAddressFromPathResponse{
			Code:    500,
			Message: "Seed验证失败",
		}, nil
	}

	// 4. 创建HD钱包
	hdWallet, err := NewHDWalletFromSeed(seedBytes, in.Chain)
	if err != nil {
		l.Errorf("创建HD钱包失败: %v", err)
		return &pb.GetAddressFromPathResponse{
			Code:    500,
			Message: "创建HD钱包失败",
		}, nil
	}

	// 5. 获取公钥
	uncompressed, compressed, err := hdWallet.GetPublicKey(in.DerivationPath)
	if err != nil {
		l.Errorf("获取公钥失败: %v", err)
		return &pb.GetAddressFromPathResponse{
			Code:    500,
			Message: "获取公钥失败",
		}, nil
	}

	// 6. 生成地址
	address, err := PublicKeyToAddress(in.Chain, uncompressed)
	if err != nil {
		l.Errorf("生成地址失败: %v", err)
		return &pb.GetAddressFromPathResponse{
			Code:    500,
			Message: "生成地址失败: " + err.Error(),
		}, nil
	}

	return &pb.GetAddressFromPathResponse{
		Code:             200,
		Message:          "成功",
		Chain:            in.Chain,
		Address:          address,
		DerivationPath:   in.DerivationPath,
		PublicKey:        uncompressed,
		CompressedPubkey: compressed,
	}, nil
}
