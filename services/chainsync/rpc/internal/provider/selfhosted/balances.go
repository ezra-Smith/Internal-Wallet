package selfhosted

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"internalwallet/proto/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

func (p *SelfHostedProvider) GetAddressBalance(ctx context.Context, address string, tokens []string) (balanceResp *pb.GetAddressBalanceResp, err error) {
	start := time.Now()
	defer p.finishRequest(start, &err)

	if p.config.Chain == pb.BlockChainType_CHAIN_TYPE_TRON {
		resp, err := p.getTronAccount(ctx, address)
		if err != nil {
			return nil, fmt.Errorf("failed to get TRON balance for address %s: %v", address, err)
		}

		// 查询 TRC20 代币余额
		logx.Infof("TRON: querying %d token balances for address %s", len(tokens), address)
		tokenBalances := make([]*pb.TokenBalance, 0, len(tokens))
		for _, tokenContract := range tokens {
			if tokenContract == "" {
				continue // 跳过空合约地址（原生代币）
			}
			logx.Infof("TRON: querying TRC20 balance for contract=%s address=%s", tokenContract, address)
			balanceRaw, err := p.getTRC20Balance(ctx, tokenContract, address)
			if err != nil {
				logx.Errorf("Failed to get TRC20 balance for %s on %s: %v", tokenContract, address, err)
				continue
			}
			logx.Infof("TRON: TRC20 balance result contract=%s balance=%s", tokenContract, balanceRaw)
			if balanceRaw == "0" {
				continue
			}
			tokenBalances = append(tokenBalances, &pb.TokenBalance{
				TokenAddress:  tokenContract,
				Balance:       balanceRaw,
				TokenSymbol:   "", // 由调用方填充
				TokenDecimals: 0,  // 由调用方填充
			})
		}
		logx.Infof("TRON: returning %d token balances", len(tokenBalances))

		return &pb.GetAddressBalanceResp{
			Success:                true,
			NativeBalance:          fmt.Sprintf("%d", resp.Balance),
			FormattedNativeBalance: fmt.Sprintf("%.6f", float64(resp.Balance)/1000000),
			TokenBalances:          tokenBalances,
			TotalUsdValue:          0,
			LastUpdated:            time.Now().Unix(),
		}, nil
	}

	resp, err := p.callRPC(ctx, "eth_getBalance", []interface{}{address, "latest"})
	if err != nil {
		return nil, fmt.Errorf("failed to get balance for address %s: %v", address, err)
	}

	var balanceHex string
	if err := json.Unmarshal(resp.Result, &balanceHex); err != nil {
		return nil, fmt.Errorf("failed to unmarshal balance: %v", err)
	}

	balance, err := strconv.ParseInt(balanceHex[2:], 16, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse balance: %v", err)
	}

	balanceFloat := float64(balance) / 1e18
	formattedBalance := fmt.Sprintf("%.6f", balanceFloat)

	// 查询 ERC20 代币余额
	logx.Infof("EVM: querying %d token balances for address %s", len(tokens), address)
	tokenBalances := make([]*pb.TokenBalance, 0, len(tokens))
	for _, tokenContract := range tokens {
		if tokenContract == "" {
			continue
		}
		logx.Infof("EVM: querying ERC20 balance for contract=%s address=%s", tokenContract, address)
		balanceRaw, err := p.getERC20Balance(ctx, tokenContract, address)
		if err != nil {
			logx.Errorf("Failed to get ERC20 balance for %s on %s: %v", tokenContract, address, err)
			continue
		}
		logx.Infof("EVM: ERC20 balance result contract=%s balance=%s", tokenContract, balanceRaw)
		if balanceRaw == "0" {
			continue
		}
		tokenBalances = append(tokenBalances, &pb.TokenBalance{
			TokenAddress:  tokenContract,
			Balance:       balanceRaw,
			TokenSymbol:   "",
			TokenDecimals: 0,
		})
	}
	logx.Infof("EVM: returning %d token balances", len(tokenBalances))

	return &pb.GetAddressBalanceResp{
		Success:                true,
		NativeBalance:          balanceHex,
		FormattedNativeBalance: formattedBalance,
		TokenBalances:          tokenBalances,
		TotalUsdValue:          0,
		LastUpdated:            time.Now().Unix(),
	}, nil
}
