<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import Button from '$lib/components/ui/Button.svelte';
	import Card from '$lib/components/ui/Card.svelte'
import CardContent from '$lib/components/ui/CardContent.svelte'
import CardHeader from '$lib/components/ui/CardHeader.svelte'
import CardTitle from '$lib/components/ui/CardTitle.svelte';
	import Badge from '$lib/components/ui/Badge.svelte';
	import Progress from '$lib/components/ui/Progress.svelte';
	import { budgetApi } from '$lib/api/modules/budgets';
	import { appStore } from '$lib/stores/app';
	import { hzToast } from '$lib/components/ui/toast';
	import { formatMoney, getMonthRange } from '$lib/utils/format';
	import type { BudgetView } from '$lib/types';
	import { Plus, Target, AlertTriangle } from '@lucide/svelte';

	let budgets = $state<BudgetView[]>([]);
	let loading = $state(true);

	async function loadData() {
		loading = true;
		try {
			budgets = await budgetApi.list({ book_id: appStore.currentBookId });
		} catch {}
		loading = false;
	}

	onMount(loadData);
	$effect(() => {
		if (appStore.currentBookId) loadData();
	});

	const overBudgetCount = $derived(budgets.filter((b) => b.is_over_budget).length);
</script>

<svelte:head>
	<title>预算管理 · 货殖</title>
</svelte:head>

<div class="space-y-6">
	<!-- 汇总 -->
	{#if overBudgetCount > 0}
		<Card class="border-destructive/50 bg-destructive/5">
			<CardContent class="py-3 px-4 flex items-center gap-3">
				<AlertTriangle class="text-destructive" size={20} />
				<div class="flex-1 text-sm">
					{overBudgetCount} 个预算已超额，注意控制支出！
				</div>
			</CardContent>
		</Card>
	{/if}

	<div class="flex items-center justify-between">
		<h2 class="text-sm font-medium">月度预算</h2>
		<Button size="sm" onclick={() => hzToast.info('预算编辑功能开发中')}>
			<Plus size={16} />
			新增预算
		</Button>
	</div>

	{#if loading}
		<div class="grid gap-3 md:grid-cols-2">
			{#each [1, 2] as i}
				<div class="h-32 rounded-xl animate-pulse bg-muted" />
			{/each}
		</div>
	{:else if budgets.length === 0}
		<Card>
			<div class="py-16 text-center">
				<Target size={32} class="mx-auto mb-3 text-muted-foreground opacity-50" />
				<p class="text-muted-foreground text-sm">还没有设置任何预算</p>
				<p class="text-xs text-muted-foreground mt-1">设置预算帮助你控制支出</p>
			</div>
		</Card>
	{:else}
		<div class="grid gap-3 md:grid-cols-2">
			{#each budgets as budget (budget.id)}
				<Card
					class="transition"

				>
					<CardContent class="p-4">
						<div class="flex items-start justify-between mb-2">
							<div class="min-w-0">
								<div class="font-medium truncate">
									{appStore.categories.expense.find((c) => c.id === budget.category_id)?.name || '全部分类'}
								</div>
								<div class="text-xs text-muted-foreground">
									{budget.period_type === 'monthly' ? '月度' : '年度'}预算
								</div>
							</div>
							{#if budget.is_over_budget}
								<Badge variant="destructive">已超额</Badge>
							{:else if budget.usage_rate >= budget.alert_rate}
								<Badge variant="secondary">接近超限</Badge>
							{/if}
						</div>

						<Progress value={Math.min(budget.usage_rate, 100)} />

						<div class="flex items-end justify-between mt-3">
							<div class="text-sm">
								<span
									class="font-semibold tabular-nums"

								>
									{formatMoney(budget.used_amount)}
								</span>
								<span class="text-muted-foreground"> / {formatMoney(budget.amount)}</span>
							</div>
							<div class="text-xs text-muted-foreground">
								剩余 {formatMoney(Math.max(0, budget.remaining))}
							</div>
						</div>
					</CardContent>
				</Card>
			{/each}
		</div>
	{/if}
</div>
