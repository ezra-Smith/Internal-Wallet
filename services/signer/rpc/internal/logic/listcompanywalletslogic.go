package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/signer/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListCompanyWalletsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListCompanyWalletsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListCompanyWalletsLogic {
	return &ListCompanyWalletsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListCompanyWallets 查询所有公司钱包地址（支持多条件筛选）
func (l *ListCompanyWalletsLogic) ListCompanyWallets(in *pb.ListCompanyWalletsRequest) (*pb.ListCompanyWalletsResponse, error) {
	// 1. 参数处理
	chain := in.Chain                   // 链类型，空字符串表示不筛选
	temperature := int8(in.Temperature) // 1=热钱包, 2=冷钱包, 0=不筛选
	status := int8(in.Status)           // 1=启用, 0=禁用, -1=所有
	addressType := in.AddressType       // 地址类型，空字符串表示不筛选

	// 2. 从数据库查询
	wallets, err := l.svcCtx.CompanyWalletRepo.ListWithFilters(l.ctx, chain, temperature, status, addressType)
	if err != nil {
		l.Errorf("Failed to list company wallets: chain=%s, temperature=%d, status=%d, address_type=%s, error=%v",
			chain, temperature, status, addressType, err)
		return &pb.ListCompanyWalletsResponse{
			Code:    500,
			Message: "failed to query company wallets",
		}, nil
	}

	// 3. 构建返回数据
	var walletInfos []*pb.CompanyWalletInfo
	for _, wallet := range wallets {
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
		walletInfos = append(walletInfos, walletInfo)
	}

	// 4. 记录访问日志
	l.Infof("ListCompanyWallets: chain=%s, temperature=%d, status=%d, address_type=%s, total=%d",
		chain, temperature, status, addressType, len(walletInfos))

	return &pb.ListCompanyWalletsResponse{
		Code:    0,
		Message: "success",
		Wallets: walletInfos,
		Total:   int32(len(walletInfos)),
	}, nil
}
