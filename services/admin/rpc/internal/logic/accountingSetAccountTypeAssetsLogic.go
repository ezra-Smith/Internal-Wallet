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

type AccountingSetAccountTypeAssetsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAccountingSetAccountTypeAssetsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AccountingSetAccountTypeAssetsLogic {
	return &AccountingSetAccountTypeAssetsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AccountingSetAccountTypeAssetsLogic) AccountingSetAccountTypeAssets(in *pb.AccountingSetAccountTypeAssetsRequest) (*pb.AccountingSetAccountTypeAssetsResponse, error) {
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
	assetCodes := normalizeAccountingAssetCodes(in.AssetCodes)

	_, callErr := l.svcCtx.AccountingRpc.SetAccountTypeAssets(l.ctx, &pb.SetAccountTypeAssetsRequest{
		AccountTypeCode: code,
		AssetCodes:      assetCodes,
	})
	if callErr != nil {
		l.Logger.Errorf("AccountingSetAccountTypeAssets call accounting failed: %v", callErr)
		return nil, errx.New(codes.Unavailable, 503, errx.CodeInternalError, "ACCOUNTING_RPC_ERROR", "accounting rpc error", nil)
	}

	auditAdminAction(l.ctx, l.svcCtx.AdminAuditLogRepo, current,
		"accounting.account_type.set_assets",
		"acct_account_type",
		code,
		"设置账户类型资产: "+code,
		map[string]any{
			"code":        code,
			"asset_codes": assetCodes,
		},
	)

	return &pb.AccountingSetAccountTypeAssetsResponse{
		Success:   true,
		Message:   "ok",
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
