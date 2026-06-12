import type { MenuDataItem } from '@ant-design/pro-layout';
import { Icon as IconifyIcon } from '@iconify/react';
import React from 'react';
import type { AdminAdminMenu } from '@/api/generated/schemas';
import { hasPermissionCode, normalizePermissionCodes } from '@/utils/permission';

function stripQueryAndHashFromPath(path: string): string {
  return path.split('#')?.[0]?.split('?')?.[0] ?? path;
}

function isUrl(path: string): boolean {
  return /^(https?:)?\/\//i.test(path) || /^mailto:/i.test(path) || /^tel:/i.test(path);
}

function isDataUri(value: string): boolean {
  return /^data:/i.test(value);
}

function isImagePath(value: string): boolean {
  // Keep this intentionally small; ProLayout can also handle string icons.
  return /\.(png|jpe?g|gif|svg|webp|ico)(\?.*)?$/i.test(value);
}

function inferLocaleIdFromPath(path: string | undefined): string | null {
  if (!path) return null;

  const cleaned = stripQueryAndHashFromPath(path).trim();
  if (!cleaned) return null;
  if (isUrl(cleaned)) return null;

  const withoutSlashes = cleaned.replace(/^\/+/, '').replace(/\/+$/, '');
  if (!withoutSlashes) return null;

  const segments = withoutSlashes.split('/').filter(Boolean);
  const stableSegments: string[] = [];

  for (const segment of segments) {
    if (segment === '*' || segment.startsWith(':')) break;
    stableSegments.push(segment);
  }

  if (!stableSegments.length) return null;
  return `menu.${stableSegments.join('.')}`;
}

/**
 * 检查是否是 Ant Design 图标名称
 * 例如: AccountBookOutlined, FireFilled, HomeTwoTone, dashboard, send
 */
function isAntDesignIconName(value: string): boolean {
  // 完整的 PascalCase 名称 (如 FireFilled, HomeOutlined)
  if (/^[A-Z][a-zA-Z0-9]*(Outlined|Filled|TwoTone)$/.test(value)) return true;
  // 简写名称 (如 dashboard, send, wallet)
  if (/^[a-z][a-z0-9-]*$/.test(value)) return true;
  return false;
}

/**
 * 将 Ant Design 图标名称转换为 Iconify 格式
 * 例如: 
 *   FireFilled -> ant-design:fire-filled
 *   dashboard -> ant-design:dashboard-outlined
 *   send -> ant-design:send-outlined
 */
function toIconifyAntDesign(name: string): string {
  // 如果已经是完整的 PascalCase 名称
  if (/^[A-Z]/.test(name)) {
    const kebabName = name
      .replace(/([a-z0-9])([A-Z])/g, '$1-$2')
      .toLowerCase();
    return `ant-design:${kebabName}`;
  }
  // 简写名称，默认添加 -outlined 后缀
  return `ant-design:${name}-outlined`;
}

/**
 * Best-practice persisted formats for `admin_menu.icon`:
 * - Ant Design icons: `SafetyOutlined` / `WalletOutlined` / `dashboard` (legacy compatible)
 * - Iconify icons: `iconify:<collection>:<name>` e.g. `iconify:lucide:user`
 *
 * NOTE: For string icons like:
 * - iconfont (`icon-xxx`)
 * - image urls (`https://.../x.png`) or static image paths (`/x.png`)
 *
 * We return the string so ProLayout can render it via its internal `getIcon()`.
 */
function renderIcon(icon?: string): React.ReactNode | string | undefined {
  if (!icon) return undefined;

  const raw = icon.trim();
  if (!raw) return undefined;

  // Allow ProLayout to handle iconfont or image icons when stored as strings.
  if (raw.startsWith('icon-')) return raw;
  if (isUrl(raw) || isDataUri(raw) || (raw.startsWith('/') && isImagePath(raw)) || isImagePath(raw)) return raw;

  // iconify:<collection>:<name>
  if (raw.toLowerCase().startsWith('iconify:')) {
    const rest = raw.slice('iconify:'.length).trim();
    const [collection, name, ...extra] = rest.split(':').filter(Boolean);
    if (collection && name && extra.length === 0) {
      return React.createElement(IconifyIcon, { icon: `${collection}:${name}` });
    }
    return undefined;
  }

  // Ant Design 图标名称 - 使用 Iconify 渲染
  if (isAntDesignIconName(raw)) {
    const iconifyName = toIconifyAntDesign(raw);
    return React.createElement(IconifyIcon, { icon: iconifyName });
  }

  return undefined;
}

type BuildMenuOptions = {
  permissionCodes?: string[];
  isSuperAdmin?: boolean;
  rpcMethodPermissions?: Record<string, string>;
};

function isDisallowedStaticPath(path: string): boolean {
  const cleaned = stripQueryAndHashFromPath(path).trim();
  if (cleaned === '/welcome') return true;
  if (cleaned === '/admin' || cleaned.startsWith('/admin/')) return true;
  if (cleaned === '/blacklist' || cleaned.startsWith('/blacklist/')) return true;
  if (cleaned === '/list' || cleaned.startsWith('/list/')) return true;
  return false;
}

function rewriteLegacyMenuPath(path: string | undefined): string | undefined {
  if (!path) return path;
  const cleaned = stripQueryAndHashFromPath(path).trim();
  // Legacy approval page path was renamed to `/withdrawals`
  if (cleaned === '/approval') return '/withdrawals';
  return path;
}

function rewriteLegacyMenuKey(key: string | undefined): string | undefined {
  if (!key) return key;
  const raw = key.trim();
  if (!raw) return key;
  // Legacy approval menu key was renamed to withdrawals
  if (raw === 'approval') return 'withdrawals';
  if (raw === 'menu.approval') return 'menu.withdrawals';
  return key;
}

function canAccessMenu(menu: AdminAdminMenu, opts: BuildMenuOptions): boolean {
  const path = menu.path?.trim();
  if (path && isDisallowedStaticPath(path)) return false;

  const access = menu.access?.trim();
  if (!access) return true;
  if (opts.isSuperAdmin) return true;

  // Backward compatibility for legacy `rpc:MethodName` menu access values:
  // once the permission map is loaded, translate it to canonical `rbac.*`.
  // If the map is not loaded yet, we will fall back to checking the raw `rpc:MethodName`
  // against the granted permissions (legacy permission sets still include `rpc:*` / `rpc:MethodName`).
  let required = access;
  if (required.startsWith('rpc:')) {
    const methodName = required.slice('rpc:'.length);
    const mapped = opts.rpcMethodPermissions?.[methodName];
    if (mapped) required = mapped;
  }

  const permissionCodes = normalizePermissionCodes(opts.permissionCodes ?? []);
  return hasPermissionCode(permissionCodes, required);
}

function toMenuDataItem(menu: AdminAdminMenu, opts: BuildMenuOptions): MenuDataItem | null {
  if (!canAccessMenu(menu, opts)) return null;

  const originalPath = menu.path;
  const rewrittenPath = rewriteLegacyMenuPath(originalPath);

  const originalKey = menu.key;
  const rewrittenKey = rewriteLegacyMenuKey(originalKey);
  const rawKey = rewrittenKey?.trim();
  const localeFromKey = rawKey
    ? rawKey.startsWith('menu.')
      ? rawKey
      : `menu.${rawKey}`
    : null;
  const localeId = localeFromKey ?? inferLocaleIdFromPath(rewrittenPath) ?? false;
  const rawName = menu.name?.trim();
  const name = rawName || (typeof localeId === 'string' ? localeId.replace(/^menu\./, '') : undefined);

  const children = (menu.children || [])
    .map((child) => toMenuDataItem(child, opts))
    .filter((x): x is MenuDataItem => !!x);

  return {
    name,
    locale: localeId,
    path: rewrittenPath,
    // ProLayout supports both ReactNode and some string formats (iconfont/url/img);
    // keep type compatibility with @ant-design/pro-layout's `MenuDataItem`.
    icon: renderIcon(menu.icon) as any,
    target: menu.target,
    access: menu.access,
    key: rewrittenKey || menu.id || rewrittenPath,
    hideInMenu: menu.hide_in_menu,
    hideChildrenInMenu: menu.hide_children_in_menu,
    children: children.length ? children : undefined,
  };
}

export function buildMenuDataFromApi(
  menuTree: AdminAdminMenu[] | undefined,
  opts: BuildMenuOptions = {},
): MenuDataItem[] {
  if (!menuTree?.length) return [];
  return menuTree
    .map((m) => toMenuDataItem(m, opts))
    .filter((x): x is MenuDataItem => !!x);
}
