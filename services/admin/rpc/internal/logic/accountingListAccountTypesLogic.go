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

type AccountingListAccountTypesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAccountingListAccountTypesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AccountingListAccountTypesLogic {
	return &AccountingListAccountTypesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// -------- Accounting (Ledger) --------
func (l *AccountingListAccountTypesLogic) AccountingListAccountTypes(in *pb.AccountingListAccountTypesRequest) (*pb.AccountingListAccountTypesResponse, error) {
	if l.svcCtx.AccountingRpc == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "ACCOUNTING_NOT_CONFIGURED", "accounting rpc not configured", nil)
	}
	if _, ok := admininterceptor.GetCurrentAdmin(l.ctx); !ok {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}

	accResp, callErr := l.svcCtx.AccountingRpc.ListAccountTypes(l.ctx, &pb.ListAccountTypesRequest{IncludeDeleted: false})
	if callErr != nil {
		l.Logger.Errorf("AccountingListAccountTypes call accounting failed: %v", callErr)
		return nil, errx.New(codes.Unavailable, 503, errx.CodeInternalError, "ACCOUNTING_RPC_ERROR", "accounting rpc error", nil)
	}

	items := make([]*pb.AccountingAccountTypeItem, 0)
	for _, it := range accResp.GetItems() {
		if v := toAdminAccountingAccountTypeItem(it); v != nil {
			items = append(items, v)
		}
	}

	return &pb.AccountingListAccountTypesResponse{
		Success: true,
		Message: "ok",
		Data: &pb.AccountingListAccountTypesData{
			Items: items,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
