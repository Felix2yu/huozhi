<script lang="ts">
	import Card from '$lib/components/ui/Card.svelte';
	import CardContent from '$lib/components/ui/CardContent.svelte';
	import Tabs from '$lib/components/ui/Tabs.svelte';
	import TabsTrigger from '$lib/components/ui/TabsTrigger.svelte';
	import { appStore } from '$lib/stores/app';
	import { statsApi } from '$lib/api/modules/statistics';
	import { formatMoney } from '$lib/utils/format';
	import type { StatisticsData, AssetOverview } from '$lib/types';
	import { BarChart3, ArrowUpRight, ArrowDownRight } from '@lucide/svelte';

	let data = $state<StatisticsData | null>(null);
	let assets = $state<AssetOverview | null>(null);
	let range = $state<'month' | 'quarter' | 'year'>('month');
	let loading = $state(true);

	function getDateRange(r: string) {
		const now = new Date();
		const year = now.getFullYear();
		const month = now.getMonth();

		if (r === 'quarter') {
			const qStart = Math.floor(month / 3) * 3;
			const start = new Date(year, qStart, 1);
			const end = new Date(year, qStart + 3, 0);
			return {
				start: start.toISOString().split('T')[0],
				end: end.toISOString().split('T')[0]
			};
		}

		if (r === 'year') {
			return {
				start: `${year}-01-01`,
				end: `${year}-12-31`
			};
		}

		// month
		const start = new Date(year, month, 1);
		const end = new Date(year, month + 1, 0);
		return {
			start: start.toISOString().split('T')[0],
			end: end.toISOString().split('T')[0]
		};
	}

	async function loadData() {
		loading = true;
		try {
			const params: any = { book_id: appStore.currentBookId };
			const { start, end } = getDateRange(range);
			params.start_date = start;
			params.end_date = end;

			const [statsRes, assetsRes] = await Promise.allSettled([
				statsApi.overview(params),
				statsApi.assets()
			]);
			if (statsRes.status === 'fulfilled') data = statsRes.value;
			if (assetsRes.status === 'fulfilled') assets = assetsRes.value;
		} catch {}
		loading = false;
	}

	// 账本切换（含切到「全部账本」=0）时重新加载；book_id=0 由后端聚合全部账本
	$effect(() => {
		void appStore.currentBookId;
		loadData();
	});
</script>

<svelte:head>
	<title>统计分析 · 货殖</title>
</svelte:head>

<div class="space-y-6">
	<!-- 时间范围选择 -->
	<div class="flex items-center gap-2">
		<Tabs bind:value={range}>
			<TabsTrigger value="month">本月</TabsTrigger>
			<TabsTrigger value="quarter">本季</TabsTrigger>
			<TabsTrigger value="year">本年</TabsTrigger>
		</Tabs>
	</div>

	{#if loading}
		<div class="space-y-4">
			{#each [1, 2, 3] as i}
				<div class="h-32 rounded-xl animate-pulse bg-muted" ></div>
			{/each}
		</div>
	{:else}
		<!-- 汇总 -->
		{#if data}
			<section class="grid grid-cols-2 md:grid-cols-4 gap-3">
				<Card>
					<CardContent class="pt-4">
						<div class="text-xs text-muted-foreground flex items-center gap-1">
							<ArrowUpRight size={12} class="text-[var(--color-income)]" />
							总收入
						</div>
						<div class="mt-1 text-lg font-bold text-[var(--color-income)] tabular-nums">
							{formatMoney(data.summary.total_income)}
						</div>
						<div class="text-[11px] text-muted-foreground">{data.summary.income_count} 笔</div>
					</CardContent>
				</Card>
				<Card>
					<CardContent class="pt-4">
						<div class="text-xs text-muted-foreground flex items-center gap-1">
							<ArrowDownRight size={12} class="text-[var(--color-expense)]" />
							总支出
						</div>
						<div class="mt-1 text-lg font-bold text-[var(--color-expense)] tabular-nums">
							{formatMoney(data.summary.total_expense)}
						</div>
						<div class="text-[11px] text-muted-foreground">{data.summary.expense_count} 笔</div>
					</CardContent>
				</Card>
				<Card>
					<CardContent class="pt-4">
						<div class="text-xs text-muted-foreground">日均支出</div>
						<div class="mt-1 text-lg font-bold tabular-nums">
							{formatMoney(data.summary.avg_daily_expense)}
						</div>
					</CardContent>
				</Card>
				<Card>
					<CardContent class="pt-4">
						<div class="text-xs text-muted-foreground">交易笔数</div>
						<div class="mt-1 text-lg font-bold tabular-nums">
							{data.summary.transaction_count}
						</div>
					</CardContent>
				</Card>
			</section>

			<!-- 分类占比 -->
			<section>
				<h2 class="text-sm font-medium mb-3">支出分类占比</h2>
				<Card>
					<CardContent class="p-4 space-y-3">
						{#each data.by_category_expense.slice(0, 8) as item (item.id)}
							<div>
								<div class="flex items-center justify-between mb-1">
									<div class="flex items-center gap-2">
										<span class="w-6 h-6 rounded bg-muted grid place-items-center text-xs">
											{item.icon || '📁'}
										</span>
										<span class="text-sm">{item.name}</span>
									</div>
									<div class="text-sm tabular-nums text-muted-foreground">
										{formatMoney(item.amount)}
										<span class="ml-1 text-[11px]">({item.percent.toFixed(1)}%)</span>
									</div>
								</div>
							<div class="h-1.5 rounded-full bg-muted overflow-hidden">
								<div
									class="h-full bg-[var(--color-primary)] rounded-full"
									style={`width: ${item.percent}%`}
								></div>
							</div>
							</div>
						{/each}
						{#if data.by_category_expense.length === 0}
							<p class="text-center text-muted-foreground py-4 text-sm">暂无数据</p>
						{/if}
					</CardContent>
				</Card>
			</section>

			<!-- 收支趋势 -->
			<section>
				<h2 class="text-sm font-medium mb-3">收支趋势</h2>
				<Card>
					<CardContent class="p-4">
						{#if data.trend.length > 0}
							<div class="space-y-1">
								{#each data.trend as point}
									<div class="flex items-center gap-3 text-xs">
										<span class="w-16 text-muted-foreground shrink-0">{point.date}</span>
										<div class="flex-1 h-6 rounded bg-muted/50 flex gap-px items-center overflow-hidden">
											<div
												class="h-full bg-[var(--color-income)] flex items-center justify-end px-1 text-white text-[10px]"
												style={`width: ${Math.min(100, (point.income / Math.max(point.income, point.expense, 1)) * 100)}%`}
											>
												{#if point.income > 0}{formatMoney(point.income).replace('¥', '')}{/if}
											</div>
											<div
												class="h-full bg-[var(--color-expense)] flex items-center px-1 text-white text-[10px]"
												style={`width: ${Math.min(100, (point.expense / Math.max(point.income, point.expense, 1)) * 100)}%`}
											>
												{#if point.expense > 0}{formatMoney(point.expense).replace('¥', '')}{/if}
											</div>
										</div>
									</div>
								{/each}
							</div>
						{:else}
							<p class="text-center text-muted-foreground py-4 text-sm">暂无数据</p>
						{/if}
					</CardContent>
				</Card>
			</section>
		{/if}

		<!-- 资产概览 -->
		{#if assets}
			<section>
				<h2 class="text-sm font-medium mb-3">资产概览</h2>
				<Card>
					<CardContent class="p-4 grid grid-cols-2 gap-4">
						<div class="text-center p-3 rounded-lg bg-primary/5">
							<div class="text-xs text-muted-foreground">总资产</div>
							<div class="mt-1 font-bold text-primary tabular-nums">
								{formatMoney(assets.total_asset)}
							</div>
						</div>
						<div class="text-center p-3 rounded-lg bg-[var(--color-expense)]/5">
							<div class="text-xs text-muted-foreground">总负债</div>
							<div class="mt-1 font-bold text-[var(--color-expense)] tabular-nums">
								{formatMoney(assets.total_debt)}
							</div>
						</div>
						<div class="text-center p-3 rounded-lg bg-muted/50">
							<div class="text-xs text-muted-foreground">现金</div>
							<div class="mt-1 font-bold tabular-nums">
								{formatMoney(assets.cash_on_hand)}
							</div>
						</div>
						<div class="text-center p-3 rounded-lg bg-muted/50">
							<div class="text-xs text-muted-foreground">账户数</div>
							<div class="mt-1 font-bold tabular-nums">
								{assets.account_count}
							</div>
						</div>
					</CardContent>
				</Card>
			</section>
		{/if}
	{/if}
</div>
