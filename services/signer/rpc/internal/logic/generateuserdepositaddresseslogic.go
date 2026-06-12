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

type GenerateUserDepositAddressesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGenerateUserDepositAddressesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GenerateUserDepositAddressesLogic {
	return &GenerateUserDepositAddressesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ========== HD钱包地址管理 ==========
// GenerateUserDepositAddresses 为用户生成充值地址（三条链）
func (l *GenerateUserDepositAddressesLogic) GenerateUserDepositAddresses(in *pb.GenerateUserDepositAddressesRequest) (*pb.GenerateUserDepositAddressesResponse, error) {
	// 1. 参数验证
	if in.SeedId == "" {
		return &pb.GenerateUserDepositAddressesResponse{
			Code:    400,
			Message: "seed_id不能为空",
		}, nil
	}

	if in.UserId <= 0 {
		return &pb.GenerateUserDepositAddressesResponse{
			Code:    400,
			Message: "user_id必须大于0",
		}, nil
	}

	if len(in.Chains) == 0 {
		return &pb.GenerateUserDepositAddressesResponse{
			Code:    400,
			Message: "chains不能为空",
		}, nil
	}

	// 2. 从数据库加载Seed
	seedRecord, err := l.svcCtx.MasterSeedRepo.FindBySeedID(l.ctx, in.SeedId)
	if err != nil {
		l.Errorf("查询Seed失败: %v", err)
		return &pb.GenerateUserDepositAddressesResponse{
			Code:    404,
			Message: "Seed不存在",
		}, nil
	}

	if seedRecord.Status != 1 {
		return &pb.GenerateUserDepositAddressesResponse{
			Code:    403,
			Message: "Seed未激活",
		}, nil
	}

	// 3. signer 必须先解锁（seed 仅驻留内存）
	seedBytes, ok := l.svcCtx.WalletRuntime.GetSeedCopy(seedRecord.SeedID)
	if !ok {
		return &pb.GenerateUserDepositAddressesResponse{
			Code:    int32(errcode.SignerWalletLocked),
			Message: "wallet is locked",
		}, nil
	}

	// 验证Seed哈希（可选）
	if !utils.VerifySeedHash(seedBytes, seedRecord.SeedHash) {
		l.Errorf("Seed哈希验证失败，可能数据已损坏")
		return &pb.GenerateUserDepositAddressesResponse{
			Code:    500,
			Message: "Seed验证失败",
		}, nil
	}

	// 4. 创建HD钱包
	hdWallet, err := NewHDWalletFromSeed(seedBytes, "")
	if err != nil {
		l.Errorf("创建HD钱包失败: %v", err)
		return &pb.GenerateUserDepositAddressesResponse{
			Code:    500,
			Message: "创建HD钱包失败",
		}, nil
	}

	// 5. 为每条链生成地址
	addresses := make([]*pb.UserDepositAddress, 0, len(in.Chains))

	for _, chain := range in.Chains {
		chain = strings.ToUpper(strings.TrimSpace(chain))
		if chain == "" {
			continue
		}

		// 验证链是否支持
		if err := ValidateChain(chain); err != nil {
			l.Errorf("不支持的链: %s", chain)
			continue
		}

		// 固定生成 change=0（默认地址）
		derivationPath, err := BuildDerivationPathWithChange(chain, in.UserId, 0)
		if err != nil {
			l.Errorf("构建派生路径失败: %v", err)
			continue
		}

		// 如果该派生路径已生成过，直接复用（保证默认地址稳定）
		if existing, err := l.svcCtx.AddressGenerationLogRepo.FindByUserIDChainAndPath(l.ctx, in.UserId, chain, derivationPath); err == nil && existing != nil {
			addresses = append(addresses, &pb.UserDepositAddress{
				Chain:            existing.Chain,
				Address:          existing.Address,
				DerivationPath:   "",
				PublicKey:        "",
				CompressedPubkey: "",
			})
			l.Infof("用户%d的%s默认地址已存在: %s", in.UserId, chain, existing.Address)
			continue
		}

		// 获取公钥
		uncompressed, compressed, err := hdWallet.GetPublicKey(derivationPath)
		if err != nil {
			l.Errorf("获取公钥失败: %v", err)
			continue
		}

		// 生成地址
		address, err := PublicKeyToAddress(chain, uncompressed)
		if err != nil {
			l.Errorf("生成地址失败: %v", err)
			continue
		}

		// 6. 保存到地址生成日志
		generatedBy := "system"
		if in.Requester != "" {
			generatedBy = in.Requester // admin/business/system
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

		// 保存地址生成日志（如果失败，记录错误但继续返回地址）
		if err := l.svcCtx.AddressGenerationLogRepo.Create(l.ctx, addrLog); err != nil {
			// 如果是重复键错误（地址已存在），记录警告但继续返回地址
			// 因为 BSC 和 ETH 可能使用相同地址（EVM 兼容）
			if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
				if existing, err2 := l.svcCtx.AddressGenerationLogRepo.FindByUserIDChainAndPath(l.ctx, in.UserId, chain, derivationPath); err2 == nil && existing != nil {
					address = existing.Address
				}
				l.Infof("地址 %s 的 %s 链日志已存在，跳过保存: %v", address, chain, err)
			} else {
				l.Errorf("保存%s地址生成日志失败: %v", chain, err)
			}
			// 即使日志保存失败，也返回地址（地址生成成功）
		} else {
			l.Infof("✓ 为用户%d生成并保存%s地址: %s, 路径: %s", in.UserId, chain, address, derivationPath)
		}

		addresses = append(addresses, &pb.UserDepositAddress{
			Chain:            chain,
			Address:          address,
			DerivationPath:   "",
			PublicKey:        "",
			CompressedPubkey: "",
		})
	}

	if len(addresses) == 0 {
		return &pb.GenerateUserDepositAddressesResponse{
			Code:    500,
			Message: "所有链地址生成失败",
		}, nil
	}

	// 7. 返回结果
	return &pb.GenerateUserDepositAddressesResponse{
		Code:      200,
		Message:   "成功",
		UserId:    in.UserId,
		Addresses: addresses,
		CreatedAt: time.Now().Format("2006-01-02 15:04:05"),
	}, nil
}
