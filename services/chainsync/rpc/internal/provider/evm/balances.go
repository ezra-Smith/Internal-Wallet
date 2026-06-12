package evm

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"internalwallet/proto/pb"
)

func (p *Web3Provider) GetAddressBalance(ctx context.Context, address string, tokens []string) (*pb.GetAddressBalanceResp, error) {
	start := time.Now()
	var err error
	defer p.finishRequest(start, &err)

	_ = tokens // TODO: token balances

	balance, err := p.client.BalanceAt(ctx, common.HexToAddress(address), nil)
	if err != nil {
		err = fmt.Errorf("failed to get balance for address %s: %v", address, err)
		return nil, err
	}

	ethBalance := new(big.Float).Quo(new(big.Float).SetInt(balance), big.NewFloat(1e18))
	formattedBalance := ethBalance.String()

	return &pb.GetAddressBalanceResp{
		Success:                true,
		NativeBalance:          balance.String(),
		FormattedNativeBalance: formattedBalance,
		TokenBalances:          []*pb.TokenBalance{},
		TotalUsdValue:          0,
		LastUpdated:            time.Now().Unix(),
	}, nil
}
