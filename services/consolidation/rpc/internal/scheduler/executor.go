package scheduler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	tronCore "github.com/fbsobreira/gotron-sdk/pkg/proto/core"
	"google.golang.org/protobuf/proto"

	"internalwallet/proto/pb"
	"internalwallet/services/consolidation/rpc/internal/chainutil"
	"internalwallet/services/consolidation/rpc/internal/config"
	"internalwallet/services/consolidation/rpc/internal/models"
	"internalwallet/services/consolidation/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

// Executor claims Pending tasks (per chain) and executes consolidation transfers.
//
// Key guarantees (multi-instance safe):
// - Claiming happens only when a worker has capacity (no claim-then-queue backlog).
// - All state transitions are conditional on (claim owner + lease valid + expected version).
// - One active task per (chain, from_address) is enforced by DB unique key (active_task_key).
type Executor struct {
	svcCtx *svc.ServiceContext

	globalSem *Semaphore

	hotWalletMu    sync.Mutex
	hotWalletCache map[string]string // chain -> system hot wallet address

	feeGuardMu    sync.Mutex
	feeGuardState map[string]*feeGuardState // chain -> state (shared across workers)
}

func NewExecutor(svcCtx *svc.ServiceContext) *Executor {
	return &Executor{svcCtx: svcCtx}
}

type feeGuardState struct {
	checking     bool
	lastChecked  time.Time
	blockedUntil time.Time
}

func (e *Executor) Run(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	if e == nil || e.svcCtx == nil {
		logx.WithContext(ctx).Error("consolidation executor disabled: svcCtx not configured")
		return
	}
	cfg := e.svcCtx.Config.Consolidation
	if !cfg.Enabled {
		logx.WithContext(ctx).Info("consolidation executor disabled by config")
		return
	}
	if e.svcCtx.DB == nil || e.svcCtx.ConsolidationTaskRepo == nil {
		logx.WithContext(ctx).Error("consolidation executor disabled: db/repository not configured")
		return
	}
	if e.svcCtx.Chain == nil || e.svcCtx.SignerRpc == nil {
		logx.WithContext(ctx).Error("consolidation executor disabled: chain nodes / SignerRpc not configured")
		return
	}

	e.globalSem = NewSemaphore(cfg.MaxConcurrentTasks)

	lease := time.Duration(cfg.ClaimLeaseSeconds) * time.Second
	if lease <= 0 {
		lease = 60 * time.Second
	}
	idle := time.Duration(cfg.ExecutorTickInterval) * time.Second
	if idle <= 0 {
		idle = 2 * time.Second
	}

	var wg sync.WaitGroup
	for _, chain := range chainutil.SupportedChains() {
		workers := executorWorkersForChain(cfg, chain)
		if workers <= 0 {
			continue
		}
		for i := 0; i < workers; i++ {
			wg.Add(1)
			workerID := i
			chain := chain
			go func() {
				defer wg.Done()
				e.workerLoop(ctx, chain, workerID, lease, idle)
			}()
		}
	}

	logx.WithContext(ctx).Infof("consolidation executor started (max=%d lease=%s)", cfg.MaxConcurrentTasks, lease)
	wg.Wait()
	logx.WithContext(ctx).Info("consolidation executor stopped")
}

func executorWorkersForChain(cfg config.ConsolidationConfig, chain string) int {
	switch strings.ToUpper(strings.TrimSpace(chain)) {
	case "TRON":
		return cfg.MaxConcurrentPerChain.TRON
	case "ETH":
		return cfg.MaxConcurrentPerChain.ETH
	case "BSC":
		return cfg.MaxConcurrentPerChain.BSC
	default:
		return 0
	}
}

func (e *Executor) feeGuardMaybeBlock(ctx context.Context, chain string) (bool, time.Duration) {
	if ctx == nil {
		ctx = context.Background()
	}
	if e == nil || e.svcCtx == nil {
		return false, 0
	}

	cfg := e.svcCtx.Config.Consolidation
	if !cfg.Enabled || !cfg.FeeGuard.Enabled {
		return false, 0
	}

	chain = strings.ToUpper(strings.TrimSpace(chain))
	// FeeGuard is currently EVM-only (ETH/BSC). TRON fees are resource-dependent and not suitable for a global gate.
	if chain != "ETH" && chain != "BSC" {
		return false, 0
	}

	// Apply only when this chain is configured to consolidate USDT (source of truth: thresholds list).
	rt := e.svcCtx.Runtime()
	if thresholds, ok := rt.EffectiveMinBalanceThresholds[chain]; !ok || strings.TrimSpace(thresholds["USDT"]) == "" {
		return false, 0
	}

	maxStr := strings.TrimSpace(cfg.Risk.MaxNativeSpendPerTask[chain])
	if maxStr == "" {
		// Should be validated on startup, but be defensive.
		return false, 0
	}
	maxAllowed, err := parseBigInt10(maxStr)
	if err != nil || maxAllowed.Sign() <= 0 {
		return false, 0
	}

	checkInterval := time.Duration(cfg.FeeGuard.CheckIntervalSeconds) * time.Second
	if checkInterval <= 0 {
		checkInterval = 30 * time.Second
	}
	blockFor := time.Duration(cfg.FeeGuard.BlockSeconds) * time.Second
	if blockFor <= 0 {
		blockFor = 60 * time.Second
	}

	for {
		now := time.Now().Local()

		e.feeGuardMu.Lock()
		if e.feeGuardState == nil {
			e.feeGuardState = make(map[string]*feeGuardState)
		}
		st := e.feeGuardState[chain]
		if st == nil {
			st = &feeGuardState{}
			e.feeGuardState[chain] = st
		}
		if now.Before(st.blockedUntil) {
			wait := time.Until(st.blockedUntil)
			e.feeGuardMu.Unlock()
			return true, wait
		}
		if st.checking {
			e.feeGuardMu.Unlock()
			time.Sleep(150 * time.Millisecond)
			continue
		}
		if !st.lastChecked.IsZero() && now.Sub(st.lastChecked) < checkInterval {
			e.feeGuardMu.Unlock()
			return false, 0
		}
		st.checking = true
		e.feeGuardMu.Unlock()

		// Perform the RPC check outside the mutex.
		chainEnum, err := chainutil.ChainStringToEnum(chain)
		if err != nil {
			chainEnum = pb.ChainRpcType_CHAIN_TYPE_UNSPECIFIED
		}

		gasLimit := cfg.FeeGuard.EvmUsdtTransferGasLimit
		if gasLimit == 0 {
			gasLimit = 70_000
		}

		gasPriceWei := ""
		estimatedFee := (*big.Int)(nil)
		estimatedFeeWithBuf := (*big.Int)(nil)
		block := false
		gasSafetyMult := cfg.GasSafetyMultipliers[chain]

		gasPriceWei, err = e.svcCtx.Chain.SuggestGasPrice(ctx, chainEnum)
		if err == nil {
			gp, perr := parseBigInt10(gasPriceWei)
			if perr != nil {
				err = perr
			} else {
				estimatedFee = new(big.Int).Mul(gp, new(big.Int).SetUint64(gasLimit))
				buf, berr := mulCeilBigInt(estimatedFee, gasSafetyMult)
				if berr != nil {
					err = berr
				} else {
					estimatedFeeWithBuf = buf
					if buf.Cmp(maxAllowed) > 0 {
						block = true
					}
				}
			}
		}

		doneAt := time.Now().Local()
		e.feeGuardMu.Lock()
		st.checking = false
		st.lastChecked = doneAt
		if err != nil || block {
			st.blockedUntil = doneAt.Add(blockFor)
		}
		e.feeGuardMu.Unlock()

		// Fail-closed: if fee check fails, block temporarily to avoid uncontrolled spending.
		if err != nil {
			logx.WithContext(ctx).Errorw("fee guard check failed; executor blocked temporarily",
				append(workerLogFields(e.svcCtx, "executor", -1),
					logx.Field("event", "fee_guard_error"),
					logx.Field("chain", chain),
					logx.Field("token_symbol", "USDT"),
					logx.Field("max_allowed", maxStr),
					logx.Field("gas_limit", gasLimit),
					logx.Field("error", err),
					logx.Field("blocked_for_seconds", int64(blockFor.Seconds())),
				)...,
			)
			return true, blockFor
		}
		if block {
			logx.WithContext(ctx).Infow("fee guard blocked executor (USDT fee exceeds risk cap)",
				append(workerLogFields(e.svcCtx, "executor", -1),
					logx.Field("event", "fee_guard_blocked"),
					logx.Field("chain", chain),
					logx.Field("token_symbol", "USDT"),
					logx.Field("gas_price_wei", strings.TrimSpace(gasPriceWei)),
					logx.Field("gas_limit", gasLimit),
					logx.Field("estimated_fee", func() string {
						if estimatedFee == nil {
							return ""
						}
						return estimatedFee.String()
					}()),
					logx.Field("estimated_fee_with_buffer", func() string {
						if estimatedFeeWithBuf == nil {
							return ""
						}
						return estimatedFeeWithBuf.String()
					}()),
					logx.Field("max_allowed", maxStr),
					logx.Field("gas_safety_multiplier", gasSafetyMult),
					logx.Field("blocked_for_seconds", int64(blockFor.Seconds())),
				)...,
			)
			return true, blockFor
		}
		return false, 0
	}
}

func (e *Executor) workerLoop(ctx context.Context, chain string, workerID int, lease time.Duration, idle time.Duration) {
	// Startup jitter to avoid thundering herd across instances.
	time.Sleep(time.Duration((time.Now().UnixNano()+int64(workerID))*37%250) * time.Millisecond)

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
		rt := e.svcCtx.Runtime()
		if thresholds, ok := rt.EffectiveMinBalanceThresholds[strings.ToUpper(strings.TrimSpace(chain))]; !ok || len(thresholds) == 0 {
			time.Sleep(idle)
			continue
		}

		if blocked, wait := e.feeGuardMaybeBlock(ctx, chain); blocked {
			if wait <= 0 {
				wait = idle
			}
			time.Sleep(wait)
			continue
		}

		release, ok := e.globalSem.Acquire(ctx)
		if !ok {
			return
		}

		now := time.Now().Local()
		claimStart := time.Now()
		tasks, err := e.svcCtx.ConsolidationTaskRepo.ClaimByStatuses(ctx, []models.ConsolidationTaskStatus{models.ConsolidationTaskStatusPending}, chain, 1, now, lease, e.svcCtx.InstanceID)
		claimDur := time.Since(claimStart)
		if err != nil {
			claimErrors++
			logx.WithContext(ctx).Errorw("executor claim failed",
				append(workerLogFields(e.svcCtx, "executor", workerID),
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
			e.processOne(ctx, &task, workerID, lease)
			release()
		}

		if time.Since(lastSummary) >= 60*time.Second {
			logx.WithContext(ctx).Infow("consolidation executor loop summary",
				append(workerLogFields(e.svcCtx, "executor", workerID),
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

func (e *Executor) processOne(ctx context.Context, task *models.ConsolidationTask, workerID int, lease time.Duration) {
	if task == nil {
		return
	}

	taskCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	hbDone := make(chan struct{})
	go e.claimHeartbeat(taskCtx, hbDone, cancel, task.TaskID, task.ID, workerID, lease)
	defer close(hbDone)

	start := time.Now()
	logx.WithContext(taskCtx).Infow("consolidation executor task start",
		append(workerLogFields(e.svcCtx, "executor", workerID),
			append(taskLogFields(task),
				logx.Field("event", "task_start"),
			)...,
		)...,
	)
	err := e.execute(taskCtx, task, workerID)
	if err != nil {
		logx.WithContext(taskCtx).Errorw("execute failed",
			append(workerLogFields(e.svcCtx, "executor", workerID),
				append(taskLogFields(task),
					logx.Field("event", "task_execute_failed"),
					logx.Field("error", err),
				)...,
			)...,
		)
	} else {
		logx.WithContext(taskCtx).Infow("consolidation executor task done",
			append(workerLogFields(e.svcCtx, "executor", workerID),
				append(taskLogFields(task),
					logx.Field("event", "task_done"),
					logx.Field("duration_ms", time.Since(start).Milliseconds()),
				)...,
			)...,
		)
	}

	// Always release our claim (best-effort). State transitions may have already cleared claim.
	_ = e.svcCtx.ConsolidationTaskRepo.ReleaseClaim(context.Background(), task.ID, e.svcCtx.InstanceID)
}

func (e *Executor) claimHeartbeat(ctx context.Context, done <-chan struct{}, cancel context.CancelFunc, taskID string, taskRowID int64, workerID int, lease time.Duration) {
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
			ok, err := e.svcCtx.ConsolidationTaskRepo.RenewClaim(ctx, taskRowID, e.svcCtx.InstanceID, now, lease)
			if err != nil {
				logx.WithContext(ctx).Errorw("renew claim failed; cancel task",
					append(workerLogFields(e.svcCtx, "executor", workerID),
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
				// Lease lost or claim cleared: stop processing to avoid duplicate execution.
				logx.WithContext(ctx).Infow("claim lost; cancel task",
					append(workerLogFields(e.svcCtx, "executor", workerID),
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

func (e *Executor) execute(ctx context.Context, task *models.ConsolidationTask, workerID int) error {
	if task == nil {
		return fmt.Errorf("task is nil")
	}
	cfg := e.svcCtx.Config.Consolidation
	logger := logx.WithContext(ctx)

	needGasDelay := 5 * time.Minute
	if cfg.TopUp.Enabled && cfg.TopUp.TaskRetryDelaySeconds > 0 {
		needGasDelay = time.Duration(cfg.TopUp.TaskRetryDelaySeconds) * time.Second
	}

	chainEnum, err := chainutil.ChainStringToEnum(task.Chain)
	if err != nil {
		return err
	}

	// Validate target address against whitelist (config target address / system hot wallet).
	expectedToRaw, err := e.resolveExpectedToAddress(ctx, task, workerID)
	if err != nil {
		msg := fmt.Sprintf("resolve expected to_address failed: %v", err)
		return e.failOrRetry(ctx, task, models.ConsolidationTaskStatusFailed, msg, true, workerID)
	}
	expectedTo := chainutil.NormalizeAddress(task.Chain, expectedToRaw)
	actualTo := chainutil.NormalizeAddress(task.Chain, task.ToAddress)
	if expectedTo == "" || expectedTo != actualTo {
		msg := fmt.Sprintf("invalid to_address (not whitelisted): %s", task.ToAddress)
		from := task.Status
		ok, err := e.svcCtx.ConsolidationTaskRepo.MarkFailed(ctx, task.ID, e.svcCtx.InstanceID, task.Version, models.ConsolidationTaskStatusPermanentFailed, msg, nil, task.RetryCount)
		if err == nil && ok {
			writeStatusTransitionTaskLog(ctx, e.svcCtx, task, "ERROR", from, models.ConsolidationTaskStatusPermanentFailed, msg, nil, map[string]any{
				"decision": "invalid_to_address",
			})
		} else if err != nil {
			logger.Errorw("mark permanent failed failed",
				append(workerLogFields(e.svcCtx, "executor", workerID),
					append(taskLogFields(task),
						logx.Field("event", "task_transition_failed"),
						logx.Field("status_to", models.ConsolidationTaskStatusPermanentFailed),
						logx.Field("error", err),
					)...,
				)...,
			)
		}
		return fmt.Errorf("%s", msg)
	}

	isToken := task.TokenContract != nil && strings.TrimSpace(*task.TokenContract) != ""

	// Refresh balances (amount may change).
	var amountToSend string
	if isToken {
		start := time.Now()
		balResp, err := e.svcCtx.Chain.GetTokenBalance(ctx, &pb.GetTokenBalanceReq{
			Chain:         chainEnum,
			Address:       task.FromAddress,
			TokenContract: strings.TrimSpace(*task.TokenContract),
		})
		dur := time.Since(start)
		if err != nil || balResp == nil || !balResp.Success {
			logger.Errorw("chain rpc GetTokenBalance failed",
				append(workerLogFields(e.svcCtx, "executor", workerID),
					append(taskLogFields(task),
						logx.Field("event", "rpc_call"),
						logx.Field("rpc_method", "Chain.GetTokenBalance"),
						logx.Field("duration_ms", dur.Milliseconds()),
						logx.Field("error", err),
						logx.Field("rpc_success", balResp != nil && balResp.Success),
						logx.Field("rpc_msg", safeMsg(balResp)),
					)...,
				)...,
			)
			msg := fmt.Sprintf("GetTokenBalance failed: %v %s", err, safeMsg(balResp))
			return e.failOrRetry(ctx, task, models.ConsolidationTaskStatusFailed, msg, true, workerID)
		}
		amountToSend = strings.TrimSpace(balResp.Balance)
	} else {
		start := time.Now()
		balResp, err := e.svcCtx.Chain.GetBalance(ctx, &pb.GetBalanceReq{
			Chain:   chainEnum,
			Address: task.FromAddress,
		})
		dur := time.Since(start)
		if err != nil || balResp == nil || !balResp.Success {
			logger.Errorw("chain rpc GetBalance failed",
				append(workerLogFields(e.svcCtx, "executor", workerID),
					append(taskLogFields(task),
						logx.Field("event", "rpc_call"),
						logx.Field("rpc_method", "Chain.GetBalance"),
						logx.Field("duration_ms", dur.Milliseconds()),
						logx.Field("error", err),
						logx.Field("rpc_success", balResp != nil && balResp.Success),
						logx.Field("rpc_msg", safeMsg(balResp)),
					)...,
				)...,
			)
			msg := fmt.Sprintf("GetBalance failed: %v %s", err, safeMsg(balResp))
			return e.failOrRetry(ctx, task, models.ConsolidationTaskStatusFailed, msg, true, workerID)
		}
		amountToSend = strings.TrimSpace(balResp.Balance)
	}

	// Threshold re-check (best-effort).
	chainKey := strings.ToUpper(strings.TrimSpace(task.Chain))
	assetKey := strings.ToUpper(strings.TrimSpace(task.AssetSymbol))
	if isToken {
		rt := e.svcCtx.Runtime()
		if min := strings.TrimSpace(rt.EffectiveMinBalanceThresholds[chainKey][assetKey]); min != "" {
			ok, err := geBigIntString(amountToSend, min)
			if err == nil && !ok {
				msg := "balance below threshold; skip"
				from := task.Status
				ok, err := e.svcCtx.ConsolidationTaskRepo.MarkFailed(ctx, task.ID, e.svcCtx.InstanceID, task.Version, models.ConsolidationTaskStatusPermanentFailed, msg, nil, task.RetryCount)
				if err == nil && ok {
					writeStatusTransitionTaskLog(ctx, e.svcCtx, task, "ERROR", from, models.ConsolidationTaskStatusPermanentFailed, msg, nil, map[string]any{
						"decision":      "below_threshold",
						"min_threshold": min,
						"balance":       amountToSend,
					})
				}
				return nil
			}
		}
	} else {
		// Native consolidation eligibility:
		// available = balance - reserve
		// consolidate only if: available >= MinConsolidationAmount
		native := strings.ToUpper(strings.TrimSpace(chainutil.NativeSymbolForChain(chainKey)))
		reserve := big.NewInt(0)
		if native != "" {
			if reservesChain, ok := cfg.NativeReserves[chainKey]; ok {
				if v := strings.TrimSpace(reservesChain[native]); v != "" {
					if r, err := parseBigInt10(v); err == nil {
						reserve = r
					}
				}
			}
		}

		minStr := strings.TrimSpace(cfg.MinConsolidationAmount[chainKey][native])
		minAmt, err := parseBigInt10(minStr)
		if err != nil {
			msg := fmt.Sprintf("invalid MinConsolidationAmount[%s][%s]=%q: %v", chainKey, native, minStr, err)
			return e.failOrRetry(ctx, task, models.ConsolidationTaskStatusFailed, msg, false, workerID)
		}
		bal, err := parseBigInt10(amountToSend)
		if err != nil {
			msg := fmt.Sprintf("parse native balance failed: %v", err)
			return e.failOrRetry(ctx, task, models.ConsolidationTaskStatusFailed, msg, true, workerID)
		}
		available := new(big.Int).Sub(bal, reserve)
		if available.Sign() < 0 {
			available = big.NewInt(0)
		}
		if available.Cmp(minAmt) < 0 {
			msg := fmt.Sprintf("native available below MinConsolidationAmount; skip (available=%s reserve=%s min=%s)", available.String(), reserve.String(), minStr)
			from := task.Status
			ok, err := e.svcCtx.ConsolidationTaskRepo.MarkFailed(ctx, task.ID, e.svcCtx.InstanceID, task.Version, models.ConsolidationTaskStatusPermanentFailed, msg, nil, task.RetryCount)
			if err == nil && ok {
				writeStatusTransitionTaskLog(ctx, e.svcCtx, task, "ERROR", from, models.ConsolidationTaskStatusPermanentFailed, msg, nil, map[string]any{
					"decision":     "below_min_consolidation_amount",
					"available":    available.String(),
					"reserve":      reserve.String(),
					"min_amount":   minStr,
					"asset_symbol": assetKey,
				})
			}
			return nil
		}
	}

	var estimatedFee *string
	var evmGasLimit uint64
	var evmGasPrice string

	switch chainEnum {
	case pb.ChainRpcType_CHAIN_TYPE_ETHEREUM, pb.ChainRpcType_CHAIN_TYPE_BSC:
		prep, err := e.prepareEvmAmounts(ctx, task, amountToSend)
		if err != nil {
			msg := fmt.Sprintf("prepare evm amounts failed: %v", err)
			return e.failOrRetry(ctx, task, models.ConsolidationTaskStatusFailed, msg, true, workerID)
		}
		estimatedFee = prep.estimatedFee
		evmGasLimit = prep.gasLimit
		evmGasPrice = prep.gasPrice
		if prep.deferUntil != nil {
			ok, err := e.svcCtx.ConsolidationTaskRepo.MarkDeferredPending(ctx, task.ID, e.svcCtx.InstanceID, task.Version, prep.deferReason, prep.deferUntil)
			if err == nil && ok {
				writeStatusTransitionTaskLog(ctx, e.svcCtx, task, "WARN", models.ConsolidationTaskStatusPending, models.ConsolidationTaskStatusPending, prep.deferReason, prep.deferUntil, map[string]any{
					"decision":  "deferred_pending",
					"gas_price": strings.TrimSpace(prep.gasPrice),
					"gas_limit": prep.gasLimit,
				})
			}
			return nil
		}
		if prep.needGas {
			// Native top-up is prohibited: never move native tasks into NeedGas.
			if !isToken {
				reason := strings.TrimSpace(prep.needGasReason)
				if reason == "" {
					reason = "insufficient native balance for fee/reserve"
				}
				msg := fmt.Sprintf("native top-up prohibited: %s", reason)
				from := task.Status
				ok, err := e.svcCtx.ConsolidationTaskRepo.MarkFailed(ctx, task.ID, e.svcCtx.InstanceID, task.Version, models.ConsolidationTaskStatusPermanentFailed, msg, nil, task.RetryCount)
				if err == nil && ok {
					writeStatusTransitionTaskLog(ctx, e.svcCtx, task, "ERROR", from, models.ConsolidationTaskStatusPermanentFailed, msg, nil, map[string]any{
						"decision":        "native_topup_prohibited",
						"reason":          reason,
						"required_native": strings.TrimSpace(prep.requiredNative),
						"shortfall":       strings.TrimSpace(prep.shortfall),
						"estimated_fee": func() string {
							if prep.estimatedFee == nil {
								return ""
							}
							return strings.TrimSpace(*prep.estimatedFee)
						}(),
						"gas_price": strings.TrimSpace(prep.gasPrice),
						"gas_limit": prep.gasLimit,
					})
				}
				return nil
			}

			maxStr := strings.TrimSpace(cfg.Risk.MaxNativeSpendPerTask[strings.ToUpper(task.Chain)])
			shortfallStr := strings.TrimSpace(prep.shortfall)
			if maxStr != "" && shortfallStr != "" {
				maxAllowed, mErr := parseBigInt10(maxStr)
				shortfall, sErr := parseBigInt10(shortfallStr)
				if mErr == nil && sErr == nil && maxAllowed.Sign() > 0 && shortfall.Cmp(maxAllowed) > 0 {
					msg := fmt.Sprintf("required native top-up exceeds risk cap: shortfall=%s max=%s", shortfallStr, maxStr)
					from := task.Status
					ok, err := e.svcCtx.ConsolidationTaskRepo.MarkFailed(ctx, task.ID, e.svcCtx.InstanceID, task.Version, models.ConsolidationTaskStatusPermanentFailed, msg, nil, task.RetryCount)
					if err == nil && ok {
						writeStatusTransitionTaskLog(ctx, e.svcCtx, task, "ERROR", from, models.ConsolidationTaskStatusPermanentFailed, msg, nil, map[string]any{
							"decision":        "need_gas_over_risk_cap",
							"shortfall":       shortfallStr,
							"required_native": strings.TrimSpace(prep.requiredNative),
							"max_allowed":     maxStr,
							"estimated_fee": func() string {
								if prep.estimatedFee == nil {
									return ""
								}
								return strings.TrimSpace(*prep.estimatedFee)
							}(),
							"gas_price":    strings.TrimSpace(prep.gasPrice),
							"gas_limit":    prep.gasLimit,
							"is_token":     isToken,
							"asset_symbol": strings.ToUpper(strings.TrimSpace(task.AssetSymbol)),
							"token_contract": func() string {
								if task.TokenContract == nil {
									return ""
								}
								return strings.TrimSpace(*task.TokenContract)
							}(),
							"gas_safety_mult": cfg.GasSafetyMultipliers[strings.ToUpper(task.Chain)],
							"max_gas_price":   strings.TrimSpace(cfg.MaxGasPrice[strings.ToUpper(task.Chain)]),
							"to_address_mode": strings.TrimSpace(cfg.ToAddressMode),
						})
					}
					return nil
				}
			}

			msg := "insufficient native balance for gas"
			next := addPtrTime(time.Now().Local().Add(needGasDelay))
			ok, err := e.svcCtx.ConsolidationTaskRepo.MarkNeedGas(ctx, task.ID, e.svcCtx.InstanceID, task.Version, msg, next)
			if err == nil && ok {
				writeStatusTransitionTaskLog(ctx, e.svcCtx, task, "INFO", task.Status, models.ConsolidationTaskStatusNeedGas, msg, next, map[string]any{
					"decision":        "need_gas",
					"shortfall":       func() string { return shortfallStr }(),
					"required_native": strings.TrimSpace(prep.requiredNative),
				})
			}
			return nil
		}
		amountToSend = prep.amountToSend
	case pb.ChainRpcType_CHAIN_TYPE_TRON:
		prep, err := e.prepareTronPrecheck(ctx, task, amountToSend)
		if err != nil {
			msg := fmt.Sprintf("prepare tron precheck failed: %v", err)
			return e.failOrRetry(ctx, task, models.ConsolidationTaskStatusFailed, msg, true, workerID)
		}
		estimatedFee = prep.estimatedFee
		if prep.deferUntil != nil {
			ok, err := e.svcCtx.ConsolidationTaskRepo.MarkDeferredPending(ctx, task.ID, e.svcCtx.InstanceID, task.Version, prep.deferReason, prep.deferUntil)
			if err == nil && ok {
				writeStatusTransitionTaskLog(ctx, e.svcCtx, task, "WARN", models.ConsolidationTaskStatusPending, models.ConsolidationTaskStatusPending, prep.deferReason, prep.deferUntil, map[string]any{
					"decision": "deferred_pending",
				})
			}
			return nil
		}
		if prep.needBandwidth {
			// Native top-up is prohibited: NeedBandwidth is token-only.
			if !isToken {
				msg := "native bandwidth top-up prohibited"
				from := task.Status
				ok, err := e.svcCtx.ConsolidationTaskRepo.MarkFailed(ctx, task.ID, e.svcCtx.InstanceID, task.Version, models.ConsolidationTaskStatusPermanentFailed, msg, nil, task.RetryCount)
				if err == nil && ok {
					writeStatusTransitionTaskLog(ctx, e.svcCtx, task, "ERROR", from, models.ConsolidationTaskStatusPermanentFailed, msg, nil, map[string]any{
						"decision": "native_topup_prohibited",
					})
				}
				return nil
			}

			maxStr := strings.TrimSpace(cfg.Risk.MaxNativeSpendPerTask["TRON"])
			shortfallStr := strings.TrimSpace(prep.shortfall)
			if maxStr != "" && shortfallStr != "" {
				maxAllowed, mErr := parseBigInt10(maxStr)
				shortfall, sErr := parseBigInt10(shortfallStr)
				if mErr == nil && sErr == nil && maxAllowed.Sign() > 0 && shortfall.Cmp(maxAllowed) > 0 {
					msg := fmt.Sprintf("required TRX bandwidth top-up exceeds risk cap: shortfall=%s max=%s", shortfallStr, maxStr)
					from := task.Status
					ok, err := e.svcCtx.ConsolidationTaskRepo.MarkFailed(ctx, task.ID, e.svcCtx.InstanceID, task.Version, models.ConsolidationTaskStatusPermanentFailed, msg, nil, task.RetryCount)
					if err == nil && ok {
						writeStatusTransitionTaskLog(ctx, e.svcCtx, task, "ERROR", from, models.ConsolidationTaskStatusPermanentFailed, msg, nil, map[string]any{
							"decision":        "need_bandwidth_over_risk_cap",
							"shortfall":       shortfallStr,
							"required_native": strings.TrimSpace(prep.requiredNative),
							"max_allowed":     maxStr,
							"bandwidth_have":  prep.bandwidthHave,
							"bandwidth_need":  prep.bandwidthNeed,
							"bandwidth_topup": strings.TrimSpace(prep.bandwidthTopUp),
						})
					}
					return nil
				}
			}

			msg := strings.TrimSpace(prep.bandwidthMsg)
			if msg == "" {
				msg = "insufficient bandwidth; need TRX top-up"
			}
			next := addPtrTime(time.Now().Local().Add(needGasDelay))
			ok, err := e.svcCtx.ConsolidationTaskRepo.MarkNeedBandwidth(ctx, task.ID, e.svcCtx.InstanceID, task.Version, msg, next)
			if err == nil && ok {
				writeStatusTransitionTaskLog(ctx, e.svcCtx, task, "INFO", task.Status, models.ConsolidationTaskStatusNeedBandwidth, msg, next, map[string]any{
					"decision":        "need_bandwidth",
					"shortfall":       shortfallStr,
					"required_native": strings.TrimSpace(prep.requiredNative),
					"bandwidth_have":  prep.bandwidthHave,
					"bandwidth_need":  prep.bandwidthNeed,
					"bandwidth_topup": strings.TrimSpace(prep.bandwidthTopUp),
				})
			}
			return nil
		}
		if prep.needGas {
			// Native top-up is prohibited: never move native tasks into NeedGas.
			if !isToken {
				reason := strings.TrimSpace(prep.needGasReason)
				if reason == "" {
					reason = "insufficient TRX for fee/reserve"
				}
				msg := fmt.Sprintf("native top-up prohibited: %s", reason)
				from := task.Status
				ok, err := e.svcCtx.ConsolidationTaskRepo.MarkFailed(ctx, task.ID, e.svcCtx.InstanceID, task.Version, models.ConsolidationTaskStatusPermanentFailed, msg, nil, task.RetryCount)
				if err == nil && ok {
					writeStatusTransitionTaskLog(ctx, e.svcCtx, task, "ERROR", from, models.ConsolidationTaskStatusPermanentFailed, msg, nil, map[string]any{
						"decision":        "native_topup_prohibited",
						"reason":          reason,
						"required_native": strings.TrimSpace(prep.requiredNative),
						"shortfall":       strings.TrimSpace(prep.shortfall),
						"estimated_fee": func() string {
							if prep.estimatedFee == nil {
								return ""
							}
							return strings.TrimSpace(*prep.estimatedFee)
						}(),
						"need_gas_reason": strings.TrimSpace(prep.needGasReason),
					})
				}
				return nil
			}

			maxStr := strings.TrimSpace(cfg.Risk.MaxNativeSpendPerTask["TRON"])
			shortfallStr := strings.TrimSpace(prep.shortfall)
			if maxStr != "" && shortfallStr != "" {
				maxAllowed, mErr := parseBigInt10(maxStr)
				shortfall, sErr := parseBigInt10(shortfallStr)
				if mErr == nil && sErr == nil && maxAllowed.Sign() > 0 && shortfall.Cmp(maxAllowed) > 0 {
					msg := fmt.Sprintf("required TRX top-up exceeds risk cap: shortfall=%s max=%s (%s)", shortfallStr, maxStr, strings.TrimSpace(prep.needGasReason))
					from := task.Status
					ok, err := e.svcCtx.ConsolidationTaskRepo.MarkFailed(ctx, task.ID, e.svcCtx.InstanceID, task.Version, models.ConsolidationTaskStatusPermanentFailed, msg, nil, task.RetryCount)
					if err == nil && ok {
						writeStatusTransitionTaskLog(ctx, e.svcCtx, task, "ERROR", from, models.ConsolidationTaskStatusPermanentFailed, msg, nil, map[string]any{
							"decision":        "need_gas_over_risk_cap",
							"shortfall":       shortfallStr,
							"required_native": strings.TrimSpace(prep.requiredNative),
							"max_allowed":     maxStr,
							"estimated_fee": func() string {
								if prep.estimatedFee == nil {
									return ""
								}
								return strings.TrimSpace(*prep.estimatedFee)
							}(),
							"need_gas_reason": strings.TrimSpace(prep.needGasReason),
							"is_token":        isToken,
							"asset_symbol":    strings.ToUpper(strings.TrimSpace(task.AssetSymbol)),
						})
					}
					return nil
				}
			}

			next := addPtrTime(time.Now().Local().Add(needGasDelay))
			ok, err := e.svcCtx.ConsolidationTaskRepo.MarkNeedGas(ctx, task.ID, e.svcCtx.InstanceID, task.Version, prep.needGasReason, next)
			if err == nil && ok {
				writeStatusTransitionTaskLog(ctx, e.svcCtx, task, "INFO", task.Status, models.ConsolidationTaskStatusNeedGas, prep.needGasReason, next, map[string]any{
					"decision":        "need_gas",
					"shortfall":       shortfallStr,
					"required_native": strings.TrimSpace(prep.requiredNative),
				})
			}
			return nil
		}
		if prep.needEnergy {
			msg := "insufficient energy; moved to energy rental"
			next := addPtrTime(time.Now().Local().Add(30 * time.Second))
			ok, err := e.svcCtx.ConsolidationTaskRepo.MarkNeedEnergy(ctx, task.ID, e.svcCtx.InstanceID, task.Version, msg, next)
			if err == nil && ok {
				writeStatusTransitionTaskLog(ctx, e.svcCtx, task, "INFO", task.Status, models.ConsolidationTaskStatusNeedEnergy, msg, next, map[string]any{
					"decision": "need_energy",
				})
			}
			return nil
		}
		if strings.TrimSpace(prep.amountToSend) != "" {
			amountToSend = prep.amountToSend
		}
	}

	// Build unsigned tx
	var rawTx string
	if chainEnum == pb.ChainRpcType_CHAIN_TYPE_TRON {
		req := &pb.ChainRpcBuildTronTransactionReq{
			FromAddress:     task.FromAddress,
			ToAddress:       task.ToAddress,
			Amount:          amountToSend,
			ContractAddress: "",
		}
		if isToken {
			req.ContractAddress = strings.TrimSpace(*task.TokenContract)
		}
		start := time.Now()
		buildResp, err := e.svcCtx.Chain.BuildTronTransaction(ctx, req)
		dur := time.Since(start)
		if err != nil || buildResp == nil || !buildResp.Success {
			logger.Errorw("chain rpc BuildTronTransaction failed",
				append(workerLogFields(e.svcCtx, "executor", workerID),
					append(taskLogFields(task),
						logx.Field("event", "rpc_call"),
						logx.Field("rpc_method", "Chain.BuildTronTransaction"),
						logx.Field("duration_ms", dur.Milliseconds()),
						logx.Field("error", err),
						logx.Field("rpc_success", buildResp != nil && buildResp.Success),
						logx.Field("rpc_msg", safeMsg(buildResp)),
					)...,
				)...,
			)
			msg := fmt.Sprintf("BuildTronTransaction failed: %v %s", err, safeMsg(buildResp))
			return e.failOrRetry(ctx, task, models.ConsolidationTaskStatusFailed, msg, true, workerID)
		}
		rawTx = buildResp.RawData
	} else {
		req := &pb.BuildTransactionReq{
			Chain:         chainEnum,
			FromAddress:   task.FromAddress,
			ToAddress:     task.ToAddress,
			Amount:        amountToSend,
			TokenContract: "",
			GasLimit:      evmGasLimit,
			GasPrice:      evmGasPrice,
			Nonce:         "",
		}
		if isToken {
			req.TokenContract = strings.TrimSpace(*task.TokenContract)
		}
		start := time.Now()
		buildResp, err := e.svcCtx.Chain.BuildTransaction(ctx, req)
		dur := time.Since(start)
		if err != nil || buildResp == nil || !buildResp.Success {
			logger.Errorw("chain rpc BuildTransaction failed",
				append(workerLogFields(e.svcCtx, "executor", workerID),
					append(taskLogFields(task),
						logx.Field("event", "rpc_call"),
						logx.Field("rpc_method", "Chain.BuildTransaction"),
						logx.Field("duration_ms", dur.Milliseconds()),
						logx.Field("error", err),
						logx.Field("rpc_success", buildResp != nil && buildResp.Success),
						logx.Field("rpc_msg", safeMsg(buildResp)),
					)...,
				)...,
			)
			msg := fmt.Sprintf("BuildTransaction failed: %v %s", err, safeMsg(buildResp))
			return e.failOrRetry(ctx, task, models.ConsolidationTaskStatusFailed, msg, true, workerID)
		}
		rawTx = buildResp.RawTransaction
		if estimatedFee == nil && strings.TrimSpace(buildResp.EstimatedFee) != "" {
			v := strings.TrimSpace(buildResp.EstimatedFee)
			estimatedFee = &v
		}
	}

	// Sign
	requestID := fmt.Sprintf("consolidation_%s_%d", task.TaskID, time.Now().UnixNano())
	signReq := &pb.SignTransactionRequest{
		RequestId:      requestID,
		Chain:          strings.ToUpper(task.Chain),
		FromAddress:    task.FromAddress,
		RawTransaction: rawTx,
		OperationType:  3, // consolidation
		Amount:         amountToSend,
		ToAddress:      task.ToAddress,
		AssetSymbol:    strings.ToUpper(task.AssetSymbol),
		Requester:      "consolidation_service",
	}
	if isToken {
		signReq.TokenContract = strings.TrimSpace(*task.TokenContract)
	}
	signStart := time.Now()
	signResp, err := e.svcCtx.SignerRpc.SignTransaction(ctx, signReq)
	signDur := time.Since(signStart)
	if err != nil || signResp == nil || signResp.Code != 0 || strings.TrimSpace(signResp.Signature) == "" {
		logger.Errorw("signer rpc SignTransaction failed",
			append(workerLogFields(e.svcCtx, "executor", workerID),
				append(taskLogFields(task),
					logx.Field("event", "rpc_call"),
					logx.Field("rpc_method", "Signer.SignTransaction"),
					logx.Field("request_id", requestID),
					logx.Field("duration_ms", signDur.Milliseconds()),
					logx.Field("error", err),
					logx.Field("signer_code", func() int32 {
						if signResp == nil {
							return 0
						}
						return signResp.Code
					}()),
					logx.Field("signer_msg", safeSignerMsg(signResp)),
				)...,
			)...,
		)
		msg := fmt.Sprintf("SignTransaction failed: %v %s", err, safeSignerMsg(signResp))
		return e.failOrRetry(ctx, task, models.ConsolidationTaskStatusFailed, msg, true, workerID)
	}

	signedTx := strings.TrimSpace(signResp.Signature)
	txHash, err := signedTxHash(chainEnum, signedTx)
	if err != nil {
		msg := fmt.Sprintf("failed to compute tx hash: %v", err)
		return e.failOrRetry(ctx, task, models.ConsolidationTaskStatusFailed, msg, true, workerID)
	}

	startedAt := time.Now().Local()
	updated, err := e.svcCtx.ConsolidationTaskRepo.MarkInProgress(ctx, task.ID, e.svcCtx.InstanceID, task.Version, txHash, signedTx, startedAt, estimatedFee, amountToSend)
	if err != nil {
		return err
	}
	if !updated {
		// Lost claim or task state changed (e.g., cancelled). Do NOT broadcast.
		logger.Infow("task state changed before broadcast; skip",
			append(workerLogFields(e.svcCtx, "executor", workerID),
				append(taskLogFields(task),
					logx.Field("event", "broadcast_skipped"),
					logx.Field("tx_hash", txHash),
				)...,
			)...,
		)
		return nil
	}
	expectedVersion := task.Version + 1
	writeStatusTransitionTaskLog(ctx, e.svcCtx, task, "INFO", models.ConsolidationTaskStatusPending, models.ConsolidationTaskStatusInProgress, "task moved to InProgress", nil, map[string]any{
		"tx_hash":        txHash,
		"request_id":     requestID,
		"amount":         amountToSend,
		"estimated_fee":  estimatedFee,
		"token_transfer": isToken,
	})

	// Broadcast
	bcStart := time.Now()
	bcResp, err := e.svcCtx.Chain.BroadcastTransaction(ctx, &pb.BroadcastTransactionReq{
		Chain:             chainEnum,
		SignedTransaction: signedTx,
		RequestId:         requestID,
		WaitForReceipt:    false,
		TimeoutSeconds:    30,
	})
	bcDur := time.Since(bcStart)
	if err != nil || bcResp == nil || !bcResp.Success || strings.TrimSpace(bcResp.TxHash) == "" {
		// IMPORTANT: Do not strand the task in InProgress/Timeout with a tx that may never exist on-chain.
		// Move it back to Pending with bounded backoff + retry_count++ so the address lock is eventually released
		// (or turns into PermanentFailed after MaxRetries).
		msg := fmt.Sprintf("BroadcastTransaction failed: %v %s", err, safeMsg(bcResp))
		logger.Errorw("chain rpc BroadcastTransaction failed",
			append(workerLogFields(e.svcCtx, "executor", workerID),
				append(taskLogFields(task),
					logx.Field("event", "rpc_call"),
					logx.Field("rpc_method", "Chain.BroadcastTransaction"),
					logx.Field("request_id", requestID),
					logx.Field("duration_ms", bcDur.Milliseconds()),
					logx.Field("error", err),
					logx.Field("rpc_success", bcResp != nil && bcResp.Success),
					logx.Field("rpc_msg", safeMsg(bcResp)),
					logx.Field("tx_hash", txHash),
				)...,
			)...,
		)
		e.scheduleRetryInProgress(ctx, task, expectedVersion, msg, workerID)
		return nil
	}
	WriteTaskLog(ctx, e.svcCtx, task.TaskID, "INFO", TaskLogEventTxBroadcasted, "transaction broadcasted", map[string]any{
		"chain":        strings.ToUpper(strings.TrimSpace(task.Chain)),
		"asset_symbol": strings.ToUpper(strings.TrimSpace(task.AssetSymbol)),
		"from_address": strings.TrimSpace(task.FromAddress),
		"to_address":   strings.TrimSpace(task.ToAddress),
		"tx_hash":      strings.TrimSpace(bcResp.TxHash),
		"request_id":   requestID,
		"duration_ms":  bcDur.Milliseconds(),
	})
	logger.Infow("transaction broadcasted",
		append(workerLogFields(e.svcCtx, "executor", workerID),
			append(taskLogFields(task),
				logx.Field("event", "tx_broadcasted"),
				logx.Field("request_id", requestID),
				logx.Field("duration_ms", bcDur.Milliseconds()),
				logx.Field("tx_hash", strings.TrimSpace(bcResp.TxHash)),
			)...,
		)...,
	)

	// Sanity check: ChainRPC returns tx hash computed from the signed transaction.
	if !strings.EqualFold(strings.TrimSpace(bcResp.TxHash), strings.TrimSpace(txHash)) {
		logger.Errorw("tx hash mismatch",
			append(workerLogFields(e.svcCtx, "executor", workerID),
				append(taskLogFields(task),
					logx.Field("event", "tx_hash_mismatch"),
					logx.Field("tx_hash_computed", txHash),
					logx.Field("tx_hash_broadcast", strings.TrimSpace(bcResp.TxHash)),
				)...,
			)...,
		)
	}
	return nil
}

func (e *Executor) resolveExpectedToAddress(ctx context.Context, task *models.ConsolidationTask, workerID int) (string, error) {
	if task == nil {
		return "", fmt.Errorf("task is nil")
	}
	chain := strings.ToUpper(strings.TrimSpace(task.Chain))
	asset := strings.ToUpper(strings.TrimSpace(task.AssetSymbol))
	cfg := e.svcCtx.Config.Consolidation
	mode := strings.ToLower(strings.TrimSpace(cfg.ToAddressMode))
	if mode == "" {
		mode = "target_addresses"
	}
	switch mode {
	case "target_addresses":
		if m, ok := cfg.TargetAddresses[chain]; ok {
			return strings.TrimSpace(m[asset]), nil
		}
		return "", nil
	case "system_hot_wallet":
		return e.getSystemHotWalletAddress(ctx, chain, workerID)
	default:
		return "", fmt.Errorf("unsupported ToAddressMode: %q", cfg.ToAddressMode)
	}
}

func (e *Executor) getSystemHotWalletAddress(ctx context.Context, chain string, workerID int) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	chain = strings.ToUpper(strings.TrimSpace(chain))
	if chain == "" {
		return "", fmt.Errorf("chain is empty")
	}
	if e == nil || e.svcCtx == nil || e.svcCtx.SignerRpc == nil {
		return "", fmt.Errorf("SignerRpc not configured")
	}

	e.hotWalletMu.Lock()
	if e.hotWalletCache == nil {
		e.hotWalletCache = map[string]string{}
	}
	if v := strings.TrimSpace(e.hotWalletCache[chain]); v != "" {
		e.hotWalletMu.Unlock()
		return v, nil
	}
	e.hotWalletMu.Unlock()

	cfg := e.svcCtx.Config.Consolidation
	start := time.Now()
	resp, err := e.svcCtx.SignerRpc.GetCompanyWallet(ctx, &pb.GetCompanyWalletRequest{
		Chain:       chain,
		AddressType: strings.TrimSpace(cfg.SystemHotWalletAddressType),
		Temperature: cfg.SystemHotWalletTemperature,
	})
	dur := time.Since(start)
	if err != nil || resp == nil || resp.Code != 0 || resp.Wallet == nil || strings.TrimSpace(resp.Wallet.Address) == "" {
		logx.WithContext(ctx).Errorw("signer rpc GetCompanyWallet failed",
			append(workerLogFields(e.svcCtx, "executor", workerID),
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
				logx.Field("chain", chain),
				logx.Field("address_type", strings.TrimSpace(cfg.SystemHotWalletAddressType)),
				logx.Field("temperature", cfg.SystemHotWalletTemperature),
			)...,
		)
		return "", fmt.Errorf("GetCompanyWallet failed: %v %v", err, resp)
	}
	addr := strings.TrimSpace(resp.Wallet.Address)

	e.hotWalletMu.Lock()
	e.hotWalletCache[chain] = addr
	e.hotWalletMu.Unlock()
	return addr, nil
}

func signedTxHash(chain pb.ChainRpcType, signedTx string) (string, error) {
	signedTx = strings.TrimSpace(signedTx)
	if signedTx == "" {
		return "", fmt.Errorf("signed tx is empty")
	}
	switch chain {
	case pb.ChainRpcType_CHAIN_TYPE_ETHEREUM, pb.ChainRpcType_CHAIN_TYPE_BSC:
		signedTxBytes, err := hexutil.Decode(signedTx)
		if err != nil {
			return "", fmt.Errorf("decode EVM signed tx: %w", err)
		}
		tx := new(types.Transaction)
		if err := tx.UnmarshalBinary(signedTxBytes); err != nil {
			return "", fmt.Errorf("unmarshal EVM signed tx: %w", err)
		}
		return tx.Hash().Hex(), nil
	case pb.ChainRpcType_CHAIN_TYPE_TRON:
		h := strings.TrimPrefix(signedTx, "0x")
		signedTxBytes, err := hex.DecodeString(h)
		if err != nil {
			return "", fmt.Errorf("decode TRON signed tx: %w", err)
		}
		tx := &tronCore.Transaction{}
		if err := proto.Unmarshal(signedTxBytes, tx); err != nil {
			return "", fmt.Errorf("unmarshal TRON signed tx: %w", err)
		}
		rawDataBytes, err := proto.Marshal(tx.GetRawData())
		if err != nil {
			return "", fmt.Errorf("marshal TRON raw data: %w", err)
		}
		hash := sha256.Sum256(rawDataBytes)
		return hex.EncodeToString(hash[:]), nil
	default:
		return "", fmt.Errorf("unsupported chain: %v", chain)
	}
}

type evmPrepareResult struct {
	estimatedFee   *string
	amountToSend   string
	gasLimit       uint64
	gasPrice       string
	needGas        bool
	needGasReason  string
	requiredNative string // smallest unit
	shortfall      string // smallest unit
	deferUntil     *time.Time
	deferReason    string
}

func (e *Executor) prepareEvmAmounts(ctx context.Context, task *models.ConsolidationTask, currentAmount string) (evmPrepareResult, error) {
	cfg := e.svcCtx.Config.Consolidation
	chainEnum, err := chainutil.ChainStringToEnum(task.Chain)
	if err != nil {
		return evmPrepareResult{}, err
	}
	logger := logx.WithContext(ctx)
	base := append(workerLogFields(e.svcCtx, "executor", -1), taskLogFields(task)...)
	isToken := task.TokenContract != nil && strings.TrimSpace(*task.TokenContract) != ""

	// Estimate gas for the intended transfer.
	estReq := &pb.EstimateGasReq{
		Chain:         chainEnum,
		FromAddress:   task.FromAddress,
		ToAddress:     task.ToAddress,
		Value:         currentAmount,
		Data:          "",
		TokenContract: "",
	}
	if isToken {
		estReq.TokenContract = strings.TrimSpace(*task.TokenContract)
	}
	if !isToken {
		// For native sweep, estimate gas with value=0 to avoid insufficient-funds error.
		estReq.Value = "0"
	}
	estStart := time.Now()
	estResp, err := e.svcCtx.Chain.EstimateGas(ctx, estReq)
	estDur := time.Since(estStart)
	if err != nil || estResp == nil || !estResp.Success {
		logger.Errorw("chain rpc EstimateGas failed",
			append(base,
				logx.Field("event", "rpc_call"),
				logx.Field("rpc_method", "Chain.EstimateGas"),
				logx.Field("duration_ms", estDur.Milliseconds()),
				logx.Field("error", err),
				logx.Field("rpc_success", estResp != nil && estResp.Success),
				logx.Field("rpc_msg", safeMsg(estResp)),
				logx.Field("is_token", isToken),
			)...,
		)
		return evmPrepareResult{}, fmt.Errorf("EstimateGas failed: %v %s", err, safeMsg(estResp))
	}
	if estDur > 1500*time.Millisecond {
		logger.Infow("chain rpc EstimateGas slow",
			append(base,
				logx.Field("event", "rpc_slow"),
				logx.Field("rpc_method", "Chain.EstimateGas"),
				logx.Field("duration_ms", estDur.Milliseconds()),
				logx.Field("is_token", isToken),
			)...,
		)
	}

	fee, err := parseBigInt10(estResp.EstimatedFee)
	if err != nil {
		return evmPrepareResult{}, err
	}
	gasSafetyMult := cfg.GasSafetyMultipliers[strings.ToUpper(task.Chain)]
	feeWithBuf, err := mulCeilBigInt(fee, gasSafetyMult)
	if err != nil {
		return evmPrepareResult{}, err
	}

	// Max gas price gate (best-effort).
	if maxStr := cfg.MaxGasPrice[strings.ToUpper(task.Chain)]; strings.TrimSpace(maxStr) != "" {
		maxGP, err := parseBigInt10(maxStr)
		if err == nil {
			curGP, err := parseBigInt10(estResp.GasPrice)
			if err == nil && curGP.Cmp(maxGP) > 0 {
				wait := firstBackoff(cfg.RetryBackoffSeconds)
				t := time.Now().Local().Add(wait)
				return evmPrepareResult{
					gasLimit:    estResp.GasLimit,
					gasPrice:    estResp.GasPrice,
					deferUntil:  &t,
					deferReason: fmt.Sprintf("gas price too high: %s > %s", estResp.GasPrice, maxStr),
				}, nil
			}
		}
	}

	// Native balance check for gas.
	balStart := time.Now()
	nativeBalResp, err := e.svcCtx.Chain.GetBalance(ctx, &pb.GetBalanceReq{
		Chain:   chainEnum,
		Address: task.FromAddress,
	})
	balDur := time.Since(balStart)
	if err != nil || nativeBalResp == nil || !nativeBalResp.Success {
		logger.Errorw("chain rpc GetBalance failed",
			append(base,
				logx.Field("event", "rpc_call"),
				logx.Field("rpc_method", "Chain.GetBalance"),
				logx.Field("duration_ms", balDur.Milliseconds()),
				logx.Field("error", err),
				logx.Field("rpc_success", nativeBalResp != nil && nativeBalResp.Success),
				logx.Field("rpc_msg", safeMsg(nativeBalResp)),
			)...,
		)
		return evmPrepareResult{}, fmt.Errorf("GetBalance failed: %v %s", err, safeMsg(nativeBalResp))
	}
	if balDur > 1500*time.Millisecond {
		logger.Infow("chain rpc GetBalance slow",
			append(base,
				logx.Field("event", "rpc_slow"),
				logx.Field("rpc_method", "Chain.GetBalance"),
				logx.Field("duration_ms", balDur.Milliseconds()),
			)...,
		)
	}
	nativeBal, err := parseBigInt10(nativeBalResp.Balance)
	if err != nil {
		return evmPrepareResult{}, err
	}

	// Native reserve:
	// - native sweeps must keep the configured reserve on the deposit address.
	// - token sweeps are allowed to consume the entire native balance (reserve is ignored).
	reserve := big.NewInt(0)
	if !isToken {
		nativeSymbol := chainutil.NativeSymbolForChain(task.Chain)
		if nativeSymbol != "" {
			if reservesChain, ok := cfg.NativeReserves[strings.ToUpper(task.Chain)]; ok {
				if v := reservesChain[strings.ToUpper(nativeSymbol)]; strings.TrimSpace(v) != "" {
					if r, err := parseBigInt10(v); err == nil {
						reserve = r
					}
				}
			}
		}
	}

	availAfterReserve := new(big.Int).Sub(nativeBal, reserve)
	if availAfterReserve.Cmp(feeWithBuf) < 0 {
		feeStr := feeWithBuf.String()
		estimatedFee := &feeStr
		required := new(big.Int).Add(reserve, feeWithBuf)
		shortfall := new(big.Int).Sub(required, nativeBal)
		if shortfall.Sign() < 0 {
			shortfall = big.NewInt(0)
		}
		return evmPrepareResult{
			estimatedFee:   estimatedFee,
			gasLimit:       estResp.GasLimit,
			gasPrice:       estResp.GasPrice,
			needGas:        true,
			needGasReason:  "insufficient native balance for fee",
			requiredNative: required.String(),
			shortfall:      shortfall.String(),
		}, nil
	}

	feeStr := feeWithBuf.String()
	estimatedFee := &feeStr

	if isToken {
		return evmPrepareResult{
			estimatedFee: estimatedFee,
			amountToSend: currentAmount,
			gasLimit:     estResp.GasLimit,
			gasPrice:     estResp.GasPrice,
		}, nil
	}

	// Native sweep amount = balance - reserve - feeWithBuf
	send := new(big.Int).Sub(nativeBal, reserve)
	send.Sub(send, feeWithBuf)
	if send.Sign() <= 0 {
		required := new(big.Int).Add(reserve, feeWithBuf)
		shortfall := new(big.Int).Sub(required, nativeBal)
		if shortfall.Sign() < 0 {
			shortfall = big.NewInt(0)
		}
		return evmPrepareResult{
			estimatedFee:   estimatedFee,
			gasLimit:       estResp.GasLimit,
			gasPrice:       estResp.GasPrice,
			needGas:        true,
			needGasReason:  "net sweep amount <= 0 after reserve/fee",
			requiredNative: required.String(),
			shortfall:      shortfall.String(),
		}, nil
	}

	return evmPrepareResult{
		estimatedFee: estimatedFee,
		amountToSend: send.String(),
		gasLimit:     estResp.GasLimit,
		gasPrice:     estResp.GasPrice,
	}, nil
}

type tronPrepareResult struct {
	estimatedFee   *string
	amountToSend   string
	needBandwidth  bool
	bandwidthTopUp string // SUN (fixed cushion when bandwidth points are insufficient)
	bandwidthNeed  uint64 // estimated bandwidth_required
	bandwidthHave  uint64 // current bandwidth_available
	bandwidthMsg   string
	needEnergy     bool
	needGas        bool
	needGasReason  string
	requiredNative string // SUN
	shortfall      string // SUN
	deferUntil     *time.Time
	deferReason    string
}

func (e *Executor) prepareTronPrecheck(ctx context.Context, task *models.ConsolidationTask, currentAmount string) (tronPrepareResult, error) {
	cfg := e.svcCtx.Config.Consolidation
	logger := logx.WithContext(ctx)
	base := append(workerLogFields(e.svcCtx, "executor", -1), taskLogFields(task)...)
	isToken := task.TokenContract != nil && strings.TrimSpace(*task.TokenContract) != ""
	gasSafetyMult := cfg.GasSafetyMultipliers["TRON"]
	tronTrc20Mode := strings.ToLower(strings.TrimSpace(cfg.TronTrc20FeeMode))
	if tronTrc20Mode == "" {
		tronTrc20Mode = "energy_rental"
	}
	bwTopUpStr := strings.TrimSpace(cfg.TopUp.TronBandwidthTopUpSun)
	bwTopUpSun := big.NewInt(0)
	if bwTopUpStr != "" {
		v, err := parseBigInt10(bwTopUpStr)
		if err != nil {
			return tronPrepareResult{}, fmt.Errorf("invalid Consolidation.TopUp.TronBandwidthTopUpSun: %v", err)
		}
		bwTopUpSun = v
	}

	feeReq := &pb.EstimateTronFeeReq{
		FromAddress:     task.FromAddress,
		ToAddress:       task.ToAddress,
		Amount:          currentAmount,
		ContractAddress: "",
	}
	if isToken {
		feeReq.ContractAddress = strings.TrimSpace(*task.TokenContract)
	}
	feeStart := time.Now()
	feeResp, err := e.svcCtx.Chain.EstimateTronFee(ctx, feeReq)
	feeDur := time.Since(feeStart)
	if err != nil || feeResp == nil || !feeResp.Success {
		logger.Errorw("chain rpc EstimateTronFee failed",
			append(base,
				logx.Field("event", "rpc_call"),
				logx.Field("rpc_method", "Chain.EstimateTronFee"),
				logx.Field("duration_ms", feeDur.Milliseconds()),
				logx.Field("error", err),
				logx.Field("rpc_success", feeResp != nil && feeResp.Success),
				logx.Field("rpc_msg", safeMsg(feeResp)),
				logx.Field("is_token", isToken),
			)...,
		)
		return tronPrepareResult{}, fmt.Errorf("EstimateTronFee failed: %v %s", err, safeMsg(feeResp))
	}
	if feeDur > 1500*time.Millisecond {
		logger.Infow("chain rpc EstimateTronFee slow",
			append(base,
				logx.Field("event", "rpc_slow"),
				logx.Field("rpc_method", "Chain.EstimateTronFee"),
				logx.Field("duration_ms", feeDur.Milliseconds()),
				logx.Field("is_token", isToken),
			)...,
		)
	}

	resStart := time.Now()
	resResp, err := e.svcCtx.Chain.GetTronAccountResources(ctx, &pb.GetTronAccountResourcesReq{
		Address: task.FromAddress,
	})
	resDur := time.Since(resStart)
	if err != nil || resResp == nil || !resResp.Success {
		logger.Errorw("chain rpc GetTronAccountResources failed",
			append(base,
				logx.Field("event", "rpc_call"),
				logx.Field("rpc_method", "Chain.GetTronAccountResources"),
				logx.Field("duration_ms", resDur.Milliseconds()),
				logx.Field("error", err),
				logx.Field("rpc_success", resResp != nil && resResp.Success),
				logx.Field("rpc_msg", safeMsg(resResp)),
			)...,
		)
		return tronPrepareResult{}, fmt.Errorf("GetTronAccountResources failed: %v %s", err, safeMsg(resResp))
	}
	if resDur > 1500*time.Millisecond {
		logger.Infow("chain rpc GetTronAccountResources slow",
			append(base,
				logx.Field("event", "rpc_slow"),
				logx.Field("rpc_method", "Chain.GetTronAccountResources"),
				logx.Field("duration_ms", resDur.Milliseconds()),
			)...,
		)
	}

	trxBal, err := parseBigInt10(resResp.TrxBalanceSun)
	if err != nil {
		return tronPrepareResult{}, err
	}

	// Native reserve:
	// - native sweeps must keep the configured reserve on the deposit address.
	// - token sweeps are allowed to consume the entire TRX balance (reserve is ignored).
	reserve := big.NewInt(0)
	if !isToken {
		if reservesChain, ok := cfg.NativeReserves["TRON"]; ok {
			if v := reservesChain["TRX"]; strings.TrimSpace(v) != "" {
				if r, err := parseBigInt10(v); err == nil {
					reserve = r
				}
			}
		}
	}

	// If token transfer needs more energy than available, decide whether to rent energy.
	bandwidthInsufficient := isToken && feeResp.BandwidthRequired > 0 && resResp.BandwidthAvailable < feeResp.BandwidthRequired

	// TRON TRC20 (energy_rental mode): bandwidth points shortage may burn TRX.
	// If bandwidth is insufficient AND TRX balance is below the configured fixed cushion, request a dedicated bandwidth top-up first.
	if tronTrc20Mode == "energy_rental" && bandwidthInsufficient && bwTopUpSun.Sign() > 0 {
		availAfterReserve := new(big.Int).Sub(trxBal, reserve)
		if availAfterReserve.Cmp(bwTopUpSun) < 0 {
			required := new(big.Int).Add(bwTopUpSun, reserve)
			shortfall := new(big.Int).Sub(required, trxBal)
			if shortfall.Sign() < 0 {
				shortfall = big.NewInt(0)
			}
			return tronPrepareResult{
				needBandwidth:  true,
				bandwidthTopUp: bwTopUpSun.String(),
				bandwidthNeed:  feeResp.BandwidthRequired,
				bandwidthHave:  resResp.BandwidthAvailable,
				bandwidthMsg: fmt.Sprintf("insufficient bandwidth points (have=%d need=%d); need TRX cushion for bandwidth burn: have=%s need=%s",
					resResp.BandwidthAvailable, feeResp.BandwidthRequired, trxBal.String(), required.String()),
				requiredNative: required.String(),
				shortfall:      shortfall.String(),
			}, nil
		}
	}

	// If token transfer needs more energy than available, decide whether to rent energy.
	if isToken && feeResp.EnergyRequired > 0 && resResp.EnergyAvailable < feeResp.EnergyRequired {
		if tronTrc20Mode == "energy_rental" && cfg.EnergyRental.Enabled {
			activation := big.NewInt(0)
			if !feeResp.ToAddressActivated && strings.TrimSpace(feeResp.ActivationFeeSun) != "" {
				if v, err := parseBigInt10(feeResp.ActivationFeeSun); err == nil {
					activation = v
				}
			}

			needTrx := new(big.Int).Set(activation)
			if bandwidthInsufficient && bwTopUpSun.Sign() > 0 {
				needTrx.Add(needTrx, bwTopUpSun)
			}

			availAfterReserve := new(big.Int).Sub(trxBal, reserve)
			if availAfterReserve.Cmp(needTrx) < 0 {
				required := new(big.Int).Add(needTrx, reserve)
				shortfall := new(big.Int).Sub(required, trxBal)
				if shortfall.Sign() < 0 {
					shortfall = big.NewInt(0)
				}
				return tronPrepareResult{
					needGas:        true,
					needGasReason:  fmt.Sprintf("insufficient TRX (post-reserve) for activation/bandwidth cushion: have=%s reserve=%s need=%s", trxBal.String(), reserve.String(), needTrx.String()),
					requiredNative: required.String(),
					shortfall:      shortfall.String(),
				}, nil
			}
			return tronPrepareResult{needEnergy: true}, nil
		}

		if tronTrc20Mode == "energy_rental" && !cfg.EnergyRental.Enabled {
			wait := firstBackoff(cfg.RetryBackoffSeconds)
			t := time.Now().Local().Add(wait)
			return tronPrepareResult{
				deferUntil:  &t,
				deferReason: "insufficient energy and energy rental disabled",
			}, nil
		}
	}

	// TRX fee sufficiency check (conservative): require trx_balance - reserve >= estimated_fee_sun
	if strings.TrimSpace(feeResp.EstimatedFeeSun) != "" {
		fee, err := parseBigInt10(feeResp.EstimatedFeeSun)
		if err != nil {
			return tronPrepareResult{}, err
		}
		feeWithBuf, err := mulCeilBigInt(fee, gasSafetyMult)
		if err != nil {
			return tronPrepareResult{}, err
		}

		requiredFee := new(big.Int).Set(feeWithBuf)
		if tronTrc20Mode == "trx_fee" && bandwidthInsufficient && bwTopUpSun.Sign() > 0 {
			// NOTE: fixed bandwidth cushion is added AFTER applying buffer multiplier.
			requiredFee.Add(requiredFee, bwTopUpSun)
		}

		availAfterReserve := new(big.Int).Sub(trxBal, reserve)
		if availAfterReserve.Cmp(requiredFee) < 0 {
			feeStr := feeWithBuf.String()
			estimatedFee := &feeStr
			required := new(big.Int).Add(reserve, requiredFee)
			shortfall := new(big.Int).Sub(required, trxBal)
			if shortfall.Sign() < 0 {
				shortfall = big.NewInt(0)
			}
			reason := fmt.Sprintf("insufficient TRX (post-reserve) for fee: have=%s reserve=%s need=%s", trxBal.String(), reserve.String(), requiredFee.String())
			if tronTrc20Mode == "trx_fee" && bandwidthInsufficient && bwTopUpSun.Sign() > 0 {
				reason = fmt.Sprintf("insufficient TRX (post-reserve) for fee+bandwidth cushion: have=%s reserve=%s fee_with_buf=%s bandwidth=%s need=%s",
					trxBal.String(), reserve.String(), feeWithBuf.String(), bwTopUpSun.String(), requiredFee.String())
			}
			return tronPrepareResult{
				estimatedFee:   estimatedFee,
				needGas:        true,
				needGasReason:  reason,
				requiredNative: required.String(),
				shortfall:      shortfall.String(),
			}, nil
		}

		feeStr := feeWithBuf.String()
		estimatedFee := &feeStr

		if !isToken {
			// Native TRX sweep amount = balance - reserve - feeWithBuf
			send := new(big.Int).Sub(trxBal, reserve)
			send.Sub(send, feeWithBuf)
			if send.Sign() <= 0 {
				required := new(big.Int).Add(reserve, feeWithBuf)
				shortfall := new(big.Int).Sub(required, trxBal)
				if shortfall.Sign() < 0 {
					shortfall = big.NewInt(0)
				}
				return tronPrepareResult{
					estimatedFee:   estimatedFee,
					needGas:        true,
					needGasReason:  "net sweep amount <= 0 after reserve/fee",
					requiredNative: required.String(),
					shortfall:      shortfall.String(),
				}, nil
			}
			return tronPrepareResult{estimatedFee: estimatedFee, amountToSend: send.String()}, nil
		}

		return tronPrepareResult{estimatedFee: estimatedFee, amountToSend: currentAmount}, nil
	}

	feeStr := strings.TrimSpace(feeResp.EstimatedFeeSun)
	estimatedFee := &feeStr
	return tronPrepareResult{estimatedFee: estimatedFee, amountToSend: currentAmount}, nil
}

func (e *Executor) failOrRetry(ctx context.Context, task *models.ConsolidationTask, status models.ConsolidationTaskStatus, msg string, retriable bool, workerID int) error {
	cfg := e.svcCtx.Config.Consolidation
	now := time.Now().Local()

	nextRetry := (*time.Time)(nil)
	nextStatus := status
	nextRetryCount := task.RetryCount

	if retriable && int(task.RetryCount) < cfg.MaxRetries {
		nextRetryCount = task.RetryCount + 1
		delay := retryDelaySeconds(cfg.RetryBackoffSeconds, int(nextRetryCount))
		t := now.Add(time.Duration(delay) * time.Second)
		nextRetry = &t
		nextStatus = models.ConsolidationTaskStatusPending
	} else if retriable && int(task.RetryCount) >= cfg.MaxRetries {
		nextStatus = models.ConsolidationTaskStatusPermanentFailed
	}

	from := task.Status
	ok, err := e.svcCtx.ConsolidationTaskRepo.MarkFailed(ctx, task.ID, e.svcCtx.InstanceID, task.Version, nextStatus, msg, nextRetry, nextRetryCount)
	if err != nil {
		logx.WithContext(ctx).Errorw("mark failed/retry failed",
			append(workerLogFields(e.svcCtx, "executor", workerID),
				append(taskLogFields(task),
					logx.Field("event", "task_transition_failed"),
					logx.Field("status_to", nextStatus),
					logx.Field("error", err),
				)...,
			)...,
		)
		return err
	}
	if ok {
		writeStatusTransitionTaskLog(ctx, e.svcCtx, task, taskLogLevelForStatus(nextStatus), from, nextStatus, msg, nextRetry, map[string]any{
			"retriable":        retriable,
			"retry_count_from": task.RetryCount,
			"retry_count_to":   nextRetryCount,
			"max_retries":      cfg.MaxRetries,
			"next_status":      nextStatus,
			"next_attempt_at": func() string {
				if nextRetry == nil {
					return ""
				}
				return nextRetry.Local().Format(time.RFC3339)
			}(),
			"failure_base_type": status,
		})
	}
	return nil
}

func (e *Executor) scheduleRetryInProgress(ctx context.Context, task *models.ConsolidationTask, expectedVersion int64, msg string, workerID int) {
	if e == nil || e.svcCtx == nil || e.svcCtx.ConsolidationTaskRepo == nil || task == nil {
		return
	}
	cfg := e.svcCtx.Config.Consolidation
	now := time.Now().Local()

	if int(task.RetryCount) >= cfg.MaxRetries {
		ok, err := e.svcCtx.ConsolidationTaskRepo.MarkFailed(ctx, task.ID, e.svcCtx.InstanceID, expectedVersion, models.ConsolidationTaskStatusPermanentFailed, msg, nil, task.RetryCount)
		if err == nil && ok {
			writeStatusTransitionTaskLog(ctx, e.svcCtx, task, "ERROR", models.ConsolidationTaskStatusInProgress, models.ConsolidationTaskStatusPermanentFailed, msg, nil, map[string]any{
				"decision":    "broadcast_failed_max_retries",
				"retry_count": task.RetryCount,
				"max_retries": cfg.MaxRetries,
			})
		} else if err != nil {
			logx.WithContext(ctx).Errorw("mark permanent failed failed",
				append(workerLogFields(e.svcCtx, "executor", workerID),
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
	ok, err := e.svcCtx.ConsolidationTaskRepo.MarkRetryPending(ctx, task.ID, e.svcCtx.InstanceID, expectedVersion, msg, &next, nextRetryCount)
	if err == nil && ok {
		writeStatusTransitionTaskLog(ctx, e.svcCtx, task, "WARN", models.ConsolidationTaskStatusInProgress, models.ConsolidationTaskStatusPending, msg, &next, map[string]any{
			"decision":         "broadcast_failed_retry_pending",
			"retry_count_from": task.RetryCount,
			"retry_count_to":   nextRetryCount,
			"max_retries":      cfg.MaxRetries,
			"next_attempt_at":  next.Local().Format(time.RFC3339),
			"expected_version": expectedVersion,
		})
	} else if err != nil {
		logx.WithContext(ctx).Errorw("mark retry pending failed",
			append(workerLogFields(e.svcCtx, "executor", workerID),
				append(taskLogFields(task),
					logx.Field("event", "task_transition_failed"),
					logx.Field("status_to", models.ConsolidationTaskStatusPending),
					logx.Field("error", err),
				)...,
			)...,
		)
	}
}

func safeSignerMsg(resp *pb.SignTransactionResponse) string {
	if resp == nil {
		return ""
	}
	return fmt.Sprintf("code=%d msg=%s", resp.Code, resp.Message)
}
