package logic

import "internalwallet/proto/pb"

func mapSwapPagination(p *pb.SwapPagination) *pb.Pagination {
	if p == nil {
		return nil
	}
	return &pb.Pagination{
		Page:       p.Page,
		PageSize:   p.PageSize,
		Total:      p.Total,
		TotalPages: p.TotalPages,
		HasNext:    p.HasNext,
		HasPrev:    p.HasPrev,
	}
}

func mapSwapConfigItem(it *pb.SwapConfigItem) *pb.AdminSwapConfigItem {
	if it == nil {
		return nil
	}
	return &pb.AdminSwapConfigItem{
		Id:               it.Id,
		ProviderId:       it.ProviderId,
		Provider:         it.Provider,
		ProviderName:     it.ProviderName,
		ChainId:          it.ChainId,
		ChainName:        it.ChainName,
		ChainSymbol:      it.ChainSymbol,
		TokenSymbol:      it.TokenSymbol,
		TokenName:        it.TokenName,
		ContractAddress:  it.ContractAddress,
		Decimals:         it.Decimals,
		IconUrl:          it.IconUrl,
		IsEnabled:        it.IsEnabled,
		Priority:         it.Priority,
		RouterAddress:    it.RouterAddress,
		MinSwapAmountUsd: it.MinSwapAmountUsd,
		MaxSwapAmountUsd: it.MaxSwapAmountUsd,
		DefaultSlippage:  it.DefaultSlippage,
		MaxSlippage:      it.MaxSlippage,
		FeeRate:          it.FeeRate,
		ConfigJson:       it.ConfigJson,
		CreatedAt:        it.CreatedAt,
		UpdatedAt:        it.UpdatedAt,
	}
}

func mapSwapConfigItems(items []*pb.SwapConfigItem) []*pb.AdminSwapConfigItem {
	if len(items) == 0 {
		return nil
	}
	out := make([]*pb.AdminSwapConfigItem, 0, len(items))
	for _, it := range items {
		out = append(out, mapSwapConfigItem(it))
	}
	return out
}

func mapProjectSwapConfigItem(it *pb.ProjectSwapConfigItem) *pb.AdminProjectSwapConfigItem {
	if it == nil {
		return nil
	}
	return &pb.AdminProjectSwapConfigItem{
		EnabledId:        it.EnabledId,
		ProjectName:      it.ProjectName,
		ConfigId:         it.ConfigId,
		ProjectEnabled:   it.ProjectEnabled,
		ProviderId:       it.ProviderId,
		Provider:         it.Provider,
		ProviderName:     it.ProviderName,
		ChainId:          it.ChainId,
		ChainName:        it.ChainName,
		ChainSymbol:      it.ChainSymbol,
		TokenSymbol:      it.TokenSymbol,
		TokenName:        it.TokenName,
		ContractAddress:  it.ContractAddress,
		Decimals:         it.Decimals,
		IconUrl:          it.IconUrl,
		GlobalEnabled:    it.GlobalEnabled,
		Priority:         it.Priority,
		RouterAddress:    it.RouterAddress,
		MinSwapAmountUsd: it.MinSwapAmountUsd,
		MaxSwapAmountUsd: it.MaxSwapAmountUsd,
		DefaultSlippage:  it.DefaultSlippage,
		MaxSlippage:      it.MaxSlippage,
		FeeRate:          it.FeeRate,
		CreatedAt:        it.CreatedAt,
		UpdatedAt:        it.UpdatedAt,
	}
}

func mapProjectSwapConfigItems(items []*pb.ProjectSwapConfigItem) []*pb.AdminProjectSwapConfigItem {
	if len(items) == 0 {
		return nil
	}
	out := make([]*pb.AdminProjectSwapConfigItem, 0, len(items))
	for _, it := range items {
		out = append(out, mapProjectSwapConfigItem(it))
	}
	return out
}

func mapSwapTransactionItem(it *pb.SwapTransactionItem) *pb.AdminSwapTransactionItem {
	if it == nil {
		return nil
	}
	return &pb.AdminSwapTransactionItem{
		Id:                it.Id,
		ProjectName:       it.ProjectName,
		WalletAddress:     it.WalletAddress,
		ChainId:           it.ChainId,
		ChainName:         it.ChainName,
		Provider:          it.Provider,
		FromTokenSymbol:   it.FromTokenSymbol,
		FromTokenAddress:  it.FromTokenAddress,
		FromTokenDecimals: it.FromTokenDecimals,
		ToTokenSymbol:     it.ToTokenSymbol,
		ToTokenAddress:    it.ToTokenAddress,
		ToTokenDecimals:   it.ToTokenDecimals,
		FromAmount:        it.FromAmount,
		ToAmount:          it.ToAmount,
		TxHash:            it.TxHash,
		Status:            it.Status,
		FeeUsd:            it.FeeUsd,
		Gas:               it.Gas,
		CreatedAt:         it.CreatedAt,
		BroadcastedAt:     it.BroadcastedAt,
	}
}

func mapSwapTransactionItems(items []*pb.SwapTransactionItem) []*pb.AdminSwapTransactionItem {
	if len(items) == 0 {
		return nil
	}
	out := make([]*pb.AdminSwapTransactionItem, 0, len(items))
	for _, it := range items {
		out = append(out, mapSwapTransactionItem(it))
	}
	return out
}
