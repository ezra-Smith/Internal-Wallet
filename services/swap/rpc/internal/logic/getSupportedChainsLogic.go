package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/swap/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetSupportedChainsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetSupportedChainsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetSupportedChainsLogic {
	return &GetSupportedChainsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetSupportedChainsLogic) GetSupportedChains(in *pb.GetSupportedChainsRequest) (*pb.GetSupportedChainsResponse, error) {
	l.Infof("=== [SWAP] GetSupportedChains START ===")

	if l.svcCtx.SwapConfigRepo == nil {
		l.Errorf("SwapConfigRepo not initialized")
		return nil, status.Error(codes.Internal, "database not available")
	}

	// 从数据库获取所有已启用的配置
	configs, err := l.svcCtx.SwapConfigRepo.ListAllEnabled(l.ctx)
	if err != nil {
		l.Errorf("Failed to get enabled swap configs: %v", err)
		return nil, status.Error(codes.Internal, "failed to get supported chains")
	}

	// 使用 map 进行链去重，key 为 chain_id
	chainMap := make(map[int64]*pb.ChainInfo)
	for _, cfg := range configs {
		// 如果该链还没有添加，则添加到 map 中
		if _, exists := chainMap[cfg.ChainID]; !exists {
			chainMap[cfg.ChainID] = &pb.ChainInfo{
				ChainId: cfg.ChainID,
				Name:    cfg.ChainName,
				Enabled: true,
			}
		}
	}

	// 将 map 转换为切片
	chains := make([]*pb.ChainInfo, 0, len(chainMap))
	for _, chain := range chainMap {
		chains = append(chains, chain)
	}

	l.Infof("=== [SWAP] Total chains returned: %d ===", len(chains))
	l.Infof("=== [SWAP] GetSupportedChains END (success) ===")

	return &pb.GetSupportedChainsResponse{
		Success: true,
		Message: "ok",
		Chains:  chains,
	}, nil

	//configs, err := l.svcCtx.Provider.GetSupportedChains(l.ctx)
	//if err != nil {
	//	l.Errorf("")
	//}
	//
	//// 使用 map 进行链去重，key 为 chain_id
	//chainMap := make(map[int64]*pb.ChainInfo)
	//for _, cfg := range configs {
	//	// 如果该链还没有添加，则添加到 map 中
	//	if _, exists := chainMap[cfg.ChainID]; !exists {
	//		chainMap[cfg.ChainID] = &pb.ChainInfo{
	//			ChainId: cfg.ChainID,
	//			Name:    cfg.Name,
	//			Enabled: true,
	//		}
	//	}
	//}
	//
	//// 将 map 转换为切片
	//chains := make([]*pb.ChainInfo, 0, len(chainMap))
	//for _, chain := range chainMap {
	//	chains = append(chains, chain)
	//}
	//
	//l.Infof("=== [SWAP] Total chains returned: %d ===", len(chains))
	//l.Infof("=== [SWAP] GetSupportedChains END (success) ===")
	//return &pb.GetSupportedChainsResponse{
	//	Success: true,
	//	Message: "ok",
	//	Chains:  chains,
	//}, nil
}
