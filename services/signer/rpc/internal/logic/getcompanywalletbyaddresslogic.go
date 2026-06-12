package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/signer/rpc/internal/models"
	"internalwallet/services/signer/rpc/internal/repository"
	"internalwallet/services/signer/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetCompanyWalletByAddressLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetCompanyWalletByAddressLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetCompanyWalletByAddressLogic {
	return &GetCompanyWalletByAddressLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetCompanyWalletByAddress 根据地址查询公司钱包信息
func (l *GetCompanyWalletByAddressLogic) GetCompanyWalletByAddress(in *pb.GetCompanyWalletByAddressRequest) (*pb.GetCompanyWalletResponse, error) {
	// 1. 参数验证
	if in.Address == "" {
		return &pb.GetCompanyWalletResponse{
			Code:    400,
			Message: "address cannot be empty",
		}, nil
	}

	// 2. 从数据库查询
	var wallet, err = func() (*models.CompanyWallet, error) {
		// 如果指定了链，使用更快的查询方式（联合索引）
		if in.Chain != "" {
			return l.svcCtx.CompanyWalletRepo.FindByChainAndAddress(l.ctx, in.Chain, in.Address)
		}

		// 否则需要遍历所有链查找（性能较差，建议总是传递 chain 参数）
		// 依次尝试 TRON, BSC, ETH
		chains := []string{"TRON", "BSC", "ETH"}
		for _, chain := range chains {
			w, err := l.svcCtx.CompanyWalletRepo.FindByChainAndAddress(l.ctx, chain, in.Address)
			if err == nil {
				return w, nil
			}
		}
		return nil, repository.ErrCompanyWalletNotFound
	}()

	if err != nil {
		l.Errorf("Failed to find company wallet: address=%s, chain=%s, error=%v",
			in.Address, in.Chain, err)
		return &pb.GetCompanyWalletResponse{
			Code:    404,
			Message: "company wallet not found",
		}, nil
	}

	// 3. 构建返回数据
	walletInfo := &pb.CompanyWalletInfo{
		Id:               wallet.ID,
		AddressIndex:     int32(wallet.AddressIndex),
		AddressType:      wallet.AddressType,
		Chain:            wallet.Chain,
		Address:          wallet.Address,
		DerivationPath:   wallet.DerivationPath,
		SeedId:           wallet.SeedID,
		Temperature:      int32(wallet.Temperature),
		Status:           int32(wallet.Status),
		BalanceThreshold: wallet.BalanceThreshold,
		Remark:           wallet.Remark,
		CreatedAt:        wallet.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:        wallet.UpdatedAt.Format("2006-01-02 15:04:05"),
	}

	// 4. 记录访问日志（供审计）
	l.Infof("GetCompanyWalletByAddress: address=%s, chain=%s, address_type=%s, temperature=%d",
		wallet.Address, wallet.Chain, wallet.AddressType, wallet.Temperature)

	return &pb.GetCompanyWalletResponse{
		Code:    0,
		Message: "success",
		Wallet:  walletInfo,
	}, nil
}
