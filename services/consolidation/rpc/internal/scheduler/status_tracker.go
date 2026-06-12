package scheduler

import (
	"context"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"internalwallet/proto/pb"
	"internalwallet/services/consolidation/rpc/internal/chainutil"
	"internalwallet/services/consolidation/rpc/internal/config"
	"internalwallet/services/consolidation/rpc/internal/models"
	"internalwallet/services/consolidation/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type StatusTracker struct {
	svcCtx *svc.ServiceContext
}

func NewStatusTracker(svcCtx *svc.ServiceContext) *StatusTracker {
	return &StatusTracker{svcCtx: svcCtx}
}

func (t *StatusTracker) Run(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	if t == nil || t.svcCtx == nil {
		logx.WithContext(ctx).Error("consolidation status tracker disabled: svcCtx not configured")
		return
	}
	cfg := t.svcCtx.Config.Consolidation
	if !cfg.Enabled {
		logx.WithContext(ctx).Info("consolidation status tracker disabled by config")
		return
	}
	if t.svcCtx.DB == nil || t.svcCtx.ConsolidationTaskRepo == nil || t.svcCtx.Chain == nil {
		logx.WithContext(ctx).Error("consolidation status tracker disabled: db/repository/chain nodes not configured")
		return
	}

	idle := time.Duration(cfg.StatusCheckInterval) * time.Second
	if idle <= 0 {
		idle = 30 * time.Second
	}
	lease := time.Duration(cfg.ClaimLeaseSeconds) * time.Second
	if lease <= 0 {
		lease = 60 * time.Second
	}

	var wg sync.WaitGroup
	for _, chain := range chainutil.SupportedChains() {
		workers := statusTrackerWorkersForChain(cfg, chain)
		if workers <= 0 {
			continue
		}
		for i := 0; i < workers; i++ {
			wg.Add(1)
			workerID := i
			chain := chain
			go func() {
				defer wg.Done()
				t.workerLoop(ctx, chain, workerID, lease, idle)
			}()
		}
	}

	logx.WithContext(ctx).Infof("consolidation status tracker started (idle=%s lease=%s)", idle, lease)
	wg.Wait()
	logx.WithContext(ctx).Info("consolidation status tracker stopped")
}

func statusTrackerWorkersForChain(cfg config.ConsolidationConfig, chain string) int {
	// Status tracker is mostly IO-bound; keep worker count modest.
	max := 1
	switch strings.ToUpper(strings.TrimSpace(chain)) {
	case "TRON":
		max = cfg.MaxConcurrentPerChain.TRON
	case "ETH":
		max = cfg.MaxConcurrentPerChain.ETH
	case "BSC":
		max = cfg.MaxConcurrentPerChain.BSC
	}
	if max <= 0 {
		return 0
	}
	if max > 3 {
		return 3
	}
	return max
}

func (t *StatusTracker) workerLoop(ctx context.Context, chain string, workerID int, lease time.Duration, idle time.Duration) {
	time.Sleep(time.Duration((time.Now().UnixNano()+int64(workerID))*71%250) * time.Millisecond)

	var emptyPolls int64
	var claimErrors int64
	var processed int64
	lastSummary := time.Now()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Idle gate: skip work when this chain has no consolidation-enabled assets.
		rt := t.svcCtx.Runtime()
		if thresholds, ok := rt.EffectiveMinBalanceThresholds[strings.ToUpper(strings.TrimSpace(chain))]; !ok || len(thresholds) == 0 {
			time.Sleep(idle)
			continue
		}

		now := time.Now().Local()
		claimStart := time.Now()
		items, err := t.svcCtx.ConsolidationTaskRepo.ClaimByStatuses(
			ctx,
			[]models.ConsolidationTaskStatus{models.ConsolidationTaskStatusInProgress, models.ConsolidationTaskStatusTimeout},
			chain,
			1,
			now,
			lease,
			t.svcCtx.InstanceID,
		)
		claimDur := time.Since(claimStart)
		if err != nil {
			claimErrors++
			logx.WithContext(ctx).Errorw("status tracker claim failed",
				append(workerLogFields(t.svcCtx, "status_tracker", workerID),
					logx.Field("event", "task_claim_failed"),
					logx.Field("chain", chain),
					logx.Field("duration_ms", claimDur.Milliseconds()),
					logx.Field("error", err),
				)...,
			)
			time.Sleep(idle)
			continue
		}
		if len(items) == 0 {
			emptyPolls++
			time.Sleep(idle)
		} else {
			task := items[0]
			processed++
			t.processOne(ctx, &task, now, workerID)
			_ = t.svcCtx.ConsolidationTaskRepo.ReleaseClaim(context.Background(), task.ID, t.svcCtx.InstanceID)
		}

		if time.Since(lastSummary) >= 60*time.Second {
			logx.WithContext(ctx).Infow("consolidation status tracker loop summary",
				append(workerLogFields(t.svcCtx, "status_tracker", workerID),
					logx.Field("event", "loop_summary"),
					logx.Field("chain", chain),
					logx.Field("processed", processed),
					logx.Field("empty_polls", emptyPolls),
					logx.Field("claim_errors", claimErrors),
				)...,
			)
			emptyPolls = 0
			claimErrors = 0
			processed = 0
			lastSummary = time.Now()
		}
	}
}

func (t *StatusTracker) processOne(ctx context.Context, task *models.ConsolidationTask, now time.Time, workerID int) {
	if task == nil {
		return
	}
	cfg := t.svcCtx.Config.Consolidation
	logger := logx.WithContext(ctx)

	if task.TxHash == nil || strings.TrimSpace(*task.TxHash) == "" {
		t.scheduleRetry(ctx, task, "missing tx_hash", workerID)
		return
	}

	chainEnum, err := chainutil.ChainStringToEnum(task.Chain)
	if err != nil {
		logx.WithContext(ctx).Errorf("invalid chain (task_id=%s): %v", task.TaskID, err)
		return
	}

	receiptStart := time.Now()
	receiptResp, err := t.svcCtx.Chain.GetTransactionReceipt(ctx, &pb.GetTransactionReceiptReq{
		Chain:  chainEnum,
		TxHash: strings.TrimSpace(*task.TxHash),
	})
	receiptDur := time.Since(receiptStart)
	if err != nil || receiptResp == nil || !receiptResp.Success {
		logger.Errorw("chain rpc GetTransactionReceipt failed",
			append(workerLogFields(t.svcCtx, "status_tracker", workerID),
				append(taskLogFields(task),
					logx.Field("event", "rpc_call"),
					logx.Field("rpc_method", "Chain.GetTransactionReceipt"),
					logx.Field("duration_ms", receiptDur.Milliseconds()),
					logx.Field("error", err),
					logx.Field("rpc_success", receiptResp != nil && receiptResp.Success),
					logx.Field("rpc_msg", safeMsg(receiptResp)),
				)...,
			)...,
		)
		// Cut off: abandon this tx_hash/signed_tx and retry the task (prevents indefinite address lock).
		if task.StartedAt != nil && cfg.MaxNoReceiptDuration > 0 {
			if now.Sub(*task.StartedAt) > time.Duration(cfg.MaxNoReceiptDuration)*time.Second {
				t.scheduleRetry(ctx, task, fmt.Sprintf("no receipt beyond %ds: %v %s", cfg.MaxNoReceiptDuration, err, safeMsg(receiptResp)), workerID)
				return
			}
		}

		t.maybeRebroadcast(ctx, task, chainEnum, now)

		// If it has been pending too long (receipt not found), mark Timeout but keep tracking.
		if task.Status == models.ConsolidationTaskStatusInProgress && task.StartedAt != nil && cfg.MaxPendingDuration > 0 {
			if now.Sub(*task.StartedAt) > time.Duration(cfg.MaxPendingDuration)*time.Second {
				ok, err := t.svcCtx.ConsolidationTaskRepo.MarkTimeout(ctx, task.ID, t.svcCtx.InstanceID, task.Version, "pending timeout")
				if err == nil && ok {
					writeStatusTransitionTaskLog(ctx, t.svcCtx, task, "WARN", models.ConsolidationTaskStatusInProgress, models.ConsolidationTaskStatusTimeout, "pending timeout", nil, map[string]any{
						"decision": "max_pending_duration",
					})
				} else if err != nil {
					logger.Errorw("mark timeout failed",
						append(workerLogFields(t.svcCtx, "status_tracker", workerID),
							append(taskLogFields(task),
								logx.Field("event", "task_transition_failed"),
								logx.Field("status_to", models.ConsolidationTaskStatusTimeout),
								logx.Field("error", err),
							)...,
						)...,
					)
				}
			}
		}
		return
	}
	if receiptDur > 1500*time.Millisecond {
		logger.Infow("chain rpc GetTransactionReceipt slow",
			append(workerLogFields(t.svcCtx, "status_tracker", workerID),
				append(taskLogFields(task),
					logx.Field("event", "rpc_slow"),
					logx.Field("rpc_method", "Chain.GetTransactionReceipt"),
					logx.Field("duration_ms", receiptDur.Milliseconds()),
				)...,
			)...,
		)
	}
	if receiptResp.Receipt == nil {
		// Cut off: abandon this tx_hash/signed_tx and retry the task (prevents indefinite address lock).
		if task.StartedAt != nil && cfg.MaxNoReceiptDuration > 0 {
			if now.Sub(*task.StartedAt) > time.Duration(cfg.MaxNoReceiptDuration)*time.Second {
				t.scheduleRetry(ctx, task, fmt.Sprintf("no receipt beyond %ds", cfg.MaxNoReceiptDuration), workerID)
				return
			}
		}

		// Receipt not found. For EVM chains, try to bump gas price (replacement tx with same nonce) after a grace period.
		switch chainEnum {
		case pb.ChainRpcType_CHAIN_TYPE_ETHEREUM, pb.ChainRpcType_CHAIN_TYPE_BSC:
			t.maybeBumpEvmGas(ctx, task, chainEnum, now)
		}
		t.maybeRebroadcast(ctx, task, chainEnum, now)

		// If it has been pending too long (receipt not found), mark Timeout but keep tracking.
		if task.Status == models.ConsolidationTaskStatusInProgress && task.StartedAt != nil && cfg.MaxPendingDuration > 0 {
			if now.Sub(*task.StartedAt) > time.Duration(cfg.MaxPendingDuration)*time.Second {
				ok, err := t.svcCtx.ConsolidationTaskRepo.MarkTimeout(ctx, task.ID, t.svcCtx.InstanceID, task.Version, "pending timeout")
				if err == nil && ok {
					writeStatusTransitionTaskLog(ctx, t.svcCtx, task, "WARN", models.ConsolidationTaskStatusInProgress, models.ConsolidationTaskStatusTimeout, "pending timeout", nil, map[string]any{
						"decision": "max_pending_duration",
					})
				} else if err != nil {
					logger.Errorw("mark timeout failed",
						append(workerLogFields(t.svcCtx, "status_tracker", workerID),
							append(taskLogFields(task),
								logx.Field("event", "task_transition_failed"),
								logx.Field("status_to", models.ConsolidationTaskStatusTimeout),
								logx.Field("error", err),
							)...,
						)...,
					)
				}
			}
		}
		return
	}

	rcpt := receiptResp.Receipt
	switch rcpt.Status {
	case pb.TxStatus_TX_STATUS_CONFIRMED:
		required := uint64(0)
		switch strings.ToUpper(strings.TrimSpace(task.Chain)) {
		case "TRON":
			required = cfg.RequiredConfirmations.TRON
		case "ETH":
			required = cfg.RequiredConfirmations.ETH
		case "BSC":
			required = cfg.RequiredConfirmations.BSC
		}

		ok, err := t.hasEnoughConfirmations(ctx, chainEnum, rcpt.BlockNumber, required)
		if err != nil {
			logx.WithContext(ctx).Errorf("confirmation check failed (task_id=%s): %v", task.TaskID, err)
			return
		}
		if !ok {
			return
		}

		var actualFee *string
		if strings.TrimSpace(rcpt.GasFee) != "" {
			v := strings.TrimSpace(rcpt.GasFee)
			actualFee = &v
		}
		confirmedAt := time.Now().Local()
		ok2, err := t.svcCtx.ConsolidationTaskRepo.MarkConfirmed(ctx, task.ID, t.svcCtx.InstanceID, task.Version, confirmedAt, actualFee)
		if err == nil && ok2 {
			writeStatusTransitionTaskLog(ctx, t.svcCtx, task, "INFO", task.Status, models.ConsolidationTaskStatusConfirmed, "task confirmed", nil, map[string]any{
				"confirmed_at": confirmedAt.Format(time.RFC3339),
				"actual_fee":   actualFee,
				"block_number": strings.TrimSpace(rcpt.BlockNumber),
			})
			logger.Infow("consolidation task confirmed",
				append(workerLogFields(t.svcCtx, "status_tracker", workerID),
					append(taskLogFields(task),
						logx.Field("event", "task_confirmed"),
						logx.Field("confirmed_at", confirmedAt.Format(time.RFC3339)),
					)...,
				)...,
			)
		} else if err != nil {
			logger.Errorw("mark confirmed failed",
				append(workerLogFields(t.svcCtx, "status_tracker", workerID),
					append(taskLogFields(task),
						logx.Field("event", "task_transition_failed"),
						logx.Field("status_to", models.ConsolidationTaskStatusConfirmed),
						logx.Field("error", err),
					)...,
				)...,
			)
		}
		return

	case pb.TxStatus_TX_STATUS_FAILED, pb.TxStatus_TX_STATUS_DROPPED, pb.TxStatus_TX_STATUS_REPLACED:
		msg := fmt.Sprintf("tx status=%s", rcpt.Status.String())
		// Treat FAILED as retryable too. Do not release the per-address lock by marking Failed, otherwise discovery can
		// create a brand-new task and bypass MaxRetries (fee burn loop).
		t.scheduleRetry(ctx, task, msg, workerID)
		return

	default:
		// Pending/unknown: if it has been pending too long, move to Timeout but keep tracking.
		if task.Status == models.ConsolidationTaskStatusInProgress && task.StartedAt != nil && cfg.MaxPendingDuration > 0 {
			if now.Sub(*task.StartedAt) > time.Duration(cfg.MaxPendingDuration)*time.Second {
				ok, err := t.svcCtx.ConsolidationTaskRepo.MarkTimeout(ctx, task.ID, t.svcCtx.InstanceID, task.Version, "pending timeout")
				if err == nil && ok {
					writeStatusTransitionTaskLog(ctx, t.svcCtx, task, "WARN", models.ConsolidationTaskStatusInProgress, models.ConsolidationTaskStatusTimeout, "pending timeout", nil, map[string]any{
						"decision": "max_pending_duration",
					})
				}
			}
		}
		return
	}
}

func (t *StatusTracker) maybeBumpEvmGas(ctx context.Context, task *models.ConsolidationTask, chainEnum pb.ChainRpcType, now time.Time) {
	if task == nil || t == nil || t.svcCtx == nil || t.svcCtx.ConsolidationTaskRepo == nil {
		return
	}
	cfg := t.svcCtx.Config.Consolidation
	if cfg.EvmMaxBumps == 0 || cfg.EvmBumpAfterSeconds <= 0 {
		return
	}
	if task.BumpCount >= int32(cfg.EvmMaxBumps) {
		return
	}
	if task.SignedTx == nil || strings.TrimSpace(*task.SignedTx) == "" {
		return
	}
	if task.StartedAt == nil {
		return
	}
	if now.Sub(*task.StartedAt) < time.Duration(cfg.EvmBumpAfterSeconds)*time.Second {
		return
	}
	if t.svcCtx.Chain == nil || t.svcCtx.SignerRpc == nil {
		return
	}

	// Decode current signed tx to get nonce/gas settings.
	signedTxBytes, err := hexutil.Decode(strings.TrimSpace(*task.SignedTx))
	if err != nil {
		logx.WithContext(ctx).Errorf("bump: decode signed tx failed (task_id=%s): %v", task.TaskID, err)
		return
	}
	tx := new(types.Transaction)
	if err := tx.UnmarshalBinary(signedTxBytes); err != nil {
		logx.WithContext(ctx).Errorf("bump: unmarshal signed tx failed (task_id=%s): %v", task.TaskID, err)
		return
	}
	curGasPrice := tx.GasPrice()
	if curGasPrice == nil || curGasPrice.Sign() <= 0 {
		return
	}

	newGasPrice, err := mulCeilBigInt(curGasPrice, cfg.EvmGasBumpMultiplier)
	if err != nil {
		logx.WithContext(ctx).Errorf("bump: calc new gas price failed (task_id=%s): %v", task.TaskID, err)
		return
	}
	if newGasPrice.Cmp(curGasPrice) <= 0 {
		newGasPrice = new(big.Int).Add(curGasPrice, big.NewInt(1))
	}

	// Cap by MaxGasPrice if configured.
	if maxStr := cfg.MaxGasPrice[strings.ToUpper(strings.TrimSpace(task.Chain))]; strings.TrimSpace(maxStr) != "" {
		if maxGP, err := parseBigInt10(maxStr); err == nil {
			if maxGP.Cmp(curGasPrice) <= 0 {
				// Already at/above max gas price; do not bump.
				return
			}
			if newGasPrice.Cmp(maxGP) > 0 {
				newGasPrice = maxGP
			}
		}
	}
	if newGasPrice.Cmp(curGasPrice) <= 0 {
		return
	}

	isToken := task.TokenContract != nil && strings.TrimSpace(*task.TokenContract) != ""

	// Ensure we can afford the bumped gas cost while keeping the configured reserve.
	balResp, err := t.svcCtx.Chain.GetBalance(ctx, &pb.GetBalanceReq{
		Chain:   chainEnum,
		Address: task.FromAddress,
	})
	if err != nil || balResp == nil || !balResp.Success {
		msg := fmt.Sprintf("bump GetBalance failed: %v %s", err, safeMsg(balResp))
		next := addPtrTime(now.Add(firstBackoff(cfg.RetryBackoffSeconds)))
		_, _ = t.svcCtx.ConsolidationTaskRepo.MarkFailed(ctx, task.ID, t.svcCtx.InstanceID, task.Version, models.ConsolidationTaskStatusInProgress, msg, next, task.RetryCount)
		return
	}
	nativeBal, err := parseBigInt10(balResp.Balance)
	if err != nil {
		logx.WithContext(ctx).Errorf("bump: parse balance failed (task_id=%s): %v", task.TaskID, err)
		return
	}

	reserve := big.NewInt(0)
	nativeSymbol := chainutil.NativeSymbolForChain(task.Chain)
	if !isToken && nativeSymbol != "" {
		if reservesChain, ok := cfg.NativeReserves[strings.ToUpper(strings.TrimSpace(task.Chain))]; ok {
			if v := reservesChain[strings.ToUpper(nativeSymbol)]; strings.TrimSpace(v) != "" {
				if r, err := parseBigInt10(v); err == nil {
					reserve = r
				}
			}
		}
	}

	gasCost := new(big.Int).Mul(newGasPrice, new(big.Int).SetUint64(tx.Gas()))
	availAfterReserve := new(big.Int).Sub(nativeBal, reserve)
	if availAfterReserve.Cmp(gasCost) < 0 {
		// Can't afford the bumped gas cost; keep tracking without bumping.
		return
	}

	amountToSend := strings.TrimSpace(task.Amount)
	if amountToSend == "" {
		return
	}

	// For native sweeps, reduce the send amount to fit the bumped gas cost (replacement tx must remain valid).
	if !isToken {
		send := new(big.Int).Sub(availAfterReserve, gasCost)
		if send.Sign() <= 0 {
			return
		}
		minStr := strings.TrimSpace(cfg.MinConsolidationAmount[strings.ToUpper(strings.TrimSpace(task.Chain))][strings.ToUpper(strings.TrimSpace(nativeSymbol))])
		if minStr != "" {
			ok, err := geBigIntString(send.String(), minStr)
			if err == nil && !ok {
				return
			}
		}
		amountToSend = send.String()
	}

	// Rebuild unsigned tx with same nonce/gas_limit and higher gas_price.
	buildReq := &pb.BuildTransactionReq{
		Chain:         chainEnum,
		FromAddress:   task.FromAddress,
		ToAddress:     task.ToAddress,
		Amount:        amountToSend,
		TokenContract: "",
		GasLimit:      tx.Gas(),
		GasPrice:      newGasPrice.String(),
		Nonce:         fmt.Sprintf("%d", tx.Nonce()),
	}
	if isToken {
		buildReq.TokenContract = strings.TrimSpace(*task.TokenContract)
	}

	buildResp, err := t.svcCtx.Chain.BuildTransaction(ctx, buildReq)
	if err != nil || buildResp == nil || !buildResp.Success || strings.TrimSpace(buildResp.RawTransaction) == "" {
		msg := fmt.Sprintf("bump BuildTransaction failed: %v %s", err, safeMsg(buildResp))
		next := addPtrTime(now.Add(firstBackoff(cfg.RetryBackoffSeconds)))
		// Keep tracking, but defer next attempt to avoid hot-looping.
		_, _ = t.svcCtx.ConsolidationTaskRepo.MarkFailed(ctx, task.ID, t.svcCtx.InstanceID, task.Version, models.ConsolidationTaskStatusInProgress, msg, next, task.RetryCount)
		return
	}

	requestID := fmt.Sprintf("consolidation_bump_%s_%d", task.TaskID, time.Now().UnixNano())
	signReq := &pb.SignTransactionRequest{
		RequestId:      requestID,
		Chain:          strings.ToUpper(strings.TrimSpace(task.Chain)),
		FromAddress:    task.FromAddress,
		RawTransaction: strings.TrimSpace(buildResp.RawTransaction),
		OperationType:  3,
		Amount:         amountToSend,
		ToAddress:      task.ToAddress,
		AssetSymbol:    strings.ToUpper(strings.TrimSpace(task.AssetSymbol)),
		Requester:      "consolidation_service",
	}
	if isToken {
		signReq.TokenContract = strings.TrimSpace(*task.TokenContract)
	}
	signResp, err := t.svcCtx.SignerRpc.SignTransaction(ctx, signReq)
	if err != nil || signResp == nil || signResp.Code != 0 || strings.TrimSpace(signResp.Signature) == "" {
		msg := fmt.Sprintf("bump SignTransaction failed: %v %s", err, safeSignerMsg(signResp))
		next := addPtrTime(now.Add(firstBackoff(cfg.RetryBackoffSeconds)))
		_, _ = t.svcCtx.ConsolidationTaskRepo.MarkFailed(ctx, task.ID, t.svcCtx.InstanceID, task.Version, models.ConsolidationTaskStatusInProgress, msg, next, task.RetryCount)
		return
	}

	newSignedTx := strings.TrimSpace(signResp.Signature)
	newTxHash, err := evmSignedTxHash(newSignedTx)
	if err != nil {
		msg := fmt.Sprintf("bump compute tx hash failed: %v", err)
		next := addPtrTime(now.Add(firstBackoff(cfg.RetryBackoffSeconds)))
		_, _ = t.svcCtx.ConsolidationTaskRepo.MarkFailed(ctx, task.ID, t.svcCtx.InstanceID, task.Version, models.ConsolidationTaskStatusInProgress, msg, next, task.RetryCount)
		return
	}

	var estimatedFee *string
	if strings.TrimSpace(buildResp.EstimatedFee) != "" {
		v := strings.TrimSpace(buildResp.EstimatedFee)
		estimatedFee = &v
	}

	msg := fmt.Sprintf("evm gas bump: %s -> %s", curGasPrice.String(), newGasPrice.String())
	startedAt := time.Now().Local()
	bumpCount := task.BumpCount + 1
	updated, err := t.svcCtx.ConsolidationTaskRepo.MarkRebroadcastInProgress(ctx, task.ID, t.svcCtx.InstanceID, task.Version, newTxHash, newSignedTx, startedAt, estimatedFee, amountToSend, msg, bumpCount)
	if err != nil || !updated {
		return
	}
	oldTxHash := ""
	if task.TxHash != nil {
		oldTxHash = strings.TrimSpace(*task.TxHash)
	}
	WriteTaskLog(ctx, t.svcCtx, task.TaskID, "INFO", TaskLogEventEvmGasBumped, msg, map[string]any{
		"chain":         strings.ToUpper(strings.TrimSpace(task.Chain)),
		"nonce":         tx.Nonce(),
		"gas_limit":     tx.Gas(),
		"old_tx_hash":   oldTxHash,
		"new_tx_hash":   newTxHash,
		"old_gas_price": curGasPrice.String(),
		"new_gas_price": newGasPrice.String(),
		"bump_count":    bumpCount,
	})

	bcResp, err := t.svcCtx.Chain.BroadcastTransaction(ctx, &pb.BroadcastTransactionReq{
		Chain:             chainEnum,
		SignedTransaction: newSignedTx,
		RequestId:         requestID,
		WaitForReceipt:    false,
		TimeoutSeconds:    30,
	})
	if err != nil || bcResp == nil || !bcResp.Success || strings.TrimSpace(bcResp.TxHash) == "" {
		logx.WithContext(ctx).Errorf("bump broadcast failed (task_id=%s chain=%s): %v %s", task.TaskID, task.Chain, err, safeMsg(bcResp))
		return
	}

	if !strings.EqualFold(strings.TrimSpace(bcResp.TxHash), strings.TrimSpace(newTxHash)) {
		logx.WithContext(ctx).Errorf("bump tx hash mismatch (task_id=%s chain=%s computed=%s broadcast=%s)", task.TaskID, task.Chain, newTxHash, bcResp.TxHash)
	}
}

func evmSignedTxHash(signedTx string) (string, error) {
	signedTxBytes, err := hexutil.Decode(strings.TrimSpace(signedTx))
	if err != nil {
		return "", fmt.Errorf("decode EVM signed tx: %w", err)
	}
	tx := new(types.Transaction)
	if err := tx.UnmarshalBinary(signedTxBytes); err != nil {
		return "", fmt.Errorf("unmarshal EVM signed tx: %w", err)
	}
	return tx.Hash().Hex(), nil
}

func (t *StatusTracker) maybeRebroadcast(ctx context.Context, task *models.ConsolidationTask, chainEnum pb.ChainRpcType, now time.Time) {
	if task == nil || task.SignedTx == nil || strings.TrimSpace(*task.SignedTx) == "" {
		return
	}
	// Avoid immediate rebroadcast right after marking InProgress.
	if task.StartedAt == nil || now.Sub(*task.StartedAt) < 10*time.Second {
		return
	}

	requestID := fmt.Sprintf("consolidation_rebroadcast_%s_%d", task.TaskID, time.Now().UnixNano())
	_, err := t.svcCtx.Chain.BroadcastTransaction(ctx, &pb.BroadcastTransactionReq{
		Chain:             chainEnum,
		SignedTransaction: strings.TrimSpace(*task.SignedTx),
		RequestId:         requestID,
		WaitForReceipt:    false,
		TimeoutSeconds:    30,
	})
	if err != nil {
		logx.WithContext(ctx).Errorf("rebroadcast failed (task_id=%s chain=%s): %v", task.TaskID, task.Chain, err)
	}
}

func (t *StatusTracker) hasEnoughConfirmations(ctx context.Context, chain pb.ChainRpcType, txBlock string, required uint64) (bool, error) {
	if required == 0 {
		return true, nil
	}
	txHeight, err := strconv.ParseUint(strings.TrimSpace(txBlock), 10, 64)
	if err != nil {
		// If block number is missing/unparseable, be conservative and keep pending.
		return false, nil
	}
	start := time.Now()
	h, err := t.svcCtx.Chain.GetBlockHeight(ctx, &pb.GetBlockHeightReq{Chain: chain})
	dur := time.Since(start)
	if err != nil || h == nil || !h.Success {
		logx.WithContext(ctx).Errorw("chain rpc GetBlockHeight failed",
			append(workerLogFields(t.svcCtx, "status_tracker", -1),
				logx.Field("event", "rpc_call"),
				logx.Field("rpc_method", "Chain.GetBlockHeight"),
				logx.Field("chain", chain),
				logx.Field("duration_ms", dur.Milliseconds()),
				logx.Field("error", err),
				logx.Field("rpc_success", h != nil && h.Success),
				logx.Field("rpc_msg", safeMsg(h)),
			)...,
		)
		return false, nil
	}
	if dur > 1500*time.Millisecond {
		logx.WithContext(ctx).Infow("chain rpc GetBlockHeight slow",
			append(workerLogFields(t.svcCtx, "status_tracker", -1),
				logx.Field("event", "rpc_slow"),
				logx.Field("rpc_method", "Chain.GetBlockHeight"),
				logx.Field("chain", chain),
				logx.Field("duration_ms", dur.Milliseconds()),
			)...,
		)
	}
	if h.BlockHeight < txHeight {
		return false, nil
	}
	confs := h.BlockHeight - txHeight + 1
	return confs >= required, nil
}

func (t *StatusTracker) scheduleRetry(ctx context.Context, task *models.ConsolidationTask, msg string, workerID int) {
	if task == nil {
		return
	}
	cfg := t.svcCtx.Config.Consolidation
	now := time.Now().Local()

	if int(task.RetryCount) >= cfg.MaxRetries {
		from := task.Status
		ok, err := t.svcCtx.ConsolidationTaskRepo.MarkFailed(ctx, task.ID, t.svcCtx.InstanceID, task.Version, models.ConsolidationTaskStatusPermanentFailed, msg, nil, task.RetryCount)
		if err == nil && ok {
			writeStatusTransitionTaskLog(ctx, t.svcCtx, task, "ERROR", from, models.ConsolidationTaskStatusPermanentFailed, msg, nil, map[string]any{
				"decision":    "status_tracker_retry_exhausted",
				"retry_count": task.RetryCount,
				"max_retries": cfg.MaxRetries,
			})
		} else if err != nil {
			logx.WithContext(ctx).Errorw("mark permanent failed failed",
				append(workerLogFields(t.svcCtx, "status_tracker", workerID),
					append(taskLogFields(task),
						logx.Field("event", "task_transition_failed"),
						logx.Field("status_to", models.ConsolidationTaskStatusPermanentFailed),
						logx.Field("error", err),
					)...,
				)...,
			)
		}
		return
	}

	nextRetryCount := task.RetryCount + 1
	delay := retryDelaySeconds(cfg.RetryBackoffSeconds, int(nextRetryCount))
	next := now.Add(time.Duration(delay) * time.Second)
	from := task.Status
	oldTxHash := ""
	if task.TxHash != nil {
		oldTxHash = strings.TrimSpace(*task.TxHash)
	}
	ok, err := t.svcCtx.ConsolidationTaskRepo.MarkRetryPending(ctx, task.ID, t.svcCtx.InstanceID, task.Version, msg, &next, nextRetryCount)
	if err == nil && ok {
		writeStatusTransitionTaskLog(ctx, t.svcCtx, task, "WARN", from, models.ConsolidationTaskStatusPending, msg, &next, map[string]any{
			"decision":         "retry_pending",
			"retry_count_from": task.RetryCount,
			"retry_count_to":   nextRetryCount,
			"max_retries":      cfg.MaxRetries,
			"old_tx_hash":      oldTxHash,
		})
		logx.WithContext(ctx).Infow("consolidation task retry scheduled",
			append(workerLogFields(t.svcCtx, "status_tracker", workerID),
				append(taskLogFields(task),
					logx.Field("event", "task_retry_scheduled"),
					logx.Field("next_attempt_at", next.Format(time.RFC3339)),
					logx.Field("retry_count_to", nextRetryCount),
					logx.Field("reason", msg),
				)...,
			)...,
		)
	} else if err != nil {
		logx.WithContext(ctx).Errorw("mark retry pending failed",
			append(workerLogFields(t.svcCtx, "status_tracker", workerID),
				append(taskLogFields(task),
					logx.Field("event", "task_transition_failed"),
					logx.Field("status_to", models.ConsolidationTaskStatusPending),
					logx.Field("error", err),
				)...,
			)...,
		)
	}
}
