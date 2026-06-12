import { useModel } from '@umijs/max';
import React, { useEffect } from 'react';
import { adminGetRbacPermissionMap } from '@/api/generated/rbac';

const PermissionBootstrap: React.FC = () => {
  const { initialState, setInitialState } = useModel('@@initialState');
  const currentUser = initialState?.currentUser;

  useEffect(() => {
    if (!currentUser) return;
    if (initialState?.rbacPermissionMap) return;

    let cancelled = false;
    void (async () => {
      try {
        const res = await adminGetRbacPermissionMap({ skipErrorHandler: true });
        const payload = res?.data;
        const map = payload?.rpc_method_permissions;
        if (!res?.success || !map) return;
        if (cancelled) return;
        setInitialState((s) => {
          const base = s || ({} as any);
          return {
            ...base,
            rbacPermissionMap: map,
            rbacPermissionMapVersion: payload?.version,
          };
        });
      } catch {
        // ignore
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [currentUser, initialState?.rbacPermissionMap, setInitialState]);

  return null;
};

export default PermissionBootstrap;
