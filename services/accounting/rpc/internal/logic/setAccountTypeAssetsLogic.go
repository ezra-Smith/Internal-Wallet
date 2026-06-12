package logic

import (
	"context"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/accounting/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type SetAccountTypeAssetsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSetAccountTypeAssetsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SetAccountTypeAssetsLogic {
	return &SetAccountTypeAssetsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SetAccountTypeAssetsLogic) SetAccountTypeAssets(in *pb.SetAccountTypeAssetsRequest) (*pb.Empty, error) {
	if err := requireDB(l.svcCtx); err != nil {
		return &pb.Empty{}, nil
	}
	if l.svcCtx.AccountTypeRepo == nil {
		return &pb.Empty{}, nil
	}
	if in == nil {
		return &pb.Empty{}, nil
	}
	code := strings.ToUpper(strings.TrimSpace(in.AccountTypeCode))
	if code == "" {
		return &pb.Empty{}, nil
	}
	if err := l.svcCtx.AccountTypeRepo.ReplaceAssets(l.ctx, code, in.AssetCodes); err != nil {
		l.Logger.Errorf("SetAccountTypeAssets failed: %v", err)
	}
	return &pb.Empty{}, nil
}
