package logic

import (
	"context"
	"time"

	"internalwallet/common/constants"
	"internalwallet/common/errcode"
	"internalwallet/proto/pb"
	"internalwallet/services/signer/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateSeedStatusLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateSeedStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateSeedStatusLogic {
	return &UpdateSeedStatusLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateSeedStatusLogic) UpdateSeedStatus(in *pb.UpdateSeedStatusRequest) (*pb.UpdateSeedStatusResponse, error) {
	// 1. 参数验证
	if in.SeedId == "" {
		return &pb.UpdateSeedStatusResponse{
			Code:    int32(errcode.SignerInvalidParams),
			Message: "seed_id is required",
		}, nil
	}

	if in.Status != constants.SeedStatusActive &&
		in.Status != constants.SeedStatusInactive &&
		in.Status != constants.SeedStatusDeprecated {
		return &pb.UpdateSeedStatusResponse{
			Code:    int32(errcode.SignerInvalidParams),
			Message: "invalid status value",
		}, nil
	}

	// 2. 查询Seed（验证存在性）
	_, err := l.svcCtx.MasterSeedRepo.FindBySeedID(l.ctx, in.SeedId)
	if err != nil {
		l.Logger.Errorf("Failed to query seed: %v", err)
		return &pb.UpdateSeedStatusResponse{
			Code:    int32(errcode.SignerSeedNotFound),
			Message: "seed not found",
		}, nil
	}

	// 3. 更新状态
	err = l.svcCtx.MasterSeedRepo.UpdateStatus(l.ctx, in.SeedId, int(in.Status))
	if err != nil {
		l.Logger.Errorf("Failed to update seed status: %v", err)
		return &pb.UpdateSeedStatusResponse{
			Code:    int32(errcode.SignerDatabaseError),
			Message: "failed to update status",
		}, nil
	}

	l.Logger.Infof("✓ Seed status updated: %s -> %s (reason: %s)",
		in.SeedId, constants.GetSeedStatusName(int(in.Status)), in.Reason)

	return &pb.UpdateSeedStatusResponse{
		Code:      0,
		Message:   "success",
		SeedId:    in.SeedId,
		Status:    in.Status,
		UpdatedAt: time.Now().Format(time.RFC3339),
	}, nil
}
