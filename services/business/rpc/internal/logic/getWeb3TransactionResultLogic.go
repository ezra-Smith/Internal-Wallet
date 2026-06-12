package logic

import (
	"context"
	"fmt"
	"strings"
	"time"

	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/model"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetWeb3TransactionResultLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetWeb3TransactionResultLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetWeb3TransactionResultLogic {
	return &GetWeb3TransactionResultLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetWeb3TransactionResult 获取 Web3 交易结果（公开查询，无需认证）
// 优先从数据库查询，如果数据库没有且提供了 network 参数，则实时查询链上状态
func (l *GetWeb3TransactionResultLogic) GetWeb3TransactionResult(in *pb.GetWeb3TransactionResultReq) (*pb.GetWeb3TransactionResultResp, error) {
	// 验证 tx_hash
	txHash := strings.TrimSpace(in.TxHash)
	if txHash == "" {
		return nil, errx.Web3TxHashRequired()
	}

	network := strings.TrimSpace(in.Network)

	// 步骤1：先从数据库查询（如果已记录）
	transaction, err := l.svcCtx.Web3BalanceChangeRepository.FindByTxHash(l.ctx, txHash)
	if err == nil && transaction != nil {
		// 如果数据库中是 pending，并且能推断网络（请求提供 or DB 里有），则优先实时查链，避免长期停留在占位状态。
		if strings.TrimSpace(transaction.Status) == model.Web3TxStatusPending {
			chainHint := network
			if strings.TrimSpace(chainHint) == "" {
				chainHint = transaction.ChainCode
			}
			if strings.TrimSpace(chainHint) != "" {
				if result, err := l.queryFromChain(txHash, chainHint); err == nil && result != nil && result.Success {
					// 仅当链上状态更“新”时才覆盖数据库返回（否则保持数据库结果，避免抖动）
					dbBlockNumber := int64(0)
					if transaction.BlockNumber != nil {
						dbBlockNumber = *transaction.BlockNumber
					}
					if result.Status != transaction.Status || result.Confirmations > transaction.Confirmations || (result.BlockNumber > 0 && dbBlockNumber == 0) {
						l.tryUpdateWeb3BalanceChangeFromChainResult(transaction, result)
						return result, nil
					}
				}
			}
		}

		return l.buildResultFromDatabase(transaction, "database"), nil
	}

	// 步骤2：如果数据库没有，且提供了 network 参数，则实时查询链上状态
	if network != "" {
		result, err := l.queryFromChain(txHash, network)
		if err != nil {
			return result, err
		}
		return result, nil
	}

	// 步骤3：既没有数据库记录，也没有提供 network 参数
	return nil, errx.Web3TransactionNotFound()
}

// buildResultFromDatabase 从数据库记录构建结果
func (l *GetWeb3TransactionResultLogic) buildResultFromDatabase(tx *model.Web3BalanceChangeModel, dataSource string) *pb.GetWeb3TransactionResultResp {
	// 获取状态文案（结合交易类型生成更准确的文案）
	statusText := getStatusText(tx.Status, tx.TxType)

	// 判断是否成功（confirmed 且不是 failed）
	isSuccess := tx.Status == "confirmed" && tx.Status != "failed"

	// 获取需要的确认数
	requiredConfirmations := l.getRequiredConfirmations(tx.ChainCode)

	// 获取区块时间戳
	var blockTime int64
	if tx.BlockTime != nil {
		// 统一对外时间戳单位为 Unix 秒（与 ListWeb3Transactions / GetWeb3TransactionDetail 保持一致）
		blockTime = tx.BlockTime.Unix()
	}

	return &pb.GetWeb3TransactionResultResp{
		Success: true,
		Message: "ok",

		// 基本信息
		TxHash:  tx.TxHash,
		Network: tx.ChainCode,
		ChainId: tx.ChainID,

		// 状态信息
		Status:                tx.Status,
		StatusText:            statusText,
		IsSuccess:             isSuccess,
		Confirmations:         tx.Confirmations,
		RequiredConfirmations: requiredConfirmations,

		// 区块信息
		BlockNumber: func() int64 {
			if tx.BlockNumber == nil {
				return 0
			}
			return *tx.BlockNumber
		}(),
		BlockTime: blockTime,

		// 数据来源
		DataSource: dataSource,
	}
}

func (l *GetWeb3TransactionResultLogic) tryUpdateWeb3BalanceChangeFromChainResult(tx *model.Web3BalanceChangeModel, res *pb.GetWeb3TransactionResultResp) {
	if l == nil || tx == nil || res == nil || l.svcCtx == nil || l.svcCtx.Web3BalanceChangeRepository == nil {
		return
	}
	if strings.TrimSpace(tx.Status) != model.Web3TxStatusPending {
		return
	}
	if !res.Success {
		return
	}

	updates := map[string]interface{}{}

	if s := strings.TrimSpace(res.Status); s != "" && s != tx.Status {
		tx.Status = s
		updates["status"] = s
	}
	if res.Confirmations > tx.Confirmations {
		tx.Confirmations = res.Confirmations
		updates["confirmations"] = res.Confirmations
	}
	if res.BlockNumber > 0 {
		if tx.BlockNumber == nil || *tx.BlockNumber != res.BlockNumber {
			bn := res.BlockNumber
			tx.BlockNumber = &bn
			updates["block_number"] = bn
		}
	}
	if res.BlockTime > 0 {
		bt := time.Unix(res.BlockTime, 0).Local()
		if tx.BlockTime == nil || !tx.BlockTime.Equal(bt) {
			tx.BlockTime = &bt
			updates["block_time"] = &bt
		}
	}

	if len(updates) == 0 {
		return
	}
	updates["updated_at"] = time.Now().Local()
	_ = l.svcCtx.Web3BalanceChangeRepository.GetDB().WithContext(l.ctx).
		Model(&model.Web3BalanceChangeModel{}).
		Where("id = ? AND deleted_at IS NULL", tx.ID).
		Updates(updates).Error
}

// queryFromChain 实时查询链上状态
func (l *GetWeb3TransactionResultLogic) queryFromChain(txHash, network string) (*pb.GetWeb3TransactionResultResp, error) {
	if l.svcCtx.ChainRpc == nil {
		return &pb.GetWeb3TransactionResultResp{
			Success:    false,
			Message:    "链上查询服务不可用",
			TxHash:     txHash,
			Network:    network,
			DataSource: "chain",
		}, nil
	}

	chainCode := normalizeNetworkToChainCode(network)
	chainType := chainCodeToChainRpcType(chainCode)
	if chainType == pb.ChainRpcType_CHAIN_TYPE_UNSPECIFIED {
		return &pb.GetWeb3TransactionResultResp{
			Success:    false,
			Message:    fmt.Sprintf("不支持的网络: %s", network),
			TxHash:     txHash,
			Network:    network,
			DataSource: "chain",
		}, nil
	}

	ctx, cancel := context.WithTimeout(l.ctx, 5*time.Second)
	defer cancel()

	// Prefer GetTransaction to include block_timestamp and confirmations.
	txResp, err := l.svcCtx.ChainRpc.GetTransaction(ctx, &pb.GetTransactionReq{
		Chain:       chainType,
		TxHash:      txHash,
		IncludeLogs: false,
	})
	if err != nil {
		l.Errorf("查询链上交易详情失败: tx_hash=%s, network=%s, error=%v", txHash, network, err)
		return &pb.GetWeb3TransactionResultResp{
			Success:    false,
			Message:    fmt.Sprintf("查询链上状态失败: %v", err),
			TxHash:     txHash,
			Network:    network,
			DataSource: "chain",
		}, err
	}
	if txResp == nil || !txResp.Success || txResp.Transaction == nil {
		msg := ""
		if txResp != nil {
			msg = txResp.Message
		}
		return &pb.GetWeb3TransactionResultResp{
			Success:    false,
			Message:    msg,
			TxHash:     txHash,
			Network:    network,
			DataSource: "chain",
		}, nil
	}

	td := txResp.Transaction
	requiredConfirmations := l.getRequiredConfirmations(chainCode)
	status := deriveWeb3TxStatusFromChain(td.Status, td.Confirmations, requiredConfirmations)
	// 从链上查询时，如果没有交易类型信息，使用空字符串（会回退到通用状态文案）
	txType := ""
	statusText := getStatusText(status, txType)
	isSuccess := status == model.Web3TxStatusConfirmed

	var blockNumber int64
	if bnPtr := parseBlockNumberPtr(td.BlockNumber); bnPtr != nil {
		blockNumber = *bnPtr
	}
	var blockTime int64
	if btPtr := blockTimestampToLocalTimePtr(td.BlockTimestamp); btPtr != nil {
		blockTime = btPtr.Unix()
	}

	chainID := int64(0)
	switch chainType {
	case pb.ChainRpcType_CHAIN_TYPE_ETHEREUM:
		chainID = 1
	case pb.ChainRpcType_CHAIN_TYPE_BSC:
		chainID = 56
	case pb.ChainRpcType_CHAIN_TYPE_TRON:
		chainID = 728126428
	}

	return &pb.GetWeb3TransactionResultResp{
		Success: true,
		Message: "ok",

		// 基本信息
		TxHash:  txHash,
		Network: chainCode,
		ChainId: chainID,

		// 状态信息
		Status:                status,
		StatusText:            statusText,
		IsSuccess:             isSuccess,
		Confirmations:         uint64ToInt32Clamp(td.Confirmations),
		RequiredConfirmations: requiredConfirmations,

		// 区块信息
		BlockNumber: blockNumber,
		BlockTime:   blockTime,

		// 数据来源
		DataSource: "chain",
	}, nil
}

// getRequiredConfirmations 获取需要的确认数（根据网络）
func (l *GetWeb3TransactionResultLogic) getRequiredConfirmations(network string) int32 {
	confirmations := map[string]int32{
		"Bitcoin":  6,
		"Ethereum": 12,
		"ETH":      12,
		"Tron":     20,
		"TRON":     20,
		"BSC":      15,
		"Polygon":  128,
	}

	if required, ok := confirmations[network]; ok {
		return required
	}

	return 12 // 默认值
}
