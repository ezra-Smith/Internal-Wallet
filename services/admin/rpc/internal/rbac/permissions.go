package rbac

import "strings"

// MatchPermission checks whether a granted permission matches a required permission.
// Supported forms:
// - "*" matches everything
// - "resource:*" matches "resource:anything" (legacy, colon)
// - "resource.*" matches "resource.anything" (dot)
// - exact match
func MatchPermission(granted, required string) bool {
	granted = strings.TrimSpace(granted)
	required = strings.TrimSpace(required)
	if granted == "" || required == "" {
		return false
	}
	if granted == "*" {
		return true
	}
	if granted == required {
		return true
	}
	if strings.HasSuffix(granted, ":*") || strings.HasSuffix(granted, ".*") {
		prefix := strings.TrimSuffix(granted, "*")
		return strings.HasPrefix(required, prefix)
	}
	return false
}

// HasPermission returns true if any granted permission matches the required one.
func HasPermission(granted []string, required string) bool {
	for _, g := range granted {
		if MatchPermission(g, required) {
			return true
		}
	}
	return false
}

// HasAnyPermission returns true if any required permission is granted.
func HasAnyPermission(granted []string, required ...string) bool {
	for _, r := range required {
		if HasPermission(granted, r) {
			return true
		}
	}
	return false
}
