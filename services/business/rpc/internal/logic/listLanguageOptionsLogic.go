package logic

import (
	"context"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListLanguageOptionsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListLanguageOptionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListLanguageOptionsLogic {
	return &ListLanguageOptionsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListLanguageOptionsLogic) ListLanguageOptions(in *pb.ListLanguageOptionsReq) (*pb.ListLanguageOptionsResp, error) {
	rows, err := l.svcCtx.LanguageRepository.ListEnabledLanguages(l.ctx)
	if err != nil {
		l.Errorf("list enabled languages failed: %v", err)
		return nil, err
	}
	var items []*pb.LanguageItem
	for _, r := range rows {
		items = append(items, &pb.LanguageItem{
			Code: r.ID,
			Name: r.Name,
		})
	}
	return &pb.ListLanguageOptionsResp{Success: true, Items: items}, nil
}
