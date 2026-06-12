package okx

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"internalwallet/services/swap/rpc/internal/provider"
)

func adaptToken(t tokenDTO) provider.Token {
	decimals := strings.TrimSpace(t.Decimal.String())
	if decimals == "" {
		decimals = strings.TrimSpace(t.Decimals.String())
	}
	return provider.Token{
		Address:  strings.TrimSpace(t.TokenContractAddress),
		Symbol:   strings.TrimSpace(t.TokenSymbol),
		Name:     strings.TrimSpace(t.TokenName),
		Decimals: parseDecimals(decimals),
		LogoURI:  strings.TrimSpace(t.TokenLogoUrl),
	}
}

func parseDecimals(s string) uint32 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	n, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return 0
	}
	return uint32(n)
}

func parseUint64(s string) uint64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0
	}
	return n
}

func parseUint64FromSON(sn stringOrNumber) int64 {
	s := strings.TrimSpace(sn.String())
	if s == "" {
		return 0
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return n
}

func AdaptQuote(chainID int64, fromAddr string, toAddr string, dto *quoteDataDTO) (*provider.QuoteResponse, error) {
	if dto == nil {
		return nil, fmt.Errorf("nil quote dto")
	}

	from := provider.Token{Address: strings.TrimSpace(fromAddr)}
	to := provider.Token{Address: strings.TrimSpace(toAddr)}

	// Use top-level token info if available
	if dto.FromToken.TokenContractAddress != "" {
		from = adaptQuoteToken(dto.FromToken)
	}
	if dto.ToToken.TokenContractAddress != "" {
		to = adaptQuoteToken(dto.ToToken)
	}

	estimatedGas := parseUint64(dto.EstimateGasFee.String())
	if estimatedGas == 0 {
		// Some responses only provide gas limit in swap response; keep 0 here.
		estimatedGas = 0
	}

	// 提取价格冲击百分比（例如 "0.5" 表示 0.5%）
	priceImpact := strings.TrimSpace(dto.PriceImpactPercent)

	// 提取交易手续费（USD 计价）
	tradeFee := strings.TrimSpace(dto.TradeFee)

	return &provider.QuoteResponse{
		ChainID:      chainID,
		Provider:     "okx",
		FromToken:    from,
		ToToken:      to,
		FromAmount:   strings.TrimSpace(dto.FromTokenAmount),
		ToAmount:     strings.TrimSpace(dto.ToTokenAmount),
		EstimatedGas: estimatedGas,
		RawProtocols: dto.DexRouterList,
		TradeFee:     tradeFee,
		PriceImpact:  priceImpact,
	}, nil
}

// adaptDexRouterToken converts dexRouterTokenDTO to provider.Token
func adaptDexRouterToken(t dexRouterTokenDTO) provider.Token {
	return provider.Token{
		Address:  strings.TrimSpace(t.TokenContractAddress),
		Symbol:   strings.TrimSpace(t.TokenSymbol),
		Decimals: parseDecimals(t.Decimal.String()),
	}
}

// adaptQuoteToken converts quoteTokenDTO to provider.Token
func adaptQuoteToken(t quoteTokenDTO) provider.Token {
	return provider.Token{
		Address:  strings.TrimSpace(t.TokenContractAddress),
		Symbol:   strings.TrimSpace(t.TokenSymbol),
		Decimals: parseDecimals(t.Decimal.String()),
	}
}

func AdaptSwap(chainID int64, dto *swapDataDTO) (*provider.SwapBuildResult, error) {
	if dto == nil {
		return nil, fmt.Errorf("nil swap dto")
	}

	quote := &provider.QuoteResponse{
		ChainID:      chainID,
		Provider:     "okx",
		FromToken:    adaptToken(dto.RouterResult.FromToken),
		ToToken:      adaptToken(dto.RouterResult.ToToken),
		FromAmount:   strings.TrimSpace(dto.RouterResult.FromTokenAmount),
		ToAmount:     strings.TrimSpace(dto.RouterResult.ToTokenAmount),
		EstimatedGas: 0,
		RawProtocols: dto.RouterResult.DexRouterList,
		TradeFee:     strings.TrimSpace(dto.RouterResult.TradeFee),
	}

	gas := strings.TrimSpace(dto.Tx.Gas.String())
	gasPrice := strings.TrimSpace(dto.Tx.GasPrice.String())
	if gasPrice == "" {
		// Best-effort for EIP-1559 responses.
		gasPrice = strings.TrimSpace(dto.Tx.MaxPriorityFeePerGas.String())
	}

	quote.EstimatedGas = parseUint64(gas)

	// 处理 signatureData（OKX 特定：某些交易需要额外的签名数据）
	// 注意：txDTO 中目前没有 SignatureData 字段，所以设置为空数组
	signatureData := make([]string, 0)

	tx := provider.UnsignedTransaction{
		ChainID:         chainID,
		From:            strings.TrimSpace(dto.Tx.From),
		To:              strings.TrimSpace(dto.Tx.To),
		Data:            strings.TrimSpace(dto.Tx.Data),
		Value:           strings.TrimSpace(dto.Tx.Value.String()),
		Gas:             gas,
		GasPrice:        gasPrice,
		SlippagePercent: dto.Tx.SlippagePercent,
		SignatureData:   signatureData,
	}

	return &provider.SwapBuildResult{
		Provider: "okx",
		Quote:    quote,
		SwapTx:   tx,
	}, nil
}

func AdaptApproveTx(chainID int64, from string, tokenAddress string, dto *approveTxDataDTO) (*provider.UnsignedTransaction, error) {
	if dto == nil {
		return nil, fmt.Errorf("nil approve dto")
	}
	// ERC20 approve 交易应该发送到代币合约地址
	// Data 字段中已经包含了 approve(spender, amount) 的调用
	// 其中 spender 参数是 dexContractAddress（DEX Router）
	return &provider.UnsignedTransaction{
		ChainID:  chainID,
		From:     strings.TrimSpace(from),
		To:       strings.TrimSpace(tokenAddress),
		Data:     strings.TrimSpace(dto.Data),
		Value:    "0",
		Gas:      strings.TrimSpace(dto.GasLimit.String()),
		GasPrice: strings.TrimSpace(dto.GasPrice.String()),
	}, nil
}

func AdaptTokens(tokens []tokenDTO) ([]provider.Token, error) {
	if len(tokens) == 0 {
		return []provider.Token{}, nil
	}

	out := make([]provider.Token, 0, len(tokens))
	for _, t := range tokens {
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

// ============================================================================
// New Adapter Functions for OKX DEX API
// ============================================================================

// AdaptLiquiditySources converts liquidity source DTOs to provider type
func AdaptLiquiditySources(dtos []liquiditySourceDTO) []provider.LiquiditySource {
	if len(dtos) == 0 {
		return []provider.LiquiditySource{}
	}

	out := make([]provider.LiquiditySource, 0, len(dtos))
	for _, dto := range dtos {
		out = append(out, provider.LiquiditySource{
			ID:   strings.TrimSpace(dto.ID),
			Name: strings.TrimSpace(dto.Name),
			Logo: strings.TrimSpace(dto.Logo),
		})
	}

	// Make output stable.
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})

	return out
}

// AdaptSwapHistory converts swap history DTO to provider type
func AdaptSwapHistory(dto *swapHistoryDTO) (*provider.ChainSwapHistoryItem, error) {
	if dto == nil {
		return nil, fmt.Errorf("nil swap history dto")
	}

	// Parse status string to ChainSwapStatus
	status := provider.ChainSwapStatusPending
	switch strings.ToLower(strings.TrimSpace(dto.Status)) {
	case "success":
		status = provider.ChainSwapStatusSuccess
	case "fail", "failed":
		status = provider.ChainSwapStatusFailed
	case "pending":
		status = provider.ChainSwapStatusPending
	default:
		status = provider.ChainSwapStatusUnspecified
	}

	// Adapt token details (FromTokenDetails and ToTokenDetails are single objects, not arrays)
	fromTokens := make([]provider.TokenDetail, 0, 1)
	if dto.FromTokenDetails.TokenAddress != "" {
		fromTokens = append(fromTokens, provider.TokenDetail{
			Symbol:       strings.TrimSpace(dto.FromTokenDetails.Symbol),
			Amount:       strings.TrimSpace(dto.FromTokenDetails.Amount),
			TokenAddress: strings.TrimSpace(dto.FromTokenDetails.TokenAddress),
		})
	}

	toTokens := make([]provider.TokenDetail, 0, 1)
	if dto.ToTokenDetails.TokenAddress != "" {
		toTokens = append(toTokens, provider.TokenDetail{
			Symbol:       strings.TrimSpace(dto.ToTokenDetails.Symbol),
			Amount:       strings.TrimSpace(dto.ToTokenDetails.Amount),
			TokenAddress: strings.TrimSpace(dto.ToTokenDetails.TokenAddress),
		})
	}

	return &provider.ChainSwapHistoryItem{
		ChainIndex:     strings.TrimSpace(dto.ChainIndex),
		TxHash:         strings.TrimSpace(dto.TxHash),
		BlockHeight:    strings.TrimSpace(dto.Height),
		TxTime:         parseUint64FromSON(dto.TxTime),
		Status:         status,
		TxType:         strings.TrimSpace(dto.TxType),
		FromAddress:    strings.TrimSpace(dto.FromAddress),
		DexRouter:      strings.TrimSpace(dto.DexRouter),
		ToAddress:      strings.TrimSpace(dto.ToAddress),
		FromTokens:     fromTokens,
		ToTokens:       toTokens,
		ReferralAmount: strings.TrimSpace(dto.ReferralAmount),
		ErrorMsg:       strings.TrimSpace(dto.ErrorMsg),
		GasLimit:       strings.TrimSpace(dto.GasLimit.String()),
		GasUsed:        strings.TrimSpace(dto.GasUsed.String()),
		GasPrice:       strings.TrimSpace(dto.GasPrice.String()),
		TxFee:          strings.TrimSpace(dto.TxFee),
	}, nil
}
