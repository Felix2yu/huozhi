<script lang="ts">
	import { goto } from '$app/navigation';
	import Button from '$lib/components/ui/Button.svelte';
	import Card from '$lib/components/ui/Card.svelte'
import CardContent from '$lib/components/ui/CardContent.svelte'
import CardHeader from '$lib/components/ui/CardHeader.svelte'
import CardTitle from '$lib/components/ui/CardTitle.svelte';
	import { appStore } from '$lib/stores/app';
	import AccountIcon from '$lib/components/AccountIcon.svelte';
	import { accountApi, txApi, statsApi } from '$lib/api/modules';
	import { formatMoney, getMonthRange, formatRelativeDate } from '$lib/utils/format';
	import type { CreditRepayItem, Transaction, AssetOverview } from '$lib/types';
	import {
		Wallet,
		ArrowUpRight,
		ArrowDownRight,
		TrendingUp,
		Receipt,
		CreditCard,
		Plus,
		Target,
		BarChart3,
		ChevronRight
	} from '@lucide/svelte';

	let assetOverview = $state<AssetOverview | null>(null);
	let recentTxs = $state<Transaction[]>([]);
	let creditItems = $state<CreditRepayItem[]>([]);
	let loading = $state(true);

	async function loadData() {
		loading = true;
		try {
			const { start, end } = getMonthRange();
			const [assets, txs, credits] = await Promise.allSettled([
				statsApi.assets(),
				txApi.list({
					// book_id 传 0 表示「全部账本」，后端会按用户聚合所有账本
					book_id: appStore.currentBookId || 0,
					start_date: start,
					end_date: end,
					page_size: 5,
					sort: 'desc'
				}),
				accountApi.creditSummary()
			]);
			if (assets.status === 'fulfilled') assetOverview = assets.value;
			if (txs.status === 'fulfilled') {
				const data = txs.value as any;
				recentTxs = (data.flat_list || data.transactions || []) as Transaction[];
			}
			if (credits.status === 'fulfilled') creditItems = credits.value ?? [];
		} catch (e) {
			console.warn(e);
		} finally {
			// 无论成功还是失败都必须结束加载态，否则页面会永久停在骨架屏
			loading = false;
		}
	}

	// 账本切换（含切到「全部账本」=0）时重新加载
	$effect(() => {
		void appStore.currentBookId;
		loadData();
	});
</script>

<svelte:head>
	<title>首页总览 · 货殖</title>
</svelte:head>

{#if loading}
	<div class="space-y-4">
		{#each [1, 2, 3] as i}
			<div class="h-28 rounded-xl animate-pulse bg-muted" ></div>
		{/each}
	</div>
{:else}
	<!-- ============ 资产总览 ============ -->
	<section class="grid grid-cols-3 gap-3 md:gap-4">
		<Card class="col-span-1 md:col-span-1">
			<CardContent class="pt-4">
				<div class="flex items-center gap-2 text-xs text-muted-foreground">
					<Wallet size={14} />
					<span>总资产</span>
				</div>
				<div class="mt-2 text-lg md:text-2xl font-bold tabular-nums text-primary">
					{formatMoney(assetOverview?.total_asset || 0)}
				</div>
			</CardContent>
		</Card>
		<Card class="col-span-1 md:col-span-1">
			<CardContent class="pt-4">
				<div class="flex items-center gap-2 text-xs text-muted-foreground">
					<ArrowDownRight size={14} />
					<span>总负债</span>
				</div>
				<div class="mt-2 text-lg md:text-2xl font-bold tabular-nums text-[var(--color-expense)]">
					{formatMoney(assetOverview?.total_debt || 0)}
				</div>
			</CardContent>
		</Card>
		<Card class="col-span-1 md:col-span-1">
			<CardContent class="pt-4">
				<div class="flex items-center gap-2 text-xs text-muted-foreground">
					<TrendingUp size={14} />
					<span>净资产</span>
				</div>
				<div class="mt-2 text-lg md:text-2xl font-bold tabular-nums">
					{formatMoney(assetOverview?.net_asset || 0)}
				</div>
			</CardContent>
		</Card>
	</section>

	<!-- ============ 本月收支 ============ -->
	{#if assetOverview}
		<section class="mt-4 grid grid-cols-2 gap-3 md:gap-4">
			<Card>
				<CardContent class="pt-4 flex items-center justify-between">
					<div>
						<div class="text-xs text-muted-foreground">本月收入</div>
						<div class="mt-1 text-xl font-bold text-[var(--color-income)] tabular-nums">
							{formatMoney(assetOverview.month_income)}
						</div>
					</div>
					<div class="w-10 h-10 rounded-full bg-[var(--color-income)]/10 grid place-items-center">
						<ArrowUpRight class="text-[var(--color-income)]" size={20} />
					</div>
				</CardContent>
			</Card>
			<Card>
				<CardContent class="pt-4 flex items-center justify-between">
					<div>
						<div class="text-xs text-muted-foreground">本月支出</div>
						<div class="mt-1 text-xl font-bold text-[var(--color-expense)] tabular-nums">
							{formatMoney(assetOverview.month_expense)}
						</div>
					</div>
					<div class="w-10 h-10 rounded-full bg-[var(--color-expense)]/10 grid place-items-center">
						<ArrowDownRight class="text-[var(--color-expense)]" size={20} />
					</div>
				</CardContent>
			</Card>
		</section>
	{/if}

	<!-- ============ 快捷操作 ============ -->
	<section class="mt-6">
		<h2 class="text-sm font-medium text-muted-foreground mb-3">快捷操作</h2>
		<div class="grid grid-cols-4 md:grid-cols-5 gap-2 md:gap-3">
			<button
				class="flex flex-col items-center gap-2 p-3 rounded-xl border hover:bg-accent transition"
				onclick={() => goto('/transactions/add')}
			>
				<div class="w-10 h-10 rounded-xl bg-primary grid place-items-center text-primary-foreground">
					<Plus size={20} />
				</div>
				<span class="text-xs">记一笔</span>
			</button>
			<button
				class="flex flex-col items-center gap-2 p-3 rounded-xl border hover:bg-accent transition"
				onclick={() => goto('/budgets')}
			>
				<div class="w-10 h-10 rounded-xl bg-muted grid place-items-center">
					<Target size={20} />
				</div>
				<span class="text-xs">预算</span>
			</button>
			<button
				class="flex flex-col items-center gap-2 p-3 rounded-xl border hover:bg-accent transition"
				onclick={() => goto('/statistics')}
			>
				<div class="w-10 h-10 rounded-xl bg-muted grid place-items-center">
					<BarChart3 size={20} />
				</div>
				<span class="text-xs">统计</span>
			</button>
			<button
				class="flex flex-col items-center gap-2 p-3 rounded-xl border hover:bg-accent transition"
				onclick={() => goto('/accounts')}
			>
				<div class="w-10 h-10 rounded-xl bg-muted grid place-items-center">
					<Wallet size={20} />
				</div>
				<span class="text-xs">账户</span>
			</button>
			<button
				class="hidden md:flex flex-col items-center gap-2 p-3 rounded-xl border hover:bg-accent transition"
				onclick={() => goto('/cards')}
			>
				<div class="w-10 h-10 rounded-xl bg-muted grid place-items-center">
					<CreditCard size={20} />
				</div>
				<span class="text-xs">银行卡</span>
			</button>
		</div>
	</section>

	<!-- ============ 信用卡还款 ============ -->
	{#if creditItems.length > 0}
		<section class="mt-6">
			<div class="flex items-center justify-between mb-3">
				<h2 class="text-sm font-medium">信用卡还款</h2>
				<a href="/cards" class="text-xs text-primary hover:underline flex items-center">
					查看全部
					<ChevronRight size={14} />
				</a>
			</div>
			<div class="space-y-2">
				{#each creditItems.slice(0, 3) as item (item.id)}
					<Card class={item.overdue ? 'border-destructive/50' : ''}>
						<CardContent class="py-3 px-4 flex items-center gap-3">
						<AccountIcon bankName={item.bank_name} size={40} />
							<div class="flex-1 min-w-0">
								<div class="text-sm font-medium truncate">
									{item.bank_name} · •••• {item.card_no4}
								</div>
								<div class="text-xs text-muted-foreground">
									应还 {formatMoney(item.bill_amount)} · 还剩 {item.days_left} 天
								</div>
							</div>
							<div
								class={item.overdue ? 'text-destructive text-xs font-medium' : 'text-muted-foreground text-xs'}
							>
								{item.repay_date}
							</div>
						</CardContent>
					</Card>
				{/each}
			</div>
		</section>
	{/if}

	<!-- ============ 最近交易 ============ -->
	<section class="mt-6">
		<div class="flex items-center justify-between mb-3">
			<h2 class="text-sm font-medium">最近交易</h2>
			<a href="/transactions" class="text-xs text-primary hover:underline flex items-center">
				查看全部
				<ChevronRight size={14} />
			</a>
		</div>
		<Card>
			<CardContent class="p-0 divide-y">
				{#if recentTxs.length === 0}
					<div class="py-12 text-center text-muted-foreground">
						<Receipt size={32} class="mx-auto mb-3 opacity-50" />
						<p class="text-sm">暂无交易记录</p>
						<Button class="mt-4" size="sm" onclick={() => goto('/transactions/add')}>
							<Plus size={16} />
							记一笔
						</Button>
					</div>
				{:else}
					{#each recentTxs as tx (tx.id)}
						<button
							type="button"
							class="flex items-center gap-3 p-3 hover:bg-accent/50 transition cursor-pointer text-left w-full"
							onclick={() => goto(`/transactions/edit/${tx.id}`)}
						>
							<div
								class="w-9 h-9 rounded-lg grid place-items-center text-sm font-semibold"






							>
								{tx.description?.[0] || tx.merchant?.[0] || '¥'}
							</div>
							<div class="flex-1 min-w-0">
								<div class="text-sm font-medium truncate">
									{tx.description || tx.merchant || '未分类'}
								</div>
								<div class="text-xs text-muted-foreground">
									{formatRelativeDate(tx.tx_date)}
								</div>
							</div>
							<div
								class="font-semibold tabular-nums text-sm"



							>
								{tx.type === 'income' ? '+' : tx.type === 'expense' ? '-' : ''}
								{formatMoney(tx.amount).replace('¥', '')}
							</div>
						</button>
					{/each}
				{/if}
			</CardContent>
		</Card>
	</section>
{/if}
