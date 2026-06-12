package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/swap/rpc/internal/provider"
	"internalwallet/services/swap/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetLiquiditySourcesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetLiquiditySourcesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetLiquiditySourcesLogic {
	return &GetLiquiditySourcesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetLiquiditySourcesLogic) GetLiquiditySources(in *pb.GetLiquiditySourcesRequest) (*pb.GetLiquiditySourcesResponse, error) {
	if in == nil || in.ChainId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "chain_id is required")
	}
	if l.svcCtx.Provider == nil {
		return nil, status.Error(codes.Internal, "swap provider not ready")
	}

	// Check if provider supports extended methods
	extProvider, ok := l.svcCtx.Provider.(provider.ExtendedProvider)
	if !ok {
		return nil, status.Error(codes.Unimplemented, "provider does not support liquidity sources")
	}

	sources, err := extProvider.GetLiquiditySources(l.ctx, in.ChainId)
	if err != nil {
		return nil, err
	}

	items := make([]*pb.LiquiditySource, 0, len(sources))
	for _, src := range sources {
		items = append(items, &pb.LiquiditySource{
			Id:   src.ID,
			Name: src.Name,
			Logo: src.Logo,
		})
	}

	return &pb.GetLiquiditySourcesResponse{
		Success:          true,
		Message:          "ok",
		LiquiditySources: items,
	}, nil
}
