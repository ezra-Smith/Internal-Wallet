package tron

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/logx"

	"internalwallet/services/chainsync/rpc/internal/token"
)

func (t *TronProvider) GetTRC20TokenInfo(contractAddress string) (*token.Info, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client := t.createClient()
	if client == nil {
		return nil, fmt.Errorf("failed to create TRON client")
	}
	defer client.Stop()

	if err := t.connectWithTimeout(ctx, client); err != nil {
		return nil, fmt.Errorf("failed to connect to TRON client: %v", err)
	}

	tokenInfo := &token.Info{
		Address: strings.ToLower(contractAddress),
		Chain:   "tron",
	}

	if name, err := client.TRC20GetName(contractAddress); err != nil {
		logx.Errorf("Failed to get TRC20 token name for %s: %v", contractAddress, err)
		tokenInfo.Name = "Unknown TRC20 Token"
	} else {
		tokenInfo.Name = name
	}

	if symbol, err := client.TRC20GetSymbol(contractAddress); err != nil {
		logx.Errorf("Failed to get TRC20 token symbol for %s: %v", contractAddress, err)
		tokenInfo.Symbol = "UNKNOWN"
	} else {
		tokenInfo.Symbol = symbol
	}

	if decimals, err := client.TRC20GetDecimals(contractAddress); err != nil {
		logx.Errorf("Failed to get TRC20 token decimals for %s: %v", contractAddress, err)
		tokenInfo.Decimals = 18
	} else {
		tokenInfo.Decimals = uint8(decimals.Int64())
	}

	tokenInfo.IsStablecoin = isStablecoin(tokenInfo.Symbol)

	logx.Infof("Successfully queried TRC20 token info: %s (%s) with %d decimals",
		tokenInfo.Symbol, tokenInfo.Name, tokenInfo.Decimals)

	return tokenInfo, nil
}

func (t *TronProvider) CallTRC20Contract(ctx context.Context, contractAddress, functionSelector string) ([]byte, error) {
	client := t.createClient()
	if client == nil {
		return nil, fmt.Errorf("failed to create TRON client")
	}
	defer client.Stop()

	if err := t.connectWithTimeout(ctx, client); err != nil {
		return nil, fmt.Errorf("failed to connect to TRON client: %v", err)
	}

	result, err := client.TRC20Call("", contractAddress, functionSelector, true, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to call TRC20 contract %s: %v", contractAddress, err)
	}

	data := result.GetConstantResult()
	if len(data) == 0 {
		return nil, fmt.Errorf("no data returned from TRC20 contract call")
	}

	return data[0], nil
}

func isStablecoin(symbol string) bool {
	symbol = strings.ToUpper(symbol)
	stablecoins := []string{"USDT", "USDC", "USDD", "BUSD", "DAI", "TUSD", "FDUSD", "PYUSD"}
	for _, stablecoin := range stablecoins {
		if symbol == stablecoin {
			return true
		}
	}
	return false
}

