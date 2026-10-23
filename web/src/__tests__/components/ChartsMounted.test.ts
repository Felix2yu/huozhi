/**
 * 图表组件的 mounted 守卫（CategoryPie / MonthlyBars / TrendLine）
 *
 * 修复点：`{#if mounted && …}` → `{#if mounted() && …}`。
 * `useMounted()` 返回的是函数，函数引用恒为真，旧守卫等于没有：
 * SSR / 预渲染阶段也会去挂 Chart.js，而 chart.js 依赖 canvas，此时不可用。
 *
 * jsdom 没有 canvas 2D 上下文，这里把 svelte-chartjs 换成 ChartStub.svelte，
 * 这样「渲染 / 不渲染」就是一条干净的可断言分支：
 *  - 有数据且已挂载 → 出现 [data-testid="chart-stub"]，且带正确的 labels；
 *  - 无数据 → 不出现图表，落到「暂无数据」占位；
 *  - useMounted() 本身：脚本执行期（等价 SSR）为 false，挂载后为 true，
 *    而函数引用自身永远是真 —— 正是旧代码写错的地方。
 */
import { describe, it, expect } from 'vitest';
import { render, screen, waitFor } from '@testing-library/svelte';

vi.mock('svelte-chartjs', async () => {
	const mod = await import('./ChartStub.svelte');
	return { Pie: mod.default, Bar: mod.default, Line: mod.default, Chart: mod.default };
});

const CategoryPie = (await import('#lib/components/charts/CategoryPie.svelte')).default;
const MonthlyBars = (await import('#lib/components/charts/MonthlyBars.svelte')).default;
const TrendLine = (await import('#lib/components/charts/TrendLine.svelte')).default;
const MountedProbeHost = (await import('./MountedProbeHost.svelte')).default;

const PIE_ITEMS = [
	{ id: 1, name: '餐饮', icon: '🍔', amount: 120000 },
	{ id: 2, name: '交通', icon: '🚇', amount: 30000 }
];
const BAR_POINTS = [
	{ month: '2026-08', income: 100000, expense: 60000 },
	{ month: '2026-09', income: 120000, expense: 70000 }
];
const LINE_POINTS = [
	{ date: '2026-09-01', income: 10000, expense: 4000 },
	{ date: '2026-09-02', income: 0, expense: 9000 }
];

function labelsOf(index = 0): string[] {
	const stubs = screen.getAllByTestId('chart-stub');
	return JSON.parse(stubs[index].getAttribute('data-labels') || '[]');
}

describe('useMounted - 守卫标志的真实语义', () => {
	it('返回函数；脚本执行期为 false，挂载后才为 true', async () => {
		render(MountedProbeHost);

		// 函数引用本身恒为真 —— 这正是旧写法 `{#if mounted}` 失效的原因
		expect(screen.getByTestId('is-function').textContent).toBe('true');
		expect(screen.getByTestId('fn-truthy').textContent).toBe('true');

		// 真正的守卫值：初始化（SSR 阶段）取到 false
		expect(screen.getByTestId('during-init').textContent).toBe('false');
		// 客户端挂载后翻转成 true
		await waitFor(() => expect(screen.getByTestId('after-mount').textContent).toBe('true'));
	});
});

describe('CategoryPie - {#if mounted() && items.length > 0}', () => {
	it('有数据且已挂载 → 渲染图表，labels 为「图标 名称」', async () => {
		render(CategoryPie, { props: { items: PIE_ITEMS } });

		await waitFor(() => expect(screen.queryAllByTestId('chart-stub').length).toBeGreaterThan(0));
		expect(labelsOf()).toEqual(['🍔 餐饮', '🚇 交通']);
		expect(screen.queryByText('暂无数据')).toBeNull();
	});

	it('无数据 → 不渲染图表，显示「暂无数据」', async () => {
		render(CategoryPie, { props: { items: [] } });
		await waitFor(() => expect(screen.getByText('暂无数据')).toBeTruthy());
		expect(screen.queryAllByTestId('chart-stub')).toHaveLength(0);
	});

	it('超出 limit 的条目合并为「其他」，仍走 mounted 为真的分支', async () => {
		const items = Array.from({ length: 10 }, (_, i) => ({
			id: i + 1,
			name: `C${i + 1}`,
			amount: 1000
		}));
		render(CategoryPie, { props: { items, limit: 2 } });

		await waitFor(() => expect(screen.queryAllByTestId('chart-stub').length).toBeGreaterThan(0));
		expect(labelsOf()).toEqual(['C1', 'C2', '其他']);
	});
});

describe('MonthlyBars - {#if mounted() && points.length > 0}', () => {
	it('有数据 → 渲染柱状图，labels 为月份', async () => {
		render(MonthlyBars, { props: { points: BAR_POINTS } });

		await waitFor(() => expect(screen.queryAllByTestId('chart-stub').length).toBeGreaterThan(0));
		expect(labelsOf()).toEqual(['2026-08', '2026-09']);
	});

	it('无数据 → 只有占位文案，没有图表', async () => {
		render(MonthlyBars, { props: { points: [] } });
		await waitFor(() => expect(screen.getByText('暂无数据')).toBeTruthy());
		expect(screen.queryAllByTestId('chart-stub')).toHaveLength(0);
	});
});

describe('TrendLine - {#if mounted() && points.length > 0}', () => {
	it('有数据 → 渲染折线图，labels 为日期', async () => {
		render(TrendLine, { props: { points: LINE_POINTS } });

		await waitFor(() => expect(screen.queryAllByTestId('chart-stub').length).toBeGreaterThan(0));
		expect(labelsOf()).toEqual(['2026-09-01', '2026-09-02']);
	});

	it('无数据 → 只有占位文案，没有图表', async () => {
		render(TrendLine, { props: { points: [] } });
		await waitFor(() => expect(screen.getByText('暂无数据')).toBeTruthy());
		expect(screen.queryAllByTestId('chart-stub')).toHaveLength(0);
	});
});
