package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	admininterceptor "internalwallet/services/admin/rpc/internal/interceptor"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
)

type AccountingGetUserBalancesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAccountingGetUserBalancesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AccountingGetUserBalancesLogic {
	return &AccountingGetUserBalancesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AccountingGetUserBalancesLogic) AccountingGetUserBalances(in *pb.AccountingGetUserBalancesRequest) (*pb.AccountingGetUserBalancesResponse, error) {
	if in == nil || in.UserId <= 0 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid params", map[string]string{"user_id": "required"})
	}
	if l.svcCtx.AccountingRpc == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "ACCOUNTING_NOT_CONFIGURED", "accounting rpc not configured", nil)
	}
	if l.svcCtx.UserRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "user repo not configured", nil)
	}
	if _, ok := admininterceptor.GetCurrentAdmin(l.ctx); !ok {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}

	// Validate user exists
	if _, err := l.svcCtx.UserRepo.FindByID(l.ctx, in.UserId); err != nil {
		return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "USER_NOT_FOUND", "user not found", nil)
	}

	accResp, callErr := l.svcCtx.AccountingRpc.GetUserBalances(l.ctx, &pb.GetUserBalancesRequest{UserId: in.UserId})
	if callErr != nil {
		l.Logger.Errorf("AccountingGetUserBalances call accounting failed: %v", callErr)
		return nil, errx.New(codes.Unavailable, 503, errx.CodeInternalError, "ACCOUNTING_RPC_ERROR", "accounting rpc error", nil)
	}

	items := make([]*pb.AccountingUserBalanceItem, 0)
	for _, it := range accResp.GetItems() {
		if v := toAdminAccountingUserBalanceItem(it); v != nil {
			items = append(items, v)
		}
	}

	return &pb.AccountingGetUserBalancesResponse{
		Success: true,
		Message: "ok",
		Data: &pb.AccountingGetUserBalancesData{
			Items: items,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
