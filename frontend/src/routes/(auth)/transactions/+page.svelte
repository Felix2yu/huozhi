<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import Button from '$lib/components/ui/Button.svelte';
	import Card from '$lib/components/ui/Card.svelte';
	import Tabs from '$lib/components/ui/Tabs.svelte'
import TabsTrigger from '$lib/components/ui/TabsTrigger.svelte';
	import { txApi } from '$lib/api/modules/transactions';
	import { appStore } from '$lib/stores/app';
	import { formatMoney, formatRelativeDate } from '$lib/utils/format';
	import type { Transaction, DayGroup, TransactionListData } from '$lib/types';
	import { Plus, Search, Filter } from '@lucide/svelte';

	let type = $state<'all' | 'expense' | 'income' | 'transfer'>('all');
	let grouped = $state<DayGroup[]>([]);
	let summary = $state<{ total_income: number; total_expense: number; net: number }>({
		total_income: 0,
		total_expense: 0,
		net: 0
	});
	let loading = $state(true);

	async function loadData() {
		const bid = appStore.currentBookId;
		if (!bid) return;
		loading = true;
		try {
			const params: any = { book_id: bid };
			if (type !== 'all') params.type = type;
			const data = (await txApi.list(params)) as TransactionListData;
			grouped = data.grouped || [];
			summary = data.summary || { total_income: 0, total_expense: 0, net: 0 };
		} catch (e) {
			console.warn(e);
		} finally {
			loading = false;
		}
	}

	onMount(loadData);

	$effect(() => {
		if (appStore.currentBookId) {
			loadData();
		}
	});
</script>

<svelte:head>
	<title>账单流水 · 货殖</title>
</svelte:head>

<div class="flex flex-col gap-4">
	<!-- 汇总卡片 -->
	<Card>
		<div class="grid grid-cols-3 p-4 gap-4">
			<div class="text-center">
				<div class="text-xs text-muted-foreground">收入</div>
				<div class="mt-1 font-bold text-[var(--color-income)] tabular-nums">
					{formatMoney(summary.total_income)}
				</div>
			</div>
			<div class="text-center border-x">
				<div class="text-xs text-muted-foreground">支出</div>
				<div class="mt-1 font-bold text-[var(--color-expense)] tabular-nums">
					{formatMoney(summary.total_expense)}
				</div>
			</div>
			<div class="text-center">
				<div class="text-xs text-muted-foreground">结余</div>
				<div class="mt-1 font-bold tabular-nums"
					class:text-primary={summary.net >= 0}
					class:text-[var(--color-expense)]={summary.net < 0}>
					{formatMoney(summary.net)}
				</div>
			</div>
		</div>
	</Card>

	<!-- 类型筛选 -->
	<div class="flex items-center gap-2">
		<Tabs bind:value={type}>
			<TabsTrigger value="all">全部</TabsTrigger>
			<TabsTrigger value="expense">支出</TabsTrigger>
			<TabsTrigger value="income">收入</TabsTrigger>
			<TabsTrigger value="transfer">转账</TabsTrigger>
		</Tabs>
	</div>

	<!-- 交易列表 -->
	{#if loading}
		<div class="space-y-2">
			{#each [1, 2, 3, 4] as i}
				<div class="h-16 rounded-lg animate-pulse bg-muted" />
			{/each}
		</div>
	{:else if grouped.length === 0}
		<Card>
			<div class="py-16 text-center">
				<div class="text-4xl mb-4 opacity-50">📋</div>
				<p class="text-muted-foreground mb-4">还没有交易记录</p>
				<Button onclick={() => goto('/transactions/add')}>
					<Plus size={16} />
					记一笔
				</Button>
			</div>
		</Card>
	{:else}
		<div class="space-y-4">
			{#each grouped as dayGroup (dayGroup.date)}
				<div>
					<div class="flex items-center justify-between mb-2 px-1">
						<span class="text-xs text-muted-foreground">
							{formatRelativeDate(dayGroup.date)}
						</span>
						<div class="text-xs text-muted-foreground tabular-nums">
							<span class="text-[var(--color-income)]">+{formatMoney(dayGroup.day_income).replace('¥', '')}</span>
							<span class="mx-1">/</span>
							<span class="text-[var(--color-expense)]">-{formatMoney(dayGroup.day_expense).replace('¥', '')}</span>
						</div>
					</div>
					<Card class="divide-y">
						{#each dayGroup.transactions as tx (tx.id)}
							<button
								class="w-full flex items-center gap-3 p-3 hover:bg-accent/50 transition text-left"
								onclick={() => goto(`/transactions/edit/${tx.id}`)}
							>
								<div
									class="w-10 h-10 rounded-lg grid place-items-center text-sm font-semibold"






								>
									{(tx.description || '¥')[0]}
								</div>
								<div class="flex-1 min-w-0">
									<div class="text-sm font-medium truncate">
										{tx.description || tx.merchant || '未分类'}
									</div>
									<div class="text-xs text-muted-foreground truncate">
										{tx.merchant || tx.location || '—'}
									</div>
								</div>
								<div
									class="font-semibold tabular-nums text-sm"



								>
									{tx.type === 'income' ? '+' : tx.type === 'expense' ? '-' : ''}
									{formatMoney(tx.amount).replace('¥', '¥')}
								</div>
							</button>
						{/each}
					</Card>
				</div>
			{/each}
		</div>
	{/if}
</div>
