package oneinch

import (
	"fmt"
	"sort"
	"strings"

	"internalwallet/services/swap/rpc/internal/provider"
)

func adaptToken(t tokenDTO) provider.Token {
	return provider.Token{
		Address:  strings.TrimSpace(t.Address),
		Symbol:   strings.TrimSpace(t.Symbol),
		Name:     strings.TrimSpace(t.Name),
		Decimals: t.Decimals,
		LogoURI:  strings.TrimSpace(t.LogoURI),
	}
}

func AdaptQuote(chainID int64, dto *quoteResponseDTO) (*provider.QuoteResponse, error) {
	if dto == nil {
		return nil, fmt.Errorf("nil quote dto")
	}
	return &provider.QuoteResponse{
		ChainID:      chainID,
		Provider:     "1inch",
		FromToken:    adaptToken(dto.FromToken),
		ToToken:      adaptToken(dto.ToToken),
		FromAmount:   strings.TrimSpace(dto.FromAmount),
		ToAmount:     strings.TrimSpace(dto.ToAmount),
		EstimatedGas: dto.EstimatedGas,
		RawProtocols: dto.Protocols,
		TradeFee:     "", // 1inch API 不提供交易手续费，留空
		PriceImpact:  "", // 1inch API 不提供价格冲击，留空
	}, nil
}

func AdaptSwap(chainID int64, dto *swapResponseDTO) (*provider.SwapBuildResult, error) {
	if dto == nil {
		return nil, fmt.Errorf("nil swap dto")
	}

	quote := &provider.QuoteResponse{
		ChainID:      chainID,
		Provider:     "1inch",
		FromToken:    adaptToken(dto.FromToken),
		ToToken:      adaptToken(dto.ToToken),
		FromAmount:   strings.TrimSpace(dto.FromAmount),
		ToAmount:     strings.TrimSpace(dto.ToAmount),
		EstimatedGas: dto.EstimatedGas,
		RawProtocols: dto.Protocols,
		TradeFee:     "", // 1inch API 不提供交易手续费，留空
		PriceImpact:  "", // 1inch API 不提供价格冲击，留空
	}

	tx := provider.UnsignedTransaction{
		ChainID:  chainID,
		From:     strings.TrimSpace(dto.Tx.From),
		To:       strings.TrimSpace(dto.Tx.To),
		Data:     strings.TrimSpace(dto.Tx.Data),
		Value:    strings.TrimSpace(dto.Tx.Value),
		Gas:      strings.TrimSpace(dto.Tx.Gas.String()),
		GasPrice: strings.TrimSpace(dto.Tx.GasPrice.String()),
	}

	return &provider.SwapBuildResult{
		Provider: "1inch",
		Quote:    quote,
		SwapTx:   tx,
	}, nil
}

func AdaptApproveTx(chainID int64, from string, dto *approveTxResponseDTO) (*provider.UnsignedTransaction, error) {
	if dto == nil {
		return nil, fmt.Errorf("nil approve tx dto")
	}
	return &provider.UnsignedTransaction{
		ChainID:  chainID,
		From:     strings.TrimSpace(from),
		To:       strings.TrimSpace(dto.To),
		Data:     strings.TrimSpace(dto.Data),
		Value:    strings.TrimSpace(dto.Value),
		Gas:      strings.TrimSpace(dto.Gas.String()),
		GasPrice: strings.TrimSpace(dto.GasPrice.String()),
	}, nil
}

func AdaptTokens(dto *tokensResponseDTO) ([]provider.Token, error) {
	if dto == nil {
		return nil, fmt.Errorf("nil tokens dto")
	}
	if len(dto.Tokens) == 0 {
		return []provider.Token{}, nil
	}
	out := make([]provider.Token, 0, len(dto.Tokens))
	for _, t := range dto.Tokens {
		out = append(out, adaptToken(t))
	}

	// Make output stable.
	sort.Slice(out, func(i, j int) bool {
		if out[i].Symbol == out[j].Symbol {
			return out[i].Address < out[j].Address
		}
		return out[i].Symbol < out[j].Symbol
	})

	return out, nil
}
