package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/accounting/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListAccountTypesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListAccountTypesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListAccountTypesLogic {
	return &ListAccountTypesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListAccountTypesLogic) ListAccountTypes(in *pb.ListAccountTypesRequest) (*pb.ListAccountTypesResponse, error) {
	if err := requireDB(l.svcCtx); err != nil {
		return &pb.ListAccountTypesResponse{}, nil
	}
	if l.svcCtx.AccountTypeRepo == nil {
		return &pb.ListAccountTypesResponse{}, nil
	}
	includeDeleted := false
	if in != nil {
		includeDeleted = in.IncludeDeleted
	}
	items, err := l.svcCtx.AccountTypeRepo.List(l.ctx, includeDeleted)
	if err != nil {
		l.Logger.Errorf("ListAccountTypes failed: %v", err)
		return &pb.ListAccountTypesResponse{}, nil
	}
	out := make([]*pb.AcctAccountType, 0, len(items))
	for _, it := range items {
		assets, _ := l.svcCtx.AccountTypeRepo.ListAssets(l.ctx, it.Code)
		out = append(out, &pb.AcctAccountType{
			Code:        it.Code,
			Name:        it.Name,
			Description: it.Description,
			NormalSide:  normalSideFromString(it.NormalSide),
			AssetCodes:  assets,
		})
	}
	return &pb.ListAccountTypesResponse{Items: out}, nil
}
