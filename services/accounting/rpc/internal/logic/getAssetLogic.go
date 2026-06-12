package logic

import (
	"context"
	"errors"

	"internalwallet/proto/pb"
	"internalwallet/services/accounting/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type GetAssetLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetAssetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetAssetLogic {
	return &GetAssetLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetAssetLogic) GetAsset(in *pb.GetAssetRequest) (*pb.AcctAssetResponse, error) {
	if err := requireDB(l.svcCtx); err != nil {
		return &pb.AcctAssetResponse{Success: false, Message: err.Error()}, nil
	}
	if l.svcCtx.AssetRepo == nil {
		return &pb.AcctAssetResponse{Success: false, Message: "repository not initialized"}, nil
	}
	if in == nil || normalizeAssetCode(in.Code) == "" {
		return &pb.AcctAssetResponse{Success: false, Message: "code required"}, nil
	}

	code := normalizeAssetCode(in.Code)
	m, err := l.svcCtx.AssetRepo.FindByCode(l.ctx, code)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &pb.AcctAssetResponse{Success: false, Message: "asset not found"}, nil
	}
	if err != nil {
		l.Logger.Errorf("FindByCode failed: %v", err)
		return &pb.AcctAssetResponse{Success: false, Message: "query failed"}, nil
	}
	return &pb.AcctAssetResponse{Success: true, Message: "ok", Item: toAcctAssetPB(m)}, nil
}
