import { hasPermissionCode, matchPermission, normalizePermissionCodes } from '@/utils/permission';

describe('permission utils', () => {
  it('normalizes and de-duplicates permission codes', () => {
    expect(normalizePermissionCodes([' a ', 'a', '', 'rpc:*'])).toEqual(['a', '*']);
  });

  it('matches wildcard permissions', () => {
    expect(matchPermission('*', 'rbac.role.create')).toBe(true);
    expect(matchPermission('rbac.role.*', 'rbac.role.create')).toBe(true);
    expect(matchPermission('rpc:*', 'rpc:ListRoles')).toBe(true);
    expect(matchPermission('rbac.role.*', 'rbac.permission.list')).toBe(false);
  });

  it('checks permissions against a list', () => {
    expect(hasPermissionCode(['rbac.role.*'], 'rbac.role.delete')).toBe(true);
    expect(hasPermissionCode(['rbac.role.list'], 'rbac.role.delete')).toBe(false);
  });
});

