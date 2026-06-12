export function normalizePermissionCodes(raw: unknown): string[] {
  if (!Array.isArray(raw)) return [];
  const cleaned = raw
    .filter((x): x is string => typeof x === 'string')
    .map((x) => x.trim())
    .filter(Boolean)
    .map((x) => (x === 'rpc:*' ? '*' : x)); // legacy alias
  return Array.from(new Set(cleaned));
}

export function matchPermission(grantedRaw: string, requiredRaw: string): boolean {
  const granted = grantedRaw.trim();
  const required = requiredRaw.trim();
  if (!granted || !required) return false;
  if (granted === '*') return true;
  if (granted === required) return true;
  if (granted.endsWith(':*') || granted.endsWith('.*')) {
    const prefix = granted.slice(0, -1); // keep delimiter (':' or '.')
    return required.startsWith(prefix);
  }
  return false;
}

export function hasPermissionCode(permissionCodes: string[], required: string): boolean {
  for (const granted of permissionCodes) {
    if (matchPermission(granted, required)) return true;
  }
  return false;
}

