package rbac

// PermissionMapVersion is used by frontend caches to detect changes.
// Update this value when you change permission semantics or method mappings.
const PermissionMapVersion = "2025-12-25-2"

// rbacPermissionMap maps Admin RPC method names to canonical permission codes.
//
// Canonical format: <domain>.<resource>.<action>
// Example: rbac.role.create
//
// Notes:
//   - This map is primarily for the admin console (scheme B) to avoid hard-coding
//     permission strings on every page.
//   - Keep method names in sync with proto/admin.proto.
var rbacPermissionMap = map[string]string{
	// Users (Web2)
	"ListUsers":                             "user.list",
	"GetUsersStatistics":                    "user.list",
	"ListUserWithdrawAuditWhitelistRules":   "user.withdraw_audit_whitelist.update",
	"UpsertUserWithdrawAuditWhitelistRules": "user.withdraw_audit_whitelist.update",
	"ListChains":                            "user.withdraw_audit_whitelist.update",
	// Chain logo is managed from the currencies config page; reuse UpdateCurrencyConfig permission.
	"UpdateChainIcon": "rpc:UpdateCurrencyConfig",
	"GetUserDetail":   "user.view",
	"FreezeUser":      "user.status.freeze",
	"UnfreezeUser":    "user.status.unfreeze",
	"TerminateUser":   "user.status.terminate",
	"UpdateUserRole":  "user.member_level.update",

	"UpdateUserMemberLevel":            "user.member_level.update",
	"ResetUserPassword":                "user.password.reset",
	"ResetUserTradePassword":           "user.trade_password.reset",
	"UpdateUserWithdrawAuditWhitelist": "user.withdraw_audit_whitelist.update",
	"UnbindUserTotp":                   "user.2fa.unbind",
	"ListUser2FAHistory":               "user.2fa.history.read",
	"AddUserNote":                      "user.note.create",
	"ExportUsers":                      "user.export",

	// Menus
	"ListMenus":              "rbac.menu.list",
	"CreateMenu":             "rbac.menu.create",
	"GetMenuById":            "rbac.menu.read",
	"UpdateMenu":             "rbac.menu.update",
	"DeleteMenu":             "rbac.menu.delete",
	"GetMenuTree":            "rbac.menu.tree",
	"GetPermissionsByMenuId": "rbac.menu.permissions.list",

	// Permissions
	"ListPermissions":   "rbac.permission.list",
	"CreatePermission":  "rbac.permission.create",
	"GetPermissionById": "rbac.permission.read",
	"UpdatePermission":  "rbac.permission.update",
	"DeletePermission":  "rbac.permission.delete",

	// Roles
	"ListRoles":               "rbac.role.list",
	"CreateRole":              "rbac.role.create",
	"GetRoleById":             "rbac.role.read",
	"UpdateRole":              "rbac.role.update",
	"DeleteRole":              "rbac.role.delete",
	"GetRoleMenus":            "rbac.role.menus.read",
	"AssignMenusToRole":       "rbac.role.menus.assign",
	"GetRolePermissions":      "rbac.role.permissions.read",
	"AssignPermissionsToRole": "rbac.role.permissions.assign",

	// User-role association
	"GetUserRoles":      "rbac.user.roles.read",
	"AssignRolesToUser": "rbac.user.roles.assign",

	// Blacklist (Sensitive Addresses)
	"ListBlacklistAddresses":              "blacklist.address.list",
	"CreateBlacklistAddress":              "blacklist.address.create",
	"BatchCreateBlacklistAddresses":       "blacklist.address.create",
	"UpdateBlacklistAddressMonitorStatus": "blacklist.address.monitor.update",
	"DeleteBlacklistAddress":              "blacklist.address.delete",
	"ExportBlacklistAddresses":            "blacklist.address.export",

	// UI helpers
	"GetRbacPermissionMap": "rbac.meta.permission_map.read",
}

func CopyRbacPermissionMap() map[string]string {
	out := make(map[string]string, len(rbacPermissionMap))
	for k, v := range rbacPermissionMap {
		out[k] = v
	}
	return out
}

func RequiredPermissionForMethod(methodName string) string {
	if code, ok := rbacPermissionMap[methodName]; ok {
		return code
	}
	// Backward-compatible default (legacy permission model).
	return "rpc:" + methodName
}
