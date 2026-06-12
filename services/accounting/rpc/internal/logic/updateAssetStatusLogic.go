package logic

import (
	"context"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/accounting/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateAssetStatusLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateAssetStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateAssetStatusLogic {
	return &UpdateAssetStatusLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateAssetStatusLogic) UpdateAssetStatus(in *pb.UpdateAssetStatusRequest) (*pb.AcctAssetResponse, error) {
	if err := requireDB(l.svcCtx); err != nil {
		return &pb.AcctAssetResponse{Success: false, Message: err.Error()}, nil
	}
	if l.svcCtx.AssetRepo == nil {
		return &pb.AcctAssetResponse{Success: false, Message: "repository not initialized"}, nil
	}
	if in == nil || strings.TrimSpace(in.Code) == "" {
		return &pb.AcctAssetResponse{Success: false, Message: "code required"}, nil
	}
	code := normalizeAssetCode(in.Code)
	if !isValidAssetCode(code) {
		return &pb.AcctAssetResponse{Success: false, Message: "invalid code"}, nil
	}
	if in.Status != 1 && in.Status != 2 {
		return &pb.AcctAssetResponse{Success: false, Message: "invalid status"}, nil
	}

	if err := l.svcCtx.AssetRepo.UpdateStatusByCode(l.ctx, code, in.Status); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			return &pb.AcctAssetResponse{Success: false, Message: "asset not found"}, nil
		}
		l.Logger.Errorf("UpdateStatusByCode failed: %v", err)
		return &pb.AcctAssetResponse{Success: false, Message: "update failed"}, nil
	}

	m, err := l.svcCtx.AssetRepo.FindByCode(l.ctx, code)
	if err != nil {
		l.Logger.Errorf("FindByCode after update failed: %v", err)
		return &pb.AcctAssetResponse{Success: true, Message: "ok"}, nil
	}
	return &pb.AcctAssetResponse{Success: true, Message: "ok", Item: toAcctAssetPB(m)}, nil
}
