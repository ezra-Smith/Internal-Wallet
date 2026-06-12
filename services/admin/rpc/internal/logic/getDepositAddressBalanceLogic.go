package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
)

type GetDepositAddressBalanceLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetDepositAddressBalanceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetDepositAddressBalanceLogic {
	return &GetDepositAddressBalanceLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetDepositAddressBalanceLogic) GetDepositAddressBalance(in *pb.GetDepositAddressBalanceRequest) (*pb.GetDepositAddressBalanceResponse, error) {
	if in == nil || in.Id <= 0 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "id required", nil)
	}
	if l.svcCtx.DB == nil || l.svcCtx.WalletDepositAddressBalanceRepo == nil || l.svcCtx.WalletDepositAddressRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	// 查询余额记录
	var balance *model.WalletDepositAddressBalanceModel
	var err error

	balance, err = l.svcCtx.WalletDepositAddressBalanceRepo.FindByID(l.ctx, in.Id)

	if err != nil || balance == nil {
		return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "NOT_FOUND", "deposit address balance not found", nil)
	}

	// 查询充值地址信息
	address := ""
	addr, err := l.svcCtx.WalletDepositAddressRepo.FindByID(l.ctx, balance.DepositAddressID)
	if err == nil && addr != nil {
		address = addr.Address
	}

	return &pb.GetDepositAddressBalanceResponse{
		Success: true,
		Message: "ok",
		Data: &pb.GetDepositAddressBalanceData{
			Balance: toPBDepositAddressBalanceItem(balance, address),
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
