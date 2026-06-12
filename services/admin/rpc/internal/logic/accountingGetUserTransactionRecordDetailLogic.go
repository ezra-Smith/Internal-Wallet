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

type AccountingGetUserTransactionRecordDetailLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAccountingGetUserTransactionRecordDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AccountingGetUserTransactionRecordDetailLogic {
	return &AccountingGetUserTransactionRecordDetailLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AccountingGetUserTransactionRecordDetailLogic) AccountingGetUserTransactionRecordDetail(in *pb.AccountingGetUserTransactionRecordDetailRequest) (*pb.AccountingGetUserTransactionRecordDetailResponse, error) {
	// 验证管理员权限
	if _, ok := admininterceptor.GetCurrentAdmin(l.ctx); !ok {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}

	// 调用 accounting 服务
	if l.svcCtx.AccountingRpc == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "ACCOUNTING_NOT_CONFIGURED", "accounting rpc not configured", nil)
	}

	accResp, err := l.svcCtx.AccountingRpc.GetUserTransactionRecordDetail(l.ctx, &pb.GetUserTransactionRecordDetailRequest{
		Id: in.Id,
	})
	if err != nil {
		l.Logger.Errorf("GetUserTransactionRecordDetail call accounting failed: %v", err)
		return nil, errx.New(codes.Unavailable, 503, errx.CodeInternalError, "ACCOUNTING_RPC_ERROR", "accounting rpc error", nil)
	}

	var item *pb.AccountingUserTransactionRecordItem
	if accResp.Item != nil {
		item = toAdminUserTransactionRecordItem(accResp.Item)
	}

	return &pb.AccountingGetUserTransactionRecordDetailResponse{
		Success:   accResp.Success,
		Message:   accResp.Message,
		Item:      item,
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
