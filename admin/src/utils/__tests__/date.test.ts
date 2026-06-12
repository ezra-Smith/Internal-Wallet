import { formatLocalDateTime } from '@/utils/date';

const formatWithLocalDate = (value: string) => {
  const d = new Date(value);
  const pad = (num: number) => String(num).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(
    d.getMinutes(),
  )}:${pad(d.getSeconds())}`;
};

describe('date utils', () => {
  it('formats ISO datetime to local time', () => {
    const value = '2026-01-15T09:49:42Z';
    expect(formatLocalDateTime(value)).toBe(formatWithLocalDate(value));
  });

  it('returns dash for empty values', () => {
    expect(formatLocalDateTime(undefined)).toBe('-');
    expect(formatLocalDateTime(null)).toBe('-');
    expect(formatLocalDateTime('')).toBe('-');
  });
});
