package selfhosted

import (
	"context"
	"fmt"

	"internalwallet/proto/pb"
)

func (p *SelfHostedProvider) GetBlockTransactions(ctx context.Context, blockNumber uint64) ([]*pb.Transaction, error) {
	switch p.config.Chain {
	case pb.BlockChainType_CHAIN_TYPE_ETHEREUM, pb.BlockChainType_CHAIN_TYPE_BSC:
		return p.getEthLikeBlockTransactions(ctx, blockNumber)
	case pb.BlockChainType_CHAIN_TYPE_TRON:
		return p.getTronBlockTransactions(ctx, blockNumber)
	default:
		return nil, fmt.Errorf("unsupported chain type: %v", p.config.Chain)
	}
}

