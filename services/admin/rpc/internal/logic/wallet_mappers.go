package logic

import (
	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/repository"
	"strings"
)

func toPBDepositAddressItem(m *model.WalletDepositAddressModel) *pb.WalletDepositAddressItem {
	if m == nil {
		return nil
	}
	memo := ""
	if m.Memo != nil {
		memo = *m.Memo
	}
	return &pb.WalletDepositAddressItem{
		Id:        m.ID,
		UserId:    m.UserID,
		ChainCode: m.ChainCode,
		Address:   m.Address,
		Status:    m.Status,
		Memo:      memo,
		CreatedAt: formatTimePtr(m.CreatedAt),
		UpdatedAt: formatTimePtr(m.UpdatedAt),
	}
}

// toPBDepositAddressWithBalanceItem 转换带余额信息的地址（用于管理后台）
func toPBDepositAddressWithBalanceItem(m *repository.DepositAddressWithBalance) *pb.WalletDepositAddressWithBalanceItem {
	if m == nil {
		return nil
	}
	memo := ""
	if m.Memo != nil {
		memo = *m.Memo
	}

	// 处理余额字段（可能为NULL）
	balance := "0"
	if m.Balance != nil {
		balance = *m.Balance
	}
	// BalanceRaw 和 BalanceUSDRaw 现在是 string (DECIMAL(65,0))，proto 也是 string，直接使用
	balanceRaw := "0"
	if m.BalanceRaw != nil {
		balanceRaw = *m.BalanceRaw
	}
	balanceUSD := "0.00"
	if m.BalanceUSD != nil {
		balanceUSD = *m.BalanceUSD
	}
	balanceUSDRaw := "0"
	if m.BalanceUSDRaw != nil {
		balanceUSDRaw = *m.BalanceUSDRaw
	}
	needsSweep := false
	if m.NeedsSweep != nil && *m.NeedsSweep == 1 {
		needsSweep = true
	}
	syncStatus := "unknown"
	if m.SyncStatus != nil {
		syncStatus = *m.SyncStatus
	}
	assetCode := ""
	if m.AssetCode != nil {
		assetCode = strings.TrimSpace(*m.AssetCode)
	}

	return &pb.WalletDepositAddressWithBalanceItem{
		Id:            m.ID,
		UserId:        m.UserID,
		AssetCode:     assetCode,
		ChainCode:     m.ChainCode,
		Address:       m.Address,
		Status:        m.Status,
		Memo:          memo,
		CreatedAt:     formatTimePtr(m.CreatedAt),
		UpdatedAt:     formatTimePtr(m.UpdatedAt),
		Balance:       balance,
		BalanceRaw:    balanceRaw,
		BalanceUsd:    balanceUSD,
		BalanceUsdRaw: balanceUSDRaw,
		NeedsSweep:    needsSweep,
		LastSyncedAt:  formatTimePtr(m.LastSyncedAt),
		SyncStatus:    syncStatus,
	}
}

func toPBDepositItem(m *model.WalletDepositModel) *pb.WalletDepositItem {
	if m == nil {
		return nil
	}
	var blockNumber int64
	if m.BlockNumber != nil {
		blockNumber = *m.BlockNumber
	}
	memo := ""
	if m.Memo != nil {
		memo = *m.Memo
	}
	var adminID int64
	if m.AdminID != nil {
		adminID = *m.AdminID
	}
	adminNote := ""
	if m.AdminNote != nil {
		adminNote = *m.AdminNote
	}
	return &pb.WalletDepositItem{
		Id:              m.ID,
		UserId:          m.UserID,
		AssetCode:       m.AssetCode,
		ChainCode:       m.ChainCode,
		DepositAddress:  m.DepositAddress,
		Amount:          m.Amount,
		TransactionHash: m.TransactionHash,
		BlockNumber:     blockNumber,
		Confirmations:   m.Confirmations,
		Status:          m.Status,
		Memo:            memo,
		AdminId:         adminID,
		AdminNote:       adminNote,
		CreatedAt:       formatTimePtr(m.CreatedAt),
		UpdatedAt:       formatTimePtr(m.UpdatedAt),
	}
}

func toPBDepositAddressBalanceItem(m *model.WalletDepositAddressBalanceModel, address string) *pb.DepositAddressBalanceItem {
	if m == nil {
		return nil
	}

	needsSweep := m.NeedsSweep == 1

	// balance_raw 等字段现在是 string (DECIMAL(65,0))，proto 也是 string，直接使用

	return &pb.DepositAddressBalanceItem{
		Id:                 m.ID,
		DepositAddressId:   m.DepositAddressID,
		UserId:             m.UserID,
		AssetCode:          m.AssetCode,
		ChainCode:          m.ChainCode,
		Address:            address,
		Balance:            m.Balance,
		BalanceRaw:         m.BalanceRaw,
		BalanceUsd:         m.BalanceUSD,
		BalanceUsdRaw:      m.BalanceUSDRaw,
		NeedsSweep:         needsSweep,
		SweepThresholdRaw:  m.SweepThresholdRaw,
		LastSweepAt:        formatTimePtr(m.LastSweepAt),
		LastSweepAmountRaw: m.LastSweepAmountRaw,
		LastSyncedAt:       formatTimePtr(m.LastSyncedAt),
		SyncStatus:         m.SyncStatus,
		CreatedAt:          formatTimePtr(m.CreatedAt),
		UpdatedAt:          formatTimePtr(m.UpdatedAt),
	}
}
