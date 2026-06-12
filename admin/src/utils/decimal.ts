/**
 * 数字字符串格式化：
 * - 去掉小数部分末尾无效 0
 * - 但至少保留 minDecimals 位小数（不足则补 0）
 *
 * 仅处理“普通十进制”字符串（不处理科学计数法、千分位、特殊格式）。
 */
export function formatDecimalTrimZerosMinDecimals(raw: unknown, minDecimals = 2): string {
  if (raw === null || raw === undefined) return '-';
  const s = String(raw).trim();
  if (!s) return '-';

  // 科学计数法/非标准形式：保持原样
  if (/[eE]/.test(s)) return s;
  if (minDecimals < 0) minDecimals = 0;

  // 只接受：+/-、数字、小数点
  // 允许 ".5" / "-.5" / "1." 等
  const m = /^([+-]?)(\d*)(?:\.(\d*))?$/.exec(s);
  if (!m) return s;

  const sign = m[1] || '';
  let intPart = m[2] ?? '';
  let fracPart = m[3] ?? '';

  // 至少要有一位数字（int 或 frac）
  if (!intPart && !fracPart) return s;
  if (!intPart) intPart = '0';

  // 负零归一化
  const allZero = /^0+$/.test(intPart) && (fracPart === '' || /^0+$/.test(fracPart));
  const normalizedSign = allZero ? '' : sign;

  // 先去掉尾随 0，再补足最小位数
  fracPart = fracPart.replace(/0+$/, '');
  if (fracPart.length < minDecimals) {
    fracPart = fracPart + '0'.repeat(minDecimals - fracPart.length);
  }

  if (minDecimals === 0) {
    return fracPart ? `${normalizedSign}${intPart}.${fracPart}` : `${normalizedSign}${intPart}`;
  }
  return `${normalizedSign}${intPart}.${fracPart || '0'.repeat(minDecimals)}`;
}


