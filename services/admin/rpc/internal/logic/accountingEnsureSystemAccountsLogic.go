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

type AccountingEnsureSystemAccountsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAccountingEnsureSystemAccountsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AccountingEnsureSystemAccountsLogic {
	return &AccountingEnsureSystemAccountsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AccountingEnsureSystemAccountsLogic) AccountingEnsureSystemAccounts(in *pb.AccountingEnsureSystemAccountsRequest) (*pb.AccountingEnsureSystemAccountsResponse, error) {
	if l.svcCtx.AccountingRpc == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "ACCOUNTING_NOT_CONFIGURED", "accounting rpc not configured", nil)
	}

	current, ok := admininterceptor.GetCurrentAdmin(l.ctx)
	if !ok || current == nil {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}

	chainCode := ""
	if in != nil {
		chainCode = strings.TrimSpace(in.ChainCode)
	}

	accResp, callErr := l.svcCtx.AccountingRpc.EnsureSystemAccounts(l.ctx, &pb.EnsureSystemAccountsRequest{ChainCode: chainCode})
	if callErr != nil {
		l.Logger.Errorf("AccountingEnsureSystemAccounts call accounting failed: %v", callErr)
		return nil, errx.New(codes.Unavailable, 503, errx.CodeInternalError, "ACCOUNTING_RPC_ERROR", "accounting rpc error", nil)
	}
	if accResp == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "ACCOUNTING_RPC_EMPTY", "accounting rpc empty response", nil)
	}

	out := &pb.AccountingEnsureSystemAccountsResponse{
		Success:   accResp.Success,
		Message:   strings.TrimSpace(accResp.Message),
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}
	if accResp.Success {
		auditAdminAction(l.ctx, l.svcCtx.AdminAuditLogRepo, current,
			"accounting.system.ensure_accounts",
			"system",
			chainCode,
			"确保系统会计账户初始化",
			map[string]any{"chain_code": chainCode},
		)
	}

	return out, nil
}
