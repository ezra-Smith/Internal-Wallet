import React from 'react';
import { buildMenuDataFromApi } from '@/utils/menu';

jest.mock(
  '@iconify/react',
  () => ({
    Icon: () => null,
  }),
  { virtual: true },
);

describe('buildMenuDataFromApi', () => {
  it('filters menus by access when permissionCodes provided', () => {
    const menu = buildMenuDataFromApi(
      [
        {
          id: '1',
          name: 'A',
          path: '/a',
          access: 'perm.a',
        },
        {
          id: '2',
          name: 'B',
          path: '/b',
          access: 'perm.b',
        },
      ] as any,
      { permissionCodes: ['perm.a'], isSuperAdmin: false },
    );

    expect(menu.map((m) => m.path)).toEqual(['/a']);
  });

  it('translates legacy rpc:Method menu access via permission map', () => {
    const menu = buildMenuDataFromApi(
      [{ id: '1', name: 'RoleCreate', path: '/rbac/roles', access: 'rpc:CreateRole' }] as any,
      {
        permissionCodes: ['rbac.role.create'],
        isSuperAdmin: false,
        rpcMethodPermissions: { CreateRole: 'rbac.role.create' },
      },
    );
    expect(menu).toHaveLength(1);
  });

  it('does not allow legacy rpc:Method menu access without permission', () => {
    const menu = buildMenuDataFromApi(
      [{ id: '1', name: 'Admins', path: '/rbac/admins', access: 'rpc:ListAdmins' }] as any,
      { permissionCodes: [], isSuperAdmin: false },
    );
    expect(menu).toHaveLength(0);
  });

  it('does not filter menus for super admin', () => {
    const menu = buildMenuDataFromApi(
      [
        {
          id: '1',
          name: 'A',
          path: '/a',
          access: 'perm.a',
        },
      ] as any,
      { permissionCodes: [], isSuperAdmin: true },
    );

    expect(menu).toHaveLength(1);
  });

  it('filters nested children by access', () => {
    const menu = buildMenuDataFromApi(
      [
        {
          id: '1',
          name: 'Root',
          path: '/root',
          children: [
            { id: '1-1', name: 'Child1', path: '/root/1', access: 'perm.1' },
            { id: '1-2', name: 'Child2', path: '/root/2', access: 'perm.2' },
          ],
        },
      ] as any,
      { permissionCodes: ['perm.2'], isSuperAdmin: false },
    );

    expect(menu[0]?.children?.map((c) => c.path)).toEqual(['/root/2']);
  });

  it('filters legacy built-in paths', () => {
    const menu = buildMenuDataFromApi(
      [
        { id: '1', name: 'Welcome', path: '/welcome' },
        { id: '1-1', name: 'AdminSub', path: '/admin/sub-page' },
        { id: '1-2', name: 'ListSub', path: '/list/table-list' },
        { id: '2', name: 'RBAC', path: '/rbac' },
      ] as any,
      { permissionCodes: [], isSuperAdmin: true },
    );

    expect(menu.map((m) => m.path)).toEqual(['/rbac']);
  });

  it('infers locale ids from paths for i18n', () => {
    const menu = buildMenuDataFromApi([{ id: '1', name: 'Deposits', path: '/deposits' }] as any);
    expect(menu[0]).toMatchObject({ locale: 'menu.deposits', name: 'Deposits' });
  });

  it('disables locale for external urls', () => {
    const menu = buildMenuDataFromApi([{ id: '1', name: 'Docs', path: 'https://example.com' }] as any);
    expect(menu[0]).toMatchObject({ locale: false, name: 'Docs' });
  });

  it('uses inferred name when api name is missing', () => {
    const menu = buildMenuDataFromApi([{ id: '1', path: '/users' }] as any);
    expect(menu[0]).toMatchObject({ locale: 'menu.users', name: 'users' });
  });

  it('renders iconify:* icons as a React element', () => {
    const menu = buildMenuDataFromApi([{ id: '1', name: 'A', path: '/a', icon: 'iconify:lucide:user' }] as any);
    expect(React.isValidElement(menu[0]?.icon)).toBe(true);
  });

  it('keeps iconfont string icons (icon-*) as string for ProLayout', () => {
    const menu = buildMenuDataFromApi([{ id: '1', name: 'A', path: '/a', icon: 'icon-wallet' }] as any);
    expect(menu[0]?.icon).toBe('icon-wallet');
  });

  it('keeps image url icons as string for ProLayout', () => {
    const menu = buildMenuDataFromApi([{ id: '1', name: 'A', path: '/a', icon: 'https://example.com/x.png' }] as any);
    expect(menu[0]?.icon).toBe('https://example.com/x.png');
  });
});
