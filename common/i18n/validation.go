package i18n

import (
	"context"
	"regexp"
	"strings"
)

var validationKeyMap = map[string]string{
	"required":                  "validation.required",
	"invalid":                   "validation.invalid",
	"not found":                 "validation.not_found",
	"not_found":                 "validation.not_found",
	"exists":                    "validation.exists",
	"empty":                     "validation.empty",
	"inactive":                  "validation.inactive",
	"too weak":                  "validation.too_weak",
	"too_weak":                  "validation.too_weak",
	"too long":                  "validation.too_long",
	"too_long":                  "validation.too_long",
	"too_large":                 "validation.too_large",
	"too large":                 "validation.too_large",
	"invalid email":             "validation.invalid_email",
	"invalid rfc3339":           "validation.invalid_rfc3339",
	"invalid_format":            "validation.invalid_format",
	"invalid format":            "validation.invalid_format",
	"not allowed":               "validation.not_allowed",
	"not_allowed":               "validation.not_allowed",
	"not match":                 "validation.not_match",
	"not_match":                 "validation.not_match",
	"same":                      "validation.same",
	"circular":                  "validation.circular",
	"contains disabled":         "validation.contains_disabled",
	"contains username":         "validation.contains_username",
	"use disable endpoint":      "validation.use_disable_endpoint",
	"use assignrolestouser":     "validation.use_assign_roles_to_user",
	"chain_mismatch":            "validation.chain_mismatch",
	"low<critical":              "validation.low_lt_critical",
	"lt_min_amount":             "validation.lt_min_amount",
	"too_many_rows":             "validation.too_many_rows",
	"invalid rfc3339 timestamp": "validation.invalid_rfc3339",
}

var nonWord = regexp.MustCompile(`[^a-z0-9]+`)

// Validation translates a validation/field-violation code into a localized human message.
//
// Input examples:
// - "required"
// - "invalid email"
// - "too_many_rows"
// - "validation.required" (already a key)
func Validation(ctx context.Context, code string) string {
	code = strings.TrimSpace(code)
	if code == "" {
		return ""
	}
	if strings.HasPrefix(code, "validation.") {
		msg := T(ctx, code, nil)
		if msg == code {
			return code
		}
		return msg
	}

	norm := strings.ToLower(code)
	if key, ok := validationKeyMap[norm]; ok {
		msg := T(ctx, key, nil)
		if msg == key {
			return code
		}
		return msg
	}

	// Best-effort slug to allow gradual standardization without breaking.
	slug := strings.Trim(nonWord.ReplaceAllString(norm, "_"), "_")
	if slug == "" {
		return code
	}
	key := "validation." + slug
	msg := T(ctx, key, nil)
	if msg == key {
		return code
	}
	return msg
}
