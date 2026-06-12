package logic

import (
	"context"
	"fmt"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	admininterceptor "internalwallet/services/admin/rpc/internal/interceptor"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
)

type UpdateCurrencyStatusLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateCurrencyStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateCurrencyStatusLogic {
	return &UpdateCurrencyStatusLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateCurrencyStatusLogic) UpdateCurrencyStatus(in *pb.UpdateCurrencyStatusRequest) (*pb.UpdateCurrencyStatusResponse, error) {
	if in == nil || strings.TrimSpace(in.AssetCode) == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid params", map[string]string{"asset_code": "required"})
	}
	if in.Status != 1 && in.Status != 2 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_STATUS", "invalid status", map[string]string{"status": "invalid"})
	}
	if l.svcCtx.AccountingRpc == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "accounting rpc not configured", nil)
	}

	current, ok := admininterceptor.GetCurrentAdmin(l.ctx)
	if !ok || current == nil {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}

	assetCode := normalizeCode(in.AssetCode)
	_ = current // reserved for audit; Accounting manages asset source-of-truth
	accResp, err := l.svcCtx.AccountingRpc.UpdateAssetStatus(l.ctx, &pb.UpdateAssetStatusRequest{
		Code:   assetCode,
		Status: in.Status,
	})
	if err != nil {
		l.Logger.Errorf("call accounting UpdateAssetStatus failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "accounting service error", nil)
	}
	if accResp == nil || !accResp.Success {
		return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "NOT_FOUND", "asset not found", map[string]string{"asset_code": "not found"})
	}

	return &pb.UpdateCurrencyStatusResponse{
		Success: true,
		Message: fmt.Sprintf("ok (%s)", assetCode),
		Data: &pb.UpdateCurrencyStatusData{
			AssetCode: assetCode,
			Status:    in.Status,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
