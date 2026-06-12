package logic

import (
	"context"
	admininterceptor "internalwallet/services/admin/rpc/internal/interceptor"
	"net/url"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
)

type UpdateCurrencyLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateCurrencyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateCurrencyLogic {
	return &UpdateCurrencyLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateCurrencyLogic) UpdateCurrency(in *pb.UpdateCurrencyRequest) (*pb.UpdateCurrencyResponse, error) {
	if in == nil || strings.TrimSpace(in.AssetCode) == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid params", map[string]string{
			"asset_code": "required",
		})
	}
	if l.svcCtx.AccountingRpc == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "accounting rpc not configured", nil)
	}

	current, ok := admininterceptor.GetCurrentAdmin(l.ctx)
	if !ok || current == nil {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}

	assetCode := normalizeCode(in.AssetCode)

	// 获取当前资产信息
	getResp, err := l.svcCtx.AccountingRpc.GetAsset(l.ctx, &pb.GetAssetRequest{Code: assetCode})
	if err != nil {
		l.Logger.Errorf("call accounting GetAsset failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "accounting service error", nil)
	}
	if getResp == nil || !getResp.Success || getResp.Item == nil {
		return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "NOT_FOUND", "asset not found", map[string]string{"asset_code": "not found"})
	}

	assetItem := getResp.Item

	// 关键检查：只有禁用状态（status=2）才能修改基础信息
	if assetItem.Status != 2 {
		l.Logger.Errorf("asset must be disabled to update basic info: asset_code=%s, current_status=%d", assetCode, assetItem.Status)
		return nil, errx.New(codes.FailedPrecondition, 400, errx.CodeInvalidParam, "ASSET_MUST_BE_DISABLED",
			"asset must be disabled (status=2) to update basic information", map[string]string{
				"status":  "asset is enabled",
				"message": "please disable the asset first before updating",
			})
	}

	hasChanges := false

	// 准备更新 asset_name 和 precision（合并到一个请求）
	needUpdateAsset := false
	updateAssetName := assetItem.Name
	updateAssetPrecision := assetItem.Precision

	// 检查 asset_name
	if in.AssetName != "" {
		assetName := strings.TrimSpace(in.AssetName)
		if len(assetName) > 64 {
			return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_ASSET_NAME", "invalid asset_name", map[string]string{
				"asset_name": "too long",
			})
		}
		if assetName != strings.TrimSpace(assetItem.Name) {
			updateAssetName = assetName
			needUpdateAsset = true
		}
	}

	// 更新 icon_url
	if in.IconUrl != nil {
		iconURL := strings.TrimSpace(in.IconUrl.GetValue())
		if iconURL != "" {
			if len(iconURL) > 2048 {
				return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_ICON_URL", "icon_url too long", map[string]string{"icon_url": "too long"})
			}
			u, parseErr := url.ParseRequestURI(iconURL)
			if parseErr != nil || u == nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
				return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_ICON_URL", "invalid icon_url", map[string]string{"icon_url": "invalid"})
			}
		}
		if iconURL != strings.TrimSpace(assetItem.IconUrl) {
			iconResp, err := l.svcCtx.AccountingRpc.UpdateAssetIcon(l.ctx, &pb.UpdateAssetIconRequest{
				Code:    assetCode,
				IconUrl: iconURL,
			})
			if err != nil {
				l.Logger.Errorf("call accounting UpdateAssetIcon failed: %v", err)
				return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "accounting service error", nil)
			}
			if iconResp == nil || !iconResp.Success {
				return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "update icon failed", nil)
			}
			assetItem = iconResp.Item
			hasChanges = true
		}
	}

	// 检查 precision
	if in.Precision != nil {
		precision := in.Precision.GetValue()
		if precision < 0 || precision > 30 {
			return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PRECISION", "invalid precision", map[string]string{
				"precision": "must be between 0 and 30",
			})
		}
		if precision != assetItem.Precision {
			updateAssetPrecision = precision
			needUpdateAsset = true
		}
	}

	// 统一调用 UpdateAsset 更新 name 和 precision
	if needUpdateAsset {
		updateResp, err := l.svcCtx.AccountingRpc.UpdateAsset(l.ctx, &pb.UpdateAssetRequest{
			Code:      assetCode,
			Name:      updateAssetName,
			Precision: updateAssetPrecision,
		})
		if err != nil {
			l.Logger.Errorf("call accounting UpdateAsset failed: %v", err)
			return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "accounting service error", nil)
		}
		if updateResp == nil || !updateResp.Success {
			return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "update asset failed", nil)
		}
		assetItem = updateResp.Item
		hasChanges = true
	}

	if !hasChanges {
		// 没有任何更新，返回当前数据
		l.Logger.Infof("no changes detected for asset: %s", assetCode)
	}

	// 获取最新的完整信息（如果有更新的话）
	if hasChanges {
		finalResp, err := l.svcCtx.AccountingRpc.GetAsset(l.ctx, &pb.GetAssetRequest{Code: assetCode})
		if err != nil {
			l.Logger.Errorf("call accounting GetAsset failed: %v", err)
			return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "accounting service error", nil)
		}
		if finalResp == nil || !finalResp.Success || finalResp.Item == nil {
			return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "get updated asset failed", nil)
		}
		assetItem = finalResp.Item
	}

	// 只返回资产基础信息，不包括链配置等其他信息
	return &pb.UpdateCurrencyResponse{
		Success: true,
		Message: "ok",
		Data: &pb.UpdateCurrencyData{
			Currency: &pb.CurrencyItem{
				AssetCode: assetCode,
				AssetName: func() string {
					name := strings.TrimSpace(assetItem.Name)
					if name == "" {
						return assetCode
					}
					return name
				}(),
				Status:  assetItem.Status,
				IconUrl: strings.TrimSpace(assetItem.IconUrl),
			},
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
