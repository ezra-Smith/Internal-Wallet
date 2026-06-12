package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"internalwallet/services/consolidation/rpc/internal/models"
	"internalwallet/services/consolidation/rpc/internal/svc"
)

type TaskLogEvent string

const (
	TaskLogEventTaskCreated            TaskLogEvent = "task_created"
	TaskLogEventStatusTransition       TaskLogEvent = "status_transition"
	TaskLogEventTxBroadcasted          TaskLogEvent = "tx_broadcasted"
	TaskLogEventEvmGasBumped           TaskLogEvent = "evm_gas_bumped"
	TaskLogEventTopUpBroadcasted       TaskLogEvent = "topup_broadcasted"
	TaskLogEventTopUpVerified          TaskLogEvent = "topup_verified"
	TaskLogEventEnergyRentalOrder      TaskLogEvent = "energy_rental_order"
	TaskLogEventTaskCancelled          TaskLogEvent = "task_cancelled"
	TaskLogEventTaskManualRetry        TaskLogEvent = "task_manual_retry"
	TaskLogEventTaskManualTrigger      TaskLogEvent = "task_manual_trigger"
	TaskLogEventTaskManualDiscoveryDry TaskLogEvent = "task_manual_discovery_dry"
)

var allowedTaskLogEvents = map[TaskLogEvent]struct{}{
	TaskLogEventTaskCreated:            {},
	TaskLogEventStatusTransition:       {},
	TaskLogEventTxBroadcasted:          {},
	TaskLogEventEvmGasBumped:           {},
	TaskLogEventTopUpBroadcasted:       {},
	TaskLogEventTopUpVerified:          {},
	TaskLogEventEnergyRentalOrder:      {},
	TaskLogEventTaskCancelled:          {},
	TaskLogEventTaskManualRetry:        {},
	TaskLogEventTaskManualTrigger:      {},
	TaskLogEventTaskManualDiscoveryDry: {},
}

// WriteTaskLog writes an audit-style, DB-backed task timeline log.
//
// IMPORTANT:
// - This is intentionally whitelist-based to avoid DB log explosions from high-frequency loops.
// - Do NOT put sensitive data in details (private keys, raw/signed tx, API secrets).
func WriteTaskLog(ctx context.Context, svcCtx *svc.ServiceContext, taskID string, level string, event TaskLogEvent, message string, details map[string]any) {
	if svcCtx == nil || svcCtx.ConsolidationLogRepo == nil {
		return
	}
	taskID = strings.TrimSpace(taskID)
	level = strings.ToUpper(strings.TrimSpace(level))
	event = TaskLogEvent(strings.TrimSpace(string(event)))
	message = strings.TrimSpace(message)
	if taskID == "" || level == "" || string(event) == "" || message == "" {
		return
	}
	if _, ok := allowedTaskLogEvents[event]; !ok {
		return
	}

	if details == nil {
		details = map[string]any{}
	}
	details["event"] = string(event)
	if _, ok := details["task_id"]; !ok {
		details["task_id"] = taskID
	}
	if svcCtx.InstanceID != "" {
		details["instance_id"] = svcCtx.InstanceID
	}

	var raw []byte
	if b, err := json.Marshal(details); err == nil {
		raw = b
	}

	item := &models.ConsolidationLog{
		TaskID:   taskID,
		LogLevel: level,
		Message:  message,
		Details:  raw,
	}
	_ = svcCtx.ConsolidationLogRepo.Create(ctx, item)
}

func writeStatusTransitionTaskLog(ctx context.Context, svcCtx *svc.ServiceContext, task *models.ConsolidationTask, level string, from models.ConsolidationTaskStatus, to models.ConsolidationTaskStatus, msg string, nextAttemptAt *time.Time, extra map[string]any) {
	if task == nil {
		return
	}
	// Avoid DB log explosions for high-frequency “reschedule” updates where status doesn't change.
	// We still allow Pending->Pending logs because they commonly represent deferrals/backoff decisions.
	if from == to && from != models.ConsolidationTaskStatusPending {
		return
	}
	details := map[string]any{
		"chain":        strings.ToUpper(strings.TrimSpace(task.Chain)),
		"asset_symbol": strings.ToUpper(strings.TrimSpace(task.AssetSymbol)),
		"from_address": strings.TrimSpace(task.FromAddress),
		"to_address":   strings.TrimSpace(task.ToAddress),
		"status_from":  from,
		"status_to":    to,
		"retry_count":  task.RetryCount,
		"bump_count":   task.BumpCount,
		"version":      task.Version,
	}
	if task.TxHash != nil && strings.TrimSpace(*task.TxHash) != "" {
		details["tx_hash"] = strings.TrimSpace(*task.TxHash)
	}
	if task.TokenContract != nil && strings.TrimSpace(*task.TokenContract) != "" {
		details["token_contract"] = strings.TrimSpace(*task.TokenContract)
	}
	if nextAttemptAt != nil {
		details["next_attempt_at"] = nextAttemptAt.Local().Format(time.RFC3339)
	}
	if extra != nil {
		for k, v := range extra {
			details[k] = v
		}
	}
	WriteTaskLog(ctx, svcCtx, task.TaskID, level, TaskLogEventStatusTransition, msg, details)
}

func taskLogLevelForStatus(status models.ConsolidationTaskStatus) string {
	switch status {
	case models.ConsolidationTaskStatusPermanentFailed:
		return "ERROR"
	case models.ConsolidationTaskStatusFailed:
		return "ERROR"
	case models.ConsolidationTaskStatusTimeout:
		return "WARN"
	default:
		return "INFO"
	}
}

func formatDurationMs(d time.Duration) int64 {
	if d <= 0 {
		return 0
	}
	return d.Milliseconds()
}

func formatErr(err error) any {
	if err == nil {
		return nil
	}
	return fmt.Sprintf("%v", err)
}
