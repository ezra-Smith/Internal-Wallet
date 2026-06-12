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

type GetDepositAddressLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetDepositAddressLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetDepositAddressLogic {
	return &GetDepositAddressLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetDepositAddressLogic) GetDepositAddress(in *pb.GetDepositAddressRequest) (*pb.GetDepositAddressResponse, error) {
	if in == nil || (in.Id <= 0 && strings.TrimSpace(in.Address) == "") {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid id/address", map[string]string{
			"id":      "required",
			"address": "required",
		})
	}
	if l.svcCtx.DB == nil || l.svcCtx.WalletDepositAddressRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	var item *pb.WalletDepositAddressItem
	if in.Id > 0 {
		m, err := l.svcCtx.WalletDepositAddressRepo.FindByID(l.ctx, in.Id)
		if err != nil {
			return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "NOT_FOUND", "deposit address not found", nil)
		}
		item = toPBDepositAddressItem(m)
	} else {
		m, err := l.svcCtx.WalletDepositAddressRepo.FindByAddress(l.ctx, strings.TrimSpace(in.Address))
		if err != nil {
			return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "NOT_FOUND", "deposit address not found", nil)
		}
		item = toPBDepositAddressItem(m)
	}

	return &pb.GetDepositAddressResponse{
		Success: true,
		Message: "ok",
		Data: &pb.GetDepositAddressData{
			Address: item,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
