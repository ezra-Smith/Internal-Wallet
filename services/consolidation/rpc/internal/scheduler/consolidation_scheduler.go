package scheduler

import (
	"context"
	"fmt"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"internalwallet/proto/pb"
	"internalwallet/services/consolidation/rpc/internal/chainutil"
	"internalwallet/services/consolidation/rpc/internal/models"
	"internalwallet/services/consolidation/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type DiscoveryOptions struct {
	Chain       string
	AssetSymbol string
	FromAddress string
	DryRun      bool
	Force       bool // bypass scheduled strategy gate
}

type DiscoveryResult struct {
	TotalCandidates int64
	TasksCreated    int64
	Tasks           []models.ConsolidationTask
}

type ConsolidationScheduler struct {
	svcCtx *svc.ServiceContext

	running int32

	lastScheduledRunDay string // YYYY-MM-DD in UTC

	hotWalletMu    sync.Mutex
	hotWalletCache map[string]string // chain -> system hot wallet address
}

type discoveryChainStats struct {
	chain            string
	addressesScanned int64
	rpcErrors        int64
	rpcUnsuccessful  int64
	skippedActive    int64
	skippedTerminal  int64
	duplicates       int64
	tasksCreated     int64
	totalCandidates  int64
}

func NewConsolidationScheduler(svcCtx *svc.ServiceContext) *ConsolidationScheduler {
	return &ConsolidationScheduler{svcCtx: svcCtx}
}

func (s *ConsolidationScheduler) Run(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	if s == nil || s.svcCtx == nil {
		logx.WithContext(ctx).Error("consolidation discovery scheduler disabled: svcCtx not configured")
		return
	}
	if !s.svcCtx.Config.Consolidation.Enabled {
		logx.WithContext(ctx).Info("consolidation discovery scheduler disabled by config")
		return
	}
	if s.svcCtx.DB == nil || s.svcCtx.AddressRepo == nil || s.svcCtx.ConsolidationTaskRepo == nil {
		logx.WithContext(ctx).Error("consolidation discovery scheduler disabled: db/repositories not configured")
		return
	}
	if s.svcCtx.Chain == nil {
		logx.WithContext(ctx).Error("consolidation discovery scheduler disabled: chain nodes not configured")
		return
	}

	interval := time.Duration(s.svcCtx.Config.Consolidation.DiscoveryInterval) * time.Second
	if interval <= 0 {
		interval = 5 * time.Minute
	}

	t := time.NewTicker(interval)
	defer t.Stop()

	logx.WithContext(ctx).Infof("consolidation discovery scheduler started (interval=%s strategy=%s)", interval, s.svcCtx.Config.Consolidation.Strategy)

	for {
		select {
		case <-ctx.Done():
			logx.WithContext(ctx).Info("consolidation discovery scheduler stopped")
			return
		case <-t.C:
			s.tick(ctx, DiscoveryOptions{})
		}
	}
}

func (s *ConsolidationScheduler) DiscoverOnce(ctx context.Context, opts DiscoveryOptions) (*DiscoveryResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	opts.Force = true
	return s.tick(ctx, opts)
}

func (s *ConsolidationScheduler) tick(ctx context.Context, opts DiscoveryOptions) (*DiscoveryResult, error) {
	if !atomic.CompareAndSwapInt32(&s.running, 0, 1) {
		return &DiscoveryResult{}, nil
	}
	defer atomic.StoreInt32(&s.running, 0)

	cfg := s.svcCtx.Config.Consolidation
	if !cfg.Enabled {
		return &DiscoveryResult{}, nil
	}

	// Strategy gate (manual DiscoverOnce bypasses it).
	if !opts.Force {
		strategy := strings.ToLower(strings.TrimSpace(cfg.Strategy))
		if strategy == "scheduled" {
			ok, err := s.shouldRunScheduled(time.Now().UTC())
			if err != nil {
				logx.WithContext(ctx).Errorf("scheduled strategy parse failed: %v", err)
				return &DiscoveryResult{}, err
			}
			if !ok {
				return &DiscoveryResult{}, nil
			}
		}
	}

	res := &DiscoveryResult{}
	tickStart := time.Now()
	now := time.Now().Local()

	chainFilter := strings.ToUpper(strings.TrimSpace(opts.Chain))
	assetFilter := strings.ToUpper(strings.TrimSpace(opts.AssetSymbol))
	fromFilter := strings.TrimSpace(opts.FromAddress)

	rt := s.svcCtx.Runtime()

	// Scan deposit addresses by ID cursor for each tick.
	chains := chainutil.SupportedChains()
	if chainFilter != "" {
		chains = []string{chainFilter}
	}

	chainStats := make(map[string]*discoveryChainStats, len(chains))
	for _, chain := range chains {
		thresholds, ok := rt.EffectiveMinBalanceThresholds[chain]
		if !ok || len(thresholds) == 0 {
			continue
		}
		stats := &discoveryChainStats{chain: chain}
		chainStats[chain] = stats

		afterID := int64(0)
		for {
			rows, err := s.svcCtx.AddressRepo.ListConsolidatableDepositAddresses(ctx, []string{chain}, afterID, 1000)
			if err != nil {
				logx.WithContext(ctx).Errorf("discovery list deposit addresses failed (chain=%s): %v", chain, err)
				break
			}
			if len(rows) == 0 {
				break
			}
			afterID = rows[len(rows)-1].ID

			for _, row := range rows {
				stats.addressesScanned++
				if fromFilter != "" && chainutil.NormalizeAddress(chain, fromFilter) != chainutil.NormalizeAddress(chain, row.Address) {
					continue
				}
				addr := chainutil.NormalizeAddress(chain, row.Address)
				created, err := s.discoverOneAddress(ctx, now, chain, addr, thresholds, assetFilter, opts.DryRun, stats)
				if err != nil {
					logx.WithContext(ctx).Errorf("discovery address failed (chain=%s addr=%s): %v", chain, row.Address, err)
					continue
				}
				res.TotalCandidates += int64(created.totalCandidates)
				res.TasksCreated += int64(created.createdCount)
				res.Tasks = append(res.Tasks, created.createdTasks...)
			}
		}
	}

	// Tick summary (one line, plus per-chain anomalies).
	fields := append(workerLogFields(s.svcCtx, "discovery", -1),
		logx.Field("event", "discovery_tick"),
		logx.Field("duration_ms", time.Since(tickStart).Milliseconds()),
		logx.Field("chain_filter", chainFilter),
		logx.Field("asset_filter", assetFilter),
		logx.Field("from_filter", fromFilter),
		logx.Field("dry_run", opts.DryRun),
		logx.Field("total_candidates", res.TotalCandidates),
		logx.Field("tasks_created", res.TasksCreated),
	)
	logx.WithContext(ctx).Infow("consolidation discovery tick", fields...)
	for _, st := range chainStats {
		if st == nil {
			continue
		}
		// Log only when there is signal (created tasks or RPC issues).
		if st.tasksCreated == 0 && st.rpcErrors == 0 && st.rpcUnsuccessful == 0 {
			continue
		}
		logx.WithContext(ctx).Infow("consolidation discovery tick chain", append(workerLogFields(s.svcCtx, "discovery", -1),
			logx.Field("event", "discovery_tick_chain"),
			logx.Field("chain", st.chain),
			logx.Field("addresses_scanned", st.addressesScanned),
			logx.Field("total_candidates", st.totalCandidates),
			logx.Field("tasks_created", st.tasksCreated),
			logx.Field("duplicates", st.duplicates),
			logx.Field("skipped_active_task", st.skippedActive),
			logx.Field("skipped_terminal_failed", st.skippedTerminal),
			logx.Field("rpc_errors", st.rpcErrors),
			logx.Field("rpc_unsuccessful", st.rpcUnsuccessful),
		)...)
	}

	return res, nil
}

type discoverOutcome struct {
	totalCandidates int
	createdCount    int
	createdTasks    []models.ConsolidationTask
}

func (s *ConsolidationScheduler) discoverOneAddress(ctx context.Context, now time.Time, chain string, address string, thresholds map[string]string, assetFilter string, dryRun bool, stats *discoveryChainStats) (discoverOutcome, error) {
	out := discoverOutcome{}

	cfg := s.svcCtx.Config.Consolidation

	address = chainutil.NormalizeAddress(chain, address)

	chainEnum, err := chainutil.ChainStringToEnum(chain)
	if err != nil {
		return out, err
	}

	thresholdsNorm := make(map[string]string, len(thresholds))
	for k, v := range thresholds {
		thresholdsNorm[strings.ToUpper(strings.TrimSpace(k))] = strings.TrimSpace(v)
	}

	// Address-level dedupe: only one active task per (chain, from_address).
	// If any active task exists, do nothing for this address (prevents nonce/resource contention across instances).
	active, err := s.svcCtx.ConsolidationTaskRepo.HasActiveTask(ctx, chain, address)
	if err == nil && active {
		if stats != nil {
			stats.skippedActive++
		}
		return out, nil
	}

	// Terminal failure guard (token-only): once a token task hits PermanentFailed (or Failed), stop auto-discovery for this address.
	// This prevents bypassing MaxRetries via "new task" creation loops, while allowing native failures to recover naturally.
	terminalFailed, err := s.svcCtx.ConsolidationTaskRepo.HasTerminalFailedTokenTask(ctx, chain, address)
	if err == nil && terminalFailed {
		if stats != nil {
			stats.skippedTerminal++
		}
		return out, nil
	}

	// Token-first: if any token balance is above threshold, skip native sweep for this address (prevents “no gas/TRX”).
	nativeSymbol := chainutil.NativeSymbolForChain(chain)
	var tokenAssets []string
	for asset := range thresholdsNorm {
		a := strings.ToUpper(strings.TrimSpace(asset))
		if assetFilter != "" && a != assetFilter {
			continue
		}
		if a == "" {
			continue
		}
		if a == strings.ToUpper(nativeSymbol) {
			continue
		}
		tokenAssets = append(tokenAssets, a)
	}
	sort.Slice(tokenAssets, func(i, j int) bool {
		pi := 2
		pj := 2
		if tokenAssets[i] == "USDT" {
			pi = 0
		} else if tokenAssets[i] == "USDC" {
			pi = 1
		}
		if tokenAssets[j] == "USDT" {
			pj = 0
		} else if tokenAssets[j] == "USDC" {
			pj = 1
		}
		if pi != pj {
			return pi < pj
		}
		return tokenAssets[i] < tokenAssets[j]
	})
	var nativeAssets []string
	for asset := range thresholdsNorm {
		a := strings.ToUpper(strings.TrimSpace(asset))
		if assetFilter != "" && a != assetFilter {
			continue
		}
		if a == "" {
			continue
		}
		if a == strings.ToUpper(nativeSymbol) {
			nativeAssets = append(nativeAssets, a)
		}
	}
	sort.Strings(nativeAssets)

	for _, asset := range tokenAssets {
		threshold := thresholdsNorm[asset]
		toAddr := s.getTargetAddress(ctx, chain, asset)
		if toAddr == "" {
			continue
		}

		contract, err := s.resolveTokenContract(asset, chain)
		if err != nil {
			continue
		}

		balResp, err := s.svcCtx.Chain.GetTokenBalance(ctx, &pb.GetTokenBalanceReq{
			Chain:         chainEnum,
			Address:       address,
			TokenContract: *contract,
		})
		if err != nil {
			if stats != nil {
				stats.rpcErrors++
			}
			continue
		}
		if balResp == nil || !balResp.Success {
			if stats != nil {
				stats.rpcUnsuccessful++
			}
			continue
		}
		out.totalCandidates++
		if stats != nil {
			stats.totalCandidates++
		}

		ok, err := geBigIntString(balResp.Balance, threshold)
		if err != nil || !ok {
			continue
		}

		cooldownSince := addSeconds(now, -s.svcCtx.Config.Consolidation.ConsolidationCooldown)
		recent, err := s.svcCtx.ConsolidationTaskRepo.HasRecentConfirmed(ctx, chain, asset, address, contract, cooldownSince)
		if err == nil && recent {
			continue
		}

		task := models.ConsolidationTask{
			TaskID:        buildTaskID(chain, asset, address, now),
			Chain:         chain,
			AssetSymbol:   asset,
			TokenContract: contract,
			FromAddress:   address,
			ToAddress:     toAddr,
			Amount:        balResp.Balance,
			Status:        models.ConsolidationTaskStatusPending,
		}
		if dryRun {
			out.createdTasks = append(out.createdTasks, task)
			out.createdCount++
			return out, nil
		}

		created, err := s.svcCtx.ConsolidationTaskRepo.CreateIgnoreDuplicate(ctx, &task)
		if err != nil {
			return out, err
		}
		if !created {
			// Most likely blocked by address-level active task uniqueness.
			if stats != nil {
				stats.duplicates++
			}
			return out, nil
		}
		out.createdCount++
		if stats != nil {
			stats.tasksCreated++
		}
		out.createdTasks = append(out.createdTasks, task)

		WriteTaskLog(ctx, s.svcCtx, task.TaskID, "INFO", TaskLogEventTaskCreated, "consolidation task created", map[string]any{
			"chain":          strings.ToUpper(strings.TrimSpace(task.Chain)),
			"asset_symbol":   strings.ToUpper(strings.TrimSpace(task.AssetSymbol)),
			"from_address":   strings.TrimSpace(task.FromAddress),
			"to_address":     strings.TrimSpace(task.ToAddress),
			"token_contract": strings.TrimSpace(*task.TokenContract),
			"amount":         strings.TrimSpace(task.Amount),
			"threshold":      strings.TrimSpace(threshold),
		})
		logx.WithContext(ctx).Infow("consolidation task created",
			append(workerLogFields(s.svcCtx, "discovery", -1),
				append(taskLogFields(&task),
					logx.Field("event", "task_created"),
					logx.Field("threshold", strings.TrimSpace(threshold)),
				)...,
			)...,
		)
		return out, nil
	}

	// Native (optional)
	for _, asset := range nativeAssets {
		toAddr := s.getTargetAddress(ctx, chain, asset)
		if toAddr == "" {
			continue
		}
		balResp, err := s.svcCtx.Chain.GetBalance(ctx, &pb.GetBalanceReq{
			Chain:   chainEnum,
			Address: address,
		})
		if err != nil {
			if stats != nil {
				stats.rpcErrors++
			}
			continue
		}
		if balResp == nil || !balResp.Success {
			if stats != nil {
				stats.rpcUnsuccessful++
			}
			continue
		}
		out.totalCandidates++
		if stats != nil {
			stats.totalCandidates++
		}

		bal, err := parseBigInt10(balResp.Balance)
		if err != nil {
			continue
		}
		reserve := big.NewInt(0)
		if m, ok := cfg.NativeReserves[chain]; ok {
			if v := strings.TrimSpace(m[asset]); v != "" {
				if r, err := parseBigInt10(v); err == nil {
					reserve = r
				}
			}
		}
		minStr := strings.TrimSpace(cfg.MinConsolidationAmount[chain][asset])
		minAmt, err := parseBigInt10(minStr)
		if err != nil {
			continue
		}
		available := new(big.Int).Sub(bal, reserve)
		if available.Sign() < 0 {
			available = big.NewInt(0)
		}
		if available.Cmp(minAmt) < 0 {
			continue
		}

		cooldownSince := addSeconds(now, -s.svcCtx.Config.Consolidation.ConsolidationCooldown)
		recent, err := s.svcCtx.ConsolidationTaskRepo.HasRecentConfirmed(ctx, chain, asset, address, nil, cooldownSince)
		if err == nil && recent {
			continue
		}

		task := models.ConsolidationTask{
			TaskID:      buildTaskID(chain, asset, address, now),
			Chain:       chain,
			AssetSymbol: asset,
			FromAddress: address,
			ToAddress:   toAddr,
			Amount:      balResp.Balance,
			Status:      models.ConsolidationTaskStatusPending,
		}
		if dryRun {
			out.createdTasks = append(out.createdTasks, task)
			out.createdCount++
			return out, nil
		}

		created, err := s.svcCtx.ConsolidationTaskRepo.CreateIgnoreDuplicate(ctx, &task)
		if err != nil {
			return out, err
		}
		if !created {
			if stats != nil {
				stats.duplicates++
			}
			return out, nil
		}
		out.createdCount++
		if stats != nil {
			stats.tasksCreated++
		}
		out.createdTasks = append(out.createdTasks, task)

		WriteTaskLog(ctx, s.svcCtx, task.TaskID, "INFO", TaskLogEventTaskCreated, "consolidation task created", map[string]any{
			"chain":        strings.ToUpper(strings.TrimSpace(task.Chain)),
			"asset_symbol": strings.ToUpper(strings.TrimSpace(task.AssetSymbol)),
			"from_address": strings.TrimSpace(task.FromAddress),
			"to_address":   strings.TrimSpace(task.ToAddress),
			"amount":       strings.TrimSpace(task.Amount),
			"reserve":      reserve.String(),
			"available":    available.String(),
			"min_amount":   minStr,
		})
		logx.WithContext(ctx).Infow("consolidation task created",
			append(workerLogFields(s.svcCtx, "discovery", -1),
				append(taskLogFields(&task),
					logx.Field("event", "task_created"),
					logx.Field("reserve", reserve.String()),
					logx.Field("available", available.String()),
					logx.Field("min_amount", strings.TrimSpace(minStr)),
				)...,
			)...,
		)
		return out, nil
	}

	return out, nil
}

func buildTaskID(chain string, asset string, from string, now time.Time) string {
	// Keep <= 64 chars: {chain}_{asset}_{from}_{ts}
	return fmt.Sprintf("%s_%s_%s_%d", strings.ToUpper(chain), strings.ToUpper(asset), strings.TrimSpace(from), now.Unix())
}

func (s *ConsolidationScheduler) getTargetAddress(ctx context.Context, chain, asset string) string {
	if ctx == nil {
		ctx = context.Background()
	}
	chain = strings.ToUpper(strings.TrimSpace(chain))
	asset = strings.ToUpper(strings.TrimSpace(asset))
	if s.svcCtx == nil {
		return ""
	}

	cfg := s.svcCtx.Config.Consolidation
	switch strings.ToLower(strings.TrimSpace(cfg.ToAddressMode)) {
	case "", "target_addresses":
		if m, ok := cfg.TargetAddresses[chain]; ok {
			return strings.TrimSpace(m[asset])
		}
		return ""
	case "system_hot_wallet":
		addr, err := s.getSystemHotWalletAddress(ctx, chain)
		if err != nil {
			return ""
		}
		return addr
	default:
		// validated on startup; treat as disabled
		return ""
	}
}

func (s *ConsolidationScheduler) getSystemHotWalletAddress(ctx context.Context, chain string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	chain = strings.ToUpper(strings.TrimSpace(chain))
	if chain == "" {
		return "", fmt.Errorf("chain is empty")
	}
	if s == nil || s.svcCtx == nil || s.svcCtx.SignerRpc == nil {
		return "", fmt.Errorf("SignerRpc not configured")
	}

	s.hotWalletMu.Lock()
	if s.hotWalletCache == nil {
		s.hotWalletCache = map[string]string{}
	}
	if v := strings.TrimSpace(s.hotWalletCache[chain]); v != "" {
		s.hotWalletMu.Unlock()
		return v, nil
	}
	s.hotWalletMu.Unlock()

	cfg := s.svcCtx.Config.Consolidation
	start := time.Now()
	resp, err := s.svcCtx.SignerRpc.GetCompanyWallet(ctx, &pb.GetCompanyWalletRequest{
		Chain:       chain,
		AddressType: strings.TrimSpace(cfg.SystemHotWalletAddressType),
		Temperature: cfg.SystemHotWalletTemperature,
	})
	dur := time.Since(start)
	if err != nil || resp == nil || resp.Code != 0 || resp.Wallet == nil || strings.TrimSpace(resp.Wallet.Address) == "" {
		logx.WithContext(ctx).Errorw("signer rpc GetCompanyWallet failed",
			append(workerLogFields(s.svcCtx, "discovery", -1),
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

	s.hotWalletMu.Lock()
	s.hotWalletCache[chain] = addr
	s.hotWalletMu.Unlock()
	return addr, nil
}

func (s *ConsolidationScheduler) resolveTokenContract(asset string, chain string) (*string, error) {
	chain = strings.ToUpper(strings.TrimSpace(chain))
	asset = strings.ToUpper(strings.TrimSpace(asset))
	if s == nil || s.svcCtx == nil {
		return nil, fmt.Errorf("svcCtx not configured")
	}
	rt := s.svcCtx.Runtime()
	chainMap, ok := rt.TokenConfigs[chain]
	if !ok {
		return nil, fmt.Errorf("token config not found for chain %s", chain)
	}
	cfg, ok := chainMap[asset]
	if !ok || strings.TrimSpace(cfg.Contract) == "" {
		return nil, fmt.Errorf("token config not found for %s/%s", chain, asset)
	}
	contract := chainutil.NormalizeTokenContract(chain, strings.TrimSpace(cfg.Contract))
	return &contract, nil
}

func (s *ConsolidationScheduler) shouldRunScheduled(nowUTC time.Time) (bool, error) {
	t := strings.TrimSpace(s.svcCtx.Config.Consolidation.ScheduledTime)
	if t == "" {
		return false, fmt.Errorf("ScheduledTime is empty")
	}
	parts := strings.Split(t, ":")
	if len(parts) != 2 {
		return false, fmt.Errorf("invalid ScheduledTime: %s", t)
	}
	hour, err := strconv.Atoi(parts[0])
	if err != nil {
		return false, fmt.Errorf("invalid scheduled hour: %v", err)
	}
	minute, err := strconv.Atoi(parts[1])
	if err != nil {
		return false, fmt.Errorf("invalid scheduled minute: %v", err)
	}
	if hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return false, fmt.Errorf("invalid ScheduledTime: %s", t)
	}

	day := nowUTC.Format("2006-01-02")
	if s.lastScheduledRunDay == day {
		return false, nil
	}
	scheduledAt := time.Date(nowUTC.Year(), nowUTC.Month(), nowUTC.Day(), hour, minute, 0, 0, time.UTC)
	if nowUTC.Before(scheduledAt) {
		return false, nil
	}
	s.lastScheduledRunDay = day
	return true, nil
}
