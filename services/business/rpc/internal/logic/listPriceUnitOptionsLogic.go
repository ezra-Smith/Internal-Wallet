package logic

import (
	"context"
	"github.com/zeromicro/go-zero/core/logx"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/svc"
)

type ListPriceUnitOptionsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListPriceUnitOptionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListPriceUnitOptionsLogic {
	return &ListPriceUnitOptionsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListPriceUnitOptionsLogic) ListPriceUnitOptions(in *pb.ListPriceUnitOptionsReq) (*pb.ListPriceUnitOptionsResp, error) {
	rows, err := l.svcCtx.PriceUnitRepository.ListEnabledUnits(l.ctx)
	if err != nil {
		l.Errorf("list enabled price_unit failed: %v", err)
		return nil, err
	}
	var items []*pb.PriceUnitItem
	for _, r := range rows {
		items = append(items, &pb.PriceUnitItem{
			Code: r.ID,
			Name: r.Name,
		})
	}
	return &pb.ListPriceUnitOptionsResp{Success: true, Items: items}, nil
}
