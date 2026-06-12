import { useModel } from '@umijs/max';
import { useCallback, useMemo } from 'react';
import { adminGetMyRBAC } from '@/api/generated/rbac';
import { hasPermissionCode, normalizePermissionCodes } from '@/utils/permission';

function normalizeRole(role: string | undefined): string {
  return (role || '').trim().toLowerCase();
}

export function useRbac() {
  const { initialState, setInitialState } = useModel('@@initialState');

  const currentUser = initialState?.currentUser;
  const isLoggedIn = !!currentUser;

  const permissionCodes = useMemo(() => {
    return normalizePermissionCodes([
      ...(currentUser?.permissions ?? []),
      ...(currentUser?.rbac?.user_permission_codes ?? []),
    ] as unknown);
  }, [currentUser?.permissions, currentUser?.rbac?.user_permission_codes]);

  const role = useMemo(() => normalizeRole(currentUser?.role), [currentUser?.role]);
  const isSuperAdmin = useMemo(() => {
    return role === 'admin' || role === 'super_admin' || permissionCodes.includes('*');
  }, [permissionCodes, role]);

  const hasPermission = useCallback((code: string | undefined): boolean => {
    const normalized = code?.trim();
    if (!normalized) return true;
    if (!isLoggedIn) return false;
    if (isSuperAdmin) return true;
    return hasPermissionCode(permissionCodes, normalized);
  }, [isLoggedIn, isSuperAdmin, permissionCodes]);

  const rbacPermissionMap = initialState?.rbacPermissionMap;
  const rbacPermissionMapVersion = initialState?.rbacPermissionMapVersion;

  const permissionForRpc = useCallback((methodName: string): string => {
    const name = methodName.trim();
    if (!name) return '';
    return rbacPermissionMap?.[name] || `rpc:${name}`;
  }, [rbacPermissionMap]);

  const canRpc = useCallback((methodName: string): boolean => {
    const required = permissionForRpc(methodName);
    if (!required) return false;
    return hasPermission(required);
  }, [hasPermission, permissionForRpc]);

  const refreshMyRbac = useCallback(async (): Promise<void> => {
    if (!isLoggedIn) return;
    const res = await adminGetMyRBAC({ skipErrorHandler: true });
    const rbac = res?.data;
    if (!res?.success || !rbac) return;

    setInitialState((s) => {
      const base = s || ({} as any);
      if (!base.currentUser) return base;
      return {
        ...base,
        currentUser: {
          ...base.currentUser,
          rbac,
        },
      };
    });
  }, [isLoggedIn, setInitialState]);

  return {
    isLoggedIn,
    isSuperAdmin,
    permissionCodes,
    hasPermission,
    canRpc,
    rbacPermissionMap,
    rbacPermissionMapVersion,
    refreshMyRbac,
  };
}
