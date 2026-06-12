/**
 * @see https://umijs.org/docs/max/access#access
 * */
import { hasPermissionCode, normalizePermissionCodes } from '@/utils/permission';

export default function access(
  initialState: { currentUser?: API.CurrentUser } | undefined,
) {
  const { currentUser } = initialState ?? {};
  const role = (currentUser?.role || '').trim().toLowerCase();
  const permissionCodes = normalizePermissionCodes([
    ...(currentUser?.permissions ?? []),
    ...(currentUser?.rbac?.user_permission_codes ?? []),
  ]);

  const isSuperAdmin = role === 'admin' || role === 'super_admin' || permissionCodes.includes('*');

  return {
    // Legacy route guard key: keep it simple and base on login state.
    canAdmin: !!currentUser,
    isSuperAdmin,
    hasPermission: (code?: string) => {
      const normalized = code?.trim();
      if (!normalized) return true;
      if (!currentUser) return false;
      if (isSuperAdmin) return true;
      return hasPermissionCode(permissionCodes, normalized);
    },
  };
}
