package withdrawcalc

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/shopspring/decimal"
)

// AccountingPrecisionScale is the decimal scale used when persisting/freeze/unfreeze
// amounts with Accounting RPC. Today it is fixed at 6 across withdraw flows.
const AccountingPrecisionScale int32 = 6

// TruncateAccounting truncates a decimal to AccountingPrecisionScale digits.
func TruncateAccounting(d decimal.Decimal) decimal.Decimal {
	return d.Truncate(AccountingPrecisionScale)
}

type FeeRuleSnapshotItem struct {
	ID        int64   `json:"id"`
	RuleType  string  `json:"rule_type"` // fixed|percent
	Value     string  `json:"value"`
	MinFee    *string `json:"min_fee,omitempty"`
	MaxFee    *string `json:"max_fee,omitempty"`
	MinAmount *string `json:"min_amount,omitempty"`
	MaxAmount *string `json:"max_amount,omitempty"`
	SortOrder int32   `json:"sort_order,omitempty"`
}

type feeCalcPart struct {
	RuleID     int64   `json:"rule_id"`
	RuleType   string  `json:"rule_type"`
	Value      string  `json:"value"`
	Computed   string  `json:"computed"`
	MinApplied *string `json:"min_fee_applied,omitempty"`
	MaxApplied *string `json:"max_fee_applied,omitempty"`
	Result     string  `json:"result"`
}

type feeSnapshot struct {
	Source      string `json:"source"`
	AssetCode   string `json:"asset_code"`
	ChainCode   string `json:"chain_code"`
	Amount      string `json:"amount"`
	Fee         string `json:"fee"`
	RuleSummary string `json:"rule_summary,omitempty"`

	Rules []FeeRuleSnapshotItem `json:"rules"`

	Breakdown struct {
		FixedTotal   string        `json:"fixed_total"`
		PercentTotal string        `json:"percent_total"`
		Total        string        `json:"total"`
		Parts        []feeCalcPart `json:"parts"`
	} `json:"breakdown"`
}

func normalizeCode(s string) string {
	return strings.ToUpper(strings.TrimSpace(s))
}

func truncate255(s string) string {
	s = strings.TrimSpace(s)
	if len(s) <= 255 {
		return s
	}
	if len(s) <= 3 {
		return s
	}
	return s[:252] + "..."
}

func buildFeeRuleSummary(assetCode string, rules []FeeRuleSnapshotItem) string {
	assetCode = normalizeCode(assetCode)
	if len(rules) == 0 {
		if assetCode == "" {
			return "0"
		}
		return "0 " + assetCode
	}

	parts := make([]string, 0, len(rules))
	for _, r := range rules {
		rt := strings.ToLower(strings.TrimSpace(r.RuleType))
		val := strings.TrimSpace(r.Value)
		if val == "" {
			continue
		}
		switch rt {
		case "fixed":
			if assetCode != "" {
				parts = append(parts, fmt.Sprintf("%s %s", val, assetCode))
			} else {
				parts = append(parts, val)
			}
		case "percent":
			p := fmt.Sprintf("%s%%", val)
			minTxt := ""
			maxTxt := ""
			if r.MinFee != nil && strings.TrimSpace(*r.MinFee) != "" {
				minTxt = "min " + strings.TrimSpace(*r.MinFee)
			}
			if r.MaxFee != nil && strings.TrimSpace(*r.MaxFee) != "" {
				maxTxt = "max " + strings.TrimSpace(*r.MaxFee)
			}
			if minTxt != "" || maxTxt != "" {
				bound := strings.TrimSpace(strings.Join([]string{minTxt, maxTxt}, " "))
				p = p + " (" + bound + ")"
			}
			parts = append(parts, p)
		}
	}

	if len(parts) == 0 {
		if assetCode == "" {
			return "0"
		}
		return "0 " + assetCode
	}
	return truncate255(strings.Join(parts, " + "))
}

// BuildWithdrawFeeSnapshot calculates withdraw fee based on rules and returns:
// - fee total (decimal)
// - human readable summary (string, <=255 chars)
// - a JSON snapshot blob for audit transparency
func BuildWithdrawFeeSnapshot(amount decimal.Decimal, amountStr, assetCode, chainCode, source string, rules []FeeRuleSnapshotItem) (decimal.Decimal, string, []byte) {
	assetCode = normalizeCode(assetCode)
	chainCode = normalizeCode(chainCode)

	total := decimal.Zero
	fixedTotal := decimal.Zero
	percentTotal := decimal.Zero
	parts := make([]feeCalcPart, 0, len(rules))

	for _, r := range rules {
		// Amount range filtering.
		if r.MinAmount != nil && strings.TrimSpace(*r.MinAmount) != "" {
			minAmt, err := decimal.NewFromString(strings.TrimSpace(*r.MinAmount))
			if err == nil && amount.LessThan(minAmt) {
				continue
			}
		}
		if r.MaxAmount != nil && strings.TrimSpace(*r.MaxAmount) != "" {
			maxAmt, err := decimal.NewFromString(strings.TrimSpace(*r.MaxAmount))
			if err == nil && !amount.LessThan(maxAmt) {
				continue
			}
		}

		rt := strings.ToLower(strings.TrimSpace(r.RuleType))
		val, err := decimal.NewFromString(strings.TrimSpace(r.Value))
		if err != nil || val.IsNegative() {
			continue
		}

		switch rt {
		case "fixed":
			fixedTotal = fixedTotal.Add(val)
			total = total.Add(val)
			parts = append(parts, feeCalcPart{
				RuleID:   r.ID,
				RuleType: "fixed",
				Value:    strings.TrimSpace(r.Value),
				Computed: val.String(),
				Result:   val.String(),
			})
		case "percent":
			raw := amount.Mul(val).Div(decimal.NewFromInt(100))
			result := raw

			var minApplied *string
			if r.MinFee != nil && strings.TrimSpace(*r.MinFee) != "" {
				if minD, err := decimal.NewFromString(strings.TrimSpace(*r.MinFee)); err == nil && minD.GreaterThan(result) {
					v := minD.String()
					minApplied = &v
					result = minD
				}
			}
			var maxApplied *string
			if r.MaxFee != nil && strings.TrimSpace(*r.MaxFee) != "" {
				if maxD, err := decimal.NewFromString(strings.TrimSpace(*r.MaxFee)); err == nil && result.GreaterThan(maxD) {
					v := maxD.String()
					maxApplied = &v
					result = maxD
				}
			}
			if result.IsNegative() {
				result = decimal.Zero
			}

			percentTotal = percentTotal.Add(result)
			total = total.Add(result)
			parts = append(parts, feeCalcPart{
				RuleID:     r.ID,
				RuleType:   "percent",
				Value:      strings.TrimSpace(r.Value),
				Computed:   raw.String(),
				MinApplied: minApplied,
				MaxApplied: maxApplied,
				Result:     result.String(),
			})
		}
	}

	if total.IsNegative() {
		total = decimal.Zero
	}

	summary := buildFeeRuleSummary(assetCode, rules)
	snap := feeSnapshot{
		Source:      strings.TrimSpace(source),
		AssetCode:   assetCode,
		ChainCode:   chainCode,
		Amount:      strings.TrimSpace(amountStr),
		Fee:         total.String(),
		RuleSummary: summary,
		Rules:       rules,
	}
	snap.Breakdown.FixedTotal = fixedTotal.String()
	snap.Breakdown.PercentTotal = percentTotal.String()
	snap.Breakdown.Total = total.String()
	snap.Breakdown.Parts = parts

	b, _ := json.Marshal(snap)
	return total, summary, b
}

type GrossAmountValidationResult struct {
	Code    string
	Message string
	Meta    map[string]string
}

// ValidateGrossAmountForInnerDeduct validates withdraw amounts for inner-deduct mode:
// - net = amount - fee
// - enforce amount > fee
// - enforce amount >= min_withdraw_amount + fee (equivalent to net >= min_withdraw_amount)
//
// All inputs should be in the same precision domain. It is strongly recommended to
// pass values truncated by TruncateAccounting.
func ValidateGrossAmountForInnerDeduct(amount, fee, minWithdraw decimal.Decimal) *GrossAmountValidationResult {
	if fee.IsNegative() {
		return &GrossAmountValidationResult{
			Code:    "INVALID_FEE",
			Message: "invalid fee",
		}
	}

	if !amount.GreaterThan(decimal.Zero) {
		return &GrossAmountValidationResult{
			Code:    "AMOUNT_MUST_BE_POSITIVE",
			Message: "amount must be positive",
		}
	}

	if minWithdraw.LessThanOrEqual(decimal.Zero) {
		return &GrossAmountValidationResult{
			Code:    "MIN_WITHDRAW_AMOUNT_NOT_CONFIGURED",
			Message: "min_withdraw_amount not configured",
		}
	}

	if amount.LessThanOrEqual(fee) {
		return &GrossAmountValidationResult{
			Code:    "AMOUNT_MUST_EXCEED_FEE",
			Message: "amount must exceed fee",
			Meta: map[string]string{
				"amount": amount.String(),
				"fee":    fee.String(),
			},
		}
	}

	required := minWithdraw.Add(fee)
	if amount.LessThan(required) {
		return &GrossAmountValidationResult{
			Code:    "AMOUNT_BELOW_MIN_PLUS_FEE",
			Message: "amount below minimum withdraw amount plus fee",
			Meta: map[string]string{
				"amount":          amount.String(),
				"fee":             fee.String(),
				"min_withdraw":    minWithdraw.String(),
				"required_amount": required.String(),
				"net_amount":      amount.Sub(fee).String(),
			},
		}
	}

	return nil
}

