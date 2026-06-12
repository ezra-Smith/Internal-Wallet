package scheduler

import (
	"strings"

	"internalwallet/services/consolidation/rpc/internal/models"
	"internalwallet/services/consolidation/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

func workerLogFields(svcCtx *svc.ServiceContext, worker string, workerID int) []logx.LogField {
	worker = strings.TrimSpace(worker)
	fields := []logx.LogField{}
	if worker != "" {
		fields = append(fields, logx.Field("worker", worker))
	}
	if svcCtx != nil && strings.TrimSpace(svcCtx.InstanceID) != "" {
		fields = append(fields, logx.Field("instance_id", strings.TrimSpace(svcCtx.InstanceID)))
	}
	if workerID >= 0 {
		fields = append(fields, logx.Field("worker_id", workerID))
	}
	return fields
}

func taskLogFields(task *models.ConsolidationTask) []logx.LogField {
	if task == nil {
		return nil
	}
	fields := []logx.LogField{
		logx.Field("task_id", strings.TrimSpace(task.TaskID)),
		logx.Field("chain", strings.ToUpper(strings.TrimSpace(task.Chain))),
		logx.Field("asset_symbol", strings.ToUpper(strings.TrimSpace(task.AssetSymbol))),
		logx.Field("from_address", strings.TrimSpace(task.FromAddress)),
		logx.Field("to_address", strings.TrimSpace(task.ToAddress)),
		logx.Field("status", task.Status),
		logx.Field("retry_count", task.RetryCount),
		logx.Field("bump_count", task.BumpCount),
		logx.Field("version", task.Version),
	}
	if task.TokenContract != nil && strings.TrimSpace(*task.TokenContract) != "" {
		fields = append(fields, logx.Field("token_contract", strings.TrimSpace(*task.TokenContract)))
	}
	if task.TxHash != nil && strings.TrimSpace(*task.TxHash) != "" {
		fields = append(fields, logx.Field("tx_hash", strings.TrimSpace(*task.TxHash)))
	}
	if task.EnergyRentalID != nil && *task.EnergyRentalID > 0 {
		fields = append(fields, logx.Field("energy_rental_id", *task.EnergyRentalID))
	}
	return fields
}
