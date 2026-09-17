import { describe, it, expect, vi, beforeEach } from 'vitest';
import { currencySymbol, currencyLabel } from '$lib/utils/format';
import type { FxSnapshot } from '$lib/types';

// fxApi 打真实网络，测试里替换成可控返回值
vi.mock('$lib/api/modules/exrate', () => {
	const snapshot: FxSnapshot = {
		base: 'CNY',
		rates: { CNY: 1, USD: 7.2, JPY: 0.05 },
		currencies: ['CNY', 'USD', 'JPY'],
		source: 'er-api',
		fetched_at: '2026-09-16T00:02:31Z',
		stale: false,
		enabled: true,
		auto_refresh: true,
		refresh_hours: 12
	};
	return {
		fxApi: {
			list: vi.fn(async () => snapshot),
			refresh: vi.fn(async () => snapshot)
		}
	};
});

describe('币种符号与名称', () => {
	it('已知币种返回对应符号', () => {
		expect(currencySymbol('USD')).toBe('$');
		expect(currencySymbol('cny')).toBe('¥');
		expect(currencySymbol('HKD')).toBe('HK$');
	});
	it('未知币种回显代码而不是猜成 ¥', () => {
		expect(currencySymbol('XYZ')).toBe('XYZ ');
	});
	it('空值兜底为 ¥', () => {
		expect(currencySymbol()).toBe('¥');
	});
	it('币种中文名', () => {
		expect(currencyLabel('JPY')).toBe('日元');
		expect(currencyLabel('')).toBe('');
	});
});

describe('ratesStore', () => {
	beforeEach(() => {
		// store 是模块级单例，每个用例前重置到未加载状态
		return import('$lib/stores/rates.svelte').then((m) => m.ratesStore.invalidate());
	});

	it('未加载时外币汇率未知（返回 null，而不是拿 1 冒充折算结果）', async () => {
		const { ratesStore } = await import('$lib/stores/rates.svelte');
		ratesStore.invalidate();
		expect(ratesStore.rate('CNY')).toBe(1); // 基准币自身恒为 1:1
		expect(ratesStore.rate('USD')).toBeNull();
		expect(ratesStore.convert(100, 'USD')).toBeNull();
	});

	it('加载后按「1 外币 = N 基准币」折算', async () => {
		const { ratesStore } = await import('$lib/stores/rates.svelte');
		ratesStore.invalidate();
		await ratesStore.load('CNY');
		expect(ratesStore.base).toBe('CNY');
		expect(ratesStore.rate('USD')).toBe(7.2);
		expect(ratesStore.rate('usd')).toBe(7.2); // 大小写不敏感
		expect(ratesStore.rate('CNY')).toBe(1);
		expect(ratesStore.convert(100, 'USD')).toBe(720);
	});

	it('没有该币种汇率时 convert 返回 null', async () => {
		const { ratesStore } = await import('$lib/stores/rates.svelte');
		ratesStore.invalidate();
		await ratesStore.load('CNY');
		expect(ratesStore.rate('GBP')).toBeNull();
		expect(ratesStore.convert(100, 'GBP')).toBeNull();
	});
});
