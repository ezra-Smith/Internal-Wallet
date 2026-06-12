package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/accounting/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetAssetOverviewLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetAssetOverviewLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetAssetOverviewLogic {
	return &GetAssetOverviewLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ==================== Assets (source of truth) ====================
func (l *GetAssetOverviewLogic) GetAssetOverview(in *pb.Empty) (*pb.AcctAssetOverviewResponse, error) {
	_ = in
	if err := requireDB(l.svcCtx); err != nil {
		return &pb.AcctAssetOverviewResponse{Success: false, Message: err.Error()}, nil
	}
	if l.svcCtx.AssetRepo == nil {
		return &pb.AcctAssetOverviewResponse{Success: false, Message: "repository not initialized"}, nil
	}

	total, enabled, disabled, err := l.svcCtx.AssetRepo.CountOverview(l.ctx)
	if err != nil {
		l.Logger.Errorf("CountOverview failed: %v", err)
		return &pb.AcctAssetOverviewResponse{Success: false, Message: "query failed"}, nil
	}
	return &pb.AcctAssetOverviewResponse{
		Success:  true,
		Message:  "ok",
		Total:    total,
		Enabled:  enabled,
		Disabled: disabled,
	}, nil
}
