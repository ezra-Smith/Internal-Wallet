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

type AccountingDeleteAccountTypeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAccountingDeleteAccountTypeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AccountingDeleteAccountTypeLogic {
	return &AccountingDeleteAccountTypeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AccountingDeleteAccountTypeLogic) AccountingDeleteAccountType(in *pb.AccountingDeleteAccountTypeRequest) (*pb.AccountingDeleteAccountTypeResponse, error) {
	if in == nil || strings.TrimSpace(in.Code) == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid params", map[string]string{"code": "required"})
	}
	if l.svcCtx.AccountingRpc == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "ACCOUNTING_NOT_CONFIGURED", "accounting rpc not configured", nil)
	}

	current, ok := admininterceptor.GetCurrentAdmin(l.ctx)
	if !ok || current == nil {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}

	code := normalizeCode(in.Code)

	_, callErr := l.svcCtx.AccountingRpc.DeleteAccountType(l.ctx, &pb.DeleteAccountTypeRequest{Code: code})
	if callErr != nil {
		l.Logger.Errorf("AccountingDeleteAccountType call accounting failed: %v", callErr)
		return nil, errx.New(codes.Unavailable, 503, errx.CodeInternalError, "ACCOUNTING_RPC_ERROR", "accounting rpc error", nil)
	}

	auditAdminAction(l.ctx, l.svcCtx.AdminAuditLogRepo, current,
		"accounting.account_type.delete",
		"acct_account_type",
		code,
		"删除账户类型: "+code,
		map[string]any{"code": code},
	)

	return &pb.AccountingDeleteAccountTypeResponse{
		Success:   true,
		Message:   "ok",
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
