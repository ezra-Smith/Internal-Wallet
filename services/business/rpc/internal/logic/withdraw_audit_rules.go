package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"internalwallet/services/business/rpc/internal/svc"

	"github.com/shopspring/decimal"
)

type withdrawAuditRuleItem struct {
	ID        int64
	Source    string // asset|global
	AssetCode string
	ChainCode string
	MinAmount string
	MaxAmount *string
	Strategy  string
}

func isValidWithdrawAuditStrategy(strategy string) bool {
	switch strings.ToLower(strings.TrimSpace(strategy)) {
	case "auto", "manual_auto", "manual_manual":
		return true
	default:
		return false
	}
}

func parseNonNegativeDecimal(s string) (decimal.Decimal, bool) {
	d, err := decimal.NewFromString(strings.TrimSpace(s))
	if err != nil || d.IsNegative() {
		return decimal.Zero, false
	}
	return d, true
}

func matchWithdrawAuditStrategy(amountStr string, rules []withdrawAuditRuleItem) (*withdrawAuditRuleItem, bool, error) {
	amt, ok := parseNonNegativeDecimal(amountStr)
	if !ok || !amt.GreaterThan(decimal.Zero) {
		return nil, false, fmt.Errorf("invalid amount")
	}

	var matched *withdrawAuditRuleItem
	for _, r := range rules {
		strategy := strings.ToLower(strings.TrimSpace(r.Strategy))
		if !isValidWithdrawAuditStrategy(strategy) {
			return nil, false, fmt.Errorf("invalid strategy")
		}
		minD, ok := parseNonNegativeDecimal(r.MinAmount)
		if !ok {
			return nil, false, fmt.Errorf("invalid min_amount")
		}
		if amt.LessThan(minD) {
			continue
		}
		if r.MaxAmount != nil && strings.TrimSpace(*r.MaxAmount) != "" {
			maxD, ok := parseNonNegativeDecimal(*r.MaxAmount)
			if !ok {
				return nil, false, fmt.Errorf("invalid max_amount")
			}
			if !amt.LessThan(maxD) {
				continue
			}
		}
		if matched != nil {
			return nil, false, fmt.Errorf("multiple rules matched")
		}
		copied := r
		copied.Strategy = strategy
		matched = &copied
	}
	if matched == nil {
		return nil, false, nil
	}
	return matched, true, nil
}

type withdrawAuditRuleSnapshot struct {
	MatchedBy      string  `json:"matched_by"` // rule|whitelist|legacy
	Source         string  `json:"source"`     // asset|global|user|legacy
	RuleID         int64   `json:"rule_id"`
	AssetCode      string  `json:"asset_code,omitempty"`
	ChainCode      string  `json:"chain_code,omitempty"`
	MinAmount      string  `json:"min_amount,omitempty"`
	MaxAmount      *string `json:"max_amount,omitempty"`
	Strategy       string  `json:"strategy"`
	UseGlobal      bool    `json:"use_global"`
	FallbackReason string  `json:"fallback_reason,omitempty"`
	Amount         string  `json:"amount"`
	ConsideredIDs  []int64 `json:"considered_rule_ids,omitempty"`
	ConsideredN    int     `json:"considered_count"`
}

func pickWithdrawAuditStrategy(ctx context.Context, svcCtx *svc.ServiceContext, assetCode, chainCode, amountStr string, useGlobal bool) (string, bool, []byte, error) {
	assetCode = normalizeCode(assetCode)
	chainCode = normalizeCode(chainCode)

	rules := make([]withdrawAuditRuleItem, 0)
	fallbackReason := ""

	if !useGlobal && svcCtx.CurrencyWithdrawAuditRuleRepository != nil {
		rr, err := svcCtx.CurrencyWithdrawAuditRuleRepository.ListByAssetChain(ctx, assetCode, chainCode)
		if err != nil {
			return "", false, nil, err
		}
		if len(rr) > 0 {
			for _, r := range rr {
				rules = append(rules, withdrawAuditRuleItem{
					ID:        r.ID,
					Source:    "asset",
					AssetCode: assetCode,
					ChainCode: chainCode,
					MinAmount: strings.TrimSpace(r.MinAmount),
					MaxAmount: r.MaxAmount,
					Strategy:  strings.TrimSpace(r.Strategy),
				})
			}
		}
	}

	// Fallback to global when:
	// - useGlobal == true
	// - or per-chain rules are empty
	if len(rules) == 0 {
		if useGlobal {
			fallbackReason = "use_global_withdraw_audit=1"
		} else {
			fallbackReason = "asset rules empty, fallback to global"
		}
		if svcCtx.CurrencyGlobalWithdrawAuditRuleRepository == nil {
			return "", false, nil, fmt.Errorf("global audit rules repo not configured")
		}
		gr, err := svcCtx.CurrencyGlobalWithdrawAuditRuleRepository.ListByChain(ctx, chainCode)
		if err != nil {
			return "", false, nil, err
		}
		for _, g := range gr {
			rules = append(rules, withdrawAuditRuleItem{
				ID:        g.ID,
				Source:    "global",
				ChainCode: chainCode,
				MinAmount: strings.TrimSpace(g.MinAmount),
				MaxAmount: g.MaxAmount,
				Strategy:  strings.TrimSpace(g.Strategy),
			})
		}
	}

	matched, ok, err := matchWithdrawAuditStrategy(amountStr, rules)
	if err != nil {
		return "", false, nil, err
	}
	if !ok || matched == nil {
		return "", false, nil, nil
	}

	consideredIDs := make([]int64, 0, len(rules))
	for _, r := range rules {
		if r.ID > 0 {
			consideredIDs = append(consideredIDs, r.ID)
		}
	}

	snap := withdrawAuditRuleSnapshot{
		MatchedBy:      "rule",
		Source:         matched.Source,
		RuleID:         matched.ID,
		AssetCode:      strings.TrimSpace(matched.AssetCode),
		ChainCode:      strings.TrimSpace(matched.ChainCode),
		MinAmount:      strings.TrimSpace(matched.MinAmount),
		MaxAmount:      matched.MaxAmount,
		Strategy:       strings.TrimSpace(matched.Strategy),
		UseGlobal:      useGlobal,
		FallbackReason: fallbackReason,
		Amount:         strings.TrimSpace(amountStr),
		ConsideredIDs:  consideredIDs,
		ConsideredN:    len(rules),
	}
	b, _ := json.Marshal(snap)
	return matched.Strategy, true, b, nil
}
