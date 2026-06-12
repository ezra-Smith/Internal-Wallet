package logic

import (
	"context"
	"fmt"

	"internalwallet/proto/pb"
	"internalwallet/services/accounting/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListAssetsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListAssetsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListAssetsLogic {
	return &ListAssetsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListAssetsLogic) ListAssets(in *pb.ListAssetsRequest) (*pb.ListAssetsResponse, error) {
	if err := requireDB(l.svcCtx); err != nil {
		return &pb.ListAssetsResponse{Success: false, Message: err.Error()}, nil
	}
	if l.svcCtx.AssetRepo == nil {
		return &pb.ListAssetsResponse{Success: false, Message: "repository not initialized"}, nil
	}
	if in == nil {
		in = &pb.ListAssetsRequest{}
	}

	status := in.Status
	if status != 0 && status != 1 && status != 2 {
		return &pb.ListAssetsResponse{Success: false, Message: "invalid status"}, nil
	}

	rows, total, err := l.svcCtx.AssetRepo.List(l.ctx, in.Page, in.PageSize, in.Keyword, status)
	if err != nil {
		l.Logger.Errorf("List assets failed: %v", err)
		return &pb.ListAssetsResponse{Success: false, Message: "query failed"}, nil
	}

	items := make([]*pb.AcctAsset, 0, len(rows))
	for i := range rows {
		row := rows[i]
		v := toAcctAssetPB(&row)
		if v != nil {
			items = append(items, v)
		}
	}

	return &pb.ListAssetsResponse{
		Success: true,
		Message: fmt.Sprintf("ok (%d)", total),
		Total:   total,
		Items:   items,
	}, nil
}
