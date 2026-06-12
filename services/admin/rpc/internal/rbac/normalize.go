package rbac

import (
	"sort"
	"strings"
)

func CanonicalizePermissionCode(code string) string {
	c := strings.TrimSpace(code)
	if c == "" {
		return ""
	}

	// Backward compatibility: `rpc:*` is historically used as "all permissions".
	if c == "rpc:*" {
		return "*"
	}

	// Translate known RBAC method permissions into canonical `rbac.*` codes.
	if strings.HasPrefix(c, "rpc:") {
		method := strings.TrimPrefix(c, "rpc:")
		if mapped, ok := rbacPermissionMap[method]; ok {
			return mapped
		}
	}

	return c
}

func CanonicalizePermissionCodes(codes []string) []string {
	seen := make(map[string]struct{}, len(codes))
	out := make([]string, 0, len(codes))
	for _, raw := range codes {
		c := CanonicalizePermissionCode(raw)
		if c == "" {
			continue
		}
		if _, ok := seen[c]; ok {
			continue
		}
		seen[c] = struct{}{}
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}
