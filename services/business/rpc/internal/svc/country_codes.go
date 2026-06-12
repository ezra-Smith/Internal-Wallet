package svc

import "strings"

var defaultSupportedCountryCodes = []string{"+1", "+44", "+86"}

// NormalizeCountryCode normalizes the input to E.164 country/region code format (e.g. "+1", "+86").
func NormalizeCountryCode(code string) (string, bool) {
	code = strings.TrimSpace(code)
	if code == "" {
		return "", false
	}
	// Backward compatible:
	// - historical DB/config may store "86" / "1" (without '+')
	// - client may send "+86"
	// Normalize all to "+<digits>".
	if !strings.HasPrefix(code, "+") {
		code = "+" + code
	}

	digits := strings.TrimPrefix(code, "+")
	if digits == "" || len(digits) > 4 {
		return "", false
	}
	for _, r := range digits {
		if r < '0' || r > '9' {
			return "", false
		}
	}
	return "+" + digits, true
}

func buildCountryCodeSet(codes []string) ([]string, map[string]struct{}) {
	if len(codes) == 0 {
		codes = defaultSupportedCountryCodes
	}

	normalized := make([]string, 0, len(codes))
	set := make(map[string]struct{}, len(codes))
	for _, code := range codes {
		n, ok := NormalizeCountryCode(code)
		if !ok {
			continue
		}
		if _, exists := set[n]; exists {
			continue
		}
		set[n] = struct{}{}
		normalized = append(normalized, n)
	}

	if len(normalized) == 0 {
		for _, code := range defaultSupportedCountryCodes {
			n, _ := NormalizeCountryCode(code)
			set[n] = struct{}{}
			normalized = append(normalized, n)
		}
	}

	return normalized, set
}
