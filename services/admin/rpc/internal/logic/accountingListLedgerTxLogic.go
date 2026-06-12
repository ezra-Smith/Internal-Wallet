package logic

import (
	"context"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	admininterceptor "internalwallet/services/admin/rpc/internal/interceptor"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
)

type AccountingListLedgerTxLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAccountingListLedgerTxLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AccountingListLedgerTxLogic {
	return &AccountingListLedgerTxLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AccountingListLedgerTxLogic) AccountingListLedgerTx(in *pb.AccountingListLedgerTxRequest) (*pb.AccountingListLedgerTxResponse, error) {
	if l.svcCtx.AccountingRpc == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "ACCOUNTING_NOT_CONFIGURED", "accounting rpc not configured", nil)
	}
	if _, ok := admininterceptor.GetCurrentAdmin(l.ctx); !ok {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}
	if in == nil {
		in = &pb.AccountingListLedgerTxRequest{}
	}

	// Validate dates (best-effort).
	if _, _, err := parseDateFromTo(in.CreatedFrom, in.CreatedTo); err != nil {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_DATE", "invalid date", map[string]string{"created_from": "invalid"})
	}

	accResp, callErr := l.svcCtx.AccountingRpc.ListLedgerTx(l.ctx, &pb.ListLedgerTxRequest{
		Page:           in.Page,
		PageSize:       in.PageSize,
		TxId:           in.TxId,
		OpType:         strings.TrimSpace(in.OpType),
		BizRef:         strings.TrimSpace(in.BizRef),
		IdempotencyKey: strings.TrimSpace(in.IdempotencyKey),
		AssetCode:      normalizeCode(in.AssetCode),
		UserId:         in.UserId,
		CreatedFrom:    strings.TrimSpace(in.CreatedFrom),
		CreatedTo:      strings.TrimSpace(in.CreatedTo),
	})
	if callErr != nil {
		l.Logger.Errorf("AccountingListLedgerTx call accounting failed: %v", callErr)
		return nil, errx.New(codes.Unavailable, 503, errx.CodeInternalError, "ACCOUNTING_RPC_ERROR", "accounting rpc error", nil)
	}
	if accResp == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "ACCOUNTING_RPC_EMPTY", "accounting rpc empty response", nil)
	}

	out := &pb.AccountingListLedgerTxResponse{
		Success:   accResp.Success,
		Message:   strings.TrimSpace(accResp.Message),
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}
	if !accResp.Success {
		return out, nil
	}

	items := make([]*pb.AccountingLedgerTxItem, 0)
	for _, it := range accResp.GetItems() {
		if v := toAdminAccountingLedgerTxItem(it); v != nil {
			items = append(items, v)
		}
	}
	out.Items = items
	out.Pagination = calcPagination(in.Page, in.PageSize, accResp.Total)
	return out, nil
}
