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
	let { points, height = 280, showNet = true }: Props = $props();

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

	const options = $derived({
		...baseOptions,
		scales: {
			y: { beginAtZero: true, ticks: { font: { size: 10 } } },
			x: { ticks: { font: { size: 10 }, maxRotation: 0, autoSkip: true, maxTicksLimit: 12 } }
		}
	});
</script>

<div style={`height:${height}px`}>
	{#if mounted && points.length > 0}
		<Line {data} {options} />
	{:else}
		<div class="h-full grid place-items-center text-sm text-muted-foreground">
			{points.length === 0 ? '暂无数据' : ''}
		</div>
	{/if}
</div>
