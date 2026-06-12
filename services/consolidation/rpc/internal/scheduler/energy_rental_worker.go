package scheduler

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"internalwallet/proto/pb"
	"internalwallet/services/consolidation/rpc/internal/client"
	"internalwallet/services/consolidation/rpc/internal/models"
	"internalwallet/services/consolidation/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type EnergyRentalWorker struct {
	svcCtx *svc.ServiceContext
}

func NewEnergyRentalWorker(svcCtx *svc.ServiceContext) *EnergyRentalWorker {
	return &EnergyRentalWorker{svcCtx: svcCtx}
}

func (w *EnergyRentalWorker) Run(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	if w == nil || w.svcCtx == nil {
		logx.WithContext(ctx).Error("energy rental worker disabled: svcCtx not configured")
		return
	}
	cfg := w.svcCtx.Config.Consolidation
	if !cfg.Enabled || !cfg.EnergyRental.Enabled {
		logx.WithContext(ctx).Info("energy rental worker disabled by config")
		return
	}
	if strings.ToLower(strings.TrimSpace(cfg.TronTrc20FeeMode)) != "energy_rental" {
		logx.WithContext(ctx).Infof("energy rental worker disabled by TronTrc20FeeMode=%s", cfg.TronTrc20FeeMode)
		return
	}
	if w.svcCtx.DB == nil || w.svcCtx.ConsolidationTaskRepo == nil || w.svcCtx.EnergyRentalRepo == nil || w.svcCtx.Chain == nil {
		logx.WithContext(ctx).Error("energy rental worker disabled: db/repository/chain nodes not configured")
		return
	}
	if strings.TrimSpace(cfg.EnergyRental.ApiEndpoint) == "" || strings.TrimSpace(cfg.EnergyRental.ApiKey) == "" || strings.TrimSpace(cfg.EnergyRental.ApiSecret) == "" {
		logx.WithContext(ctx).Error("energy rental worker disabled: iTRX api endpoint/key/secret not configured")
		return
	}

	idle := time.Duration(cfg.EnergyCheckInterval) * time.Second
	if idle <= 0 {
		idle = 10 * time.Second
	}
	lease := time.Duration(cfg.ClaimLeaseSeconds) * time.Second
	if lease <= 0 {
		lease = 60 * time.Second
	}

	workers := cfg.MaxConcurrentPerChain.TRON
	if workers <= 0 {
		workers = 1
	}

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		workerID := i
		go func() {
			defer wg.Done()
			w.workerLoop(ctx, workerID, lease, idle)
		}()
	}

	logx.WithContext(ctx).Infof("energy rental worker started (workers=%d idle=%s lease=%s)", workers, idle, lease)
	wg.Wait()
	logx.WithContext(ctx).Info("energy rental worker stopped")
}

func (w *EnergyRentalWorker) workerLoop(ctx context.Context, workerID int, lease time.Duration, idle time.Duration) {
	time.Sleep(time.Duration((time.Now().UnixNano()+int64(workerID))*53%250) * time.Millisecond)

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

		// Idle gate: skip work when TRON has no consolidation-enabled tokens.
		rt := w.svcCtx.Runtime()
		tronHasToken := false
		if assets, ok := rt.EffectiveMinBalanceThresholds["TRON"]; ok {
			for asset := range assets {
				if strings.ToUpper(strings.TrimSpace(asset)) != "TRX" {
					tronHasToken = true
					break
				}
			}
		}
		if !tronHasToken {
			time.Sleep(idle)
			continue
		}

		now := time.Now().Local()
		claimStart := time.Now()
		tasks, err := w.svcCtx.ConsolidationTaskRepo.ClaimByStatuses(
			ctx,
			[]models.ConsolidationTaskStatus{models.ConsolidationTaskStatusNeedEnergy},
			"TRON",
			1,
			now,
			lease,
			w.svcCtx.InstanceID,
		)
		claimDur := time.Since(claimStart)
		if err != nil {
			claimErrors++
			logx.WithContext(ctx).Errorw("energy rental claim failed",
				append(workerLogFields(w.svcCtx, "energy_rental", workerID),
					logx.Field("event", "task_claim_failed"),
					logx.Field("duration_ms", claimDur.Milliseconds()),
					logx.Field("error", err),
				)...,
			)
			time.Sleep(idle)
			continue
		}
		if len(tasks) == 0 {
			emptyPolls++
			time.Sleep(idle)
		} else {
			task := tasks[0]
			processed++
			w.processOne(ctx, &task, workerID, lease)
			_ = w.svcCtx.ConsolidationTaskRepo.ReleaseClaim(context.Background(), task.ID, w.svcCtx.InstanceID)
		}

		if time.Since(lastSummary) >= 60*time.Second {
			logx.WithContext(ctx).Infow("energy rental worker loop summary",
				append(workerLogFields(w.svcCtx, "energy_rental", workerID),
					logx.Field("event", "loop_summary"),
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

func (w *EnergyRentalWorker) processOne(ctx context.Context, task *models.ConsolidationTask, workerID int, lease time.Duration) {
	if task == nil {
		return
	}
	if task.TokenContract == nil || strings.TrimSpace(*task.TokenContract) == "" {
		msg := "token_contract is required for TRON energy rental"
		from := task.Status
		ok, err := w.svcCtx.ConsolidationTaskRepo.MarkFailed(ctx, task.ID, w.svcCtx.InstanceID, task.Version, models.ConsolidationTaskStatusPermanentFailed, msg, nil, task.RetryCount)
		if err == nil && ok {
			writeStatusTransitionTaskLog(ctx, w.svcCtx, task, "ERROR", from, models.ConsolidationTaskStatusPermanentFailed, msg, nil, map[string]any{
				"decision": "missing_token_contract",
			})
		} else if err != nil {
			logx.WithContext(ctx).Errorw("mark permanent failed failed",
				append(workerLogFields(w.svcCtx, "energy_rental", workerID),
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

	taskCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	hbDone := make(chan struct{})
	go w.claimHeartbeat(taskCtx, hbDone, cancel, task.TaskID, task.ID, workerID, lease)
	defer close(hbDone)

	ver := task.Version

	cfg := w.svcCtx.Config.Consolidation.EnergyRental
	retryBase := cfg.RetryBaseSeconds
	if retryBase <= 0 {
		retryBase = 2
	}
	timeoutSeconds := cfg.TimeoutSeconds
	if timeoutSeconds <= 0 {
		timeoutSeconds = 300
	}

	// Re-estimate required energy and current resources.
	feeResp, err := w.svcCtx.Chain.EstimateTronFee(taskCtx, &pb.EstimateTronFeeReq{
		FromAddress:     task.FromAddress,
		ToAddress:       task.ToAddress,
		Amount:          task.Amount,
		ContractAddress: strings.TrimSpace(*task.TokenContract),
	})
	if err != nil || feeResp == nil || !feeResp.Success {
		logx.WithContext(taskCtx).Errorw("chain rpc EstimateTronFee failed",
			append(workerLogFields(w.svcCtx, "energy_rental", workerID),
				append(taskLogFields(task),
					logx.Field("event", "rpc_call"),
					logx.Field("rpc_method", "Chain.EstimateTronFee"),
					logx.Field("error", err),
					logx.Field("rpc_success", feeResp != nil && feeResp.Success),
					logx.Field("rpc_msg", safeMsg(feeResp)),
				)...,
			)...,
		)
		msg := fmt.Sprintf("EstimateTronFee failed: %v %s", err, safeMsg(feeResp))
		_, _ = w.svcCtx.ConsolidationTaskRepo.MarkNeedEnergy(taskCtx, task.ID, w.svcCtx.InstanceID, ver, msg, addPtrTime(time.Now().Local().Add(time.Duration(retryBase)*time.Second)))
		return
	}
	resResp, err := w.svcCtx.Chain.GetTronAccountResources(taskCtx, &pb.GetTronAccountResourcesReq{
		Address: task.FromAddress,
	})
	if err != nil || resResp == nil || !resResp.Success {
		logx.WithContext(taskCtx).Errorw("chain rpc GetTronAccountResources failed",
			append(workerLogFields(w.svcCtx, "energy_rental", workerID),
				append(taskLogFields(task),
					logx.Field("event", "rpc_call"),
					logx.Field("rpc_method", "Chain.GetTronAccountResources"),
					logx.Field("error", err),
					logx.Field("rpc_success", resResp != nil && resResp.Success),
					logx.Field("rpc_msg", safeMsg(resResp)),
				)...,
			)...,
		)
		msg := fmt.Sprintf("GetTronAccountResources failed: %v %s", err, safeMsg(resResp))
		_, _ = w.svcCtx.ConsolidationTaskRepo.MarkNeedEnergy(taskCtx, task.ID, w.svcCtx.InstanceID, ver, msg, addPtrTime(time.Now().Local().Add(time.Duration(retryBase)*time.Second)))
		return
	}

	// Guard: TRON account must be activated before it can reliably receive delegated energy / send TRC20.
	// Move to NeedGas so TopUpWorker can fund activation (and any TRX reserve/bandwidth needs).
	if !resResp.IsActivated {
		msg := "deposit address not activated; need TRX top-up"
		next := addPtrTime(time.Now().Local().Add(time.Duration(w.svcCtx.Config.Consolidation.TopUp.TaskRetryDelaySeconds) * time.Second))
		from := task.Status
		ok, err := w.svcCtx.ConsolidationTaskRepo.MarkNeedGas(taskCtx, task.ID, w.svcCtx.InstanceID, ver, msg, next)
		if err == nil && ok {
			writeStatusTransitionTaskLog(taskCtx, w.svcCtx, task, "INFO", from, models.ConsolidationTaskStatusNeedGas, msg, next, map[string]any{
				"decision": "tron_activation_required",
			})
		}
		return
	}

	if feeResp.EnergyRequired == 0 || resResp.EnergyAvailable >= feeResp.EnergyRequired {
		// Already sufficient.
		from := task.Status
		ok, err := w.svcCtx.ConsolidationTaskRepo.MarkPending(taskCtx, task.ID, w.svcCtx.InstanceID, ver, nil)
		if err == nil && ok {
			writeStatusTransitionTaskLog(taskCtx, w.svcCtx, task, "INFO", from, models.ConsolidationTaskStatusPending, "energy sufficient; moved to Pending", nil, map[string]any{
				"energy_required":  feeResp.EnergyRequired,
				"energy_available": resResp.EnergyAvailable,
			})
		}
		return
	}

	itrx := client.NewItrxClient(
		w.svcCtx.Config.Consolidation.EnergyRental.ApiEndpoint,
		w.svcCtx.Config.Consolidation.EnergyRental.ApiKey,
		w.svcCtx.Config.Consolidation.EnergyRental.ApiSecret,
		10*time.Second,
	)

	// If we already created an order for this task, do a single status check and reschedule.
	if task.EnergyRentalID != nil && *task.EnergyRentalID > 0 {
		rec, err := w.svcCtx.EnergyRentalRepo.GetByID(taskCtx, *task.EnergyRentalID)
		if err != nil {
			next := addPtrTime(time.Now().Local().Add(30 * time.Second))
			_, _ = w.svcCtx.ConsolidationTaskRepo.MarkNeedEnergy(taskCtx, task.ID, w.svcCtx.InstanceID, ver, fmt.Sprintf("energy rental record load failed: %v", err), next)
			return
		}

		// Expire old pending orders.
		deadline := rec.CreatedAt.Add(time.Duration(timeoutSeconds) * time.Second)
		if rec.Status == models.EnergyRentalStatusPending && time.Now().Local().After(deadline) {
			msg := "energy rental timeout"
			_ = w.svcCtx.EnergyRentalRepo.UpdateStatus(taskCtx, rec.ID, models.EnergyRentalStatusExpired, nil, &msg, nil)
			WriteTaskLog(taskCtx, w.svcCtx, task.TaskID, "WARN", TaskLogEventEnergyRentalOrder, msg, map[string]any{
				"order_id":         rec.OrderID,
				"rental_record_id": rec.ID,
				"deadline":         deadline.Format(time.RFC3339),
			})
			if ok, _ := w.svcCtx.ConsolidationTaskRepo.ClearEnergyRental(taskCtx, task.ID, w.svcCtx.InstanceID, ver); ok {
				ver++
				task.EnergyRentalID = nil
			}
			next := addPtrTime(time.Now().Local().Add(5 * time.Minute))
			_, _ = w.svcCtx.ConsolidationTaskRepo.MarkNeedEnergy(taskCtx, task.ID, w.svcCtx.InstanceID, ver, msg, next)
			return
		}

		switch rec.Status {
		case models.EnergyRentalStatusConfirmed:
			// Verify energy received.
			resAfter, err := w.svcCtx.Chain.GetTronAccountResources(taskCtx, &pb.GetTronAccountResourcesReq{Address: task.FromAddress})
			if err != nil || resAfter == nil || !resAfter.Success {
				logx.WithContext(taskCtx).Errorf("GetTronAccountResources after rental failed (task_id=%s): %v %s", task.TaskID, err, safeMsg(resAfter))
				next := addPtrTime(time.Now().Local().Add(time.Duration(retryBase) * time.Second))
				_, _ = w.svcCtx.ConsolidationTaskRepo.MarkNeedEnergy(taskCtx, task.ID, w.svcCtx.InstanceID, ver, "energy confirmed but resource query failed", next)
				return
			}
			if resAfter.EnergyAvailable < feeResp.EnergyRequired {
				msg := fmt.Sprintf("energy not received yet (have=%d need=%d)", resAfter.EnergyAvailable, feeResp.EnergyRequired)
				next := addPtrTime(time.Now().Local().Add(time.Duration(retryBase) * time.Second))
				_, _ = w.svcCtx.ConsolidationTaskRepo.MarkNeedEnergy(taskCtx, task.ID, w.svcCtx.InstanceID, ver, msg, next)
				return
			}
			from := task.Status
			ok, err := w.svcCtx.ConsolidationTaskRepo.MarkPending(taskCtx, task.ID, w.svcCtx.InstanceID, ver, nil)
			if err == nil && ok {
				writeStatusTransitionTaskLog(taskCtx, w.svcCtx, task, "INFO", from, models.ConsolidationTaskStatusPending, "energy rental confirmed; moved to Pending", nil, map[string]any{
					"energy_required":  feeResp.EnergyRequired,
					"energy_available": resAfter.EnergyAvailable,
					"rental_record_id": rec.ID,
					"order_id":         rec.OrderID,
				})
			}
			return

		case models.EnergyRentalStatusPending:
			qStart := time.Now()
			q, raw, err := itrx.QueryOrder(taskCtx, rec.OrderID)
			qDur := time.Since(qStart)
			if err != nil {
				logx.WithContext(taskCtx).Errorw("itrx QueryOrder failed",
					append(workerLogFields(w.svcCtx, "energy_rental", workerID),
						append(taskLogFields(task),
							logx.Field("event", "rpc_call"),
							logx.Field("rpc_method", "iTRX.QueryOrder"),
							logx.Field("duration_ms", qDur.Milliseconds()),
							logx.Field("error", err),
							logx.Field("order_id", rec.OrderID),
							logx.Field("rental_record_id", rec.ID),
						)...,
					)...,
				)
				next := addPtrTime(time.Now().Local().Add(time.Duration(retryBase) * time.Second))
				_, _ = w.svcCtx.ConsolidationTaskRepo.MarkNeedEnergy(taskCtx, task.ID, w.svcCtx.InstanceID, ver, fmt.Sprintf("itrx QueryOrder failed: %v", err), next)
				return
			}
			if qDur > 1500*time.Millisecond {
				logx.WithContext(taskCtx).Infow("itrx QueryOrder slow",
					append(workerLogFields(w.svcCtx, "energy_rental", workerID),
						append(taskLogFields(task),
							logx.Field("event", "rpc_slow"),
							logx.Field("rpc_method", "iTRX.QueryOrder"),
							logx.Field("duration_ms", qDur.Milliseconds()),
							logx.Field("order_id", rec.OrderID),
							logx.Field("rental_record_id", rec.ID),
						)...,
					)...,
				)
			}

			_ = w.svcCtx.EnergyRentalRepo.UpdateStatus(taskCtx, rec.ID, models.EnergyRentalStatusPending, raw, nil, nil)

			// Heuristic mapping: 2=confirmed, 3/4=failed/expired.
			if q.Status == 2 {
				confirmedAt := time.Now().Local()
				_ = w.svcCtx.EnergyRentalRepo.UpdateStatus(taskCtx, rec.ID, models.EnergyRentalStatusConfirmed, raw, nil, &confirmedAt)

				resAfter, err := w.svcCtx.Chain.GetTronAccountResources(taskCtx, &pb.GetTronAccountResourcesReq{Address: task.FromAddress})
				if err != nil || resAfter == nil || !resAfter.Success {
					logx.WithContext(taskCtx).Errorf("GetTronAccountResources after rental failed (task_id=%s): %v %s", task.TaskID, err, safeMsg(resAfter))
					next := addPtrTime(time.Now().Local().Add(time.Duration(retryBase) * time.Second))
					_, _ = w.svcCtx.ConsolidationTaskRepo.MarkNeedEnergy(taskCtx, task.ID, w.svcCtx.InstanceID, ver, "energy confirmed but resource query failed", next)
					return
				}
				if resAfter.EnergyAvailable < feeResp.EnergyRequired {
					msg := fmt.Sprintf("energy not received yet (have=%d need=%d)", resAfter.EnergyAvailable, feeResp.EnergyRequired)
					next := addPtrTime(time.Now().Local().Add(time.Duration(retryBase) * time.Second))
					_, _ = w.svcCtx.ConsolidationTaskRepo.MarkNeedEnergy(taskCtx, task.ID, w.svcCtx.InstanceID, ver, msg, next)
					return
				}
				from := task.Status
				ok, err := w.svcCtx.ConsolidationTaskRepo.MarkPending(taskCtx, task.ID, w.svcCtx.InstanceID, ver, nil)
				if err == nil && ok {
					writeStatusTransitionTaskLog(taskCtx, w.svcCtx, task, "INFO", from, models.ConsolidationTaskStatusPending, "energy rental confirmed; moved to Pending", nil, map[string]any{
						"energy_required":  feeResp.EnergyRequired,
						"energy_available": resAfter.EnergyAvailable,
						"rental_record_id": rec.ID,
						"order_id":         rec.OrderID,
					})
				}
				return
			}
			if q.Status == 3 || q.Status == 4 {
				status := models.EnergyRentalStatusFailed
				if q.Status == 4 {
					status = models.EnergyRentalStatusExpired
				}
				msg := fmt.Sprintf("energy rental failed (status=%d)", q.Status)
				_ = w.svcCtx.EnergyRentalRepo.UpdateStatus(taskCtx, rec.ID, status, raw, &msg, nil)
				WriteTaskLog(taskCtx, w.svcCtx, task.TaskID, "WARN", TaskLogEventEnergyRentalOrder, msg, map[string]any{
					"order_id":         rec.OrderID,
					"rental_record_id": rec.ID,
					"status":           q.Status,
				})
				if ok, _ := w.svcCtx.ConsolidationTaskRepo.ClearEnergyRental(taskCtx, task.ID, w.svcCtx.InstanceID, ver); ok {
					ver++
					task.EnergyRentalID = nil
				}
				next := addPtrTime(time.Now().Local().Add(10 * time.Minute))
				_, _ = w.svcCtx.ConsolidationTaskRepo.MarkNeedEnergy(taskCtx, task.ID, w.svcCtx.InstanceID, ver, msg, next)
				return
			}

			next := addPtrTime(time.Now().Local().Add(time.Duration(retryBase) * time.Second))
			_, _ = w.svcCtx.ConsolidationTaskRepo.MarkNeedEnergy(taskCtx, task.ID, w.svcCtx.InstanceID, ver, fmt.Sprintf("energy rental pending (status=%d)", q.Status), next)
			return

		case models.EnergyRentalStatusFailed, models.EnergyRentalStatusExpired:
			// Clear energy_rental_id so we can create a new order below.
			if ok, err := w.svcCtx.ConsolidationTaskRepo.ClearEnergyRental(taskCtx, task.ID, w.svcCtx.InstanceID, ver); err != nil || !ok {
				next := addPtrTime(time.Now().Local().Add(10 * time.Minute))
				_, _ = w.svcCtx.ConsolidationTaskRepo.MarkNeedEnergy(taskCtx, task.ID, w.svcCtx.InstanceID, ver, "failed to reset energy_rental_id for re-order", next)
				return
			}
			ver++
			task.EnergyRentalID = nil
			// Fall through and create a new order below.
		default:
			// Unknown status: be conservative and do not create a new order.
			next := addPtrTime(time.Now().Local().Add(10 * time.Minute))
			_, _ = w.svcCtx.ConsolidationTaskRepo.MarkNeedEnergy(taskCtx, task.ID, w.svcCtx.InstanceID, ver, fmt.Sprintf("unknown energy rental record status=%d", rec.Status), next)
			return
		}
	}

	// Safety caps: prevent runaway spending on a single stuck task.
	if cfg.MaxOrdersPerTask > 0 && int(task.EnergyOrderCount) >= cfg.MaxOrdersPerTask {
		msg := fmt.Sprintf("energy rental exceeded max orders per task: count=%d max=%d", task.EnergyOrderCount, cfg.MaxOrdersPerTask)
		from := task.Status
		ok, err := w.svcCtx.ConsolidationTaskRepo.MarkFailed(taskCtx, task.ID, w.svcCtx.InstanceID, ver, models.ConsolidationTaskStatusPermanentFailed, msg, nil, task.RetryCount)
		if err == nil && ok {
			writeStatusTransitionTaskLog(taskCtx, w.svcCtx, task, "ERROR", from, models.ConsolidationTaskStatusPermanentFailed, msg, nil, map[string]any{
				"energy_order_count":     task.EnergyOrderCount,
				"energy_total_cost_sun":  task.EnergyTotalCostSun,
				"max_orders_per_task":    cfg.MaxOrdersPerTask,
				"max_total_cost_sun":     cfg.MaxTotalCostSunPerTask,
				"max_energy_per_order":   cfg.MaxEnergyPerOrder,
				"token_contract_present": task.TokenContract != nil && strings.TrimSpace(*task.TokenContract) != "",
			})
		}
		return
	}
	if cfg.MaxTotalCostSunPerTask > 0 && task.EnergyTotalCostSun >= cfg.MaxTotalCostSunPerTask {
		msg := fmt.Sprintf("energy rental exceeded max total cost per task: total_sun=%d max_sun=%d", task.EnergyTotalCostSun, cfg.MaxTotalCostSunPerTask)
		from := task.Status
		ok, err := w.svcCtx.ConsolidationTaskRepo.MarkFailed(taskCtx, task.ID, w.svcCtx.InstanceID, ver, models.ConsolidationTaskStatusPermanentFailed, msg, nil, task.RetryCount)
		if err == nil && ok {
			writeStatusTransitionTaskLog(taskCtx, w.svcCtx, task, "ERROR", from, models.ConsolidationTaskStatusPermanentFailed, msg, nil, map[string]any{
				"energy_order_count":     task.EnergyOrderCount,
				"energy_total_cost_sun":  task.EnergyTotalCostSun,
				"max_orders_per_task":    cfg.MaxOrdersPerTask,
				"max_total_cost_sun":     cfg.MaxTotalCostSunPerTask,
				"max_energy_per_order":   cfg.MaxEnergyPerOrder,
				"token_contract_present": task.TokenContract != nil && strings.TrimSpace(*task.TokenContract) != "",
			})
		}
		return
	}

	shortage := int64(feeResp.EnergyRequired - resResp.EnergyAvailable)
	rentAmount := int64(w.svcCtx.Config.Consolidation.EnergyRental.DefaultRentalAmount)
	if rentAmount < shortage {
		rentAmount = shortage
	}
	if cfg.MaxEnergyPerOrder > 0 && rentAmount > int64(cfg.MaxEnergyPerOrder) {
		rentAmount = int64(cfg.MaxEnergyPerOrder)
	}
	if rentAmount <= 0 {
		rentAmount = 1
	}

	platStart := time.Now()
	platform, _, err := itrx.GetPlatformData(taskCtx)
	platDur := time.Since(platStart)
	if err != nil {
		logx.WithContext(taskCtx).Errorw("itrx GetPlatformData failed",
			append(workerLogFields(w.svcCtx, "energy_rental", workerID),
				append(taskLogFields(task),
					logx.Field("event", "rpc_call"),
					logx.Field("rpc_method", "iTRX.GetPlatformData"),
					logx.Field("duration_ms", platDur.Milliseconds()),
					logx.Field("error", err),
				)...,
			)...,
		)
		next := addPtrTime(time.Now().Local().Add(time.Duration(retryBase) * time.Second))
		_, _ = w.svcCtx.ConsolidationTaskRepo.MarkNeedEnergy(taskCtx, task.ID, w.svcCtx.InstanceID, ver, fmt.Sprintf("itrx GetPlatformData failed: %v", err), next)
		return
	}
	if platDur > 1500*time.Millisecond {
		logx.WithContext(taskCtx).Infow("itrx GetPlatformData slow",
			append(workerLogFields(w.svcCtx, "energy_rental", workerID),
				append(taskLogFields(task),
					logx.Field("event", "rpc_slow"),
					logx.Field("rpc_method", "iTRX.GetPlatformData"),
					logx.Field("duration_ms", platDur.Milliseconds()),
				)...,
			)...,
		)
	}
	if err := client.ValidatePlatformData(platform, int(rentAmount)); err != nil {
		next := addPtrTime(time.Now().Local().Add(10 * time.Minute))
		_, _ = w.svcCtx.ConsolidationTaskRepo.MarkNeedEnergy(taskCtx, task.ID, w.svcCtx.InstanceID, ver, fmt.Sprintf("itrx platform validation failed: %v", err), next)
		return
	}

	period := itrxPeriodFromSeconds(w.svcCtx.Config.Consolidation.EnergyRental.RentalDurationSeconds)
	orderStart := time.Now()
	orderResp, rawOrder, err := w.createOrderWithRetry(taskCtx, itrx, task.FromAddress, int(rentAmount), period)
	orderDur := time.Since(orderStart)
	if err != nil {
		logx.WithContext(taskCtx).Errorw("itrx CreateOrder failed",
			append(workerLogFields(w.svcCtx, "energy_rental", workerID),
				append(taskLogFields(task),
					logx.Field("event", "rpc_call"),
					logx.Field("rpc_method", "iTRX.CreateOrder"),
					logx.Field("duration_ms", orderDur.Milliseconds()),
					logx.Field("error", err),
					logx.Field("energy_amount", rentAmount),
					logx.Field("period", period),
				)...,
			)...,
		)
		msg := fmt.Sprintf("itrx CreateOrder failed: %v", err)
		next := addPtrTime(time.Now().Local().Add(60 * time.Second))
		_, _ = w.svcCtx.ConsolidationTaskRepo.MarkNeedEnergy(taskCtx, task.ID, w.svcCtx.InstanceID, ver, msg, next)
		return
	}
	if orderDur > 2000*time.Millisecond {
		logx.WithContext(taskCtx).Infow("itrx CreateOrder slow",
			append(workerLogFields(w.svcCtx, "energy_rental", workerID),
				append(taskLogFields(task),
					logx.Field("event", "rpc_slow"),
					logx.Field("rpc_method", "iTRX.CreateOrder"),
					logx.Field("duration_ms", orderDur.Milliseconds()),
					logx.Field("energy_amount", rentAmount),
					logx.Field("period", period),
				)...,
			)...,
		)
	}

	pricePerEnergy := "0"
	if rentAmount > 0 && orderResp.Amount > 0 {
		pricePerEnergy = fmt.Sprintf("%d", orderResp.Amount/int64(rentAmount))
	}

	rec := &models.EnergyRentalRecord{
		OrderID:          orderResp.Serial,
		ReceiverAddress:  task.FromAddress,
		EnergyAmount:     rentAmount,
		RentalDuration:   int32(w.svcCtx.Config.Consolidation.EnergyRental.RentalDurationSeconds),
		PricePerEnergy:   pricePerEnergy,
		TotalCost:        fmt.Sprintf("%d", orderResp.Amount),
		Provider:         w.svcCtx.Config.Consolidation.EnergyRental.Provider,
		Status:           models.EnergyRentalStatusPending,
		ProviderResponse: rawOrder,
	}
	created, err := w.svcCtx.EnergyRentalRepo.CreateIgnoreDuplicate(taskCtx, rec)
	if err != nil {
		logx.WithContext(taskCtx).Errorf("energy rental record create failed (task_id=%s): %v", task.TaskID, err)
		next := addPtrTime(time.Now().Local().Add(60 * time.Second))
		_, _ = w.svcCtx.ConsolidationTaskRepo.MarkNeedEnergy(taskCtx, task.ID, w.svcCtx.InstanceID, ver, "energy rental record create failed", next)
		return
	}
	if !created {
		existing, err := w.svcCtx.EnergyRentalRepo.GetByOrderID(taskCtx, rec.OrderID)
		if err != nil {
			logx.WithContext(taskCtx).Errorf("energy rental record already exists but load failed (task_id=%s): %v", task.TaskID, err)
			next := addPtrTime(time.Now().Local().Add(60 * time.Second))
			_, _ = w.svcCtx.ConsolidationTaskRepo.MarkNeedEnergy(taskCtx, task.ID, w.svcCtx.InstanceID, ver, "energy rental record load failed", next)
			return
		}
		rec = existing
	}
	if rec.ID == 0 {
		next := addPtrTime(time.Now().Local().Add(60 * time.Second))
		_, _ = w.svcCtx.ConsolidationTaskRepo.MarkNeedEnergy(taskCtx, task.ID, w.svcCtx.InstanceID, ver, "energy rental record id is empty", next)
		return
	}

	ok, err := w.svcCtx.ConsolidationTaskRepo.AttachEnergyRental(taskCtx, task.ID, w.svcCtx.InstanceID, ver, rec.ID, orderResp.Amount)
	if err != nil {
		logx.WithContext(taskCtx).Errorf("AttachEnergyRental failed (task_id=%s): %v", task.TaskID, err)
		next := addPtrTime(time.Now().Local().Add(60 * time.Second))
		_, _ = w.svcCtx.ConsolidationTaskRepo.MarkNeedEnergy(taskCtx, task.ID, w.svcCtx.InstanceID, ver, "AttachEnergyRental failed", next)
		return
	}
	if !ok {
		// Lost claim or concurrent update: stop.
		return
	}
	ver++
	WriteTaskLog(taskCtx, w.svcCtx, task.TaskID, "INFO", TaskLogEventEnergyRentalOrder, "energy rental order created", map[string]any{
		"order_id":                rec.OrderID,
		"rental_record_id":        rec.ID,
		"energy_amount":           rentAmount,
		"total_cost_sun":          orderResp.Amount,
		"expected_order_count":    task.EnergyOrderCount + 1,
		"expected_total_cost_sun": task.EnergyTotalCostSun + orderResp.Amount,
		"rental_duration_seconds": cfg.RentalDurationSeconds,
		"provider":                cfg.Provider,
		"max_orders_per_task":     cfg.MaxOrdersPerTask,
		"max_total_cost_sun":      cfg.MaxTotalCostSunPerTask,
		"max_energy_per_order":    cfg.MaxEnergyPerOrder,
		"energy_required":         feeResp.EnergyRequired,
		"energy_available_before": resResp.EnergyAvailable,
	})

	// Reschedule for the next tick to query order status (avoid blocking this worker).
	next := addPtrTime(time.Now().Local().Add(time.Duration(retryBase) * time.Second))
	_, _ = w.svcCtx.ConsolidationTaskRepo.MarkNeedEnergy(taskCtx, task.ID, w.svcCtx.InstanceID, ver, "energy rental order created", next)
}

func (w *EnergyRentalWorker) claimHeartbeat(ctx context.Context, done <-chan struct{}, cancel context.CancelFunc, taskID string, taskRowID int64, workerID int, lease time.Duration) {
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
					append(workerLogFields(w.svcCtx, "energy_rental", workerID),
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
					append(workerLogFields(w.svcCtx, "energy_rental", workerID),
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

func (w *EnergyRentalWorker) createOrderWithRetry(ctx context.Context, itrx *client.ItrxClient, receiver string, energyAmount int, period string) (*client.ItrxOrderResponse, []byte, error) {
	cfg := w.svcCtx.Config.Consolidation.EnergyRental
	maxRetries := cfg.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}
	base := cfg.RetryBaseSeconds
	if base <= 0 {
		base = 2
	}

	var lastErr error
	for i := 0; i < maxRetries; i++ {
		resp, raw, err := itrx.CreateOrder(ctx, receiver, energyAmount, period)
		if err == nil {
			return resp, raw, nil
		}
		lastErr = err

		backoff := time.Duration(base) * time.Second * time.Duration(1<<i)
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, nil, ctx.Err()
		case <-timer.C:
		}
	}
	return nil, nil, lastErr
}

func itrxPeriodFromSeconds(seconds int) string {
	if seconds <= 0 {
		return "1"
	}
	// iTRX API uses hours; map 3600 => "1", 7200 => "2", etc.
	h := seconds / 3600
	if h <= 0 {
		h = 1
	}
	return fmt.Sprintf("%d", h)
}
