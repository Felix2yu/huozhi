import { describe, it, expect } from 'vitest';
import { cn, formatMoney, formatDate, getMonthRange, pct } from './index';

describe('cn', () => {
  it('合并类名并去重冲突的 tailwind 类', () => {
    expect(cn('a', 'b')).toBe('a b');
    // tailwind-merge 应让后出现的同类生效
    expect(cn('px-2', 'px-4')).toBe('px-4');
    expect(cn(false, null, undefined, 'x')).toBe('x');
  });
});

describe('formatMoney', () => {
  it('格式化数字为带千分位与两位小数的人民币', () => {
    expect(formatMoney(1234.5)).toBe('¥1,234.50');
    expect(formatMoney(0)).toBe('¥0.00');
  });
  it('接受字符串输入', () => {
    expect(formatMoney('9800')).toBe('¥9,800.00');
    expect(formatMoney('')).toBe('¥0.00');
  });
  it('根据币种输出符号', () => {
    expect(formatMoney(10, 'USD')).toBe('$10.00');
    // 非 CNY/USD 不附加符号
    expect(formatMoney(10, 'EUR')).toBe('10.00');
  });
});

describe('formatDate', () => {
  it('按 pattern 输出日期', () => {
    const d = new Date(2026, 0, 5, 9, 7, 3);
    expect(formatDate(d, 'YYYY-MM-DD')).toBe('2026-01-05');
    expect(formatDate(d, 'YYYY/MM/DD HH:mm:ss')).toBe('2026/01/05 09:07:03');
  });
  it('接受字符串与数字时间戳', () => {
    expect(formatDate('2026-03-09', 'YYYY-MM-DD')).toBe('2026-03-09');
  });
  it('非法日期返回空串', () => {
    expect(formatDate('not-a-date')).toBe('');
  });
});

describe('getMonthRange', () => {
  it('返回当月第一天与最后一天', () => {
    const { start, end } = getMonthRange(new Date(2026, 1, 15));
    expect(start.getDate()).toBe(1);
    expect(start.getMonth()).toBe(1);
    expect(end.getMonth()).toBe(1);
    // 2 月最后一天
    expect(end.getDate()).toBe(28);
  });
  it('正确处理大月', () => {
    const { end } = getMonthRange(new Date(2026, 0, 15));
    expect(end.getDate()).toBe(31);
  });
});

describe('pct', () => {
  it('计算完成百分比并保留指定位数', () => {
    expect(pct(1, 4)).toBe(25);
    expect(pct(1, 3, 2)).toBe(33.33);
  });
  it('total 为 0 时返回 0', () => {
    expect(pct(5, 0)).toBe(0);
  });
});
