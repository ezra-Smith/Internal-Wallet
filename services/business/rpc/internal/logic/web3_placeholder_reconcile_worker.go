package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	commonutils "internalwallet/common/utils"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/config"
	"internalwallet/services/business/rpc/internal/model"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type placeholderReconcileOutcome int

const (
	outcomeSkipped placeholderReconcileOutcome = iota
	outcomeTouched
	outcomeUpdated
	outcomeFailed
	outcomeDeleted
)

type Web3PlaceholderReconcileWorker struct {
	svcCtx *svc.ServiceContext
	cfg    config.Web3PlaceholderReconcileConfig

	interval     time.Duration
	minRecheck   time.Duration
	lookback     time.Duration
	failAfter    time.Duration
	chainTimeout time.Duration
	batchSize    int
	concurrency  int

	running int32
}

func NewWeb3PlaceholderReconcileWorker(svcCtx *svc.ServiceContext, cfg config.Web3PlaceholderReconcileConfig) *Web3PlaceholderReconcileWorker {
	if cfg.IntervalSeconds <= 0 {
		cfg.IntervalSeconds = 5
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 50
	}
	if cfg.BatchSize > 500 {
		cfg.BatchSize = 500
	}
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 5
	}
	if cfg.Concurrency > 50 {
		cfg.Concurrency = 50
	}
	if cfg.MinRecheckSeconds <= 0 {
		cfg.MinRecheckSeconds = 15
	}
	if cfg.LookbackHours <= 0 {
		cfg.LookbackHours = 48
	}
	if cfg.FailAfterHours <= 0 {
		cfg.FailAfterHours = 24
	}
	if cfg.ChainTimeoutSeconds <= 0 {
		cfg.ChainTimeoutSeconds = 5
	}

	return &Web3PlaceholderReconcileWorker{
		svcCtx:       svcCtx,
		cfg:          cfg,
		interval:     time.Duration(cfg.IntervalSeconds) * time.Second,
		minRecheck:   time.Duration(cfg.MinRecheckSeconds) * time.Second,
		lookback:     time.Duration(cfg.LookbackHours) * time.Hour,
		failAfter:    time.Duration(cfg.FailAfterHours) * time.Hour,
		chainTimeout: time.Duration(cfg.ChainTimeoutSeconds) * time.Second,
		batchSize:    cfg.BatchSize,
		concurrency:  cfg.Concurrency,
	}
}

func (w *Web3PlaceholderReconcileWorker) Run(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	if w == nil || w.svcCtx == nil {
		logx.WithContext(ctx).Error("web3 placeholder reconcile worker disabled: svcCtx not configured")
		return
	}
	if !w.cfg.Enabled {
		logx.WithContext(ctx).Info("web3 placeholder reconcile worker disabled by config")
		return
	}
	if w.svcCtx.DB == nil || w.svcCtx.Web3BalanceChangeRepository == nil || w.svcCtx.ChainRpc == nil {
		logx.WithContext(ctx).Error("web3 placeholder reconcile worker disabled: db/repository/chainrpc not configured")
		return
	}
	if w.svcCtx.Web3UserAddressBalanceRepository == nil {
		logx.WithContext(ctx).Error("web3 placeholder reconcile worker: Web3UserAddressBalanceRepository not configured (balances will NOT be refreshed)")
	}

	t := time.NewTicker(w.interval)
	defer t.Stop()

	logx.WithContext(ctx).Infof("web3 placeholder reconcile worker started (interval=%s batch=%d concurrency=%d minRecheck=%s lookback=%s failAfter=%s)",
		w.interval, w.batchSize, w.concurrency, w.minRecheck, w.lookback, w.failAfter,
	)

	for {
		select {
		case <-ctx.Done():
			logx.WithContext(ctx).Info("web3 placeholder reconcile worker stopped")
			return
		case <-t.C:
			w.tick(ctx)
		}
	}
}

func (w *Web3PlaceholderReconcileWorker) tick(ctx context.Context) {
	if !atomic.CompareAndSwapInt32(&w.running, 0, 1) {
		return
	}
	defer atomic.StoreInt32(&w.running, 0)

	now := time.Now().Local()
	recheckBefore := now.Add(-w.minRecheck)
	createdAfter := now.Add(-w.lookback)

	items, err := w.svcCtx.Web3BalanceChangeRepository.ListPendingBroadcastPlaceholders(ctx, w.batchSize, recheckBefore, createdAfter)
	if err != nil {
		logx.WithContext(ctx).Errorf("web3 placeholder reconcile list failed: %v", err)
		return
	}
	if len(items) == 0 {
		return
	}

	var (
		processed int64
		touched   int64
		updated   int64
		failed    int64
		deleted   int64
	)

	sem := make(chan struct{}, w.concurrency)
	var wg sync.WaitGroup
	for _, item := range items {
		if item == nil || item.ID <= 0 {
			continue
		}
		sem <- struct{}{}
		wg.Add(1)
		go func(it *model.Web3BalanceChangeModel) {
			defer wg.Done()
			defer func() { <-sem }()

			atomic.AddInt64(&processed, 1)
			outcome := w.processOne(ctx, it)
			switch outcome {
			case outcomeTouched:
				atomic.AddInt64(&touched, 1)
			case outcomeUpdated:
				atomic.AddInt64(&updated, 1)
			case outcomeFailed:
				atomic.AddInt64(&failed, 1)
			case outcomeDeleted:
				atomic.AddInt64(&deleted, 1)
			}
		}(item)
	}
	wg.Wait()

	logx.WithContext(ctx).Infof("web3 placeholder reconcile tick: scanned=%d processed=%d touched=%d updated=%d failed=%d deleted=%d",
		len(items), processed, touched, updated, failed, deleted,
	)
}

func (w *Web3PlaceholderReconcileWorker) processOne(ctx context.Context, tx *model.Web3BalanceChangeModel) placeholderReconcileOutcome {
	if w == nil || tx == nil || w.svcCtx == nil || w.svcCtx.Web3BalanceChangeRepository == nil {
		return outcomeSkipped
	}

	txHash := strings.TrimSpace(tx.TxHash)
	userAddress := strings.TrimSpace(tx.UserAddress)
	if txHash == "" || userAddress == "" {
		return outcomeSkipped
	}

	db := w.svcCtx.Web3BalanceChangeRepository.GetDB()
	if db == nil {
		return outcomeSkipped
	}

	now := time.Now().Local()

	// If a non-placeholder event already exists, remove the placeholder to avoid duplicates.
	if w.hasNonPlaceholderEvent(ctx, txHash, userAddress) {
		_ = db.WithContext(ctx).
			Model(&model.Web3BalanceChangeModel{}).
			Where("id = ? AND deleted_at IS NULL", tx.ID).
			Update("deleted_at", now).Error
		return outcomeDeleted
	}

	chainType := chainCodeToChainRpcType(tx.ChainCode)
	if chainType == pb.ChainRpcType_CHAIN_TYPE_UNSPECIFIED {
		w.touch(ctx, tx.ID, now)
		return outcomeTouched
	}

	callCtx, cancel := context.WithTimeout(ctx, w.chainTimeout)
	defer cancel()

	resp, err := w.svcCtx.ChainRpc.GetTransaction(callCtx, &pb.GetTransactionReq{
		Chain:       chainType,
		TxHash:      txHash,
		IncludeLogs: false,
	})
	if err != nil {
		logx.WithContext(ctx).Errorf("web3 placeholder reconcile get tx failed: tx_hash=%s chain=%s error=%v", txHash, tx.ChainCode, err)
		w.touch(ctx, tx.ID, now)
		return outcomeTouched
	}

	if resp == nil || !resp.Success || resp.Transaction == nil {
		msg := ""
		if resp != nil {
			msg = strings.TrimSpace(resp.Message)
		}
		if isNotFoundMessage(msg) && w.failAfter > 0 && now.Sub(tx.CreatedAt) >= w.failAfter {
			w.markFailedNotFound(ctx, tx, now, msg)
			return outcomeFailed
		}
		w.touch(ctx, tx.ID, now)
		return outcomeTouched
	}

	td := resp.Transaction
	confirmations := uint64ToInt32Clamp(td.Confirmations)
	required := requiredConfirmationsForChainCode(tx.ChainCode)
	status := deriveWeb3TxStatusFromChain(td.Status, td.Confirmations, required)

	updates := map[string]interface{}{
		"updated_at": now,
	}

	if s := strings.TrimSpace(status); s != "" && s != tx.Status {
		updates["status"] = s
	}
	if confirmations != tx.Confirmations {
		updates["confirmations"] = confirmations
	}
	if bnPtr := parseBlockNumberPtr(td.BlockNumber); bnPtr != nil {
		if tx.BlockNumber == nil || *tx.BlockNumber != *bnPtr {
			updates["block_number"] = *bnPtr
		}
	}
	if btPtr := blockTimestampToLocalTimePtr(td.BlockTimestamp); btPtr != nil {
		if tx.BlockTime == nil || !tx.BlockTime.Equal(*btPtr) {
			updates["block_time"] = btPtr
		}
	}
	feePtr, feeAssetPtr := commonutils.FormatWeb3Fee(tx.ChainCode, td.GasFee, td.GasUsed, td.GasPrice)
	if feePtr != nil {
		fee := strings.TrimSpace(*feePtr)
		if fee != "" && (tx.Fee == nil || strings.TrimSpace(*tx.Fee) != fee) {
			updates["fee"] = fee
		}
	}
	if feeAssetPtr != nil {
		feeAsset := strings.TrimSpace(*feeAssetPtr)
		if feeAsset != "" && (tx.FeeAsset == nil || strings.TrimSpace(*tx.FeeAsset) != feeAsset) {
			updates["fee_asset"] = feeAsset
		}
	}

	// Refresh address balances when the tx is mined/finalized on chain (best-effort, only once per placeholder).
	if w.shouldSyncBalances(td) && !hasBalanceSynced(tx.RawData) {
		synced, syncErr := w.syncBalancesForPlaceholder(ctx, tx, chainType)
		if syncErr != nil {
			rawData := mergeRawData(tx.RawData, map[string]interface{}{
				"reconcile": map[string]interface{}{
					"balance_sync_error":     strings.TrimSpace(syncErr.Error()),
					"balance_sync_failed_at": now.Unix(),
				},
			})
			if len(rawData) > 0 {
				updates["raw_data"] = rawData
			}
		} else if synced {
			rawData := mergeRawData(tx.RawData, map[string]interface{}{
				"reconcile": map[string]interface{}{
					"balance_synced_at":  now.Unix(),
					"balance_sync_error": nil,
				},
			})
			if len(rawData) > 0 {
				updates["raw_data"] = rawData
			}
		}
	}

	if err := db.WithContext(ctx).
		Model(&model.Web3BalanceChangeModel{}).
		Where("id = ? AND deleted_at IS NULL", tx.ID).
		Updates(updates).Error; err != nil {
		logx.WithContext(ctx).Errorf("web3 placeholder reconcile update failed: id=%d tx_hash=%s error=%v", tx.ID, txHash, err)
		return outcomeTouched
	}

	if len(updates) > 1 {
		return outcomeUpdated
	}
	return outcomeTouched
}

func (w *Web3PlaceholderReconcileWorker) touch(ctx context.Context, id int64, now time.Time) {
	if w == nil || w.svcCtx == nil || w.svcCtx.Web3BalanceChangeRepository == nil || id <= 0 {
		return
	}
	db := w.svcCtx.Web3BalanceChangeRepository.GetDB()
	if db == nil {
		return
	}
	_ = db.WithContext(ctx).
		Model(&model.Web3BalanceChangeModel{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Update("updated_at", now).Error
}

func (w *Web3PlaceholderReconcileWorker) hasNonPlaceholderEvent(ctx context.Context, txHash, userAddress string) bool {
	if w == nil || w.svcCtx == nil || w.svcCtx.Web3BalanceChangeRepository == nil {
		return false
	}
	db := w.svcCtx.Web3BalanceChangeRepository.GetDB()
	if db == nil {
		return false
	}

	var existing model.Web3BalanceChangeModel
	err := db.WithContext(ctx).
		Select("id").
		Where("tx_hash = ? AND user_address = ? AND event_index <> ? AND deleted_at IS NULL",
			txHash, userAddress, model.Web3BalanceChangeBroadcastEventIndex,
		).
		Limit(1).
		Take(&existing).Error
	if err == nil && existing.ID > 0 {
		return true
	}
	if err != nil && err != gorm.ErrRecordNotFound {
		logx.WithContext(ctx).Errorf("web3 placeholder reconcile check existing event failed: tx_hash=%s address=%s error=%v", txHash, userAddress, err)
	}
	return false
}

func requiredConfirmationsForChainCode(chainCode string) int32 {
	switch strings.ToUpper(strings.TrimSpace(chainCode)) {
	case "TRON", "TRX":
		return 20
	case "BSC", "BNB", "BNB SMART CHAIN", "BINANCE", "BINANCE SMART CHAIN":
		return 15
	case "ETH", "ETHEREUM":
		return 12
	default:
		return 12
	}
}

func (w *Web3PlaceholderReconcileWorker) shouldSyncBalances(td *pb.TransactionDetail) bool {
	if td == nil {
		return false
	}
	if td.Status != pb.TxStatus_TX_STATUS_PENDING {
		return true
	}
	return parseBlockNumberPtr(td.BlockNumber) != nil
}

func hasBalanceSynced(raw []byte) bool {
	if len(raw) == 0 {
		return false
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return false
	}
	reconcile, ok := m["reconcile"].(map[string]interface{})
	if !ok {
		return false
	}
	v, ok := reconcile["balance_synced_at"]
	if !ok {
		return false
	}
	switch vv := v.(type) {
	case float64:
		return vv > 0
	case int64:
		return vv > 0
	case int:
		return vv > 0
	default:
		return false
	}
}

func isNotFoundMessage(msg string) bool {
	msg = strings.ToLower(strings.TrimSpace(msg))
	if msg == "" {
		return false
	}
	if strings.Contains(msg, "not found") || strings.Contains(msg, "tx not found") || strings.Contains(msg, "transaction not found") {
		return true
	}
	if strings.Contains(msg, "未找到") || strings.Contains(msg, "不存在") {
		return true
	}
	return false
}

func (w *Web3PlaceholderReconcileWorker) syncBalancesForPlaceholder(ctx context.Context, tx *model.Web3BalanceChangeModel, chainType pb.ChainRpcType) (bool, error) {
	if w == nil || tx == nil || w.svcCtx == nil || w.svcCtx.ChainRpc == nil {
		return false, nil
	}
	if w.svcCtx.Web3UserAddressBalanceRepository == nil {
		return false, nil
	}

	chainCode := strings.ToUpper(strings.TrimSpace(tx.ChainCode))
	userAddress := strings.TrimSpace(tx.UserAddress)
	if chainCode == "" || userAddress == "" {
		return false, nil
	}

	chainID := tx.ChainID
	if chainID <= 0 {
		chainID = chainCodeToChainID(chainCode)
	}

	addr := &model.Web3UserAddressModel{
		Address: userAddress,
		ChainID: chainID,
	}

	nativeAsset := chainCodeToNativeFeeAsset(chainCode)
	assetCode := strings.ToUpper(strings.TrimSpace(tx.AssetCode))

	didAttempt := false
	var partialErr error

	// 1) Sync the tx asset (token or native) when we can resolve it.
	if assetCode != "" {
		if nativeAsset != "" && strings.EqualFold(assetCode, nativeAsset) {
			assetCode = nativeAsset
		}
		var contractAddr *string
		isNative := nativeAsset != "" && strings.EqualFold(assetCode, nativeAsset)
		if !isNative {
			contractAddr = w.resolveTokenContractAddress(ctx, assetCode, chainCode)
			if contractAddr == nil {
				partialErr = fmt.Errorf("token contract not configured: chain=%s asset=%s", chainCode, assetCode)
			}
		}
		if isNative || contractAddr != nil {
			decimals := getDefaultDecimals(chainCode, isNative)
			callCtx, cancel := context.WithTimeout(ctx, w.chainTimeout)
			didAttempt = true
			ok := syncAssetBalance(callCtx, w.svcCtx, addr, chainType, chainCode, assetCode, contractAddr, decimals)
			cancel()
			if !ok {
				return false, fmt.Errorf("sync address balance failed: chain=%s asset=%s address=%s", chainCode, assetCode, userAddress)
			}
		}
	}

	// 2) Sync native balance (fee impact) when tx asset differs from native.
	if nativeAsset != "" && !strings.EqualFold(assetCode, nativeAsset) {
		callCtx, cancel := context.WithTimeout(ctx, w.chainTimeout)
		didAttempt = true
		ok := syncAssetBalance(callCtx, w.svcCtx, addr, chainType, chainCode, nativeAsset, nil, getDefaultDecimals(chainCode, true))
		cancel()
		if !ok {
			return false, fmt.Errorf("sync address balance failed: chain=%s asset=%s address=%s", chainCode, nativeAsset, userAddress)
		}
	}

	if partialErr != nil {
		return false, partialErr
	}
	return didAttempt, nil
}

func (w *Web3PlaceholderReconcileWorker) resolveTokenContractAddress(ctx context.Context, assetCode string, chainCode string) *string {
	if w == nil || w.svcCtx == nil || w.svcCtx.CurrencyChainSettingsRepository == nil {
		return nil
	}
	assetCode = strings.ToUpper(strings.TrimSpace(assetCode))
	chainCode = strings.ToUpper(strings.TrimSpace(chainCode))
	if assetCode == "" || chainCode == "" {
		return nil
	}
	m, err := w.svcCtx.CurrencyChainSettingsRepository.FindByAssetChain(ctx, assetCode, chainCode)
	if err != nil || m == nil || m.ContractAddress == nil {
		return nil
	}
	v := strings.TrimSpace(*m.ContractAddress)
	if v == "" {
		return nil
	}
	return &v
}

func chainCodeToChainID(chainCode string) int64 {
	switch strings.ToUpper(strings.TrimSpace(chainCode)) {
	case "ETH", "ETHEREUM":
		return 1
	case "BSC", "BNB", "BNB SMART CHAIN", "BINANCE", "BINANCE SMART CHAIN":
		return 56
	case "TRON", "TRX":
		return 728126428
	default:
		return 0
	}
}


func (w *Web3PlaceholderReconcileWorker) markFailedNotFound(ctx context.Context, tx *model.Web3BalanceChangeModel, now time.Time, msg string) {
	if w == nil || tx == nil || w.svcCtx == nil || w.svcCtx.Web3BalanceChangeRepository == nil {
		return
	}
	db := w.svcCtx.Web3BalanceChangeRepository.GetDB()
	if db == nil {
		return
	}

	reconcile := map[string]interface{}{
		"reconcile": map[string]interface{}{
			"reason":    "not_found",
			"message":   strings.TrimSpace(msg),
			"failed_at": now.Unix(),
		},
	}

	rawData := mergeRawData(tx.RawData, reconcile)
	updates := map[string]interface{}{
		"status":     model.Web3TxStatusFailed,
		"updated_at": now,
	}
	if len(rawData) > 0 {
		updates["raw_data"] = rawData
	}

	if err := db.WithContext(ctx).
		Model(&model.Web3BalanceChangeModel{}).
		Where("id = ? AND deleted_at IS NULL", tx.ID).
		Updates(updates).Error; err != nil {
		logx.WithContext(ctx).Errorf("web3 placeholder reconcile mark failed failed: id=%d tx_hash=%s error=%v", tx.ID, strings.TrimSpace(tx.TxHash), err)
	}
}

func mergeRawData(existing []byte, patch map[string]interface{}) []byte {
	if len(patch) == 0 {
		return existing
	}

	out := map[string]interface{}{}
	if len(existing) > 0 {
		_ = json.Unmarshal(existing, &out)
	}
	for k, v := range patch {
		// Best-effort shallow merge for nested JSON objects (e.g., reconcile.*).
		if vm, ok := v.(map[string]interface{}); ok {
			if cur, ok := out[k].(map[string]interface{}); ok {
				for kk, vv := range vm {
					cur[kk] = vv
				}
				out[k] = cur
				continue
			}
		}
		out[k] = v
	}
	b, err := json.Marshal(out)
	if err != nil {
		return existing
	}
	return b
}
