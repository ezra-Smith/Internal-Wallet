package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type SwapGetSupportedTokensLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSwapGetSupportedTokensLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SwapGetSupportedTokensLogic {
	return &SwapGetSupportedTokensLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SwapGetSupportedTokensLogic) SwapGetSupportedTokens(in *pb.BusinessSwapGetSupportedTokensRequest) (*pb.BusinessSwapGetSupportedTokensResponse, error) {
	if in == nil {
		return nil, errx.InvalidParam("invalid params")
	}
	if l.svcCtx.SwapRpc == nil {
		return nil, errx.SwapServiceNotAvailable()
	}
	if in.ChainId <= 0 {
		return nil, errx.InvalidChainId()
	}

	resp, err := l.svcCtx.SwapRpc.GetSupportedTokens(l.ctx, &pb.GetSupportedTokensRequest{
		ChainId: in.ChainId,
	})
	if err != nil {
		l.Errorf("swap get supported tokens failed: %v", err)
		return nil, errx.SwapServiceError()
	}
	if resp == nil {
		return nil, errx.SwapServiceError()
	}
	if !resp.Success {
		return nil, errx.Internal(resp.Message)
	}

	tokens := make([]*pb.BusinessSwapTokenInfo, 0, len(resp.Tokens))
	for _, t := range resp.Tokens {
		tokens = append(tokens, mapSwapTokenToBiz(t))
	}
	return &pb.BusinessSwapGetSupportedTokensResponse{
		Success: true,
		Message: "ok",
		ChainId: resp.ChainId,
		Tokens:  tokens,
	}, nil
}
