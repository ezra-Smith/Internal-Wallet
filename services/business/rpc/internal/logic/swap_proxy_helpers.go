package logic

import "internalwallet/proto/pb"

func mapSwapStatusToBiz(status pb.SwapTxStatus) pb.BusinessSwapTxStatus {
	switch status {
	case pb.SwapTxStatus_SWAP_TX_STATUS_CREATED:
		return pb.BusinessSwapTxStatus_BUSINESS_SWAP_TX_STATUS_CREATED
	case pb.SwapTxStatus_SWAP_TX_STATUS_PENDING:
		return pb.BusinessSwapTxStatus_BUSINESS_SWAP_TX_STATUS_PENDING
	case pb.SwapTxStatus_SWAP_TX_STATUS_SUCCESS:
		return pb.BusinessSwapTxStatus_BUSINESS_SWAP_TX_STATUS_SUCCESS
	case pb.SwapTxStatus_SWAP_TX_STATUS_FAILED:
		return pb.BusinessSwapTxStatus_BUSINESS_SWAP_TX_STATUS_FAILED
	default:
		return pb.BusinessSwapTxStatus_BUSINESS_SWAP_TX_STATUS_UNSPECIFIED
	}
}

func mapSwapTokenToBiz(t *pb.SwapTokenInfo) *pb.BusinessSwapTokenInfo {
	if t == nil {
		return nil
	}
	return &pb.BusinessSwapTokenInfo{
		Address:          t.Address,
		Symbol:           t.Symbol,
		Name:             t.Name,
		Decimals:         t.Decimals,
		LogoUri:          t.LogoUri,
		MinSwapAmountUsd: t.MinSwapAmountUsd,
		MaxSwapAmountUsd: t.MaxSwapAmountUsd,
	}
}

func mapSwapQuoteToBiz(q *pb.QuoteData) *pb.BusinessSwapQuoteData {
	if q == nil {
		return nil
	}
	return &pb.BusinessSwapQuoteData{
		ChainId:          q.ChainId,
		FromToken:        mapSwapTokenToBiz(q.FromToken),
		ToToken:          mapSwapTokenToBiz(q.ToToken),
		FromAmount:       q.FromAmount,
		ToAmount:         q.ToAmount,
		EstimatedGas:     q.EstimatedGas,
		Provider:         q.Provider,
		TradeFee:         q.TradeFee,
		PriceImpact:      q.PriceImpact,
		GasFeePercentage: "", // 在 swapGetQuoteLogic 中计算并设置
	}
}

func mapSwapUnsignedTxToBiz(tx *pb.UnsignedTransaction) *pb.BusinessSwapUnsignedTransaction {
	if tx == nil {
		return nil
	}
	return &pb.BusinessSwapUnsignedTransaction{
		ChainId:  tx.ChainId,
		From:     tx.From,
		To:       tx.To,
		Data:     tx.Data,
		Value:    tx.Value,
		Gas:      tx.Gas,
		GasPrice: tx.GasPrice,
	}
}

func mapSwapPaginationToBiz(p *pb.SwapPagination) *pb.BusinessSwapPagination {
	if p == nil {
		return nil
	}
	return &pb.BusinessSwapPagination{
		Page:       p.Page,
		PageSize:   p.PageSize,
		Total:      p.Total,
		TotalPages: p.TotalPages,
		HasNext:    p.HasNext,
		HasPrev:    p.HasPrev,
	}
}

func mapSwapHistoryItemToBiz(it *pb.SwapHistoryItem) *pb.BusinessSwapHistoryItem {
	if it == nil {
		return nil
	}
	return &pb.BusinessSwapHistoryItem{
		SwapId:           it.SwapId,
		ChainId:          it.ChainId,
		Provider:         it.Provider,
		WalletAddress:    it.WalletAddress,
		FromTokenAddress: it.FromTokenAddress,
		ToTokenAddress:   it.ToTokenAddress,
		FromAmount:       it.FromAmount,
		ToAmount:         it.ToAmount,
		TxHash:           it.TxHash,
		Status:           mapSwapStatusToBiz(it.Status),
		CreatedAt:        it.CreatedAt,
		UpdatedAt:        it.UpdatedAt,
	}
}

func mapSwapChainToBiz(c *pb.ChainInfo) *pb.BusinessSwapChainInfo {
	if c == nil {
		return nil
	}
	return &pb.BusinessSwapChainInfo{
		ChainId:               c.ChainId,
		Name:                  c.Name,
		Enabled:               c.Enabled,
		RequiredConfirmations: c.RequiredConfirmations,
	}
}
