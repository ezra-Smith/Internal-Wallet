import { formatDecimalTrimZerosMinDecimals } from '@/utils/decimal';

describe('decimal utils', () => {
  it('trims trailing zeros but keeps at least 2 decimals', () => {
    expect(formatDecimalTrimZerosMinDecimals('1', 2)).toBe('1.00');
    expect(formatDecimalTrimZerosMinDecimals('1.0', 2)).toBe('1.00');
    expect(formatDecimalTrimZerosMinDecimals('1.2000', 2)).toBe('1.20');
    expect(formatDecimalTrimZerosMinDecimals('1.2300', 2)).toBe('1.23');
    expect(formatDecimalTrimZerosMinDecimals('1.234500', 2)).toBe('1.2345');
    expect(formatDecimalTrimZerosMinDecimals('0', 2)).toBe('0.00');
    expect(formatDecimalTrimZerosMinDecimals('-0.000', 2)).toBe('0.00');
    expect(formatDecimalTrimZerosMinDecimals('.5', 2)).toBe('0.50');
    expect(formatDecimalTrimZerosMinDecimals('-.5', 2)).toBe('-0.50');
  });

  it('does not touch scientific notation or invalid strings', () => {
    expect(formatDecimalTrimZerosMinDecimals('1e-7', 2)).toBe('1e-7');
    expect(formatDecimalTrimZerosMinDecimals('abc', 2)).toBe('abc');
  });

  it('supports minDecimals=0', () => {
    expect(formatDecimalTrimZerosMinDecimals('1', 0)).toBe('1');
    expect(formatDecimalTrimZerosMinDecimals('1.2300', 0)).toBe('1.23');
    expect(formatDecimalTrimZerosMinDecimals('1.000', 0)).toBe('1');
  });
});





