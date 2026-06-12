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

type GetCurrencyLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetCurrencyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetCurrencyLogic {
	return &GetCurrencyLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetCurrency 获取币种基础信息（用于前端编辑表单回显）
func (l *GetCurrencyLogic) GetCurrency(in *pb.GetCurrencyRequest) (*pb.GetCurrencyResponse, error) {
	// 参数验证
	if in == nil || strings.TrimSpace(in.AssetCode) == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid params", map[string]string{
			"asset_code": "required",
		})
	}

	// 检查依赖
	if l.svcCtx.AccountingRpc == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "accounting rpc not configured", nil)
	}

	//// 权限验证
	if _, ok := admininterceptor.GetCurrentAdmin(l.ctx); !ok {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}

	assetCode := normalizeCode(in.AssetCode)

	// 从 Accounting 服务获取资产信息
	getResp, err := l.svcCtx.AccountingRpc.GetAsset(l.ctx, &pb.GetAssetRequest{Code: assetCode})
	if err != nil {
		l.Logger.Errorf("call accounting GetAsset failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "accounting service error", nil)
	}
	if getResp == nil || !getResp.Success || getResp.Item == nil {
		return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "NOT_FOUND", "asset not found", map[string]string{
			"asset_code": "not found",
		})
	}

	assetItem := getResp.Item

	// 返回基础信息
	return &pb.GetCurrencyResponse{
		Success: true,
		Message: "ok",
		Data: &pb.GetCurrencyData{
			Currency: &pb.CurrencyItem{
				AssetCode: assetCode,
				AssetName: func() string {
					name := strings.TrimSpace(assetItem.Name)
					if name == "" {
						return assetCode
					}
					return name
				}(),
				Status:    assetItem.Status,
				IconUrl:   strings.TrimSpace(assetItem.IconUrl),
				Precision: assetItem.Precision,
			},
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
