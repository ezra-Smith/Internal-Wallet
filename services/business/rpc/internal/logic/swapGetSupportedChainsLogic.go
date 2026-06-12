package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type SwapGetSupportedChainsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSwapGetSupportedChainsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SwapGetSupportedChainsLogic {
	return &SwapGetSupportedChainsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SwapGetSupportedChainsLogic) SwapGetSupportedChains(in *pb.BusinessSwapGetSupportedChainsRequest) (*pb.BusinessSwapGetSupportedChainsResponse, error) {
	l.Infof("=== [BUSINESS] SwapGetSupportedChains START ===")
	l.Infof("=== [BUSINESS] Request: %+v ===", in)

	// 检查 SwapRpc 客户端
	l.Infof("=== [BUSINESS] Checking SwapRpc client ===")
	l.Infof("=== [BUSINESS] SwapRpc is nil: %v ===", l.svcCtx.SwapRpc == nil)
	if l.svcCtx.SwapRpc == nil {
		l.Errorf("=== [BUSINESS] ERROR: SwapRpc client is nil ===")
		return nil, errx.SwapServiceNotAvailable()
	}
	l.Infof("=== [BUSINESS] SwapRpc client type: %T ===", l.svcCtx.SwapRpc)

	// 调用 Swap RPC
	l.Infof("=== [BUSINESS] Calling SwapRpc.GetSupportedChains... ===")
	resp, err := l.svcCtx.SwapRpc.GetSupportedChains(l.ctx, &pb.GetSupportedChainsRequest{})
	if err != nil {
		l.Errorf("=== [BUSINESS] SwapRpc.GetSupportedChains FAILED: %v ===", err)
		l.Errorf("=== [BUSINESS] Error type: %T ===", err)
		return nil, errx.SwapServiceError()
	}

	l.Infof("=== [BUSINESS] SwapRpc.GetSupportedChains SUCCEEDED ===")
	l.Infof("=== [BUSINESS] Response is nil: %v ===", resp == nil)
	if resp == nil {
		l.Errorf("=== [BUSINESS] ERROR: Response is nil ===")
		return nil, errx.SwapServiceError()
	}

	l.Infof("=== [BUSINESS] Response.Success: %v ===", resp.Success)
	l.Infof("=== [BUSINESS] Response.Message: %s ===", resp.Message)
	l.Infof("=== [BUSINESS] Response.Chains count: %d ===", len(resp.Chains))

	if !resp.Success {
		l.Errorf("=== [BUSINESS] ERROR: Response.Success=false ===")
		return nil, errx.Internal(resp.Message)
	}

	chains := make([]*pb.BusinessSwapChainInfo, 0, len(resp.Chains))
	for _, c := range resp.Chains {
		l.Infof("=== [BUSINESS] Mapping chain: ChainID=%d, Name=%s ===", c.ChainId, c.Name)
		chains = append(chains, mapSwapChainToBiz(c))
	}

	l.Infof("=== [BUSINESS] Total chains to return: %d ===", len(chains))
	l.Infof("=== [BUSINESS] SwapGetSupportedChains END (success) ===")
	return &pb.BusinessSwapGetSupportedChainsResponse{
		Success: true,
		Message: "ok",
		Chains:  chains,
	}, nil
}
