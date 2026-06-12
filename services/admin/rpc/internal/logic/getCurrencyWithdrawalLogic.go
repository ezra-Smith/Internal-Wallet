package logic

import (
	"context"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
)

type GetCurrencyWithdrawalLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetCurrencyWithdrawalLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetCurrencyWithdrawalLogic {
	return &GetCurrencyWithdrawalLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetCurrencyWithdrawalLogic) GetCurrencyWithdrawal(in *pb.GetCurrencyWithdrawalRequest) (*pb.GetCurrencyWithdrawalResponse, error) {
	if in == nil || (in.Id <= 0 && strings.TrimSpace(in.TxHash) == "") {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid id/tx_hash", map[string]string{
			"id":      "required",
			"tx_hash": "required",
		})
	}
	if l.svcCtx == nil || l.svcCtx.DB == nil || l.svcCtx.CurrencyWithdrawOrderRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	var m *model.CurrencyWithdrawOrderModel
	var err error
	if in.Id > 0 {
		m, err = l.svcCtx.CurrencyWithdrawOrderRepo.FindByID(l.ctx, in.Id)
	} else {
		m, err = l.svcCtx.CurrencyWithdrawOrderRepo.FindByTxHash(l.ctx, strings.TrimSpace(in.TxHash))
	}
	if err != nil || m == nil {
		return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "NOT_FOUND", "withdrawal not found", nil)
	}

	return &pb.GetCurrencyWithdrawalResponse{
		Success: true,
		Message: "ok",
		Data: &pb.GetCurrencyWithdrawalData{
			Withdrawal: toPBCurrencyWithdrawalItem(m),
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
