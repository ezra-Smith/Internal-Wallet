package logic

import (
	"context"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/accounting/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateAssetIconLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateAssetIconLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateAssetIconLogic {
	return &UpdateAssetIconLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateAssetIconLogic) UpdateAssetIcon(in *pb.UpdateAssetIconRequest) (*pb.AcctAssetResponse, error) {
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
	iconURL := strings.TrimSpace(in.IconUrl)
	if err := validateHTTPIconURL(iconURL); err != nil {
		return &pb.AcctAssetResponse{Success: false, Message: err.Error()}, nil
	}

	if err := l.svcCtx.AssetRepo.UpdateIconURLByCode(l.ctx, code, iconURL); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			return &pb.AcctAssetResponse{Success: false, Message: "asset not found"}, nil
		}
		l.Logger.Errorf("UpdateIconURLByCode failed: %v", err)
		return &pb.AcctAssetResponse{Success: false, Message: "update failed"}, nil
	}

	m, err := l.svcCtx.AssetRepo.FindByCode(l.ctx, code)
	if err != nil {
		l.Logger.Errorf("FindByCode after update failed: %v", err)
		return &pb.AcctAssetResponse{Success: true, Message: "ok"}, nil
	}
	return &pb.AcctAssetResponse{Success: true, Message: "ok", Item: toAcctAssetPB(m)}, nil
}
