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

type AccountingCreateAccountTypeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAccountingCreateAccountTypeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AccountingCreateAccountTypeLogic {
	return &AccountingCreateAccountTypeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AccountingCreateAccountTypeLogic) AccountingCreateAccountType(in *pb.AccountingCreateAccountTypeRequest) (*pb.AccountingCreateAccountTypeResponse, error) {
	if in == nil || strings.TrimSpace(in.Code) == "" || strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.NormalSide) == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid params", map[string]string{
			"code":        "required",
			"name":        "required",
			"normal_side": "required",
		})
	}
	if l.svcCtx.AccountingRpc == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "ACCOUNTING_NOT_CONFIGURED", "accounting rpc not configured", nil)
	}

	current, ok := admininterceptor.GetCurrentAdmin(l.ctx)
	if !ok || current == nil {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}

	code := normalizeCode(in.Code)
	name := strings.TrimSpace(in.Name)
	description := strings.TrimSpace(in.Description)
	normalSide, ok := parseAccountingNormalSide(in.NormalSide)
	if !ok {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_NORMAL_SIDE", "invalid normal_side", map[string]string{"normal_side": "invalid"})
	}

	accResp, callErr := l.svcCtx.AccountingRpc.CreateAccountType(l.ctx, &pb.CreateAccountTypeRequest{
		Code:        code,
		Name:        name,
		Description: description,
		NormalSide:  normalSide,
	})
	if callErr != nil {
		l.Logger.Errorf("AccountingCreateAccountType call accounting failed: %v", callErr)
		return nil, errx.New(codes.Unavailable, 503, errx.CodeInternalError, "ACCOUNTING_RPC_ERROR", "accounting rpc error", nil)
	}
	if accResp == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "ACCOUNTING_RPC_EMPTY", "accounting rpc empty response", nil)
	}

	out := &pb.AccountingCreateAccountTypeResponse{
		Success:   accResp.Success,
		Message:   strings.TrimSpace(accResp.Message),
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}
	if item := toAdminAccountingAccountTypeItem(accResp.Item); item != nil {
		out.Data = &pb.AccountingCreateAccountTypeData{Item: item}
	}

	if accResp.Success {
		auditAdminAction(l.ctx, l.svcCtx.AdminAuditLogRepo, current,
			"accounting.account_type.create",
			"acct_account_type",
			code,
			"创建账户类型: "+code,
			map[string]any{
				"code":        code,
				"name":        name,
				"description": description,
				"normal_side": formatAccountingNormalSide(normalSide),
			},
		)
	}

	return out, nil
}
