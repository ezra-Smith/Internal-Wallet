package logic

import (
	"context"

	"internalwallet/common/errcode"
	"internalwallet/proto/pb"
	"internalwallet/services/signer/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListSeedsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListSeedsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListSeedsLogic {
	return &ListSeedsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListSeedsLogic) ListSeeds(in *pb.ListSeedsRequest) (*pb.ListSeedsResponse, error) {
	// 调用Repository查询Seeds
	seeds, err := l.svcCtx.MasterSeedRepo.List(l.ctx, int(in.Status))
	if err != nil {
		l.Logger.Errorf("Failed to query seeds: %v", err)
		return &pb.ListSeedsResponse{
			Code:    int32(errcode.SignerDatabaseError),
			Message: "failed to query seeds",
		}, nil
	}

	// 转换为响应格式
	seedInfos := make([]*pb.SeedInfo, 0, len(seeds))
	for _, seed := range seeds {
		info := &pb.SeedInfo{
			SeedId:          seed.SeedID,
			SeedName:        seed.SeedName,
			SupportedChains: seed.SupportedChains,
			Temperature:     int32(seed.Temperature),
			Status:          int32(seed.Status),
			TotalSignatures: seed.TotalSignatures,
			CreatedAt:       seed.CreatedAt.Format("2006-01-02 15:04:05"),
		}

		if seed.LastSignatureAt != nil {
			info.LastSignatureAt = seed.LastSignatureAt.Format("2006-01-02 15:04:05")
		}

		seedInfos = append(seedInfos, info)
	}

	return &pb.ListSeedsResponse{
		Code:    0,
		Message: "success",
		Seeds:   seedInfos,
	}, nil
}
