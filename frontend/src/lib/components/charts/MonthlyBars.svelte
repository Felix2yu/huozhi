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

	const options = $derived({
		...baseOptions,
		scales: {
			y: { beginAtZero: true, ticks: { font: { size: 10 } } },
			x: { ticks: { font: { size: 10 } } }
		}
	});
</script>

<div style={`height:${height}px`}>
	{#if mounted && points.length > 0}
		<Bar {data} {options} />
	{:else}
		<div class="h-full grid place-items-center text-sm text-muted-foreground">
			{points.length === 0 ? '暂无数据' : ''}
		</div>
	{/if}
</div>
