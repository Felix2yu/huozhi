<script lang="ts">
	/**
	 * 仅用于测试的 svelte-chartjs 替身：真实 Chart.js 需要 canvas 2D 上下文，
	 * jsdom 不提供。用它替换掉 Pie/Bar/Line，就能干净地观察
	 * 图表组件 `{#if mounted() && …}` 守卫到底走了哪一条分支。
	 */
	import type { ChartData } from 'chart.js';

	let { data }: { data?: ChartData<any> } = $props();
</script>

<canvas data-testid="chart-stub" data-labels={JSON.stringify((data as any)?.labels ?? [])}></canvas>
