package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListWeb3ChainsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListWeb3ChainsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListWeb3ChainsLogic {
	return &ListWeb3ChainsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListWeb3ChainsLogic) ListWeb3Chains(_ *pb.ListWeb3ChainsReq) (*pb.ListWeb3ChainsResp, error) {
	chains := []*pb.Web3ChainItem{}

	if l.svcCtx.ChainRepository != nil {
		rows, err := l.svcCtx.ChainRepository.ListEnabledChains(l.ctx)
		if err != nil {
			l.Errorf("查询链列表失败: %v", err)
			return nil, errx.DBError()
		}

		for _, c := range rows {
			chains = append(chains, &pb.Web3ChainItem{
				Name:        c.Name,
				Network:     c.Network,
				ChainId:     int32(c.ChainID),
				ExplorerUrl: c.ExplorerUrl,
				IconUrl:     c.IconUrl,
			})
		}
	}

	return &pb.ListWeb3ChainsResp{
		Success: true,
		Message: "ok",
		Chains:  chains,
	}, nil
}
