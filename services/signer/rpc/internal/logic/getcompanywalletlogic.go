package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/signer/rpc/internal/models"
	"internalwallet/services/signer/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetCompanyWalletLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetCompanyWalletLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetCompanyWalletLogic {
	return &GetCompanyWalletLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetCompanyWallet 查询公司钱包地址（根据链和地址类型）
func (l *GetCompanyWalletLogic) GetCompanyWallet(in *pb.GetCompanyWalletRequest) (*pb.GetCompanyWalletResponse, error) {
	// 1. 参数验证
	if in.Chain == "" {
		return &pb.GetCompanyWalletResponse{
			Code:    400,
			Message: "chain cannot be empty",
		}, nil
	}

	if in.AddressType == "" {
		return &pb.GetCompanyWalletResponse{
			Code:    400,
			Message: "address_type cannot be empty",
		}, nil
	}

	// 2. 根据链和地址类型查询
	wallet, err := l.svcCtx.CompanyWalletRepo.FindByAddressType(l.ctx, in.AddressType, in.Chain)
	if err != nil {
		l.Errorf("Failed to find company wallet: chain=%s, address_type=%s, error=%v",
			in.Chain, in.AddressType, err)
		return &pb.GetCompanyWalletResponse{
			Code:    404,
			Message: "company wallet not found",
		}, nil
	}

	// 3. 温度筛选（可选）
	if in.Temperature > 0 && wallet.Temperature != int8(in.Temperature) {
		return &pb.GetCompanyWalletResponse{
			Code:    404,
			Message: "company wallet not found with specified temperature",
		}, nil
	}

	// 4. 构建返回数据
	walletInfo := l.buildCompanyWalletInfo(wallet)

	// 5. 记录访问日志（供审计）
	l.Infof("GetCompanyWallet: chain=%s, address_type=%s, address=%s, temperature=%d",
		wallet.Chain, wallet.AddressType, wallet.Address, wallet.Temperature)

	return &pb.GetCompanyWalletResponse{
		Code:    0,
		Message: "success",
		Wallet:  walletInfo,
	}, nil
}

// buildCompanyWalletInfo 将数据库模型转换为 proto 消息
func (l *GetCompanyWalletLogic) buildCompanyWalletInfo(wallet *models.CompanyWallet) *pb.CompanyWalletInfo {
	return &pb.CompanyWalletInfo{
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
}
