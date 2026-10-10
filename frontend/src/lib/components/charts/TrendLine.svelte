<script lang="ts">
	/** 收支趋势折线图（A2） */
	import { Line } from 'svelte-chartjs';
	import {
		Chart as ChartJS,
		Title,
		Tooltip,
		Legend,
		LineElement,
		LinearScale,
		PointElement,
		CategoryScale,
		Filler
	} from 'chart.js';
	import { baseOptions, INCOME_COLOR, EXPENSE_COLOR, useMounted, centsToYuan } from './chartSetup';
	import { privacyStore } from '#lib/stores/privacy';

	interface Point {
		date: string;
		income: number;
		expense: number;
		net?: number;
	}
	interface Props {
		points: Point[];
		height?: number;
		showNet?: boolean;
	}
	let { points = [], height = 280, showNet = true }: Props = $props();

	ChartJS.register(Title, Tooltip, Legend, LineElement, LinearScale, PointElement, CategoryScale, Filler);
	const mounted = useMounted();

	const data = $derived({
		labels: points.map((p) => p.date),
		datasets: [
			{ label: '收入', data: points.map((p) => centsToYuan(p.income)), borderColor: INCOME_COLOR, backgroundColor: INCOME_COLOR + '22', tension: 0.3, fill: false },
			{ label: '支出', data: points.map((p) => centsToYuan(p.expense)), borderColor: EXPENSE_COLOR, backgroundColor: EXPENSE_COLOR + '22', tension: 0.3, fill: false },
			...(showNet
				? [{ label: '结余', data: points.map((p) => centsToYuan(p.net ?? p.income - p.expense)), borderColor: '#6366f1', backgroundColor: '#6366f122', tension: 0.3, fill: false, borderDash: [4, 4] }]
				: [])
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
			x: { ticks: { font: { size: 10 }, maxRotation: 0, autoSkip: true, maxTicksLimit: 12 } }
		}
	});
</script>

<div style={`height:${height}px`}>
	{#if mounted && points.length > 0}
		{#key masked}
			<Line {data} {options} />
		{/key}
	{:else}
		<div class="h-full grid place-items-center text-sm text-muted-foreground">
			{points.length === 0 ? '暂无数据' : ''}
		</div>
	{/if}
</div>
