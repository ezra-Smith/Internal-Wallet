package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetAssetConfigLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetAssetConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetAssetConfigLogic {
	return &GetAssetConfigLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetAssetConfigLogic) GetAssetConfig(in *pb.GetAssetConfigReq) (*pb.GetAssetConfigResp, error) {
	// 调用 Accounting 服务获取所有资产配置
	if l.svcCtx.AccountingRpc == nil {
		return nil, errx.Internal("accounting service not available")
	}

	// 调用 ListAssets 获取所有资产
	listResp, err := l.svcCtx.AccountingRpc.ListAssets(l.ctx, &pb.ListAssetsRequest{})
	if err != nil {
		l.Logger.Errorf("ListAssets failed: %v", err)
		return nil, errx.Internal("failed to get asset config")
	}

	if listResp == nil || !listResp.Success {
		return &pb.GetAssetConfigResp{
			Success: false,
			Message: "failed to get assets",
		}, nil
	}

	// 转换为 AssetConfigItem 列表
	items := make([]*pb.AssetConfigItem, 0, len(listResp.Items))
	for _, asset := range listResp.Items {
		if asset == nil {
			continue
		}
		items = append(items, &pb.AssetConfigItem{
			Code:      asset.Code,
			Name:      asset.Name,
			IconUrl:   asset.IconUrl,
			Precision: asset.Precision,
			Status:    asset.Status,
			IsHot:     asset.IsHot,
		})
	}

	resp := &pb.GetAssetConfigResp{
		Success: true,
		Message: "ok",
	}
	resp.Items = items
	return resp, nil
}
