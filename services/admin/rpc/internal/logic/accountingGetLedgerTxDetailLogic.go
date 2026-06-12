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

type AccountingGetLedgerTxDetailLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAccountingGetLedgerTxDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AccountingGetLedgerTxDetailLogic {
	return &AccountingGetLedgerTxDetailLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AccountingGetLedgerTxDetailLogic) AccountingGetLedgerTxDetail(in *pb.AccountingGetLedgerTxDetailRequest) (*pb.AccountingGetLedgerTxDetailResponse, error) {
	if in == nil || in.TxId <= 0 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid params", map[string]string{"tx_id": "required"})
	}
	if l.svcCtx.AccountingRpc == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "ACCOUNTING_NOT_CONFIGURED", "accounting rpc not configured", nil)
	}
	if _, ok := admininterceptor.GetCurrentAdmin(l.ctx); !ok {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}

	accResp, callErr := l.svcCtx.AccountingRpc.GetLedgerTxDetail(l.ctx, &pb.GetLedgerTxDetailRequest{TxId: in.TxId})
	if callErr != nil {
		l.Logger.Errorf("AccountingGetLedgerTxDetail call accounting failed: %v", callErr)
		return nil, errx.New(codes.Unavailable, 503, errx.CodeInternalError, "ACCOUNTING_RPC_ERROR", "accounting rpc error", nil)
	}
	if accResp == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "ACCOUNTING_RPC_EMPTY", "accounting rpc empty response", nil)
	}

	out := &pb.AccountingGetLedgerTxDetailResponse{
		Success:   accResp.Success,
		Message:   strings.TrimSpace(accResp.Message),
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}
	if accResp.Success {
		postings := make([]*pb.AccountingLedgerPostingItem, 0)
		for _, it := range accResp.GetPostings() {
			if v := toAdminAccountingLedgerPostingItem(it); v != nil {
				postings = append(postings, v)
			}
		}
		out.Tx = toAdminAccountingLedgerTxItem(accResp.GetTx())
		out.Postings = postings
	}
	return out, nil
}
