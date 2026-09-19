import dayjs from 'dayjs';
import { CURRENCIES } from '$lib/types';

/** 币种符号：CNY→¥、USD→$ … 未收录的币种直接回显代码，不要猜成 ¥ */
export function currencySymbol(code?: string): string {
	const c = (code || '').toUpperCase();
	if (!c) return '¥';
	return CURRENCIES.find((x) => x.code === c)?.symbol ?? `${c} `;
}

/** 币种中文名 */
export function currencyLabel(code?: string): string {
	const c = (code || '').toUpperCase();
	if (!c) return '';
	return CURRENCIES.find((x) => x.code === c)?.label ?? c;
}

/** 金额格式化: 1234.56 → "¥1,234.56" */
export function formatMoney(
	amount: number,
	currency = '¥',
	showSign = false
): string {
	if (!isFinite(amount)) return '—';
	const sign = showSign && amount > 0 ? '+' : '';
	const prefix = amount < 0 ? '-' : '';
	const abs = Math.abs(amount);
	const formatted = abs.toLocaleString('zh-CN', {
		minimumFractionDigits: 2,
		maximumFractionDigits: 2
	});
	return `${sign}${prefix}${currency}${formatted}`;
}

/** 简洁金额: 大数用万/亿 */
export function formatShortMoney(amount: number, currency = '¥'): string {
	if (!isFinite(amount)) return '—';
	const abs = Math.abs(amount);
	if (abs >= 100000000) {
		return `${currency}${(amount / 100000000).toFixed(2)}亿`;
	}
	if (abs >= 10000) {
		return `${currency}${(amount / 10000).toFixed(2)}万`;
	}
	return formatMoney(amount, currency);
}

/** 日期格式化 */
export function formatDate(date: string | Date, fmt = 'YYYY-MM-DD'): string {
	return dayjs(date).format(fmt);
}

/** 友好的日期标签: 今天/昨天/明天/N天前/N天后/MM-DD */
export function formatRelativeDate(date: string | Date): string {
	const d = dayjs(date);
	if (!d.isValid()) return '—';
	const now = dayjs();
	const diff = now.startOf('day').diff(d.startOf('day'), 'day');
	if (diff === 0) return '今天';
	if (diff === 1) return '昨天';
	if (diff === -1) return '明天';
	// 未来日期：周期记账的「下次执行」全部落在未来，
	// 旧实现只处理过去，负差被原样拼成 "-6天前" 这种自相矛盾的文案。
	if (diff < 0 && diff >= -7) return `${-diff}天后`;
	if (diff > 0 && diff <= 7) return `${diff}天前`;
	if (d.year() === now.year()) return d.format('MM-DD');
	return d.format('YYYY-MM-DD');
}

/** 月份范围 */
export function getMonthRange(month?: string): { start: string; end: string } {
	const d = month ? dayjs(month) : dayjs();
	return {
		start: d.startOf('month').format('YYYY-MM-DD'),
		end: d.endOf('month').format('YYYY-MM-DD')
	};
}

/** 金额颜色 class */
export function moneyColor(amount: number): string {
	if (amount > 0) return 'text-[var(--color-income)]';
	if (amount < 0) return 'text-[var(--color-expense)]';
	return '';
}
