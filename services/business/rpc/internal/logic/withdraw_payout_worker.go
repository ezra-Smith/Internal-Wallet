package logic

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"internalwallet/common/constants"
	"internalwallet/pkg/notify"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/alert"
	"internalwallet/services/business/rpc/internal/model"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type WithdrawPayoutWorker struct {
	svcCtx        *svc.ServiceContext
	owner         string
	pollInterval  time.Duration
	leaseDuration time.Duration
	batchSize     int
}

func NewWithdrawPayoutWorker(svcCtx *svc.ServiceContext) *WithdrawPayoutWorker {
	host, _ := os.Hostname()
	return &WithdrawPayoutWorker{
		svcCtx:        svcCtx,
		owner:         fmt.Sprintf("%s:%d", host, os.Getpid()),
		pollInterval:  2 * time.Second,
		leaseDuration: 30 * time.Second,
		batchSize:     20,
	}
}

func (w *WithdrawPayoutWorker) Run(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	if w.svcCtx == nil || w.svcCtx.CurrencyWithdrawPayoutTaskRepository == nil || w.svcCtx.DB == nil {
		logx.WithContext(ctx).Error("withdraw payout worker disabled: repo/db not configured")
		return
	}
	t := time.NewTicker(w.pollInterval)
	defer t.Stop()
	logx.WithContext(ctx).Infof("withdraw payout worker started (owner=%s)", w.owner)
	for {
		select {
		case <-ctx.Done():
			logx.WithContext(ctx).Info("withdraw payout worker stopped")
			return
		case <-t.C:
			w.tick(ctx)
		}
	}
}

func (w *WithdrawPayoutWorker) tick(ctx context.Context) {
	if w.svcCtx == nil ||
		w.svcCtx.CurrencyWithdrawPayoutTaskRepository == nil ||
		w.svcCtx.CurrencyWithdrawOrderRepository == nil ||
		w.svcCtx.CurrencyWithdrawOrderEventRepository == nil ||
		w.svcCtx.AccountingRpc == nil {
		return
	}

	now := time.Now()
	states := []string{
		constants.WithdrawPayoutStatePendingBroadcast,
		constants.WithdrawPayoutStateConfirming,
		constants.WithdrawPayoutStateSettlePending,
		constants.WithdrawPayoutStateAwaitingManualTxHash,
	}
	tasks, err := w.svcCtx.CurrencyWithdrawPayoutTaskRepository.ListDue(ctx, states, now, w.batchSize)
	if err != nil {
		logx.WithContext(ctx).Errorf("withdraw payout worker list due failed: %v", err)
		return
	}

	// 按地址去重：对于 pending_broadcast 状态的任务，每个 tick 内每个
	// (chain, from_address) 只处理一笔，避免 nonce 冲突。
	// ChainRPC 的 NonceManager 提供主要保障，此处为防御性二次保护。
	broadcastedAddrs := make(map[string]bool)

	for _, task := range tasks {
		if task == nil || task.WithdrawOrderID <= 0 {
			continue
		}

		// 对于待广播任务，获取锁之前先检查地址级去重。
		if strings.TrimSpace(task.State) == constants.WithdrawPayoutStatePendingBroadcast {
			order, orderErr := w.svcCtx.CurrencyWithdrawOrderRepository.FindByID(ctx, task.WithdrawOrderID)
			if orderErr == nil && order != nil {
				addrKey := strings.ToLower(strings.TrimSpace(order.ChainCode) + ":" + strings.TrimSpace(order.FromAddress))
				if broadcastedAddrs[addrKey] {
					logx.WithContext(ctx).Infof("[WithdrawPayout] Deferring task %d: another broadcast for %s already in this tick",
						task.WithdrawOrderID, addrKey)
					continue
				}
				// 标记该地址在本轮 tick 中已处理。
				broadcastedAddrs[addrKey] = true
			}
		}

		lockUntil := now.Add(w.leaseDuration)
		locked, err := w.svcCtx.CurrencyWithdrawPayoutTaskRepository.AcquireLock(ctx, task.WithdrawOrderID, w.owner, lockUntil, now)
		if err != nil {
			logx.WithContext(ctx).Errorf("withdraw payout worker acquire lock failed: %v", err)
			continue
		}
		if !locked {
			continue
		}

		taskCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		w.processOne(taskCtx, task.WithdrawOrderID)
		cancel()
		_ = w.svcCtx.CurrencyWithdrawPayoutTaskRepository.ReleaseLock(ctx, task.WithdrawOrderID, w.owner)
	}
}

func (w *WithdrawPayoutWorker) processOne(ctx context.Context, withdrawOrderID int64) {
	task, err := w.svcCtx.CurrencyWithdrawPayoutTaskRepository.FindByWithdrawOrderID(ctx, withdrawOrderID)
	if err != nil {
		return
	}
	order, err := w.svcCtx.CurrencyWithdrawOrderRepository.FindByID(ctx, withdrawOrderID)
	if err != nil {
		return
	}

	orderStatus := strings.ToLower(strings.TrimSpace(order.Status))
	switch orderStatus {
	case "completed":
		_ = w.svcCtx.CurrencyWithdrawPayoutTaskRepository.UpdateFields(ctx, withdrawOrderID, map[string]interface{}{
			"state":           constants.WithdrawPayoutStateDone,
			"next_retry_time": time.Now().Add(24 * time.Hour),
			"last_error":      nil,
		})
		return
	case "failed", "cancelled":
		_ = w.svcCtx.CurrencyWithdrawPayoutTaskRepository.UpdateFields(ctx, withdrawOrderID, map[string]interface{}{
			"state":           constants.WithdrawPayoutStateFailed,
			"next_retry_time": time.Now().Add(24 * time.Hour),
		})
		return
	}

	state := strings.TrimSpace(task.State)
	mode := strings.TrimSpace(task.Mode)
	if mode == "" {
		mode = constants.WithdrawPayoutModeSystem
	}
	if state == "" {
		state = constants.WithdrawPayoutStatePendingBroadcast
	}

	switch state {
	case constants.WithdrawPayoutStateAwaitingManualTxHash:
		w.handleAwaitingManualTxHash(ctx, task, order)
	case constants.WithdrawPayoutStatePendingBroadcast:
		w.handlePendingBroadcast(ctx, task, order)
	case constants.WithdrawPayoutStateConfirming:
		w.handleConfirming(ctx, task, order)
	case constants.WithdrawPayoutStateSettlePending:
		w.handleSettlePending(ctx, task, order)
	default:
		_ = w.svcCtx.CurrencyWithdrawPayoutTaskRepository.UpdateFields(ctx, withdrawOrderID, map[string]interface{}{
			"state":           constants.WithdrawPayoutStateFailed,
			"next_retry_time": time.Now().Add(24 * time.Hour),
			"last_error":      fmt.Sprintf("unknown task state: %s", state),
		})
	}
}

func (w *WithdrawPayoutWorker) handleAwaitingManualTxHash(ctx context.Context, task *model.CurrencyWithdrawPayoutTaskModel, order *model.CurrencyWithdrawOrderModel) {
	now := time.Now()
	txHash := ""
	if order.TxHash != nil {
		txHash = strings.TrimSpace(*order.TxHash)
	}
	if txHash == "" && task.TxHash != nil {
		txHash = strings.TrimSpace(*task.TxHash)
	}
	if txHash == "" {
		_ = w.svcCtx.CurrencyWithdrawPayoutTaskRepository.UpdateFields(ctx, task.WithdrawOrderID, map[string]interface{}{
			"mode":            constants.WithdrawPayoutModeManual,
			"state":           constants.WithdrawPayoutStateAwaitingManualTxHash,
			"next_retry_time": now.Add(1 * time.Hour),
		})
		return
	}

	_ = w.svcCtx.CurrencyWithdrawPayoutTaskRepository.UpdateFields(ctx, task.WithdrawOrderID, map[string]interface{}{
		"mode":            constants.WithdrawPayoutModeManual,
		"state":           constants.WithdrawPayoutStateConfirming,
		"tx_hash":         txHash,
		"next_retry_time": now,
		"last_error":      nil,
	})
}

func (w *WithdrawPayoutWorker) handlePendingBroadcast(ctx context.Context, task *model.CurrencyWithdrawPayoutTaskModel, order *model.CurrencyWithdrawOrderModel) {
	now := time.Now()
	strategy := strings.ToLower(strings.TrimSpace(order.Strategy))
	if strategy == "manual_manual" {
		_ = w.svcCtx.CurrencyWithdrawPayoutTaskRepository.UpdateFields(ctx, task.WithdrawOrderID, map[string]interface{}{
			"mode":            constants.WithdrawPayoutModeManual,
			"state":           constants.WithdrawPayoutStateAwaitingManualTxHash,
			"next_retry_time": now.Add(1 * time.Hour),
		})
		return
	}

	if w.svcCtx.ChainRpc == nil {
		_ = w.svcCtx.CurrencyWithdrawPayoutTaskRepository.UpdateFields(ctx, task.WithdrawOrderID, map[string]interface{}{
			"last_error":      "chain rpc not configured",
			"next_retry_time": now.Add(30 * time.Second),
		})
		return
	}

	if task.BroadcastAttempts >= task.MaxBroadcastAttempts {
		w.failAndUnfreeze(ctx, task, order, "broadcast attempts exceeded")
		return
	}

	// 获取资产的合约地址和精度，判断是原生代币还是 ERC20 代币
	var contractAddress string
	var assetPrecision int32 = 18 // 默认 18 位小数（链上原生代币精度）

	// 优先从 currency_chain_settings 获取链上的实际精度和合约地址
	if w.svcCtx.CurrencyChainSettingsRepository != nil {
		chainSetting, settingErr := w.svcCtx.CurrencyChainSettingsRepository.FindByAssetChain(ctx, order.AssetCode, order.ChainCode)
		if settingErr == nil && chainSetting != nil {
			if chainSetting.ContractAddress != nil {
				contractAddress = strings.TrimSpace(*chainSetting.ContractAddress)
			}
			// 使用链上的实际精度（token_decimals）
			if chainSetting.TokenDecimals != nil && *chainSetting.TokenDecimals > 0 {
				assetPrecision = *chainSetting.TokenDecimals
				logx.Infof("[WithdrawPayout] Using on-chain token_decimals=%d for %s on %s", assetPrecision, order.AssetCode, order.ChainCode)
			}
		}
	}

	// 若 token_decimals 未配置且有合约地址，从链上查询精度（避免用 asset.Precision 导致链上金额错误）
	if assetPrecision == 18 && contractAddress != "" && w.svcCtx.ChainRpc != nil {
		chainType := mapChainType(order.ChainCode)
		if chainType != pb.ChainRpcType_CHAIN_TYPE_UNSPECIFIED {
			tokenInfoCtx, tokenInfoCancel := context.WithTimeout(context.Background(), 3*time.Second)
			tokenInfoResp, tokenInfoErr := w.svcCtx.ChainRpc.GetTokenInfo(tokenInfoCtx, &pb.GetTokenInfoReq{
				Chain:         chainType,
				TokenContract: contractAddress,
			})
			tokenInfoCancel()
			if tokenInfoErr == nil && tokenInfoResp != nil && tokenInfoResp.Success && tokenInfoResp.TokenInfo != nil && tokenInfoResp.TokenInfo.Decimals > 0 {
				assetPrecision = int32(tokenInfoResp.TokenInfo.Decimals)
				logx.Infof("[WithdrawPayout] Queried on-chain decimals=%d for %s on %s via ChainRPC", assetPrecision, order.AssetCode, order.ChainCode)
			} else {
				logx.Errorf("[WithdrawPayout] Failed to get token decimals from chain for %s on %s, using default 18 (may cause precision issues!)", order.AssetCode, order.ChainCode)
			}
		}
	}
	// 对于 TRON 原生币（TRX），强制使用 6 位精度
	if contractAddress == "" && strings.ToUpper(order.ChainCode) == "TRON" {
		assetPrecision = 6
	}

	// 内扣模式：实际到账金额 = 提现金额 - 手续费
	amountDec, parseErr := decimal.NewFromString(strings.TrimSpace(order.Amount))
	if parseErr != nil {
		w.failAndUnfreeze(ctx, task, order, fmt.Sprintf("invalid amount: %v", parseErr))
		return
	}
	feeDec, feeParseErr := decimal.NewFromString(strings.TrimSpace(order.Fee))
	if feeParseErr != nil {
		feeDec = decimal.Zero
	}
	actualAmount := amountDec.Sub(feeDec)
	if actualAmount.LessThanOrEqual(decimal.Zero) {
		w.failAndUnfreeze(ctx, task, order, "actual transfer amount must be positive after deducting fee")
		return
	}

	// 将金额从人类可读格式转换为原始单位（乘以 10^precision）
	rawAmount := convertToRawAmount(actualAmount.String(), assetPrecision)
	logx.Infof("[WithdrawPayout] Amount conversion (fee deducted): order.Amount=%s, fee=%s, actualAmount=%s, precision=%d, raw=%s",
		order.Amount, order.Fee, actualAmount.String(), assetPrecision, rawAmount)

	var txResp *pb.TransactionResp
	var err error

	if contractAddress != "" {
		// ERC20/TRC20 代币转账
		logx.Infof("[WithdrawPayout] Using TokenTransfer for %s on %s, contract: %s", order.AssetCode, order.ChainCode, contractAddress)
		txResp, err = w.svcCtx.ChainRpc.TokenTransfer(ctx, &pb.TokenTransferReq{
			Chain:          mapChainType(order.ChainCode),
			FromAddress:    strings.TrimSpace(order.FromAddress),
			ToAddress:      strings.TrimSpace(order.ToAddress),
			TokenContract:  contractAddress,
			Amount:         rawAmount,
			GasSpeed:       pb.GasSpeed_GAS_SPEED_STANDARD,
			Uid:            order.UserID,
			WaitForReceipt: false,
		})
	} else {
		// 原生代币转账 (BNB, ETH, TRX)
		logx.Infof("[WithdrawPayout] Using NativeTransfer for %s on %s", order.AssetCode, order.ChainCode)
		txResp, err = w.svcCtx.ChainRpc.NativeTransfer(ctx, &pb.NativeTransferReq{
			Chain:          mapChainType(order.ChainCode),
			FromAddress:    strings.TrimSpace(order.FromAddress),
			ToAddress:      strings.TrimSpace(order.ToAddress),
			Amount:         rawAmount,
			GasSpeed:       pb.GasSpeed_GAS_SPEED_STANDARD,
			Uid:            order.UserID,
			WaitForReceipt: false,
		})
	}
	txHash := ""
	if txResp != nil {
		txHash = strings.TrimSpace(txResp.TxHash)
	}

	attempts := task.BroadcastAttempts + 1
	if err != nil || txResp == nil || txHash == "" {
		msg := "chain transfer failed"
		if txResp != nil && strings.TrimSpace(txResp.Message) != "" {
			msg = strings.TrimSpace(txResp.Message)
		}
		if err != nil && strings.TrimSpace(err.Error()) != "" {
			msg = msg + ": " + strings.TrimSpace(err.Error())
		}

		appendWithdrawOrderEvent(ctx, w.svcCtx.CurrencyWithdrawOrderEventRepository, newWithdrawOrderEvent(task.WithdrawOrderID, constants.WithdrawEventTransferFailed, constants.WithdrawActorSystem, 0, nil, "Transfer failed", map[string]any{
			"message": msg,
			"attempt": attempts,
		}))

		if attempts >= task.MaxBroadcastAttempts {
			w.failAndUnfreeze(ctx, task, order, msg)
			return
		}
		next := now.Add(withdrawPayoutBackoff(attempts, 10*time.Second, 2*time.Minute))
		_ = w.svcCtx.CurrencyWithdrawPayoutTaskRepository.UpdateFields(ctx, task.WithdrawOrderID, map[string]interface{}{
			"mode":               constants.WithdrawPayoutModeSystem,
			"state":              constants.WithdrawPayoutStatePendingBroadcast,
			"broadcast_attempts": attempts,
			"last_error":         msg,
			"next_retry_time":    next,
		})
		return
	}

	appendWithdrawOrderEvent(ctx, w.svcCtx.CurrencyWithdrawOrderEventRepository, newWithdrawOrderEvent(task.WithdrawOrderID, constants.WithdrawEventTransferInitiated, constants.WithdrawActorSystem, 0, nil, "Transfer initiated", map[string]any{
		"chain_code":       strings.TrimSpace(order.ChainCode),
		"from_address":     strings.TrimSpace(order.FromAddress),
		"to_address":       strings.TrimSpace(order.ToAddress),
		"amount":           strings.TrimSpace(order.Amount),
		"gas_speed":        pb.GasSpeed_GAS_SPEED_STANDARD.String(),
		"contract_address": contractAddress,
		"tx_hash":          txHash,
		"attempt":          attempts,
	}))

	txHashPtr := any(nil)
	if txHash != "" {
		txHashPtr = txHash
	}
	if w.svcCtx.AccountingRpc != nil && txHash != "" {
		_, _ = w.svcCtx.AccountingRpc.UpsertUserTransactionRecordMeta(ctx, &pb.UpsertUserTransactionRecordMetaRequest{
			Id:          order.ID,
			TxType:      2,
			Status:      "processing",
			FromAddress: strings.TrimSpace(order.FromAddress),
			ToAddress:   strings.TrimSpace(order.ToAddress),
			TxHash:      txHash,
		})
	}
	_ = w.svcCtx.CurrencyWithdrawOrderRepository.UpdateFields(ctx, order.ID, map[string]interface{}{
		"tx_hash":           txHashPtr,
		"error_message":     nil,
		"transfer_admin_id": 0,
		"updated_at":        now,
		"updated_by":        0,
	})
	_ = w.svcCtx.CurrencyWithdrawPayoutTaskRepository.UpdateFields(ctx, task.WithdrawOrderID, map[string]interface{}{
		"mode":               constants.WithdrawPayoutModeSystem,
		"state":              constants.WithdrawPayoutStateConfirming,
		"tx_hash":            txHash,
		"broadcast_attempts": attempts,
		"last_error":         nil,
		"next_retry_time":    now.Add(10 * time.Second),
	})
}

func (w *WithdrawPayoutWorker) handleConfirming(ctx context.Context, task *model.CurrencyWithdrawPayoutTaskModel, order *model.CurrencyWithdrawOrderModel) {
	now := time.Now()
	if w.svcCtx.ChainRpc == nil {
		_ = w.svcCtx.CurrencyWithdrawPayoutTaskRepository.UpdateFields(ctx, task.WithdrawOrderID, map[string]interface{}{
			"last_error":      "chain rpc not configured",
			"next_retry_time": now.Add(30 * time.Second),
		})
		return
	}

	txHash := ""
	if task.TxHash != nil {
		txHash = strings.TrimSpace(*task.TxHash)
	}
	if txHash == "" && order.TxHash != nil {
		txHash = strings.TrimSpace(*order.TxHash)
	}
	if txHash == "" {
		if strings.EqualFold(strings.TrimSpace(task.Mode), constants.WithdrawPayoutModeManual) {
			_ = w.svcCtx.CurrencyWithdrawPayoutTaskRepository.UpdateFields(ctx, task.WithdrawOrderID, map[string]interface{}{
				"mode":            constants.WithdrawPayoutModeManual,
				"state":           constants.WithdrawPayoutStateAwaitingManualTxHash,
				"next_retry_time": now.Add(1 * time.Hour),
				"last_error":      "tx_hash missing",
			})
			return
		}
		_ = w.svcCtx.CurrencyWithdrawPayoutTaskRepository.UpdateFields(ctx, task.WithdrawOrderID, map[string]interface{}{
			"mode":            constants.WithdrawPayoutModeSystem,
			"state":           constants.WithdrawPayoutStatePendingBroadcast,
			"next_retry_time": now.Add(10 * time.Second),
			"last_error":      "tx_hash missing",
		})
		return
	}

	resp, err := w.svcCtx.ChainRpc.GetTransactionStatus(ctx, &pb.GetTransactionStatusReq{
		Chain:  mapChainType(order.ChainCode),
		TxHash: txHash,
	})
	if err != nil || resp == nil || !resp.Success {
		msg := "get tx status failed"
		if resp != nil && strings.TrimSpace(resp.Message) != "" {
			msg = strings.TrimSpace(resp.Message)
		}
		if err != nil && strings.TrimSpace(err.Error()) != "" {
			msg = msg + ": " + strings.TrimSpace(err.Error())
		}
		_ = w.svcCtx.CurrencyWithdrawPayoutTaskRepository.UpdateFields(ctx, task.WithdrawOrderID, map[string]interface{}{
			"confirm_checks":  task.ConfirmChecks + 1,
			"last_error":      msg,
			"next_retry_time": now.Add(withdrawPayoutBackoff(task.ConfirmChecks+1, 15*time.Second, 5*time.Minute)),
		})
		return
	}

	// Treat confirmed by either flag or enum.
	if resp.IsConfirmed || resp.Status == pb.TxStatus_TX_STATUS_CONFIRMED {
		appendWithdrawOrderEvent(ctx, w.svcCtx.CurrencyWithdrawOrderEventRepository, newWithdrawOrderEvent(task.WithdrawOrderID, constants.WithdrawEventTransferCompleted, constants.WithdrawActorSystem, 0, nil, "Transfer completed", map[string]any{
			"tx_hash":                txHash,
			"status":                 resp.Status.String(),
			"confirmations":          resp.Confirmations,
			"required_confirmations": resp.RequiredConfirmations,
			"is_confirmed":           resp.IsConfirmed,
			"block_number":           strings.TrimSpace(resp.BlockNumber),
		}))
		_ = w.svcCtx.CurrencyWithdrawPayoutTaskRepository.UpdateFields(ctx, task.WithdrawOrderID, map[string]interface{}{
			"state":           constants.WithdrawPayoutStateSettlePending,
			"confirm_checks":  task.ConfirmChecks + 1,
			"last_error":      nil,
			"next_retry_time": now,
		})
		return
	}

	if resp.Status == pb.TxStatus_TX_STATUS_FAILED || resp.Status == pb.TxStatus_TX_STATUS_DROPPED || resp.Status == pb.TxStatus_TX_STATUS_REPLACED {
		appendWithdrawOrderEvent(ctx, w.svcCtx.CurrencyWithdrawOrderEventRepository, newWithdrawOrderEvent(task.WithdrawOrderID, constants.WithdrawEventTransferFailed, constants.WithdrawActorSystem, 0, nil, "Transfer failed", map[string]any{
			"tx_hash":                txHash,
			"status":                 resp.Status.String(),
			"confirmations":          resp.Confirmations,
			"required_confirmations": resp.RequiredConfirmations,
			"is_confirmed":           resp.IsConfirmed,
			"block_number":           strings.TrimSpace(resp.BlockNumber),
		}))

		if strings.EqualFold(strings.TrimSpace(task.Mode), constants.WithdrawPayoutModeManual) {
			msg := "on-chain tx failed, please submit a new tx_hash"
			_ = w.svcCtx.CurrencyWithdrawOrderRepository.UpdateFields(ctx, order.ID, map[string]interface{}{
				"error_message": msg,
				"updated_at":    now,
				"updated_by":    0,
			})
			_ = w.svcCtx.CurrencyWithdrawPayoutTaskRepository.UpdateFields(ctx, task.WithdrawOrderID, map[string]interface{}{
				"mode":            constants.WithdrawPayoutModeManual,
				"state":           constants.WithdrawPayoutStateAwaitingManualTxHash,
				"confirm_checks":  task.ConfirmChecks + 1,
				"last_error":      msg,
				"next_retry_time": now.Add(1 * time.Hour),
			})
			return
		}

		// system: allow retry if attempts remain.
		if task.BroadcastAttempts < task.MaxBroadcastAttempts {
			next := now.Add(withdrawPayoutBackoff(task.BroadcastAttempts+1, 30*time.Second, 10*time.Minute))
			_ = w.svcCtx.CurrencyWithdrawPayoutTaskRepository.UpdateFields(ctx, task.WithdrawOrderID, map[string]interface{}{
				"mode":            constants.WithdrawPayoutModeSystem,
				"state":           constants.WithdrawPayoutStatePendingBroadcast,
				"tx_hash":         nil,
				"confirm_checks":  task.ConfirmChecks + 1,
				"last_error":      "on-chain tx failed: " + resp.Status.String(),
				"next_retry_time": next,
			})
			return
		}

		w.failAndUnfreeze(ctx, task, order, "on-chain tx failed: "+resp.Status.String())
		return
	}

	// Pending/unknown.
	_ = w.svcCtx.CurrencyWithdrawPayoutTaskRepository.UpdateFields(ctx, task.WithdrawOrderID, map[string]interface{}{
		"confirm_checks":  task.ConfirmChecks + 1,
		"last_error":      nil,
		"next_retry_time": now.Add(30 * time.Second),
	})
}

func (w *WithdrawPayoutWorker) handleSettlePending(ctx context.Context, task *model.CurrencyWithdrawPayoutTaskModel, order *model.CurrencyWithdrawOrderModel) {
	now := time.Now()
	feeStr := strings.TrimSpace(order.Fee)
	if feeStr == "" {
		feeStr = "0"
	}
	txHash := ""
	if task != nil && task.TxHash != nil {
		txHash = strings.TrimSpace(*task.TxHash)
	}
	if txHash == "" && order.TxHash != nil {
		txHash = strings.TrimSpace(*order.TxHash)
	}
	memo := ""
	if order.MemoTag != nil {
		memo = strings.TrimSpace(*order.MemoTag)
	}

	// 内扣模式：实际到账金额 = 提现金额 - 手续费
	// Accounting 的 SettleWithdraw 会计算：totalRaw = actualAmount + fee = amount
	// 这样刚好等于冻结的金额
	amountDec, parseErr := decimal.NewFromString(strings.TrimSpace(order.Amount))
	if parseErr != nil {
		amountDec = decimal.Zero
	}
	feeDec, parseErr := decimal.NewFromString(feeStr)
	if parseErr != nil {
		feeDec = decimal.Zero
	}
	actualAmountDec := amountDec.Sub(feeDec).Truncate(6) // 截断到 6 位小数
	feeDecTrunc := feeDec.Truncate(6)

	settleReq := &pb.SettleWithdrawRequest{
		IdempotencyKey: fmt.Sprintf("withdraw:settle:%d", order.ID),
		BizRef:         fmt.Sprintf("%d", order.ID),
		UserId:         order.UserID,
		AssetCode:      strings.TrimSpace(order.AssetCode),
		ChainCode:      strings.TrimSpace(order.ChainCode),
		AmountDecimal:  actualAmountDec.String(), // 内扣模式：传入实际到账金额
		FeeDecimal:     feeDecTrunc.String(),
		TxHash:         txHash,
		FromAddress:    strings.TrimSpace(order.FromAddress),
		ToAddress:      strings.TrimSpace(order.ToAddress),
		Memo:           memo,
	}

	settleResp, settleErr := callAccountingTxWithRetry(ctx, 3, func(ctx context.Context) (*pb.LedgerTxResponse, error) {
		return w.svcCtx.AccountingRpc.SettleWithdraw(ctx, settleReq)
	})
	if settleErr != nil || settleResp == nil || !settleResp.Success {
		msg := "settle failed"
		if settleResp != nil && strings.TrimSpace(settleResp.Message) != "" {
			msg = strings.TrimSpace(settleResp.Message)
		}
		if settleErr != nil && strings.TrimSpace(settleErr.Error()) != "" {
			msg = msg + ": " + strings.TrimSpace(settleErr.Error())
		}

		// Only log the first failure to avoid flooding.
		if task.SettleAttempts == 0 {
			txHash := ""
			if order.TxHash != nil {
				txHash = strings.TrimSpace(*order.TxHash)
			}
			appendWithdrawOrderEvents(ctx, w.svcCtx.CurrencyWithdrawOrderEventRepository, []*model.CurrencyWithdrawOrderEventModel{
				newWithdrawOrderEvent(order.ID, constants.WithdrawEventAssetDeducted, constants.WithdrawActorSystem, 0, nil, "Withdraw settlement failed", map[string]any{
					"success": false,
					"message": msg,
					"tx_id": func() int64 {
						if settleResp == nil {
							return 0
						}
						return settleResp.TxId
					}(),
					"duplicate": func() bool {
						if settleResp == nil {
							return false
						}
						return settleResp.Duplicate
					}(),
					"idempotency_key": settleReq.IdempotencyKey,
					"biz_ref":         strings.TrimSpace(settleReq.BizRef),
					"asset_code":      strings.TrimSpace(order.AssetCode),
					"amount":          strings.TrimSpace(order.Amount),
					"tx_hash":         txHash,
				}),
				newWithdrawOrderEvent(order.ID, constants.WithdrawEventFeeCollected, constants.WithdrawActorSystem, 0, nil, "Withdraw settlement failed", map[string]any{
					"success": false,
					"message": msg,
					"tx_id": func() int64 {
						if settleResp == nil {
							return 0
						}
						return settleResp.TxId
					}(),
					"duplicate": func() bool {
						if settleResp == nil {
							return false
						}
						return settleResp.Duplicate
					}(),
					"idempotency_key": settleReq.IdempotencyKey,
					"biz_ref":         strings.TrimSpace(settleReq.BizRef),
					"asset_code":      strings.TrimSpace(order.AssetCode),
					"fee":             feeStr,
					"tx_hash":         txHash,
				}),
			})
		}

		_ = w.svcCtx.CurrencyWithdrawPayoutTaskRepository.UpdateFields(ctx, task.WithdrawOrderID, map[string]interface{}{
			"settle_attempts": task.SettleAttempts + 1,
			"last_error":      msg,
			"next_retry_time": now.Add(withdrawPayoutBackoff(task.SettleAttempts+1, 10*time.Second, 10*time.Minute)),
		})
		return
	}

	appendWithdrawOrderEvents(ctx, w.svcCtx.CurrencyWithdrawOrderEventRepository, []*model.CurrencyWithdrawOrderEventModel{
		newWithdrawOrderEvent(order.ID, constants.WithdrawEventAssetDeducted, constants.WithdrawActorSystem, 0, nil, "Asset deducted", map[string]any{
			"success":         true,
			"tx_id":           settleResp.TxId,
			"duplicate":       settleResp.Duplicate,
			"idempotency_key": settleReq.IdempotencyKey,
			"biz_ref":         strings.TrimSpace(settleReq.BizRef),
			"asset_code":      strings.TrimSpace(order.AssetCode),
			"amount":          strings.TrimSpace(order.Amount),
			"tx_hash":         txHash,
		}),
		newWithdrawOrderEvent(order.ID, constants.WithdrawEventFeeCollected, constants.WithdrawActorSystem, 0, nil, "Fee collected", map[string]any{
			"success":         true,
			"tx_id":           settleResp.TxId,
			"duplicate":       settleResp.Duplicate,
			"idempotency_key": settleReq.IdempotencyKey,
			"biz_ref":         strings.TrimSpace(settleReq.BizRef),
			"asset_code":      strings.TrimSpace(order.AssetCode),
			"fee":             feeStr,
			"tx_hash":         txHash,
		}),
	})

	err := w.svcCtx.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updateOrder := map[string]interface{}{
			"status":         "completed",
			"transferred_at": &now,
			"error_message":  nil,
			"updated_at":     now,
			"updated_by":     0,
		}
		if strings.EqualFold(strings.TrimSpace(task.Mode), constants.WithdrawPayoutModeSystem) {
			updateOrder["transfer_admin_id"] = 0
		}
		if err := tx.WithContext(ctx).
			Model(&model.CurrencyWithdrawOrderModel{}).
			Where("id = ?", order.ID).
			Updates(updateOrder).Error; err != nil {
			return err
		}

		if err := tx.WithContext(ctx).
			Model(&model.CurrencyWithdrawPayoutTaskModel{}).
			Where("withdraw_order_id = ?", order.ID).
			Updates(map[string]interface{}{
				"state":           constants.WithdrawPayoutStateDone,
				"last_error":      nil,
				"next_retry_time": now.Add(24 * time.Hour),
			}).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		logx.WithContext(ctx).Errorf("withdraw payout finalize transaction failed: %v", err)
		_ = w.svcCtx.CurrencyWithdrawPayoutTaskRepository.UpdateFields(ctx, task.WithdrawOrderID, map[string]interface{}{
			"last_error":      "finalize failed: " + err.Error(),
			"next_retry_time": now.Add(30 * time.Second),
		})
		return
	}

	// 异步发送提现成功邮件通知（不阻塞响应）
	go w.sendWithdrawalSuccessEmail(ctx, order, task, actualAmountDec.String())

	// 交易金额预警检查
	go w.checkWithdrawAlert(ctx, order)
}

// checkWithdrawAlert 检查提现金额是否触发预警
func (w *WithdrawPayoutWorker) checkWithdrawAlert(ctx context.Context, order *model.CurrencyWithdrawOrderModel) {
	amount, err := decimal.NewFromString(strings.TrimSpace(order.Amount))
	if err != nil {
		logx.WithContext(ctx).Errorf("[Alert] Failed to parse withdraw amount: %v", err)
		return
	}

	alertChecker := alert.NewAlertChecker(w.svcCtx)
	_ = alertChecker.CheckAndNotify(
		ctx,
		"web3_withdraw", // Web3 链上提现
		strings.TrimSpace(order.AssetCode),
		amount,
		decimal.Zero, // 自动计算 USD 金额
		map[string]interface{}{
			"order_id":   order.ID,
			"user_id":    order.UserID,
			"chain_code": strings.TrimSpace(order.ChainCode),
			"to_address": strings.TrimSpace(order.ToAddress),
			"tx_hash": func() string {
				if order.TxHash != nil {
					return *order.TxHash
				}
				return ""
			}(),
		},
	)
}

// formatWithdrawErrorMessage 将技术错误信息转换为用户友好的中文错误信息
func formatWithdrawErrorMessage(reason string) string {
	reason = strings.ToLower(strings.TrimSpace(reason))
	if reason == "" {
		return "提现失败"
	}

	// Token 相关错误（优先检查，避免误判为手续费不足）
	if strings.Contains(reason, "token") || strings.Contains(reason, "contract") {
		// 主钱包没有对应的 token
		if strings.Contains(reason, "token not found") || strings.Contains(reason, "contract not found") ||
			strings.Contains(reason, "no token") || strings.Contains(reason, "token balance") ||
			strings.Contains(reason, "token contract") {
			return "主钱包没有对应的代币，无法完成转账"
		}
		// Token 余额不足
		if strings.Contains(reason, "insufficient") && (strings.Contains(reason, "token") || strings.Contains(reason, "balance")) {
			return "主钱包代币余额不足，无法完成转账"
		}
	}

	// 链上转账失败（RPC 错误）
	if strings.Contains(reason, "chain transfer failed") || strings.Contains(reason, "rpc error") {
		return "链上转账失败，请联系客服"
	}

	// 广播次数超限
	if strings.Contains(reason, "broadcast attempts exceeded") {
		return "交易广播失败，请联系客服"
	}

	// 链上交易失败
	if strings.Contains(reason, "on-chain tx failed") {
		return "链上交易失败"
	}

	// 签名失败
	if strings.Contains(reason, "sign") || strings.Contains(reason, "signature") {
		return "交易签名失败，请联系客服"
	}

	// 网络错误
	if strings.Contains(reason, "connection") || strings.Contains(reason, "network") || strings.Contains(reason, "timeout") {
		return "网络错误，请稍后重试"
	}

	// 余额不足（热钱包）- 需要先检查是否是 token 余额不足
	if strings.Contains(reason, "insufficient") {
		// 如果已经检查过 token 相关错误，这里只处理原生代币余额不足
		if !strings.Contains(reason, "token") && !strings.Contains(reason, "contract") {
			return "系统余额不足，请联系客服"
		}
	}

	// Gas 不足（需要排除 token 相关错误）
	if (strings.Contains(reason, "gas") || strings.Contains(reason, "fee")) &&
		!strings.Contains(reason, "token") && !strings.Contains(reason, "contract") {
		return "手续费不足"
	}

	// 地址无效
	if strings.Contains(reason, "invalid address") || strings.Contains(reason, "address") {
		return "提现地址无效"
	}

	// nonce 错误
	if strings.Contains(reason, "nonce") {
		return "交易处理中，请稍后查看"
	}

	// 默认错误
	return "提现失败，请联系客服"
}

func (w *WithdrawPayoutWorker) failAndUnfreeze(ctx context.Context, task *model.CurrencyWithdrawPayoutTaskModel, order *model.CurrencyWithdrawOrderModel, reason string) {
	now := time.Now()
	// 使用中文化的错误信息存储到数据库
	userFriendlyReason := formatWithdrawErrorMessage(reason)
	// 内扣模式：冻结的是 amount（不含 fee），所以解冻也是 amount
	// 使用 StringFixed(6) 确保输出最多 6 位小数（Accounting 服务精度限制）
	unfreezeAmount := strings.TrimSpace(order.Amount)
	if amountDec, parseErr := decimal.NewFromString(unfreezeAmount); parseErr == nil {
		unfreezeAmount = amountDec.StringFixed(6)
	}
	txHash := ""
	if task != nil && task.TxHash != nil {
		txHash = strings.TrimSpace(*task.TxHash)
	}
	if txHash == "" && order.TxHash != nil {
		txHash = strings.TrimSpace(*order.TxHash)
	}
	unfreezeReq := &pb.UnfreezeWithdrawRequest{
		IdempotencyKey: fmt.Sprintf("withdraw:unfreeze:%d", order.ID),
		BizRef:         fmt.Sprintf("%d", order.ID),
		UserId:         order.UserID,
		AssetCode:      strings.TrimSpace(order.AssetCode),
		AmountDecimal:  unfreezeAmount, // 内扣模式：解冻 amount（不含 fee）
		FinalStatus:    "failed",
		Reason:         userFriendlyReason, // 使用中文化的错误信息，显示给用户
		TxHash:         txHash,
	}
	unfreezeResp, unfreezeErr := callAccountingTxWithRetry(ctx, 3, func(ctx context.Context) (*pb.LedgerTxResponse, error) {
		return w.svcCtx.AccountingRpc.UnfreezeWithdraw(ctx, unfreezeReq)
	})
	appendWithdrawOrderEvent(ctx, w.svcCtx.CurrencyWithdrawOrderEventRepository, newWithdrawOrderEvent(order.ID, constants.WithdrawEventAssetUnfrozen, constants.WithdrawActorSystem, 0, nil, "Asset unfrozen", map[string]any{
		"success": unfreezeResp != nil && unfreezeResp.Success,
		"tx_id": func() int64 {
			if unfreezeResp == nil {
				return 0
			}
			return unfreezeResp.TxId
		}(),
		"duplicate": func() bool {
			if unfreezeResp == nil {
				return false
			}
			return unfreezeResp.Duplicate
		}(),
		"message": func() string {
			if unfreezeResp == nil {
				return ""
			}
			return strings.TrimSpace(unfreezeResp.Message)
		}(),
		"error": func() string {
			if unfreezeErr == nil {
				return ""
			}
			return strings.TrimSpace(unfreezeErr.Error())
		}(),
		"asset_code":      strings.TrimSpace(order.AssetCode),
		"amount_decimal":  unfreezeAmount,
		"idempotency_key": unfreezeReq.IdempotencyKey,
		"biz_ref":         strings.TrimSpace(unfreezeReq.BizRef),
	}))

	_ = w.svcCtx.CurrencyWithdrawOrderRepository.UpdateFields(ctx, order.ID, map[string]interface{}{
		"status":        "failed",
		"error_message": userFriendlyReason, // 使用中文化的错误信息
		"updated_at":    now,
		"updated_by":    0,
	})

	_ = w.svcCtx.CurrencyWithdrawPayoutTaskRepository.UpdateFields(ctx, task.WithdrawOrderID, map[string]interface{}{
		"state":           constants.WithdrawPayoutStateFailed,
		"last_error":      strings.TrimSpace(reason), // 内部日志保留原始错误
		"next_retry_time": now.Add(24 * time.Hour),
	})
}

func sumDecimalStrings(a, b string) (string, error) {
	if strings.TrimSpace(a) == "" {
		a = "0"
	}
	if strings.TrimSpace(b) == "" {
		b = "0"
	}
	da, err := decimal.NewFromString(strings.TrimSpace(a))
	if err != nil {
		return "", err
	}
	db, err := decimal.NewFromString(strings.TrimSpace(b))
	if err != nil {
		return "", err
	}
	return da.Add(db).String(), nil
}

func withdrawPayoutBackoff(attempt int, base, max time.Duration) time.Duration {
	if attempt <= 0 {
		attempt = 1
	}
	d := time.Duration(attempt) * base
	if d > max {
		return max
	}
	return d
}

// sendWithdrawalSuccessEmail 发送提现成功邮件通知
func (w *WithdrawPayoutWorker) sendWithdrawalSuccessEmail(ctx context.Context, order *model.CurrencyWithdrawOrderModel, task *model.CurrencyWithdrawPayoutTaskModel, actualAmount string) {
	if order == nil || w.svcCtx.UserAccountRepository == nil {
		logx.WithContext(ctx).Errorf("Cannot send withdrawal email: order or repository is nil")
		return
	}

	// 获取用户信息
	user, err := w.svcCtx.UserAccountRepository.GetByID(context.Background(), order.UserID)
	if err != nil || user == nil {
		logx.WithContext(ctx).Errorf("Failed to get user info for withdrawal email: user_id=%d error=%v", order.UserID, err)
		return
	}

	// 准备邮件数据
	username := strings.TrimSpace(user.Nickname)
	if username == "" {
		username = user.Email
	}
	uid := fmt.Sprintf("%d", order.UserID)

	// 格式化时间（UTC）
	withdrawalTime := time.Now().UTC().Format("2006-01-02 15:04:05")
	if order.TransferredAt != nil {
		withdrawalTime = order.TransferredAt.UTC().Format("2006-01-02 15:04:05")
	}

	txHash := ""
	if task.TxHash != nil {
		txHash = strings.TrimSpace(*task.TxHash)
	}
	toAddress := strings.TrimSpace(order.ToAddress)
	assetCode := strings.ToUpper(strings.TrimSpace(order.AssetCode))
	chainCode := strings.ToUpper(strings.TrimSpace(order.ChainCode))

	// 异步发送邮件（使用传入的实际到账金额，已经计算好并截断到6位小数）
	err = notify.SendWithdrawalSuccessEmailAsync(
		user.Email,
		username,
		uid,
		assetCode,
		notify.FormatAmount(actualAmount), // 格式化显示（去除尾部0）
		chainCode,
		withdrawalTime,
		txHash,
		toAddress,
	)

	if err != nil {
		logx.WithContext(ctx).Errorf("Failed to send withdrawal success email to %s: %v", user.Email, err)
	} else {
		logx.WithContext(ctx).Infof("Withdrawal success email sent successfully to %s, asset=%s, amount=%s", user.Email, assetCode, actualAmount)
	}
}

// convertToRawAmount 将人类可读格式的金额转换为原始单位（整数字符串）
// 例如：1.1 USDT (precision=6) -> 1100000
func convertToRawAmount(humanAmount string, precision int32) string {
	humanAmount = strings.TrimSpace(humanAmount)
	if humanAmount == "" {
		return "0"
	}

	// 解析金额
	amt, err := decimal.NewFromString(humanAmount)
	if err != nil {
		logx.Errorf("[convertToRawAmount] Failed to parse amount %s: %v", humanAmount, err)
		return "0"
	}

	// 乘以 10^precision
	multiplier := decimal.NewFromInt(10).Pow(decimal.NewFromInt(int64(precision)))
	rawAmt := amt.Mul(multiplier)

	// 截断小数部分，只保留整数
	rawAmtInt := rawAmt.Truncate(0)

	return rawAmtInt.String()
}
