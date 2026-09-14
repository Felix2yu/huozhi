<script lang="ts">
	/** 分类占比饼图（A2） */
	import { Pie } from 'svelte-chartjs';
	import {
		Chart as ChartJS,
		Title,
		Tooltip,
		Legend,
		ArcElement
	} from 'chart.js';
	import { baseOptions, PALETTE, useMounted, centsToYuan } from './chartSetup';

	interface Item {
		id: number;
		name?: string;
		icon?: string;
		amount: number;
		percent?: number;
	}
	interface Props {
		items: Item[];
		limit?: number;
		height?: number;
	}
	let { items, limit = 8, height = 260 }: Props = $props();

	ChartJS.register(Title, Tooltip, Legend, ArcElement);
	const mounted = useMounted();

	const top = $derived(items.slice(0, limit));
	// 超出 limit 的部分合并为「其他」，避免饼图被长尾切碎
	const data = $derived.by(() => {
		const head = top;
		const rest = items.slice(limit);
		const restSum = rest.reduce((s, i) => s + (i.amount || 0), 0);
		const labels = head.map((i) => `${i.icon ?? ''} ${i.name ?? '未分类'}`.trim());
		const values = head.map((i) => centsToYuan(i.amount));
		if (restSum > 0) {
			labels.push('其他');
			values.push(centsToYuan(restSum));
		}
		return {
			labels,
			datasets: [
				{
					data: values,
					backgroundColor: PALETTE,
					borderWidth: 1
				}
			]
		};
	});
</script>

<div style={`height:${height}px`}>
	{#if mounted && items.length > 0}
		<Pie {data} options={baseOptions} />
	{:else}
		<div class="h-full grid place-items-center text-sm text-muted-foreground">
			{items.length === 0 ? '暂无数据' : ''}
		</div>
	{/if}
</div>
