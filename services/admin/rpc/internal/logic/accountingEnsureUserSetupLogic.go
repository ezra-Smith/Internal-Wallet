package logic

import (
	"context"
	"strconv"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	admininterceptor "internalwallet/services/admin/rpc/internal/interceptor"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
)

type AccountingEnsureUserSetupLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAccountingEnsureUserSetupLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AccountingEnsureUserSetupLogic {
	return &AccountingEnsureUserSetupLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AccountingEnsureUserSetupLogic) AccountingEnsureUserSetup(in *pb.AccountingEnsureUserSetupRequest) (*pb.AccountingEnsureUserSetupResponse, error) {
	if in == nil || in.UserId <= 0 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid params", map[string]string{"user_id": "required"})
	}
	if l.svcCtx.AccountingRpc == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "ACCOUNTING_NOT_CONFIGURED", "accounting rpc not configured", nil)
	}
	if l.svcCtx.UserRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "user repo not configured", nil)
	}

	current, ok := admininterceptor.GetCurrentAdmin(l.ctx)
	if !ok || current == nil {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}

	// Validate user exists
	if _, err := l.svcCtx.UserRepo.FindByID(l.ctx, in.UserId); err != nil {
		return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "USER_NOT_FOUND", "user not found", nil)
	}

	accResp, callErr := l.svcCtx.AccountingRpc.EnsureUserAccountingSetup(l.ctx, &pb.EnsureUserAccountingSetupRequest{UserId: in.UserId})
	if callErr != nil {
		l.Logger.Errorf("AccountingEnsureUserSetup call accounting failed: %v", callErr)
		return nil, errx.New(codes.Unavailable, 503, errx.CodeInternalError, "ACCOUNTING_RPC_ERROR", "accounting rpc error", nil)
	}
	if accResp == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "ACCOUNTING_RPC_EMPTY", "accounting rpc empty response", nil)
	}

	out := &pb.AccountingEnsureUserSetupResponse{
		Success:   accResp.Success,
		Message:   strings.TrimSpace(accResp.Message),
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}

	if accResp.Success {
		auditAdminAction(l.ctx, l.svcCtx.AdminAuditLogRepo, current,
			"accounting.user.ensure_setup",
			"user",
			strconv.FormatInt(in.UserId, 10),
			"确保用户会计初始化",
			map[string]any{"user_id": in.UserId},
		)
	}

	return out, nil
}
