package logic

import (
	"context"
	admininterceptor "internalwallet/services/admin/rpc/internal/interceptor"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
)

type AccountingListUserTransactionRecordsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAccountingListUserTransactionRecordsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AccountingListUserTransactionRecordsLogic {
	return &AccountingListUserTransactionRecordsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AccountingListUserTransactionRecordsLogic) AccountingListUserTransactionRecords(in *pb.AccountingListUserTransactionRecordsRequest) (*pb.AccountingListUserTransactionRecordsResponse, error) {
	// 1. 验证管理员权限
	if _, ok := admininterceptor.GetCurrentAdmin(l.ctx); !ok {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}

	// 2. 参数验证
	if l.svcCtx.AccountingRpc == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "ACCOUNTING_NOT_CONFIGURED", "accounting rpc not configured", nil)
	}
	if in == nil {
		in = &pb.AccountingListUserTransactionRecordsRequest{}
	}
	// user_id 是可选参数，0 表示查询所有用户

	// 3. 调用 accounting 服务
	accResp, err := l.svcCtx.AccountingRpc.ListUserTransactionRecords(l.ctx, &pb.ListUserTransactionRecordsRequest{
		Page:      in.Page,
		PageSize:  in.PageSize,
		UserId:    in.UserId,
		TxType:    in.TxType,
		AssetCode: strings.TrimSpace(in.AssetCode),
		ChainCode: strings.TrimSpace(in.ChainCode),
		Status:    strings.TrimSpace(in.Status),
		StartTime: in.StartTime,
		EndTime:   in.EndTime,
	})
	if err != nil {
		l.Logger.Errorf("AccountingListUserTransactionRecords call accounting failed: %v", err)
		return nil, errx.New(codes.Unavailable, 503, errx.CodeInternalError, "ACCOUNTING_RPC_ERROR", "accounting rpc error", nil)
	}

	// 4. 转换响应格式
	items := make([]*pb.AccountingUserTransactionRecordItem, 0, len(accResp.Items))
	for _, item := range accResp.Items {
		items = append(items, toAdminUserTransactionRecordItem(item))
	}

	return &pb.AccountingListUserTransactionRecordsResponse{
		Success:    accResp.Success,
		Message:    accResp.Message,
		Total:      accResp.Total,
		Items:      items,
		Pagination: calcPagination(in.Page, in.PageSize, accResp.Total),
		RequestId:  resp.RequestID(l.ctx),
		Timestamp:  resp.Timestamp(),
	}, nil
}
