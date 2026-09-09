<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import Button from '$lib/components/ui/Button.svelte';
	import Card from '$lib/components/ui/Card.svelte';
	import Tabs from '$lib/components/ui/Tabs.svelte';
	import TabsTrigger from '$lib/components/ui/TabsTrigger.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import Label from '$lib/components/ui/Label.svelte';
	import { txApi } from '$lib/api/modules/transactions';
	import { appStore } from '$lib/stores/app';
	import { formatMoney, formatRelativeDate } from '$lib/utils/format';
	import type { Transaction, DayGroup, TransactionListData } from '$lib/types';
	import { Plus, Search, Filter, X } from '@lucide/svelte';

	let type = $state<'all' | 'expense' | 'income' | 'transfer'>('all');
	let grouped = $state<DayGroup[]>([]);
	let summary = $state<{ total_income: number; total_expense: number; net: number }>({
		total_income: 0,
		total_expense: 0,
		net: 0
	});
	let loading = $state(true);

	// Filter state
	let showFilters = $state(false);
	let keyword = $state('');
	let startDate = $state('');
	let endDate = $state('');
	let categoryId = $state<number | ''>('');
	let accountId = $state<number | ''>('');

	async function loadData() {
		const bid = appStore.currentBookId;
		if (!bid) return;
		loading = true;
		try {
			const params: any = { book_id: bid };
			if (type !== 'all') params.type = type;
			if (keyword.trim()) params.keyword = keyword.trim();
			if (startDate) params.start_date = startDate;
			if (endDate) params.end_date = endDate;
			if (categoryId !== '') params.category_id = categoryId;
			if (accountId !== '') params.account_id = accountId;
			const data = (await txApi.list(params)) as TransactionListData;
			grouped = data.grouped || [];
			summary = data.summary || { total_income: 0, total_expense: 0, net: 0 };
		} catch (e) {
			console.warn(e);
		} finally {
			loading = false;
		}
	}

	function clearFilters() {
		keyword = '';
		startDate = '';
		endDate = '';
		categoryId = '';
		accountId = '';
		loadData();
	}

	const hasFilters = $derived(keyword || startDate || endDate || categoryId !== '' || accountId !== '');

	onMount(loadData);

	$effect(() => {
		if (appStore.currentBookId) {
			loadData();
		}
	});

	$effect(() => {
		// Debounce search
		const timer = setTimeout(() => {
			if (keyword !== undefined) loadData();
		}, 300);
		return () => clearTimeout(timer);
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

	<!-- 筛选栏 -->
	<div class="flex items-center gap-2">
		<Tabs bind:value={type}>
			<TabsTrigger value="all">全部</TabsTrigger>
			<TabsTrigger value="expense">支出</TabsTrigger>
			<TabsTrigger value="income">收入</TabsTrigger>
			<TabsTrigger value="transfer">转账</TabsTrigger>
		</Tabs>
		<div class="flex-1" ></div>
		<Button
			size="sm"
			variant={showFilters ? 'default' : 'outline'}
			onclick={() => (showFilters = !showFilters)}
		>
			<Filter size={14} />
			筛选
			{#if hasFilters}
				<span class="w-1.5 h-1.5 rounded-full bg-primary" />
			{/if}
		</Button>
	</div>

	<!-- 筛选面板 -->
	{#if showFilters}
		<Card>
			<div class="p-4 space-y-3">
				<div class="space-y-2">
					<Label>关键词搜索</Label>
					<div class="relative">
						<Search size={14} class="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground" />
						<Input class="pl-9" placeholder="搜索描述、商户..." bind:value={keyword} />
					</div>
				</div>

				<div class="grid grid-cols-2 gap-4">
					<div class="space-y-2">
						<Label>开始日期</Label>
						<Input type="date" bind:value={startDate} />
					</div>
					<div class="space-y-2">
						<Label>结束日期</Label>
						<Input type="date" bind:value={endDate} />
					</div>
				</div>

				<div class="grid grid-cols-2 gap-4">
					<div class="space-y-2">
						<Label>分类</Label>
						<select
							class="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm focus:outline-none focus:ring-1 focus:ring-ring"
							bind:value={categoryId}
						>
							<option value="">全部分类</option>
							{#each appStore.categories.expense as cat}
								<option value={cat.id}>{cat.icon || '📁'} {cat.name}</option>
							{/each}
						</select>
					</div>
					<div class="space-y-2">
						<Label>账户</Label>
						<select
							class="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm focus:outline-none focus:ring-1 focus:ring-ring"
							bind:value={accountId}
						>
							<option value="">全部账户</option>
							{#each appStore.accounts as acc}
								<option value={acc.id}>{acc.name}</option>
							{/each}
						</select>
					</div>
				</div>

				<div class="flex gap-2 pt-2">
					<Button size="sm" onclick={loadData}>应用筛选</Button>
					{#if hasFilters}
						<Button size="sm" variant="outline" onclick={clearFilters}>
							<X size={14} />
							清除
						</Button>
					{/if}
				</div>
			</div>
		</Card>
	{/if}

	<!-- 交易列表 -->
	{#if loading}
		<div class="space-y-2">
			{#each [1, 2, 3, 4] as i}
				<div class="h-16 rounded-lg animate-pulse bg-muted" ></div>
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
								<div class="w-10 h-10 rounded-lg bg-muted grid place-items-center text-sm font-semibold">
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
								<div class="font-semibold tabular-nums text-sm">
									{tx.type === 'income' ? '+' : tx.type === 'expense' ? '-' : ''}
									{formatMoney(tx.amount)}
								</div>
							</button>
						{/each}
					</Card>
				</div>
			{/each}
		</div>
	{/if}
</div>
