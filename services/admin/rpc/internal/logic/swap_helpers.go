package logic

import (
	"strconv"
	"strings"
)

const (
	swapConfigCategoryGlobal    = "swap_global_settings"
	swapConfigCategorySlippage  = "swap_slippage_settings"
	swapConfigCategoryPrice     = "swap_price_settings"
	swapConfigCategoryRisk      = "swap_risk_settings"
	swapConfigCategoryProviders = "swap_providers"
	swapConfigCategoryPairs     = "swap_pairs"
	swapConfigCategoryTokens    = "swap_tokens"
)

func normalizeSwapKeyPart(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "_", "-")
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return s
}

func swapTokenKey(symbol, network string) string {
	s := normalizeSwapKeyPart(symbol)
	n := normalizeSwapKeyPart(network)
	if s == "" {
		return n
	}
	if n == "" {
		return s
	}
	return s + "-" + n
}

func parseOptionalBool(s string) (bool, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return false, false
	}
	v, err := strconv.ParseBool(s)
	if err != nil {
		return false, false
	}
	return v, true
}
