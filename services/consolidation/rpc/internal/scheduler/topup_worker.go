package scheduler

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"internalwallet/proto/pb"
	"internalwallet/services/consolidation/rpc/internal/chainutil"
	"internalwallet/services/consolidation/rpc/internal/config"
	"internalwallet/services/consolidation/rpc/internal/models"
	"internalwallet/services/consolidation/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type TopUpWorker struct {
	svcCtx *svc.ServiceContext

	globalSem *Semaphore

	hotWalletMu    sync.Mutex
	hotWalletCache map[string]string // chain -> address
}

const tronActivationTopUpSun = "100000" // 0.1 TRX (SUN)

func NewTopUpWorker(svcCtx *svc.ServiceContext) *TopUpWorker {
	return &TopUpWorker{
		svcCtx:         svcCtx,
		hotWalletCache: make(map[string]string),
	}
}

func (w *TopUpWorker) Run(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	if w == nil || w.svcCtx == nil {
		logx.WithContext(ctx).Error("topup worker disabled: svcCtx not configured")
		return
	}

	cfg := w.svcCtx.Config.Consolidation
	if !cfg.Enabled || !cfg.TopUp.Enabled {
		logx.WithContext(ctx).Info("topup worker disabled by config")
		return
	}
	if w.svcCtx.DB == nil || w.svcCtx.ConsolidationTaskRepo == nil || w.svcCtx.TopUpRecordRepo == nil {
		logx.WithContext(ctx).Error("topup worker disabled: db/repository not configured")
		return
	}
	if w.svcCtx.Chain == nil || w.svcCtx.SignerRpc == nil {
		logx.WithContext(ctx).Error("topup worker disabled: chain nodes / SignerRpc not configured")
		return
	}

	w.globalSem = NewSemaphore(cfg.MaxConcurrentTasks)

	lease := time.Duration(cfg.ClaimLeaseSeconds) * time.Second
	if lease <= 0 {
		lease = 60 * time.Second
	}
	idle := time.Duration(cfg.TopUp.TopUpInterval) * time.Second
	if idle <= 0 {
		idle = 10 * time.Second
	}

	var wg sync.WaitGroup
	for _, chain := range chainutil.SupportedChains() {
		workers := topupWorkersForChain(cfg, chain)
		if workers <= 0 {
			continue
		}
		for i := 0; i < workers; i++ {
			wg.Add(1)
			workerID := i
			chain := chain
			go func() {
				defer wg.Done()
				w.workerLoop(ctx, chain, workerID, lease, idle)
			}()
		}
	}

	logx.WithContext(ctx).Infof("topup worker started (lease=%s idle=%s)", lease, idle)
	wg.Wait()
	logx.WithContext(ctx).Info("topup worker stopped")
}

func topupWorkersForChain(cfg config.ConsolidationConfig, chain string) int {
	max := 1
	switch strings.ToUpper(strings.TrimSpace(chain)) {
	case "TRON":
		max = cfg.MaxConcurrentPerChain.TRON
	case "ETH":
		max = cfg.MaxConcurrentPerChain.ETH
	case "BSC":
		max = cfg.MaxConcurrentPerChain.BSC
	default:
		return 0
	}
	if max <= 0 {
		return 0
	}
	// Top-ups are mostly IO-bound; keep worker count modest.
	if max > 3 {
		max = 3
	}
	return max
}

func (w *TopUpWorker) workerLoop(ctx context.Context, chain string, workerID int, lease time.Duration, idle time.Duration) {
	time.Sleep(time.Duration((time.Now().UnixNano()+int64(workerID))*61%250) * time.Millisecond)

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
		rt := w.svcCtx.Runtime()
		if thresholds, ok := rt.EffectiveMinBalanceThresholds[strings.ToUpper(strings.TrimSpace(chain))]; !ok || len(thresholds) == 0 {
			time.Sleep(idle)
			continue
		}

		release, ok := w.globalSem.Acquire(ctx)
		if !ok {
			return
		}

		now := time.Now().Local()
		claimStart := time.Now()
		tasks, err := w.svcCtx.ConsolidationTaskRepo.ClaimByStatuses(
			ctx,
			[]models.ConsolidationTaskStatus{models.ConsolidationTaskStatusNeedGas, models.ConsolidationTaskStatusNeedEnergy, models.ConsolidationTaskStatusNeedBandwidth},
			chain,
			1,
			now,
			lease,
			w.svcCtx.InstanceID,
		)
		claimDur := time.Since(claimStart)
		if err != nil {
			claimErrors++
			logx.WithContext(ctx).Errorw("topup claim failed",
				append(workerLogFields(w.svcCtx, "topup", workerID),
					logx.Field("event", "task_claim_failed"),
					logx.Field("chain", chain),
					logx.Field("duration_ms", claimDur.Milliseconds()),
					logx.Field("error", err),
				)...,
			)
			release()
			time.Sleep(idle)
			continue
		}
		if len(tasks) == 0 {
			emptyPolls++
			release()
			time.Sleep(idle)
		} else {
			task := tasks[0]
			processed++
			w.processOne(ctx, &task, workerID, lease)
			release()
		}

		if time.Since(lastSummary) >= 60*time.Second {
			logx.WithContext(ctx).Infow("topup worker loop summary",
				append(workerLogFields(w.svcCtx, "topup", workerID),
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

func (w *TopUpWorker) processOne(ctx context.Context, task *models.ConsolidationTask, workerID int, lease time.Duration) {
	if task == nil {
		return
	}

	taskCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	hbDone := make(chan struct{})
	go w.claimHeartbeat(taskCtx, hbDone, cancel, task.TaskID, task.ID, workerID, lease)
	defer close(hbDone)

	if err := w.handleTask(taskCtx, task, workerID); err != nil {
		logx.WithContext(taskCtx).Errorw("topup handle failed",
			append(workerLogFields(w.svcCtx, "topup", workerID),
				append(taskLogFields(task),
					logx.Field("event", "task_handle_failed"),
					logx.Field("error", err),
				)...,
			)...,
		)
	}

	_ = w.svcCtx.ConsolidationTaskRepo.ReleaseClaim(context.Background(), task.ID, w.svcCtx.InstanceID)
}

func (w *TopUpWorker) claimHeartbeat(ctx context.Context, done <-chan struct{}, cancel context.CancelFunc, taskID string, taskRowID int64, workerID int, lease time.Duration) {
	renewEvery := lease / 2
	if renewEvery < 5*time.Second {
		renewEvery = 5 * time.Second
	}
	t := time.NewTicker(renewEvery)
	defer t.Stop()

	for {
		select {
		case <-done:
			return
		case <-ctx.Done():
			return
		case <-t.C:
			now := time.Now().Local()
			ok, err := w.svcCtx.ConsolidationTaskRepo.RenewClaim(ctx, taskRowID, w.svcCtx.InstanceID, now, lease)
			if err != nil {
				logx.WithContext(ctx).Errorw("renew claim failed; cancel task",
					append(workerLogFields(w.svcCtx, "topup", workerID),
						logx.Field("event", "claim_renew_failed"),
						logx.Field("task_id", strings.TrimSpace(taskID)),
						logx.Field("task_row_id", taskRowID),
						logx.Field("error", err),
					)...,
				)
				cancel()
				return
			}
			if !ok {
				logx.WithContext(ctx).Infow("claim lost; cancel task",
					append(workerLogFields(w.svcCtx, "topup", workerID),
						logx.Field("event", "claim_lost"),
						logx.Field("task_id", strings.TrimSpace(taskID)),
						logx.Field("task_row_id", taskRowID),
					)...,
				)
				cancel()
				return
			}
		}
	}
}

func (w *TopUpWorker) handleTask(ctx context.Context, task *models.ConsolidationTask, workerID int) error {
	if task.TokenContract == nil || strings.TrimSpace(*task.TokenContract) == "" {
		// Native top-up is prohibited (never fund native sweeps).
		msg := "native top-up prohibited"
		from := task.Status
		ok, err := w.svcCtx.ConsolidationTaskRepo.MarkFailed(ctx, task.ID, w.svcCtx.InstanceID, task.Version, models.ConsolidationTaskStatusPermanentFailed, msg, nil, task.RetryCount)
		if err == nil && ok {
			writeStatusTransitionTaskLog(ctx, w.svcCtx, task, "ERROR", from, models.ConsolidationTaskStatusPermanentFailed, msg, nil, map[string]any{
				"decision": "native_topup_prohibited",
			})
		}
		return nil
	}

	switch task.Status {
	case models.ConsolidationTaskStatusNeedGas:
		return w.handleNeedGas(ctx, task, workerID)
	case models.ConsolidationTaskStatusNeedEnergy:
		return w.handleNeedEnergy(ctx, task, workerID)
	case models.ConsolidationTaskStatusNeedBandwidth:
		return w.handleNeedBandwidth(ctx, task, workerID)
	default:
		return nil
	}
}

func (w *TopUpWorker) handleNeedEnergy(ctx context.Context, task *models.ConsolidationTask, workerID int) error {
	cfg := w.svcCtx.Config.Consolidation
	topCfg := cfg.TopUp

	if strings.ToUpper(strings.TrimSpace(task.Chain)) != "TRON" {
		msg := fmt.Sprintf("NeedEnergy is only supported on TRON (chain=%s)", task.Chain)
		from := task.Status
		ok, err := w.svcCtx.ConsolidationTaskRepo.MarkFailed(ctx, task.ID, w.svcCtx.InstanceID, task.Version, models.ConsolidationTaskStatusPermanentFailed, msg, nil, task.RetryCount)
		if err == nil && ok {
			writeStatusTransitionTaskLog(ctx, w.svcCtx, task, "ERROR", from, models.ConsolidationTaskStatusPermanentFailed, msg, nil, nil)
		}
		return nil
	}

	// If deposit address isn't activated, fund activation first (separate record).
	res, err := w.svcCtx.Chain.GetTronAccountResources(ctx, &pb.GetTronAccountResourcesReq{Address: task.FromAddress})
	if err != nil || res == nil || !res.Success {
		msg := fmt.Sprintf("GetTronAccountResources failed: %v %s", err, safeMsg(res))
		next := addPtrTime(time.Now().Local().Add(time.Duration(topCfg.TaskRetryDelaySeconds) * time.Second))
		_, _ = w.svcCtx.ConsolidationTaskRepo.MarkNeedEnergy(ctx, task.ID, w.svcCtx.InstanceID, task.Version, msg, next)
		return nil
	}
	if !res.IsActivated {
		txHash, err := w.sendTopUp(ctx, task, "activation", "TRX", tronActivationTopUpSun, workerID)
		if err != nil {
			return err
		}
		if txHash == "" {
			// Active record already exists or task was blocked by risk controls.
			next := addPtrTime(time.Now().Local().Add(firstBackoff(topCfg.RetryBackoffSeconds)))
			_, _ = w.svcCtx.ConsolidationTaskRepo.MarkNeedEnergy(ctx, task.ID, w.svcCtx.InstanceID, task.Version, "activation top-up already in progress", next)
			return nil
		}
		// Block other NeedEnergy workers while activation is pending.
		next := addPtrTime(time.Now().Local().Add(firstBackoff(topCfg.RetryBackoffSeconds)))
		_, _ = w.svcCtx.ConsolidationTaskRepo.MarkNeedEnergy(ctx, task.ID, w.svcCtx.InstanceID, task.Version, "activation top-up sent", next)
		return nil
	}

	// Activated: let energy rental worker continue.
	return nil
}

func (w *TopUpWorker) handleNeedGas(ctx context.Context, task *models.ConsolidationTask, workerID int) error {
	cfg := w.svcCtx.Config.Consolidation
	topCfg := cfg.TopUp

	// If precheck already passes (e.g., manual top-up), resume task.
	needGas, shortfall, reason, err := w.stillNeedsGas(ctx, task)
	if err == nil && !needGas {
		from := task.Status
		ok, err := w.svcCtx.ConsolidationTaskRepo.MarkPending(ctx, task.ID, w.svcCtx.InstanceID, task.Version, nil)
		if err == nil && ok {
			writeStatusTransitionTaskLog(ctx, w.svcCtx, task, "INFO", from, models.ConsolidationTaskStatusPending, "top-up not needed anymore; moved to Pending", nil, nil)
		}
		return nil
	}
	if err != nil {
		msg := fmt.Sprintf("precheck failed: %v", err)
		next := addPtrTime(time.Now().Local().Add(time.Duration(topCfg.TaskRetryDelaySeconds) * time.Second))
		_, _ = w.svcCtx.ConsolidationTaskRepo.MarkNeedGas(ctx, task.ID, w.svcCtx.InstanceID, task.Version, msg, next)
		return nil
	}

	chain := strings.ToUpper(strings.TrimSpace(task.Chain))
	native := strings.ToUpper(strings.TrimSpace(chainutil.NativeSymbolForChain(chain)))
	amount := strings.TrimSpace(shortfall)
	if amount == "" {
		msg := fmt.Sprintf("cannot compute top-up shortfall (chain=%s native=%s)", chain, native)
		next := addPtrTime(time.Now().Local().Add(firstBackoff(topCfg.RetryBackoffSeconds)))
		_, _ = w.svcCtx.ConsolidationTaskRepo.MarkNeedGas(ctx, task.ID, w.svcCtx.InstanceID, task.Version, msg, next)
		return nil
	}

	// Enforce risk cap early: if shortfall exceeds max, permanently fail the task.
	maxStr := strings.TrimSpace(cfg.Risk.MaxNativeSpendPerTask[chain])
	if maxStr != "" {
		maxAllowed, mErr := parseBigInt10(maxStr)
		sf, sErr := parseBigInt10(amount)
		if mErr == nil && sErr == nil && maxAllowed.Sign() > 0 && sf.Cmp(maxAllowed) > 0 {
			msg := fmt.Sprintf("required native top-up exceeds risk cap: shortfall=%s max=%s", amount, maxStr)
			from := task.Status
			ok, err := w.svcCtx.ConsolidationTaskRepo.MarkFailed(ctx, task.ID, w.svcCtx.InstanceID, task.Version, models.ConsolidationTaskStatusPermanentFailed, msg, nil, task.RetryCount)
			if err == nil && ok {
				writeStatusTransitionTaskLog(ctx, w.svcCtx, task, "ERROR", from, models.ConsolidationTaskStatusPermanentFailed, msg, nil, map[string]any{
					"decision":    "topup_over_risk_cap",
					"shortfall":   amount,
					"max_allowed": maxStr,
					"reason":      reason,
				})
			}
			return nil
		}
	}

	// TRON: optionally fund activation first if needed.
	if chain == "TRON" {
		res, err := w.svcCtx.Chain.GetTronAccountResources(ctx, &pb.GetTronAccountResourcesReq{Address: task.FromAddress})
		if err == nil && res != nil && res.Success && !res.IsActivated {
			txHash, err := w.sendTopUp(ctx, task, "activation", "TRX", tronActivationTopUpSun, workerID)
			if err != nil {
				return err
			}
			if txHash == "" {
				next := addPtrTime(time.Now().Local().Add(firstBackoff(topCfg.RetryBackoffSeconds)))
				_, _ = w.svcCtx.ConsolidationTaskRepo.MarkNeedGas(ctx, task.ID, w.svcCtx.InstanceID, task.Version, "activation top-up already in progress", next)
				return nil
			}
			next := addPtrTime(time.Now().Local().Add(firstBackoff(topCfg.RetryBackoffSeconds)))
			_, _ = w.svcCtx.ConsolidationTaskRepo.MarkNeedGas(ctx, task.ID, w.svcCtx.InstanceID, task.Version, "activation top-up sent", next)
			return nil
		}
	}

	txHash, err := w.sendTopUp(ctx, task, "gas", native, amount, workerID)
	if err != nil {
		return err
	}
	if txHash == "" {
		// Active record already exists or task was blocked by risk controls.
		next := addPtrTime(time.Now().Local().Add(firstBackoff(topCfg.RetryBackoffSeconds)))
		_, _ = w.svcCtx.ConsolidationTaskRepo.MarkNeedGas(ctx, task.ID, w.svcCtx.InstanceID, task.Version, "top-up already in progress", next)
		return nil
	}
	msg := fmt.Sprintf("gas top-up sent (%s %s)", native, amount)
	next := addPtrTime(time.Now().Local().Add(firstBackoff(topCfg.RetryBackoffSeconds)))
	_, _ = w.svcCtx.ConsolidationTaskRepo.MarkNeedGas(ctx, task.ID, w.svcCtx.InstanceID, task.Version, msg, next)
	WriteTaskLog(ctx, w.svcCtx, task.TaskID, "INFO", TaskLogEventTopUpBroadcasted, msg, map[string]any{
		"native_symbol": native,
		"amount":        amount,
		"reason":        reason,
		"tx_hash":       txHash,
	})
	return nil
}

func (w *TopUpWorker) handleNeedBandwidth(ctx context.Context, task *models.ConsolidationTask, workerID int) error {
	cfg := w.svcCtx.Config.Consolidation
	topCfg := cfg.TopUp

	if strings.ToUpper(strings.TrimSpace(task.Chain)) != "TRON" {
		msg := fmt.Sprintf("NeedBandwidth is only supported on TRON (chain=%s)", task.Chain)
		from := task.Status
		ok, err := w.svcCtx.ConsolidationTaskRepo.MarkFailed(ctx, task.ID, w.svcCtx.InstanceID, task.Version, models.ConsolidationTaskStatusPermanentFailed, msg, nil, task.RetryCount)
		if err == nil && ok {
			writeStatusTransitionTaskLog(ctx, w.svcCtx, task, "ERROR", from, models.ConsolidationTaskStatusPermanentFailed, msg, nil, nil)
		}
		return nil
	}

	// If bandwidth cushion is now sufficient (e.g., manual top-up), resume energy-rental flow.
	needBandwidth, shortfall, reason, err := w.stillNeedsBandwidth(ctx, task)
	if err == nil && !needBandwidth {
		from := task.Status
		ok, err := w.svcCtx.ConsolidationTaskRepo.MarkNeedEnergy(ctx, task.ID, w.svcCtx.InstanceID, task.Version, "bandwidth top-up not needed anymore; moved to NeedEnergy", nil)
		if err == nil && ok {
			writeStatusTransitionTaskLog(ctx, w.svcCtx, task, "INFO", from, models.ConsolidationTaskStatusNeedEnergy, "bandwidth top-up not needed anymore; moved to NeedEnergy", nil, map[string]any{
				"reason": reason,
			})
		}
		return nil
	}
	if err != nil {
		msg := fmt.Sprintf("precheck failed: %v", err)
		next := addPtrTime(time.Now().Local().Add(time.Duration(topCfg.TaskRetryDelaySeconds) * time.Second))
		_, _ = w.svcCtx.ConsolidationTaskRepo.MarkNeedBandwidth(ctx, task.ID, w.svcCtx.InstanceID, task.Version, msg, next)
		return nil
	}

	amount := strings.TrimSpace(shortfall)
	if amount == "" {
		msg := "cannot compute bandwidth top-up shortfall"
		next := addPtrTime(time.Now().Local().Add(firstBackoff(topCfg.RetryBackoffSeconds)))
		_, _ = w.svcCtx.ConsolidationTaskRepo.MarkNeedBandwidth(ctx, task.ID, w.svcCtx.InstanceID, task.Version, msg, next)
		return nil
	}

	txHash, err := w.sendTopUp(ctx, task, "bandwidth", "TRX", amount, workerID)
	if err != nil {
		return err
	}
	if txHash == "" {
		// Active record already exists or task was blocked by risk controls.
		next := addPtrTime(time.Now().Local().Add(firstBackoff(topCfg.RetryBackoffSeconds)))
		_, _ = w.svcCtx.ConsolidationTaskRepo.MarkNeedBandwidth(ctx, task.ID, w.svcCtx.InstanceID, task.Version, "bandwidth top-up already in progress", next)
		return nil
	}

	msg := fmt.Sprintf("bandwidth top-up sent (TRX %s)", amount)
	next := addPtrTime(time.Now().Local().Add(firstBackoff(topCfg.RetryBackoffSeconds)))
	_, _ = w.svcCtx.ConsolidationTaskRepo.MarkNeedBandwidth(ctx, task.ID, w.svcCtx.InstanceID, task.Version, msg, next)
	WriteTaskLog(ctx, w.svcCtx, task.TaskID, "INFO", TaskLogEventTopUpBroadcasted, msg, map[string]any{
		"native_symbol": "TRX",
		"amount":        amount,
		"reason":        reason,
		"tx_hash":       txHash,
		"purpose":       "bandwidth",
	})
	return nil
}

func (w *TopUpWorker) stillNeedsBandwidth(ctx context.Context, task *models.ConsolidationTask) (needBandwidth bool, shortfall string, reason string, err error) {
	if task == nil {
		return false, "", "", fmt.Errorf("task is nil")
	}
	if strings.ToUpper(strings.TrimSpace(task.Chain)) != "TRON" {
		return true, "", "unsupported chain", fmt.Errorf("unsupported chain: %s", task.Chain)
	}

	executor := NewExecutor(w.svcCtx)
	amount := strings.TrimSpace(task.Amount)
	if amount == "" {
		amount = "0"
	}
	prep, err := executor.prepareTronPrecheck(ctx, task, amount)
	if err != nil {
		return true, "", "precheck error", err
	}
	if prep.needBandwidth {
		sf := strings.TrimSpace(prep.shortfall)
		if sf == "" {
			sf = "0"
		}
		r := strings.TrimSpace(prep.bandwidthMsg)
		if r == "" {
			r = "insufficient bandwidth; need TRX top-up"
		}
		return true, sf, r, nil
	}
	return false, "", "", nil
}

func (w *TopUpWorker) stillNeedsGas(ctx context.Context, task *models.ConsolidationTask) (needGas bool, shortfall string, reason string, err error) {
	if task == nil {
		return false, "", "", fmt.Errorf("task is nil")
	}

	executor := NewExecutor(w.svcCtx)
	chainEnum, err := chainutil.ChainStringToEnum(task.Chain)
	if err != nil {
		return true, "", "invalid chain", err
	}

	amount := strings.TrimSpace(task.Amount)
	if amount == "" {
		amount = "0"
	}

	switch chainEnum {
	case pb.ChainRpcType_CHAIN_TYPE_ETHEREUM, pb.ChainRpcType_CHAIN_TYPE_BSC:
		prep, err := executor.prepareEvmAmounts(ctx, task, amount)
		if err != nil {
			return true, "", "precheck error", err
		}
		if prep.needGas {
			sf := strings.TrimSpace(prep.shortfall)
			if sf == "" {
				sf = "0"
			}
			return true, sf, "insufficient native balance for gas", nil
		}
		return false, "", "", nil
	case pb.ChainRpcType_CHAIN_TYPE_TRON:
		prep, err := executor.prepareTronPrecheck(ctx, task, amount)
		if err != nil {
			return true, "", "precheck error", err
		}
		if prep.needGas {
			sf := strings.TrimSpace(prep.shortfall)
			if sf == "" {
				sf = "0"
			}
			return true, sf, prep.needGasReason, nil
		}
		return false, "", "", nil
	default:
		return true, "", "unsupported chain", fmt.Errorf("unsupported chain enum: %v", chainEnum)
	}
}

func (w *TopUpWorker) sendTopUp(ctx context.Context, task *models.ConsolidationTask, purpose string, nativeSymbol string, amount string, workerID int) (string, error) {
	cfg := w.svcCtx.Config.Consolidation
	topCfg := cfg.TopUp

	purpose = strings.ToLower(strings.TrimSpace(purpose))
	nativeSymbol = strings.ToUpper(strings.TrimSpace(nativeSymbol))
	amount = strings.TrimSpace(amount)
	logger := logx.WithContext(ctx)
	if purpose == "" {
		return "", fmt.Errorf("purpose is required")
	}
	if nativeSymbol == "" {
		return "", fmt.Errorf("nativeSymbol is required")
	}
	if amount == "" {
		return "", fmt.Errorf("amount is required")
	}

	chain := strings.ToUpper(strings.TrimSpace(task.Chain))
	toAddr := strings.TrimSpace(task.FromAddress)
	if toAddr == "" {
		return "", fmt.Errorf("task.FromAddress is empty")
	}

	base := append(workerLogFields(w.svcCtx, "topup", workerID),
		append(taskLogFields(task),
			logx.Field("topup_purpose", purpose),
			logx.Field("topup_asset_symbol", nativeSymbol),
			logx.Field("topup_amount", amount),
			logx.Field("topup_to_address", toAddr),
		)...,
	)

	// Risk controls (per task): attempts + total cost.
	history, err := w.svcCtx.TopUpRecordRepo.ListByTaskID(ctx, task.TaskID)
	if err != nil {
		return "", err
	}

	attempts := 0
	totalCost := big.NewInt(0)
	for i := range history {
		rec := history[i]
		attempts++
		if rec.Status == models.ConsolidationTopUpStatusFailed {
			continue
		}
		v, err := parseBigInt10(rec.Amount)
		if err == nil {
			totalCost.Add(totalCost, v)
		}
		// If there is already an active record for this purpose, do not create another.
		if strings.EqualFold(rec.Purpose, purpose) && rec.Status != models.ConsolidationTopUpStatusFailed && rec.Status != models.ConsolidationTopUpStatusConfirmed {
			return "", nil
		}
	}
	if topCfg.MaxTopUpAttemptsPerTask > 0 && attempts >= topCfg.MaxTopUpAttemptsPerTask {
		msg := fmt.Sprintf("top-up exceeded max attempts per task: attempts=%d max=%d", attempts, topCfg.MaxTopUpAttemptsPerTask)
		from := task.Status
		ok, err := w.svcCtx.ConsolidationTaskRepo.MarkFailed(ctx, task.ID, w.svcCtx.InstanceID, task.Version, models.ConsolidationTaskStatusPermanentFailed, msg, nil, task.RetryCount)
		if err == nil && ok {
			writeStatusTransitionTaskLog(ctx, w.svcCtx, task, "ERROR", from, models.ConsolidationTaskStatusPermanentFailed, msg, nil, map[string]any{
				"attempts": attempts,
				"purpose":  purpose,
			})
		}
		return "", nil
	}
	if maxCostStr := strings.TrimSpace(cfg.Risk.MaxNativeSpendPerTask[chain]); maxCostStr != "" {
		maxCost, err := parseBigInt10(maxCostStr)
		if err == nil {
			nextCost, err := parseBigInt10(amount)
			if err == nil {
				sum := new(big.Int).Add(totalCost, nextCost)
				if sum.Cmp(maxCost) > 0 {
					msg := fmt.Sprintf("top-up exceeded max total cost per task: total=%s next=%s max=%s", totalCost.String(), nextCost.String(), maxCost.String())
					from := task.Status
					ok, err := w.svcCtx.ConsolidationTaskRepo.MarkFailed(ctx, task.ID, w.svcCtx.InstanceID, task.Version, models.ConsolidationTaskStatusPermanentFailed, msg, nil, task.RetryCount)
					if err == nil && ok {
						writeStatusTransitionTaskLog(ctx, w.svcCtx, task, "ERROR", from, models.ConsolidationTaskStatusPermanentFailed, msg, nil, map[string]any{
							"total_cost": totalCost.String(),
							"next_cost":  nextCost.String(),
							"max_cost":   maxCost.String(),
							"purpose":    purpose,
						})
					}
					return "", nil
				}
			}
		}
	}

	hot, err := w.getHotWalletAddress(ctx, chain, workerID)
	if err != nil {
		return "", err
	}
	if hot == "" {
		return "", fmt.Errorf("hot wallet not found for chain=%s", chain)
	}
	base = append(base, logx.Field("topup_from_address", hot))

	// Create record (idempotent).
	rec := &models.ConsolidationTopUpRecord{
		TaskID:      task.TaskID,
		Chain:       chain,
		AssetSymbol: nativeSymbol,
		FromAddress: hot,
		ToAddress:   toAddr,
		Amount:      amount,
		Purpose:     purpose,
		Status:      models.ConsolidationTopUpStatusPending,
	}
	created, err := w.svcCtx.TopUpRecordRepo.CreateIgnoreDuplicate(ctx, rec)
	if err != nil {
		return "", err
	}
	if !created {
		// Another worker already created an active record.
		return "", nil
	}

	chainEnum, err := chainutil.ChainStringToEnum(chain)
	if err != nil {
		_ = w.svcCtx.TopUpRecordRepo.IncrementRetry(ctx, rec.ID)
		_ = w.svcCtx.TopUpRecordRepo.MarkFailed(ctx, rec.ID, fmt.Sprintf("invalid chain: %v", err))
		return "", err
	}

	// Build unsigned tx.
	var rawTx string
	if chainEnum == pb.ChainRpcType_CHAIN_TYPE_TRON {
		buildStart := time.Now()
		buildResp, err := w.svcCtx.Chain.BuildTronTransaction(ctx, &pb.ChainRpcBuildTronTransactionReq{
			FromAddress:     hot,
			ToAddress:       toAddr,
			Amount:          amount,
			ContractAddress: "",
		})
		buildDur := time.Since(buildStart)
		if err != nil || buildResp == nil || !buildResp.Success {
			logger.Errorw("chain rpc BuildTronTransaction failed",
				append(base,
					logx.Field("event", "rpc_call"),
					logx.Field("rpc_method", "Chain.BuildTronTransaction"),
					logx.Field("duration_ms", buildDur.Milliseconds()),
					logx.Field("error", err),
					logx.Field("rpc_success", buildResp != nil && buildResp.Success),
					logx.Field("rpc_msg", safeMsg(buildResp)),
					logx.Field("purpose", purpose),
					logx.Field("topup_record_id", rec.ID),
				)...,
			)
			_ = w.svcCtx.TopUpRecordRepo.IncrementRetry(ctx, rec.ID)
			_ = w.svcCtx.TopUpRecordRepo.MarkFailed(ctx, rec.ID, fmt.Sprintf("BuildTronTransaction failed: %v %s", err, safeMsg(buildResp)))
			return "", fmt.Errorf("BuildTronTransaction failed: %v %s", err, safeMsg(buildResp))
		}
		if buildDur > 1500*time.Millisecond {
			logger.Infow("chain rpc BuildTronTransaction slow",
				append(base,
					logx.Field("event", "rpc_slow"),
					logx.Field("rpc_method", "Chain.BuildTronTransaction"),
					logx.Field("duration_ms", buildDur.Milliseconds()),
					logx.Field("purpose", purpose),
					logx.Field("topup_record_id", rec.ID),
				)...,
			)
		}
		rawTx = buildResp.RawData
	} else {
		buildStart := time.Now()
		buildResp, err := w.svcCtx.Chain.BuildTransaction(ctx, &pb.BuildTransactionReq{
			Chain:         chainEnum,
			FromAddress:   hot,
			ToAddress:     toAddr,
			Amount:        amount,
			TokenContract: "",
			GasLimit:      0,
			GasPrice:      "",
			Nonce:         "",
		})
		buildDur := time.Since(buildStart)
		if err != nil || buildResp == nil || !buildResp.Success {
			logger.Errorw("chain rpc BuildTransaction failed",
				append(base,
					logx.Field("event", "rpc_call"),
					logx.Field("rpc_method", "Chain.BuildTransaction"),
					logx.Field("duration_ms", buildDur.Milliseconds()),
					logx.Field("error", err),
					logx.Field("rpc_success", buildResp != nil && buildResp.Success),
					logx.Field("rpc_msg", safeMsg(buildResp)),
					logx.Field("purpose", purpose),
					logx.Field("topup_record_id", rec.ID),
				)...,
			)
			_ = w.svcCtx.TopUpRecordRepo.IncrementRetry(ctx, rec.ID)
			_ = w.svcCtx.TopUpRecordRepo.MarkFailed(ctx, rec.ID, fmt.Sprintf("BuildTransaction failed: %v %s", err, safeMsg(buildResp)))
			return "", fmt.Errorf("BuildTransaction failed: %v %s", err, safeMsg(buildResp))
		}
		if buildDur > 1500*time.Millisecond {
			logger.Infow("chain rpc BuildTransaction slow",
				append(base,
					logx.Field("event", "rpc_slow"),
					logx.Field("rpc_method", "Chain.BuildTransaction"),
					logx.Field("duration_ms", buildDur.Milliseconds()),
					logx.Field("purpose", purpose),
					logx.Field("topup_record_id", rec.ID),
				)...,
			)
		}
		rawTx = buildResp.RawTransaction
	}

	// Sign.
	requestID := fmt.Sprintf("consolidation_topup_%s_%d", task.TaskID, time.Now().UnixNano())
	signStart := time.Now()
	signResp, err := w.svcCtx.SignerRpc.SignTransaction(ctx, &pb.SignTransactionRequest{
		RequestId:      requestID,
		Chain:          chain,
		FromAddress:    hot,
		RawTransaction: rawTx,
		OperationType:  3, // consolidation
		Amount:         amount,
		ToAddress:      toAddr,
		AssetSymbol:    nativeSymbol,
		Requester:      "consolidation_service_topup",
	})
	signDur := time.Since(signStart)
	if err != nil || signResp == nil || signResp.Code != 0 || strings.TrimSpace(signResp.Signature) == "" {
		logger.Errorw("signer rpc SignTransaction failed",
			append(base,
				logx.Field("event", "rpc_call"),
				logx.Field("rpc_method", "Signer.SignTransaction"),
				logx.Field("duration_ms", signDur.Milliseconds()),
				logx.Field("request_id", requestID),
				logx.Field("error", err),
				logx.Field("signer_code", func() int32 {
					if signResp == nil {
						return 0
					}
					return signResp.Code
				}()),
				logx.Field("signer_msg", safeSignerMsg(signResp)),
				logx.Field("purpose", purpose),
				logx.Field("topup_record_id", rec.ID),
			)...,
		)
		_ = w.svcCtx.TopUpRecordRepo.IncrementRetry(ctx, rec.ID)
		_ = w.svcCtx.TopUpRecordRepo.MarkFailed(ctx, rec.ID, fmt.Sprintf("SignTransaction failed: %v %s", err, safeSignerMsg(signResp)))
		return "", fmt.Errorf("SignTransaction failed: %v %s", err, safeSignerMsg(signResp))
	}
	if signDur > 1500*time.Millisecond {
		logger.Infow("signer rpc SignTransaction slow",
			append(base,
				logx.Field("event", "rpc_slow"),
				logx.Field("rpc_method", "Signer.SignTransaction"),
				logx.Field("duration_ms", signDur.Milliseconds()),
				logx.Field("request_id", requestID),
				logx.Field("purpose", purpose),
				logx.Field("topup_record_id", rec.ID),
			)...,
		)
	}

	signedTx := strings.TrimSpace(signResp.Signature)
	txHash, err := signedTxHash(chainEnum, signedTx)
	if err != nil {
		_ = w.svcCtx.TopUpRecordRepo.IncrementRetry(ctx, rec.ID)
		_ = w.svcCtx.TopUpRecordRepo.MarkFailed(ctx, rec.ID, fmt.Sprintf("compute tx hash failed: %v", err))
		return "", err
	}

	// Broadcast.
	bcStart := time.Now()
	bcResp, err := w.svcCtx.Chain.BroadcastTransaction(ctx, &pb.BroadcastTransactionReq{
		Chain:             chainEnum,
		SignedTransaction: signedTx,
		RequestId:         requestID,
		WaitForReceipt:    false,
		TimeoutSeconds:    30,
	})
	bcDur := time.Since(bcStart)
	if err != nil || bcResp == nil || !bcResp.Success || strings.TrimSpace(bcResp.TxHash) == "" {
		logger.Errorw("chain rpc BroadcastTransaction failed",
			append(base,
				logx.Field("event", "rpc_call"),
				logx.Field("rpc_method", "Chain.BroadcastTransaction"),
				logx.Field("duration_ms", bcDur.Milliseconds()),
				logx.Field("request_id", requestID),
				logx.Field("error", err),
				logx.Field("rpc_success", bcResp != nil && bcResp.Success),
				logx.Field("rpc_msg", safeMsg(bcResp)),
				logx.Field("purpose", purpose),
				logx.Field("topup_record_id", rec.ID),
				logx.Field("tx_hash_computed", txHash),
			)...,
		)
		_ = w.svcCtx.TopUpRecordRepo.IncrementRetry(ctx, rec.ID)
		_ = w.svcCtx.TopUpRecordRepo.MarkFailed(ctx, rec.ID, fmt.Sprintf("BroadcastTransaction failed: %v %s", err, safeMsg(bcResp)))
		return "", fmt.Errorf("BroadcastTransaction failed: %v %s", err, safeMsg(bcResp))
	}
	if bcDur > 1500*time.Millisecond {
		logger.Infow("chain rpc BroadcastTransaction slow",
			append(base,
				logx.Field("event", "rpc_slow"),
				logx.Field("rpc_method", "Chain.BroadcastTransaction"),
				logx.Field("duration_ms", bcDur.Milliseconds()),
				logx.Field("request_id", requestID),
				logx.Field("purpose", purpose),
				logx.Field("topup_record_id", rec.ID),
				logx.Field("tx_hash", strings.TrimSpace(bcResp.TxHash)),
			)...,
		)
	}
	if !strings.EqualFold(strings.TrimSpace(bcResp.TxHash), strings.TrimSpace(txHash)) {
		logx.WithContext(ctx).Errorf("topup tx hash mismatch (task_id=%s chain=%s computed=%s broadcast=%s)", task.TaskID, chain, txHash, bcResp.TxHash)
	}

	if err := w.svcCtx.TopUpRecordRepo.MarkSent(ctx, rec.ID, strings.TrimSpace(bcResp.TxHash), signedTx); err != nil {
		return "", err
	}

	WriteTaskLog(ctx, w.svcCtx, task.TaskID, "INFO", TaskLogEventTopUpBroadcasted, "top-up tx broadcasted", map[string]any{
		"topup_record_id": rec.ID,
		"purpose":         purpose,
		"from":            hot,
		"to":              toAddr,
		"amount":          amount,
		"asset_symbol":    nativeSymbol,
		"tx_hash":         strings.TrimSpace(bcResp.TxHash),
	})
	logger.Infow("top-up tx broadcasted",
		append(base,
			logx.Field("event", "topup_broadcasted"),
			logx.Field("purpose", purpose),
			logx.Field("topup_record_id", rec.ID),
			logx.Field("tx_hash", strings.TrimSpace(bcResp.TxHash)),
			logx.Field("duration_ms", bcDur.Milliseconds()),
		)...,
	)
	return strings.TrimSpace(bcResp.TxHash), nil
}

func (w *TopUpWorker) getHotWalletAddress(ctx context.Context, chain string, workerID int) (string, error) {
	chain = strings.ToUpper(strings.TrimSpace(chain))
	if chain == "" {
		return "", fmt.Errorf("chain is empty")
	}

	w.hotWalletMu.Lock()
	if v := strings.TrimSpace(w.hotWalletCache[chain]); v != "" {
		w.hotWalletMu.Unlock()
		return v, nil
	}
	w.hotWalletMu.Unlock()

	cfg := w.svcCtx.Config.Consolidation.TopUp
	start := time.Now()
	resp, err := w.svcCtx.SignerRpc.GetCompanyWallet(ctx, &pb.GetCompanyWalletRequest{
		Chain:       chain,
		AddressType: strings.TrimSpace(cfg.HotWalletAddressType),
		Temperature: cfg.HotWalletTemperature,
	})
	dur := time.Since(start)
	if err != nil || resp == nil || resp.Code != 0 || resp.Wallet == nil || strings.TrimSpace(resp.Wallet.Address) == "" {
		logx.WithContext(ctx).Errorw("signer rpc GetCompanyWallet failed",
			append(workerLogFields(w.svcCtx, "topup", workerID),
				logx.Field("event", "rpc_call"),
				logx.Field("rpc_method", "Signer.GetCompanyWallet"),
				logx.Field("duration_ms", dur.Milliseconds()),
				logx.Field("error", err),
				logx.Field("signer_code", func() int32 {
					if resp == nil {
						return 0
					}
					return resp.Code
				}()),
			)...,
		)
		return "", fmt.Errorf("GetCompanyWallet failed: %v %v", err, resp)
	}
	addr := strings.TrimSpace(resp.Wallet.Address)

	w.hotWalletMu.Lock()
	w.hotWalletCache[chain] = addr
	w.hotWalletMu.Unlock()
	return addr, nil
}
