package logic

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/repository"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/logx"
)

type TransferBatchWorkerOptions struct {
	PollInterval     time.Duration
	BatchSize        int
	ItemTimeout      time.Duration
	ProcessingLease  time.Duration
	MinRetryInterval time.Duration
}

type TransferBatchWorker struct {
	svcCtx *svc.ServiceContext

	owner            string
	pollInterval     time.Duration
	batchSize        int
	itemTimeout      time.Duration
	processingLease  time.Duration
	minRetryInterval time.Duration
}

func NewTransferBatchWorker(svcCtx *svc.ServiceContext, opts TransferBatchWorkerOptions) *TransferBatchWorker {
	host, _ := os.Hostname()
	if opts.PollInterval <= 0 {
		opts.PollInterval = 2 * time.Second
	}
	if opts.BatchSize <= 0 {
		opts.BatchSize = 20
	}
	if opts.ItemTimeout <= 0 {
		opts.ItemTimeout = 15 * time.Second
	}
	if opts.ProcessingLease <= 0 {
		opts.ProcessingLease = 10 * time.Minute
	}
	if opts.MinRetryInterval <= 0 {
		opts.MinRetryInterval = 3 * time.Second
	}
	return &TransferBatchWorker{
		svcCtx:           svcCtx,
		owner:            fmt.Sprintf("%s:%d", host, os.Getpid()),
		pollInterval:     opts.PollInterval,
		batchSize:        opts.BatchSize,
		itemTimeout:      opts.ItemTimeout,
		processingLease:  opts.ProcessingLease,
		minRetryInterval: opts.MinRetryInterval,
	}
}

func (w *TransferBatchWorker) Run(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	if w.svcCtx == nil || w.svcCtx.DB == nil || w.svcCtx.TransferBatchRepo == nil || w.svcCtx.TransferBatchItemRepo == nil {
		logx.WithContext(ctx).Error("transfer batch worker disabled: repo/db not configured")
		return
	}
	if w.svcCtx.AccountingRpc == nil {
		logx.WithContext(ctx).Error("transfer batch worker disabled: accounting rpc not configured")
		return
	}

	t := time.NewTicker(w.pollInterval)
	defer t.Stop()
	logx.WithContext(ctx).Infof("transfer batch worker started (owner=%s)", w.owner)
	for {
		select {
		case <-ctx.Done():
			logx.WithContext(ctx).Info("transfer batch worker stopped")
			return
		case <-t.C:
			w.tick(ctx)
		}
	}
}

func (w *TransferBatchWorker) tick(ctx context.Context) {
	now := time.Now()

	var batches []*model.TransferBatchModel
	if err := w.svcCtx.DB.WithContext(ctx).
		Model(&model.TransferBatchModel{}).
		Where("status = ?", "processing").
		Order("processed_at ASC").
		Limit(w.batchSize).
		Find(&batches).Error; err != nil {
		logx.WithContext(ctx).Errorf("transfer batch worker list batches failed: %v", err)
		return
	}

	for _, b := range batches {
		if b == nil || strings.TrimSpace(b.BatchID) == "" {
			continue
		}
		w.processOne(ctx, b, now)
	}
}

func (w *TransferBatchWorker) processOne(ctx context.Context, b *model.TransferBatchModel, now time.Time) {
	batchID := strings.TrimSpace(b.BatchID)
	if batchID == "" {
		return
	}
	// Safety guard: transfer batches are internal-ledger only.
	if !strings.EqualFold(strings.TrimSpace(b.Network), "internal") {
		logx.WithContext(ctx).Errorf("transfer batch worker skip non-internal batch: batch_id=%s network=%s", batchID, strings.TrimSpace(b.Network))
		return
	}

	// Recover stuck items.
	staleBefore := now.Add(-w.processingLease)
	if n, err := w.svcCtx.TransferBatchItemRepo.RecoverStaleProcessing(ctx, batchID, now, staleBefore, "processing lease expired"); err != nil {
		logx.WithContext(ctx).Errorf("transfer batch worker recover stale processing failed: batch_id=%s err=%v", batchID, err)
	} else if n > 0 {
		logx.WithContext(ctx).Infof("transfer batch worker recovered stale items: batch_id=%s n=%d", batchID, n)
	}

	// Process runnable items.
	for {
		item, err := w.svcCtx.TransferBatchItemRepo.ClaimNextRunnable(ctx, batchID, time.Now(), w.minRetryInterval)
		if err != nil {
			if errors.Is(err, repository.ErrNoRunnableItem) {
				break
			}
			logx.WithContext(ctx).Errorf("transfer batch worker claim item failed: batch_id=%s err=%v", batchID, err)
			break
		}
		if item == nil || strings.TrimSpace(item.ID) == "" {
			break
		}
		w.processItem(ctx, b, item)
	}

	w.refreshBatchStatus(ctx, b)
}

func (w *TransferBatchWorker) processItem(ctx context.Context, b *model.TransferBatchModel, item *model.TransferBatchItemModel) {
	now := time.Now()

	userID := int64(0)
	if item.UserID != nil && *item.UserID > 0 {
		userID = *item.UserID
	} else if strings.TrimSpace(item.ToAddress) != "" {
		if id, err := strconv.ParseInt(strings.TrimSpace(item.ToAddress), 10, 64); err == nil && id > 0 {
			userID = id
		}
	}
	if userID <= 0 {
		if err := w.svcCtx.TransferBatchItemRepo.MarkFailed(ctx, item.ID, "invalid user id", now); err != nil {
			logx.WithContext(ctx).Errorf("transfer batch worker mark failed (invalid user id) failed: batch_id=%s item_id=%s err=%v", strings.TrimSpace(b.BatchID), strings.TrimSpace(item.ID), err)
		}
		return
	}

	assetCode := strings.ToUpper(strings.TrimSpace(item.Currency))
	if assetCode == "" {
		assetCode = strings.ToUpper(strings.TrimSpace(b.Currency))
	}
	if assetCode == "" || assetCode == "MIXED" {
		if err := w.svcCtx.TransferBatchItemRepo.MarkFailed(ctx, item.ID, "invalid asset code", now); err != nil {
			logx.WithContext(ctx).Errorf("transfer batch worker mark failed (invalid asset code) failed: batch_id=%s item_id=%s err=%v", strings.TrimSpace(b.BatchID), strings.TrimSpace(item.ID), err)
		}
		return
	}

	amountRaw := strings.TrimSpace(item.Amount)
	if amountRaw == "" {
		if err := w.svcCtx.TransferBatchItemRepo.MarkFailed(ctx, item.ID, "invalid amount", now); err != nil {
			logx.WithContext(ctx).Errorf("transfer batch worker mark failed (invalid amount) failed: batch_id=%s item_id=%s err=%v", strings.TrimSpace(b.BatchID), strings.TrimSpace(item.ID), err)
		}
		return
	}
	amountDec, err := decimal.NewFromString(amountRaw)
	if err != nil || amountDec.LessThanOrEqual(decimal.Zero) {
		if err := w.svcCtx.TransferBatchItemRepo.MarkFailed(ctx, item.ID, "invalid amount", now); err != nil {
			logx.WithContext(ctx).Errorf("transfer batch worker mark failed (invalid amount format) failed: batch_id=%s item_id=%s err=%v", strings.TrimSpace(b.BatchID), strings.TrimSpace(item.ID), err)
		}
		return
	}
	// MariaDB DECIMAL(36,18) often scans as fixed 18 decimals (e.g. "10.500000000000000000").
	// Accounting expects amount_decimal to not exceed asset precision; strip trailing zeros to keep it exact.
	amount := amountDec.String()

	idemKey, bizRef := transferBatchCreditIdemKeyAndBizRef(b.BatchID, item.ID, b.TransferType)
	// Note is best-effort (Accounting ledger uses biz_ref/idempotency_key as the primary trace).
	note := strings.TrimSpace(func() string {
		if item.Note != nil {
			return *item.Note
		}
		return ""
	}())

	callCtx, cancel := context.WithTimeout(ctx, w.itemTimeout)
	resp, callErr := w.svcCtx.AccountingRpc.AdminAdjust(callCtx, &pb.AdminAdjustRequest{
		IdempotencyKey:        idemKey,
		BizRef:                bizRef,
		TargetOwnerType:       pb.OwnerType_OWNER_TYPE_USER,
		TargetOwnerId:         userID,
		TargetAccountTypeCode: "USER_LIABILITY",
		ChainCode:             "",
		Bucket:                pb.BalanceBucket_BUCKET_AVAILABLE,
		Direction:             pb.AdjustDirection_ADJUST_INCREASE,
		AssetCode:             assetCode,
		AmountDecimal:         amount,
		Note:                  note,
	})
	cancel()

	if err := errFromAccountingTx("TRANSFER_BATCH_CREDIT", resp, callErr); err != nil {
		errMsg := "accounting failed"
		if callErr != nil {
			errMsg = strings.TrimSpace(callErr.Error())
		} else if resp != nil && strings.TrimSpace(resp.Message) != "" {
			errMsg = strings.TrimSpace(resp.Message)
		}
		if markErr := w.svcCtx.TransferBatchItemRepo.MarkFailed(ctx, item.ID, errMsg, now); markErr != nil {
			logx.WithContext(ctx).Errorf("transfer batch worker mark failed (accounting failed) failed: batch_id=%s item_id=%s err=%v", strings.TrimSpace(b.BatchID), strings.TrimSpace(item.ID), markErr)
		}
		return
	}

	var txID *int64
	if resp != nil && resp.TxId > 0 {
		v := resp.TxId
		txID = &v
	}
	if err := w.svcCtx.TransferBatchItemRepo.MarkSuccess(ctx, item.ID, now, txID); err != nil {
		logx.WithContext(ctx).Errorf("transfer batch worker mark success failed: batch_id=%s item_id=%s err=%v", strings.TrimSpace(b.BatchID), strings.TrimSpace(item.ID), err)
	}
}

func (w *TransferBatchWorker) refreshBatchStatus(ctx context.Context, b *model.TransferBatchModel) {
	if b == nil {
		return
	}
	batchID := strings.TrimSpace(b.BatchID)
	if batchID == "" {
		return
	}
	statusCounts, err := w.svcCtx.TransferBatchItemRepo.CountByBatchGroupByStatus(ctx, batchID)
	if err != nil {
		logx.WithContext(ctx).Errorf("transfer batch worker count items failed: batch_id=%s err=%v", batchID, err)
		return
	}

	successCnt := int32(statusCounts["success"])
	failedCnt := int32(statusCounts["failed"])
	pending := statusCounts["pending"] + statusCounts["retrying"] + statusCounts["processing"]

	fields := map[string]interface{}{
		"success_count": successCnt,
		"failed_count":  failedCnt,
	}

	if pending == 0 {
		now := time.Now()
		fields["completed_at"] = &now
		if failedCnt > 0 {
			fields["status"] = "partial_failed"
		} else {
			fields["status"] = "completed"
		}
	}

	if pending == 0 && failedCnt == 0 {
		if usdValue, ok := parseNonNegativeDecimalOrEmpty(b.TotalAmountUSD); !ok || !usdValue.GreaterThan(decimal.Zero) {
			usd, ok := calcTransferBatchUsd(ctx, w.svcCtx.RedisClient, b.Currency, b.TotalAmount)
			if ok {
				fields["total_amount_usd"] = usd
			} else {
				logx.WithContext(ctx).Infof("transfer batch usd calc skipped: batch_id=%s currency=%s", batchID, strings.TrimSpace(b.Currency))
			}
		}
	}

	if err := w.svcCtx.TransferBatchRepo.UpdateFields(ctx, batchID, fields); err != nil {
		logx.WithContext(ctx).Errorf("transfer batch worker update batch failed: batch_id=%s err=%v", batchID, err)
	}
}
