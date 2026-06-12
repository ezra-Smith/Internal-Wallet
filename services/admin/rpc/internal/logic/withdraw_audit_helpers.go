package logic

import (
	"fmt"
	"sort"
	"strings"

	"github.com/shopspring/decimal"
)

func isValidWithdrawAuditStrategy(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "auto", "manual_auto", "manual_manual":
		return true
	default:
		return false
	}
}

func isValidTransferAuditStrategy(s string) bool {
	// 内部转账只支持 auto 和 manual_auto 两种策略
	// 不支持 manual_manual（人工审核后手动转账）
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "auto", "manual_auto":
		return true
	default:
		return false
	}
}

type auditInterval struct {
	Min decimal.Decimal
	Max *decimal.Decimal // nil = +inf
}

func validateAuditIntervalsNonOverlapping(intervals []auditInterval) error {
	if len(intervals) <= 1 {
		return nil
	}
	sort.Slice(intervals, func(i, j int) bool {
		if intervals[i].Min.Equal(intervals[j].Min) {
			// nil max means +inf -> sort last
			if intervals[i].Max == nil && intervals[j].Max == nil {
				return false
			}
			if intervals[i].Max == nil {
				return false
			}
			if intervals[j].Max == nil {
				return true
			}
			return intervals[i].Max.LessThan(*intervals[j].Max)
		}
		return intervals[i].Min.LessThan(intervals[j].Min)
	})

	var prevMax *decimal.Decimal
	for i, it := range intervals {
		if i == 0 {
			prevMax = it.Max
			continue
		}
		if prevMax == nil {
			return fmt.Errorf("range overlap")
		}
		if it.Min.LessThan(*prevMax) {
			return fmt.Errorf("range overlap")
		}
		prevMax = it.Max
	}
	return nil
}
