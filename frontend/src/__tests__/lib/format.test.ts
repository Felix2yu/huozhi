import { describe, it, expect } from 'vitest';
import {
	formatMoney,
	formatShortMoney,
	formatDate,
	formatRelativeDate,
	getMonthRange,
	moneyColor
} from '$lib/utils/format';

describe('formatMoney', () => {
	it('格式化正数金额', () => {
		expect(formatMoney(1234.56)).toBe('¥1,234.56');
		expect(formatMoney(0)).toBe('¥0.00');
		expect(formatMoney(100)).toBe('¥100.00');
	});

	it('格式化负数金额', () => {
		expect(formatMoney(-1234.56)).toBe('-¥1,234.56');
		expect(formatMoney(-100)).toBe('-¥100.00');
	});

	it('支持自定义货币符号', () => {
		expect(formatMoney(100, '$')).toBe('$100.00');
		expect(formatMoney(-100, '€')).toBe('-€100.00');
	});

	it('支持显示正号', () => {
		expect(formatMoney(100, '¥', true)).toBe('+¥100.00');
		expect(formatMoney(-100, '¥', true)).toBe('-¥100.00');
		expect(formatMoney(0, '¥', true)).toBe('¥0.00');
	});

	it('处理特殊值', () => {
		expect(formatMoney(Infinity)).toBe('—');
		expect(formatMoney(-Infinity)).toBe('—');
		expect(formatMoney(NaN)).toBe('—');
	});
});

describe('formatShortMoney', () => {
	it('小金额直接格式化', () => {
		expect(formatShortMoney(100)).toBe('¥100.00');
		expect(formatShortMoney(9999)).toBe('¥9,999.00');
	});

	it('万级金额', () => {
		expect(formatShortMoney(10000)).toBe('¥1.00万');
		expect(formatShortMoney(12345678)).toBe('¥1234.57万');
	});

	it('亿级金额', () => {
		expect(formatShortMoney(100000000)).toBe('¥1.00亿');
		expect(formatShortMoney(123456789012)).toBe('¥1234.57亿');
	});

	it('负数金额', () => {
		expect(formatShortMoney(-10000)).toBe('¥-1.00万');
		expect(formatShortMoney(-100000000)).toBe('¥-1.00亿');
	});

	it('处理特殊值', () => {
		expect(formatShortMoney(Infinity)).toBe('—');
		expect(formatShortMoney(NaN)).toBe('—');
	});
});

describe('formatDate', () => {
	it('格式化日期字符串', () => {
		expect(formatDate('2026-09-16')).toBe('2026-09-16');
		expect(formatDate('2026-01-01')).toBe('2026-01-01');
	});

	it('支持自定义格式', () => {
		expect(formatDate('2026-09-16', 'MM/DD')).toBe('09/16');
		expect(formatDate('2026-09-16', 'YYYY年MM月DD日')).toBe('2026年09月16日');
	});
});

// 取本地日历日，不要用 toISOString()：后者是 UTC 日期，
// 在 GMT+8 的 00:00–08:00 时段比本地日期慢一天，会让相对日期整体偏移 1 天。
function localDateStr(d: Date): string {
	return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(
		d.getDate()
	).padStart(2, '0')}`;
}

describe('formatRelativeDate', () => {
	it('今天', () => {
		const today = localDateStr(new Date());
		expect(formatRelativeDate(today)).toBe('今天');
	});

	it('昨天', () => {
		const yesterday = new Date();
		yesterday.setDate(yesterday.getDate() - 1);
		expect(formatRelativeDate(localDateStr(yesterday))).toBe('昨天');
	});

	it('最近7天', () => {
		const threeDaysAgo = new Date();
		threeDaysAgo.setDate(threeDaysAgo.getDate() - 3);
		expect(formatRelativeDate(localDateStr(threeDaysAgo))).toBe('3天前');
	});

	it('今年的日期', () => {
		const thisYear = new Date();
		thisYear.setMonth(thisYear.getMonth() - 1);
		const dateStr = localDateStr(thisYear);
		const result = formatRelativeDate(dateStr);
		expect(result).toMatch(/^\d{2}-\d{2}$/);
	});
});

describe('getMonthRange', () => {
	it('返回当前月份范围', () => {
		const range = getMonthRange();
		expect(range.start).toMatch(/^\d{4}-\d{2}-01$/);
		expect(range.end).toMatch(/^\d{4}-\d{2}-\d{2}$/);
	});

	it('返回指定月份范围', () => {
		const range = getMonthRange('2026-03');
		expect(range.start).toBe('2026-03-01');
		expect(range.end).toBe('2026-03-31');
	});
});

describe('moneyColor', () => {
	it('正数返回收入颜色', () => {
		expect(moneyColor(100)).toBe('text-[var(--color-income)]');
	});

	it('负数返回支出颜色', () => {
		expect(moneyColor(-100)).toBe('text-[var(--color-expense)]');
	});

	it('零返回空字符串', () => {
		expect(moneyColor(0)).toBe('');
	});
});
