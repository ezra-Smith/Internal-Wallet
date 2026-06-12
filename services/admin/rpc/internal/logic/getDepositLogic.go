package logic

import (
	"context"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
)

type GetDepositLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetDepositLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetDepositLogic {
	return &GetDepositLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetDepositLogic) GetDeposit(in *pb.GetDepositRequest) (*pb.GetDepositResponse, error) {
	if in == nil || (in.Id <= 0 && strings.TrimSpace(in.TransactionHash) == "") {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid id/transaction_hash", map[string]string{
			"id":               "required",
			"transaction_hash": "required",
		})
	}
	if l.svcCtx.DB == nil || l.svcCtx.WalletDepositRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	var item *pb.WalletDepositItem
	if in.Id > 0 {
		m, err := l.svcCtx.WalletDepositRepo.FindByID(l.ctx, in.Id)
		if err != nil {
			return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "NOT_FOUND", "deposit not found", nil)
		}
		item = toPBDepositItem(m)
	} else {
		m, err := l.svcCtx.WalletDepositRepo.FindByTxHash(l.ctx, strings.TrimSpace(in.TransactionHash))
		if err != nil {
			return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "NOT_FOUND", "deposit not found", nil)
		}
		item = toPBDepositItem(m)
	}

	return &pb.GetDepositResponse{
		Success: true,
		Message: "ok",
		Data: &pb.GetDepositData{
			Deposit: item,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
