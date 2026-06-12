package scheduler

import (
	"context"
	"fmt"
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

type TopUpVerifyWorker struct {
	svcCtx *svc.ServiceContext
}

func NewTopUpVerifyWorker(svcCtx *svc.ServiceContext) *TopUpVerifyWorker {
	return &TopUpVerifyWorker{svcCtx: svcCtx}
}

func (w *TopUpVerifyWorker) Run(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	if w == nil || w.svcCtx == nil {
		logx.WithContext(ctx).Error("topup verify worker disabled: svcCtx not configured")
		return
	}

	cfg := w.svcCtx.Config.Consolidation
	if !cfg.Enabled || !cfg.TopUp.Enabled {
		logx.WithContext(ctx).Info("topup verify worker disabled by config")
		return
	}
	if w.svcCtx.DB == nil || w.svcCtx.TopUpRecordRepo == nil || w.svcCtx.ConsolidationTaskRepo == nil {
		logx.WithContext(ctx).Error("topup verify worker disabled: db/repository not configured")
		return
	}
	if w.svcCtx.Chain == nil {
		logx.WithContext(ctx).Error("topup verify worker disabled: chain nodes not configured")
		return
	}

	idle := time.Duration(cfg.TopUp.VerifyInterval) * time.Second
	if idle <= 0 {
		idle = 15 * time.Second
	}
	lease := time.Duration(cfg.ClaimLeaseSeconds) * time.Second
	if lease <= 0 {
		lease = 60 * time.Second
	}

	workers := 1
	if cfg.MaxConcurrentTasks > 0 {
		workers = 2
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

	logx.WithContext(ctx).Infof("topup verify worker started (workers=%d idle=%s lease=%s)", workers, idle, lease)
	wg.Wait()
	logx.WithContext(ctx).Info("topup verify worker stopped")
}

func (w *TopUpVerifyWorker) workerLoop(ctx context.Context, workerID int, lease time.Duration, idle time.Duration) {
	time.Sleep(time.Duration((time.Now().UnixNano()+int64(workerID))*89%250) * time.Millisecond)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		processed, err := w.tick(ctx, lease)
		if err != nil {
			logx.WithContext(ctx).Errorf("topup verify tick failed: %v", err)
		}
		if !processed {
			time.Sleep(idle)
		}
	}
}

func (w *TopUpVerifyWorker) tick(ctx context.Context, lease time.Duration) (bool, error) {
	records, err := w.svcCtx.TopUpRecordRepo.GetSentRecords(ctx, 50)
	if err != nil {
		return false, err
	}
	if len(records) == 0 {
		return false, nil
	}

	for i := range records {
		rec := records[i]
		w.processRecord(ctx, &rec, lease)
	}
	return true, nil
}

func (w *TopUpVerifyWorker) processRecord(ctx context.Context, rec *models.ConsolidationTopUpRecord, lease time.Duration) {
	if rec == nil {
		return
	}
	cfg := w.svcCtx.Config.Consolidation
	topCfg := cfg.TopUp

	chain := strings.ToUpper(strings.TrimSpace(rec.Chain))
	chainEnum, err := chainutil.ChainStringToEnum(chain)
	if err != nil {
		logx.WithContext(ctx).Errorw("topup verify invalid chain",
			append(workerLogFields(w.svcCtx, "topup_verify", -1),
				logx.Field("event", "record_invalid"),
				logx.Field("topup_record_id", rec.ID),
				logx.Field("task_id", strings.TrimSpace(rec.TaskID)),
				logx.Field("chain", chain),
				logx.Field("error", err),
			)...,
		)
		_ = w.svcCtx.TopUpRecordRepo.MarkFailed(ctx, rec.ID, fmt.Sprintf("invalid chain: %v", err))
		return
	}

	txHash := ""
	if rec.TxHash != nil {
		txHash = strings.TrimSpace(*rec.TxHash)
	}
	if txHash == "" {
		logx.WithContext(ctx).Errorw("topup verify missing tx_hash",
			append(workerLogFields(w.svcCtx, "topup_verify", -1),
				logx.Field("event", "record_invalid"),
				logx.Field("topup_record_id", rec.ID),
				logx.Field("task_id", strings.TrimSpace(rec.TaskID)),
				logx.Field("chain", chain),
				logx.Field("purpose", strings.TrimSpace(rec.Purpose)),
			)...,
		)
		_ = w.svcCtx.TopUpRecordRepo.MarkFailed(ctx, rec.ID, "missing tx_hash")
		return
	}

	start := time.Now()
	receiptResp, err := w.svcCtx.Chain.GetTransactionReceipt(ctx, &pb.GetTransactionReceiptReq{
		Chain:  chainEnum,
		TxHash: txHash,
	})
	dur := time.Since(start)
	if err != nil || receiptResp == nil || !receiptResp.Success {
		logx.WithContext(ctx).Errorw("chain rpc GetTransactionReceipt failed",
			append(workerLogFields(w.svcCtx, "topup_verify", -1),
				logx.Field("event", "rpc_call"),
				logx.Field("rpc_method", "Chain.GetTransactionReceipt"),
				logx.Field("duration_ms", dur.Milliseconds()),
				logx.Field("topup_record_id", rec.ID),
				logx.Field("task_id", strings.TrimSpace(rec.TaskID)),
				logx.Field("chain", chain),
				logx.Field("tx_hash", txHash),
				logx.Field("error", err),
				logx.Field("rpc_success", receiptResp != nil && receiptResp.Success),
				logx.Field("rpc_msg", safeMsg(receiptResp)),
			)...,
		)
		return
	}
	if dur > 1500*time.Millisecond {
		logx.WithContext(ctx).Infow("chain rpc GetTransactionReceipt slow",
			append(workerLogFields(w.svcCtx, "topup_verify", -1),
				logx.Field("event", "rpc_slow"),
				logx.Field("rpc_method", "Chain.GetTransactionReceipt"),
				logx.Field("duration_ms", dur.Milliseconds()),
				logx.Field("topup_record_id", rec.ID),
				logx.Field("task_id", strings.TrimSpace(rec.TaskID)),
				logx.Field("chain", chain),
				logx.Field("tx_hash", txHash),
			)...,
		)
	}
	if receiptResp.Receipt == nil {
		// Still pending.
		return
	}

	rcpt := receiptResp.Receipt
	switch rcpt.Status {
	case pb.TxStatus_TX_STATUS_CONFIRMED:
		required := topupRequiredConfirmations(topCfg, chain)
		ok, err := hasEnoughConfirmations(ctx, w.svcCtx, chainEnum, rcpt.BlockNumber, required)
		if err != nil || !ok {
			return
		}

		switch strings.ToLower(strings.TrimSpace(rec.Purpose)) {
		case "activation":
			if !w.isTronAddressActivated(ctx, rec) {
				return
			}
			confirmedAt := time.Now().Local()
			_ = w.svcCtx.TopUpRecordRepo.MarkConfirmed(ctx, rec.ID, confirmedAt)
			w.rescheduleTask(ctx, rec.TaskID, lease, "activation confirmed")
			logx.WithContext(ctx).Infow("topup record confirmed",
				append(workerLogFields(w.svcCtx, "topup_verify", -1),
					logx.Field("event", "topup_confirmed"),
					logx.Field("topup_record_id", rec.ID),
					logx.Field("task_id", strings.TrimSpace(rec.TaskID)),
					logx.Field("chain", chain),
					logx.Field("purpose", strings.TrimSpace(rec.Purpose)),
					logx.Field("tx_hash", txHash),
					logx.Field("confirmed_at", confirmedAt.Format(time.RFC3339)),
				)...,
			)
		default:
			confirmedAt := time.Now().Local()
			_ = w.svcCtx.TopUpRecordRepo.MarkConfirmed(ctx, rec.ID, confirmedAt)
			w.rescheduleTask(ctx, rec.TaskID, lease, "top-up confirmed")
			logx.WithContext(ctx).Infow("topup record confirmed",
				append(workerLogFields(w.svcCtx, "topup_verify", -1),
					logx.Field("event", "topup_confirmed"),
					logx.Field("topup_record_id", rec.ID),
					logx.Field("task_id", strings.TrimSpace(rec.TaskID)),
					logx.Field("chain", chain),
					logx.Field("purpose", strings.TrimSpace(rec.Purpose)),
					logx.Field("tx_hash", txHash),
					logx.Field("confirmed_at", confirmedAt.Format(time.RFC3339)),
				)...,
			)
		}
		return

	case pb.TxStatus_TX_STATUS_FAILED, pb.TxStatus_TX_STATUS_DROPPED, pb.TxStatus_TX_STATUS_REPLACED:
		msg := fmt.Sprintf("tx status=%s", rcpt.Status.String())
		_ = w.svcCtx.TopUpRecordRepo.MarkFailed(ctx, rec.ID, msg)
		w.rescheduleTask(ctx, rec.TaskID, lease, msg)
		logx.WithContext(ctx).Errorw("topup record failed",
			append(workerLogFields(w.svcCtx, "topup_verify", -1),
				logx.Field("event", "topup_failed"),
				logx.Field("topup_record_id", rec.ID),
				logx.Field("task_id", strings.TrimSpace(rec.TaskID)),
				logx.Field("chain", chain),
				logx.Field("purpose", strings.TrimSpace(rec.Purpose)),
				logx.Field("tx_hash", txHash),
				logx.Field("tx_status", rcpt.Status.String()),
			)...,
		)
		return

	default:
		// Pending/unknown.
		return
	}
}

func topupRequiredConfirmations(cfg config.TopUpConfig, chain string) uint64 {
	switch strings.ToUpper(strings.TrimSpace(chain)) {
	case "TRON":
		return cfg.VerificationConfirmations.TRON
	case "ETH":
		return cfg.VerificationConfirmations.ETH
	case "BSC":
		return cfg.VerificationConfirmations.BSC
	default:
		return 0
	}
}

func hasEnoughConfirmations(ctx context.Context, svcCtx *svc.ServiceContext, chain pb.ChainRpcType, txBlock string, required uint64) (bool, error) {
	if required == 0 {
		return true, nil
	}
	txHeight, err := parseUint64(txBlock)
	if err != nil {
		return false, nil
	}
	h, err := svcCtx.Chain.GetBlockHeight(ctx, &pb.GetBlockHeightReq{Chain: chain})
	if err != nil || h == nil || !h.Success {
		return false, nil
	}
	if h.BlockHeight < txHeight {
		return false, nil
	}
	confs := h.BlockHeight - txHeight + 1
	return confs >= required, nil
}

func parseUint64(s string) (uint64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty")
	}
	var v uint64
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch < '0' || ch > '9' {
			return 0, fmt.Errorf("not uint64")
		}
		v = v*10 + uint64(ch-'0')
	}
	return v, nil
}

func (w *TopUpVerifyWorker) isTronAddressActivated(ctx context.Context, rec *models.ConsolidationTopUpRecord) bool {
	if rec == nil {
		return false
	}
	if strings.ToUpper(strings.TrimSpace(rec.Chain)) != "TRON" {
		return false
	}
	res, err := w.svcCtx.Chain.GetTronAccountResources(ctx, &pb.GetTronAccountResourcesReq{Address: rec.ToAddress})
	if err != nil || res == nil || !res.Success {
		return false
	}
	return res.IsActivated
}

func (w *TopUpVerifyWorker) rescheduleTask(ctx context.Context, taskID string, lease time.Duration, reason string) {
	task, ok, err := w.svcCtx.ConsolidationTaskRepo.ClaimByTaskID(ctx, taskID, time.Now().Local(), lease, w.svcCtx.InstanceID)
	if err != nil || !ok || task == nil {
		return
	}
	defer func() {
		_ = w.svcCtx.ConsolidationTaskRepo.ReleaseClaim(context.Background(), task.ID, w.svcCtx.InstanceID)
	}()

	// If task already progressed, nothing to do.
	if task.Status != models.ConsolidationTaskStatusNeedGas && task.Status != models.ConsolidationTaskStatusNeedEnergy && task.Status != models.ConsolidationTaskStatusNeedBandwidth {
		return
	}

	// Best-effort: if gas is now sufficient, move to Pending; otherwise keep current status but allow retry soon.
	if task.Status == models.ConsolidationTaskStatusNeedGas {
		needGas, _, _, err := NewTopUpWorker(w.svcCtx).stillNeedsGas(ctx, task)
		if err == nil && !needGas {
			from := task.Status
			ok, err := w.svcCtx.ConsolidationTaskRepo.MarkPending(ctx, task.ID, w.svcCtx.InstanceID, task.Version, nil)
			if err == nil && ok {
				writeStatusTransitionTaskLog(ctx, w.svcCtx, task, "INFO", from, models.ConsolidationTaskStatusPending, "top-up verified; moved to Pending", nil, map[string]any{
					"reason": reason,
				})
			}
			return
		}
		next := addPtrTime(time.Now().Local().Add(time.Duration(w.svcCtx.Config.Consolidation.TopUp.TaskRetryDelaySeconds) * time.Second))
		_, _ = w.svcCtx.ConsolidationTaskRepo.MarkNeedGas(ctx, task.ID, w.svcCtx.InstanceID, task.Version, fmt.Sprintf("top-up verified but still insufficient gas: %s", reason), next)
		return
	}

	// NeedEnergy: keep NeedEnergy but make it claimable again.
	if task.Status == models.ConsolidationTaskStatusNeedBandwidth {
		from := task.Status
		ok, err := w.svcCtx.ConsolidationTaskRepo.MarkNeedEnergy(ctx, task.ID, w.svcCtx.InstanceID, task.Version, reason, nil)
		if err == nil && ok {
			writeStatusTransitionTaskLog(ctx, w.svcCtx, task, "INFO", from, models.ConsolidationTaskStatusNeedEnergy, "bandwidth top-up verified; moved to NeedEnergy", nil, map[string]any{
				"reason": reason,
			})
		}
		return
	}

	_, _ = w.svcCtx.ConsolidationTaskRepo.MarkNeedEnergy(ctx, task.ID, w.svcCtx.InstanceID, task.Version, reason, nil)
}
