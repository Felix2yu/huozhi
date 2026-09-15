/**
 * 图表统一封装（A2）
 *
 * 背景：`chart.js` + `svelte-chartjs` 早已在 package.json 中，但
 * `lib/components/charts/` 是空目录，统计页用手写 <div> 进度条代替，
 * 长周期趋势逐条渲染还会撑爆页面。这里补齐三个通用图表组件。
 *
 * 注意：Chart.js 依赖 canvas，SSR / 预渲染阶段不可用，
 * 因此所有组件都在 onMount 之后才挂载真实图表。
 */
import { onMount } from 'svelte';
import type { ChartData, ChartOptions } from 'chart.js';

// 主题色取自 CSS 变量，保证浅色/深色主题下都可读
export const PALETTE = [
	'#ef4444', '#f97316', '#f59e0b', '#10b981', '#06b6d4',
	'#3b82f6', '#6366f1', '#8b5cf6', '#ec4899', '#64748b'
];

export const INCOME_COLOR = '#10b981';
export const EXPENSE_COLOR = '#ef4444';

/** 返回「挂载后才为 true」的标志位，避免 SSR 阶段访问 canvas */
export function useMounted(): () => boolean {
	let mounted = $state(false);
	onMount(() => {
		mounted = true;
	});
	return () => mounted;
}

/** 金额（分）→ 元，图表统一用元展示 */
export function centsToYuan(v: number): number {
	return (v ?? 0) / 100;
}

export const baseOptions: ChartOptions<any> = {
	responsive: true,
	maintainAspectRatio: false,
	plugins: {
		legend: { display: true, position: 'bottom', labels: { boxWidth: 12, font: { size: 11 } } },
		tooltip: { enabled: true }
	}
};

export type { ChartData, ChartOptions };
