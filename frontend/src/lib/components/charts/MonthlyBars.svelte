<script lang="ts">
	/** 月度收支对比柱状图（A2） */
	import { Bar } from 'svelte-chartjs';
	import {
		Chart as ChartJS,
		Title,
		Tooltip,
		Legend,
		BarElement,
		LinearScale,
		CategoryScale
	} from 'chart.js';
	import { baseOptions, INCOME_COLOR, EXPENSE_COLOR, useMounted, centsToYuan } from './chartSetup';
	import { privacyStore } from '#lib/stores/privacy';

	interface Point {
		date?: string;
		month?: string;
		income?: number;
		expense?: number;
		total_income?: number;
		total_expense?: number;
	}
	interface Props {
		points: Point[];
		height?: number;
	}
	let { points = [], height = 280 }: Props = $props();

	ChartJS.register(Title, Tooltip, Legend, BarElement, LinearScale, CategoryScale);
	const mounted = useMounted();

	const data = $derived({
		labels: points.map((p) => p.month ?? p.date ?? ''),
		datasets: [
			{
				label: '收入',
				data: points.map((p) => centsToYuan(p.income ?? p.total_income ?? 0)),
				backgroundColor: INCOME_COLOR
			},
			{
				label: '支出',
				data: points.map((p) => centsToYuan(p.expense ?? p.total_expense ?? 0)),
				backgroundColor: EXPENSE_COLOR
			}
		]
	});

	// 隐私模式：坐标轴刻度与悬浮提示一并遮蔽（masked 变化时通过 {#key} 重建图表）
	const masked = $derived(privacyStore.masked);

	const options = $derived({
		...baseOptions,
		plugins: {
			...baseOptions.plugins,
			tooltip: {
				enabled: true,
				callbacks: {
					label: (ctx: any) =>
						masked
							? `${ctx.dataset.label}: ••••`
							: `${ctx.dataset.label}: ${ctx.parsed.y} 元`
				}
			}
		},
		scales: {
			y: {
				beginAtZero: true,
				ticks: { font: { size: 10 }, callback: (v: any) => (masked ? '••••' : v) }
			},
			x: { ticks: { font: { size: 10 } } }
		}
	});
</script>

<div style={`height:${height}px`}>
	{#if mounted && points.length > 0}
		{#key masked}
			<Bar {data} {options} />
		{/key}
	{:else}
		<div class="h-full grid place-items-center text-sm text-muted-foreground">
			{points.length === 0 ? '暂无数据' : ''}
		</div>
	{/if}
</div>
