package logic

import (
	"context"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/accounting/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateAssetLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateAssetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateAssetLogic {
	return &UpdateAssetLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateAssetLogic) UpdateAsset(in *pb.UpdateAssetRequest) (*pb.AcctAssetResponse, error) {
	// 验证基础依赖
	if err := requireDB(l.svcCtx); err != nil {
		return &pb.AcctAssetResponse{Success: false, Message: err.Error()}, nil
	}
	if l.svcCtx.AssetRepo == nil {
		return &pb.AcctAssetResponse{Success: false, Message: "repository not initialized"}, nil
	}

	// 验证必填参数
	if in == nil || strings.TrimSpace(in.Code) == "" {
		return &pb.AcctAssetResponse{Success: false, Message: "code required"}, nil
	}

	code := normalizeAssetCode(in.Code)
	if !isValidAssetCode(code) {
		return &pb.AcctAssetResponse{Success: false, Message: "invalid code"}, nil
	}

	// 验证资产是否存在
	existing, err := l.svcCtx.AssetRepo.FindByCode(l.ctx, code)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			return &pb.AcctAssetResponse{Success: false, Message: "asset not found"}, nil
		}
		l.Logger.Errorf("FindByCode failed: %v", err)
		return &pb.AcctAssetResponse{Success: false, Message: "query failed"}, nil
	}
	if existing == nil {
		return &pb.AcctAssetResponse{Success: false, Message: "asset not found"}, nil
	}

	// 关键检查：只有禁用状态（status=2）才能修改基础信息
	if existing.Status != 2 {
		l.Logger.Errorf("asset must be disabled to update: code=%s, current_status=%d", code, existing.Status)
		return &pb.AcctAssetResponse{Success: false, Message: "asset must be disabled (status=2) to update"}, nil
	}

	// 验证可选参数
	name := strings.TrimSpace(in.Name)
	if name != "" && len(name) > 64 {
		return &pb.AcctAssetResponse{Success: false, Message: "invalid name: too long"}, nil
	}

	precision := in.Precision
	if precision < 0 || precision > 30 {
		return &pb.AcctAssetResponse{Success: false, Message: "invalid precision: must be between 0 and 30"}, nil
	}

	// 检查是否有实际需要更新的字段
	hasChanges := false
	if name != "" && name != existing.Name {
		hasChanges = true
	}
	if precision > 0 && precision != existing.Precision {
		hasChanges = true
	}

	if !hasChanges {
		// 没有任何变更，直接返回当前数据
		l.Logger.Infof("no changes detected for asset: %s", code)
		return &pb.AcctAssetResponse{Success: true, Message: "ok", Item: toAcctAssetPB(existing)}, nil
	}

	// 执行更新
	if err := l.svcCtx.AssetRepo.UpdateByCode(l.ctx, code, name, precision); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			return &pb.AcctAssetResponse{Success: false, Message: "asset not found"}, nil
		}
		l.Logger.Errorf("UpdateByCode failed: %v", err)
		return &pb.AcctAssetResponse{Success: false, Message: "update failed"}, nil
	}

	// 获取更新后的最新数据
	updated, err := l.svcCtx.AssetRepo.FindByCode(l.ctx, code)
	if err != nil {
		l.Logger.Errorf("FindByCode after update failed: %v", err)
		return &pb.AcctAssetResponse{Success: true, Message: "ok"}, nil
	}

	return &pb.AcctAssetResponse{Success: true, Message: "ok", Item: toAcctAssetPB(updated)}, nil
}
