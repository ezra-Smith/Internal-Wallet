package logic

import (
	"context"
	"fmt"
	"strings"
	"time"

	commonutils "internalwallet/common/utils"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/model"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetWeb3TransactionDetailLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetWeb3TransactionDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetWeb3TransactionDetailLogic {
	return &GetWeb3TransactionDetailLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetWeb3TransactionDetail 获取 Web3 交易详情（公开查询，无需认证）
func (l *GetWeb3TransactionDetailLogic) GetWeb3TransactionDetail(in *pb.GetWeb3TransactionDetailReq) (*pb.GetWeb3TransactionDetailResp, error) {
	// 验证 id
	id := in.Id
	if id <= 0 {
		return nil, errx.Web3TxIDRequired()
	}

	// 根据交易记录 ID 查询交易详情（公开查询）
	transaction, err := l.svcCtx.Web3BalanceChangeRepository.FindByID(l.ctx, id)
	if err != nil {
		l.Errorf("查询交易详情失败: id=%d, error=%v", id, err)
		return nil, errx.Web3TransactionNotFound()
	}

	// Best-effort: pending 交易尝试从链上刷新一次，避免仅依赖 chainsync/Kafka 导致状态长期不更新。
	l.tryRefreshPendingWeb3BalanceChangeFromChain(transaction)

	// 获取资产信息（图标 + 精度）
	iconURL, assetPrecision := l.getAssetIconAndPrecision(transaction.AssetCode)

	// 金额展示：优先使用 AmountRaw + (token_decimals/资产精度) 计算，避免 Amount 在极端情况下被写入 raw 值导致误展示。
	amountStr := ResolveWeb3DisplayAmount(transaction.Amount, transaction.AmountRaw, transaction.TokenDecimals, assetPrecision)
	amountDisplay := formatAmountDisplay(transaction.Direction, amountStr, transaction.AssetCode)

	// 获取状态文案（结合交易类型生成更准确的文案）
	statusText := getStatusText(transaction.Status, transaction.TxType)

	// 获取交易类型文案（结合交易状态生成更准确的文案）
	txTypeText := getTxTypeText(transaction.TxType, transaction.Status)

	// 获取地址标签
	fromAddr := derefString(transaction.FromAddress)
	toAddr := derefString(transaction.ToAddress)
	fromLabel := l.getAddressLabel(fromAddr)
	toLabel := l.getAddressLabel(toAddr)

	// 获取需要的确认数
	requiredConfirmations := l.getRequiredConfirmations(transaction.ChainCode)

	// 生成区块浏览器链接
	explorerURL := l.getExplorerURL(transaction.ChainCode, transaction.TxHash)

	l.Infof("查询 Web3 交易详情成功: id=%d, tx_hash=%s", id, transaction.TxHash)

	return &pb.GetWeb3TransactionDetailResp{
		Success: true,
		Message: "ok",

		// 基本信息
		TxHash:     transaction.TxHash,
		TxType:     transaction.TxType,
		TxTypeText: txTypeText,
		Direction:  transaction.Direction,

		// 金额信息
		AssetCode:     transaction.AssetCode,
		Amount:        amountStr,
		AmountDisplay: amountDisplay,
		AmountUsd:     "",

		// 地址信息
		FromAddress: fromAddr,
		FromLabel:   fromLabel,
		ToAddress:   toAddr,
		ToLabel:     toLabel,

		// 状态信息
		Status:                transaction.Status,
		StatusText:            statusText,
		Confirmations:         transaction.Confirmations,
		RequiredConfirmations: requiredConfirmations,

		// 网络信息
		Network:     transaction.ChainCode,
		ChainId:     transaction.ChainID,
		BlockNumber: derefInt64(transaction.BlockNumber),
		BlockTime:   getTimestamp(transaction.BlockTime),

		// 手续费
		Fee:      derefString(transaction.Fee),
		FeeAsset: derefString(transaction.FeeAsset),

		// 其他
		IconUrl:     iconURL,
		ExplorerUrl: explorerURL,
	}, nil
}

func (l *GetWeb3TransactionDetailLogic) getAssetIconAndPrecision(assetCode string) (string, int32) {
	if l.svcCtx.AccountingRpc == nil {
		return "", 0
	}

	resp, err := l.svcCtx.AccountingRpc.GetAsset(l.ctx, &pb.GetAssetRequest{Code: assetCode})
	if err != nil || resp == nil || !resp.Success || resp.Item == nil {
		return "", 0
	}

	return strings.TrimSpace(resp.Item.IconUrl), resp.Item.Precision
}

func (l *GetWeb3TransactionDetailLogic) tryRefreshPendingWeb3BalanceChangeFromChain(tx *model.Web3BalanceChangeModel) {
	if l == nil || tx == nil || l.svcCtx == nil || l.svcCtx.ChainRpc == nil || l.svcCtx.Web3BalanceChangeRepository == nil {
		return
	}
	if strings.TrimSpace(tx.TxHash) == "" {
		return
	}

	// Only refresh when current record looks like a broadcast placeholder or stale pending.
	if strings.TrimSpace(tx.Status) != model.Web3TxStatusPending {
		return
	}

	chainType := chainCodeToChainRpcType(tx.ChainCode)
	if chainType == pb.ChainRpcType_CHAIN_TYPE_UNSPECIFIED {
		return
	}

	ctx, cancel := context.WithTimeout(l.ctx, 5*time.Second)
	defer cancel()

	resp, err := l.svcCtx.ChainRpc.GetTransaction(ctx, &pb.GetTransactionReq{
		Chain:       chainType,
		TxHash:      tx.TxHash,
		IncludeLogs: false,
	})
	if err != nil || resp == nil || !resp.Success || resp.Transaction == nil {
		return
	}

	td := resp.Transaction
	if td == nil {
		return
	}

	updates := map[string]interface{}{}

	confirmations := uint64ToInt32Clamp(td.Confirmations)
	requiredConfirmations := l.getRequiredConfirmations(tx.ChainCode)
	status := deriveWeb3TxStatusFromChain(td.Status, td.Confirmations, requiredConfirmations)
	if s := strings.TrimSpace(status); s != "" && s != tx.Status {
		tx.Status = s
		updates["status"] = s
	}

	if confirmations > tx.Confirmations {
		tx.Confirmations = confirmations
		updates["confirmations"] = confirmations
	}

	if bnPtr := parseBlockNumberPtr(td.BlockNumber); bnPtr != nil {
		if tx.BlockNumber == nil || *tx.BlockNumber != *bnPtr {
			tx.BlockNumber = bnPtr
			updates["block_number"] = *bnPtr
		}
	}

	if btPtr := blockTimestampToLocalTimePtr(td.BlockTimestamp); btPtr != nil {
		if tx.BlockTime == nil || !tx.BlockTime.Equal(*btPtr) {
			tx.BlockTime = btPtr
			updates["block_time"] = btPtr
		}
	}

	// ChainRPC returns gas_fee as raw wei/sun for EVM/TRON. Keep DB/UI consistent by always formatting
	// it to a human-readable native fee string here.
	feePtr, feeAssetPtr := commonutils.FormatWeb3Fee(tx.ChainCode, td.GasFee, td.GasUsed, td.GasPrice)
	if feePtr != nil {
		fee := strings.TrimSpace(*feePtr)
		if fee != "" && (tx.Fee == nil || strings.TrimSpace(*tx.Fee) != fee) {
			tx.Fee = &fee
			updates["fee"] = fee
		}
	}
	if feeAssetPtr != nil {
		feeAsset := strings.TrimSpace(*feeAssetPtr)
		if feeAsset != "" && (tx.FeeAsset == nil || strings.TrimSpace(*tx.FeeAsset) != feeAsset) {
			tx.FeeAsset = &feeAsset
			updates["fee_asset"] = feeAsset
		}
	} else if feeAsset := chainCodeToNativeFeeAsset(tx.ChainCode); feeAsset != "" {
		// Best-effort fallback: if fee_asset is missing, fill it from chain_code.
		if tx.FeeAsset == nil || strings.TrimSpace(*tx.FeeAsset) != feeAsset {
			tx.FeeAsset = &feeAsset
			updates["fee_asset"] = feeAsset
		}
	}

	if len(updates) == 0 {
		return
	}

	updates["updated_at"] = time.Now().Local()
	_ = l.svcCtx.Web3BalanceChangeRepository.GetDB().WithContext(ctx).
		Model(&model.Web3BalanceChangeModel{}).
		Where("id = ? AND deleted_at IS NULL", tx.ID).
		Updates(updates).Error
}

// getAddressLabel 获取地址标签（公开查询，返回已知标签或简化地址）
func (l *GetWeb3TransactionDetailLogic) getAddressLabel(address string) string {
	// 已知交易所地址（可以从配置或数据库读取）
	knownExchanges := map[string]string{
		// 示例（实际应该从配置读取）
		// "0x...": "Binance",
		// "0x...": "OKX",
	}
	if label, ok := knownExchanges[strings.ToLower(address)]; ok {
		return label
	}

	// 已知合约地址
	knownContracts := map[string]string{
		// 示例
		// "0x...": "Uniswap V3",
	}
	if label, ok := knownContracts[strings.ToLower(address)]; ok {
		return label
	}

	// 简化地址显示
	return shortenAddress(address)
}

// shortenAddress 简化地址显示
func shortenAddress(address string) string {
	if len(address) <= 10 {
		return address
	}
	return fmt.Sprintf("%s...%s", address[:6], address[len(address)-4:])
}

// getRequiredConfirmations 获取需要的确认数（根据网络）
func (l *GetWeb3TransactionDetailLogic) getRequiredConfirmations(network string) int32 {
	// 这里应该从配置读取，暂时硬编码
	confirmations := map[string]int32{
		"Ethereum": 12,
		"ETH":      12,
		"Tron":     20,
		"TRON":     20,
		"BSC":      15,
	}

	if required, ok := confirmations[network]; ok {
		return required
	}

	return 12 // 默认值
}

// getExplorerURL 获取区块浏览器链接
func (l *GetWeb3TransactionDetailLogic) getExplorerURL(network, txHash string) string {
	// 根据不同网络返回不同的浏览器链接
	explorers := map[string]string{
		"Ethereum": "https://etherscan.io/tx/%s",
		"ETH":      "https://etherscan.io/tx/%s",
		"Tron":     "https://tronscan.org/#/transaction/%s",
		"TRON":     "https://tronscan.org/#/transaction/%s",
		"BSC":      "https://bscscan.com/tx/%s",
	}

	if template, ok := explorers[network]; ok {
		return fmt.Sprintf(template, txHash)
	}

	return ""
}
