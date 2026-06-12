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

type GetSupportedTokensLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetSupportedTokensLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetSupportedTokensLogic {
	return &GetSupportedTokensLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetSupportedTokensLogic) GetSupportedTokens(in *pb.GetSupportedTokensRequest) (*pb.GetSupportedTokensResponse, error) {
	if in == nil || in.ChainId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "chain_id is required")
	}
	if l.svcCtx.SwapConfigRepo == nil {
		l.Errorf("SwapConfigRepo not initialized")
		return nil, status.Error(codes.Internal, "database not available")
	}

	// 从数据库获取该链所有已启用的配置
	configs, err := l.svcCtx.SwapConfigRepo.ListByChain(l.ctx, in.ChainId)
	if err != nil {
		l.Errorf("Failed to get swap configs for chain %d: %v", in.ChainId, err)
		return nil, status.Error(codes.Internal, "failed to get supported tokens")
	}

	// 将数据库模型转换为 PB 模型
	tokens := make([]*pb.SwapTokenInfo, 0, len(configs))
	for _, cfg := range configs {
		tokens = append(tokens, &pb.SwapTokenInfo{
			Address:          cfg.ContractAddress,
			Symbol:           cfg.TokenSymbol,
			Name:             cfg.TokenName,
			Decimals:         uint32(cfg.Decimals),
			LogoUri:          cfg.IconURL,
			MinSwapAmountUsd: cfg.MinSwapAmountUsd,
			MaxSwapAmountUsd: cfg.MaxSwapAmountUsd,
		})
	}

	return &pb.GetSupportedTokensResponse{
		Success: true,
		Message: "ok",
		ChainId: in.ChainId,
		Tokens:  tokens,
	}, nil

	//if in == nil || in.ChainId <= 0 {
	//	return nil, status.Error(codes.InvalidArgument, "chain_id is required")
	//}
	//if l.svcCtx.Provider == nil {
	//	return nil, status.Error(codes.Internal, "swap provider not ready")
	//}
	//
	//chainID := in.ChainId
	//cacheKey := fmt.Sprintf("tokens:%d", chainID)
	//
	//var tokens []provider.Token
	//if l.svcCtx.TokenCache != nil {
	//	if err := l.svcCtx.TokenCache.GetJSON(l.ctx, cacheKey, &tokens); err == nil && len(tokens) > 0 {
	//		return &pb.GetSupportedTokensResponse{
	//			Success: true,
	//			Message: "ok",
	//			ChainId: chainID,
	//			Tokens:  mapTokens(tokens),
	//		}, nil
	//	}
	//}
	//
	//tokens, err := l.svcCtx.Provider.GetSupportedTokens(l.ctx, chainID)
	//if err != nil {
	//	return nil, err
	//}
	//if l.svcCtx.TokenCache != nil {
	//	ttl := time.Duration(l.svcCtx.Config.Swap.Cache.TokensTTLSeconds) * time.Second
	//	if ttl <= 0 {
	//		ttl = 12 * time.Hour
	//	}
	//	_ = l.svcCtx.TokenCache.SetJSON(l.ctx, cacheKey, tokens, ttl)
	//}
	//
	//return &pb.GetSupportedTokensResponse{
	//	Success: true,
	//	Message: "ok",
	//	ChainId: chainID,
	//	Tokens:  mapTokens(tokens),
	//}, nil

}

func mapTokens(tokens []provider.Token) []*pb.SwapTokenInfo {
	out := make([]*pb.SwapTokenInfo, 0, len(tokens))
	for _, t := range tokens {
		out = append(out, providerTokenToPB(t))
	}
	return out
}
