<script lang="ts">
	import { goto } from '$app/navigation';
	import Button from '#lib/components/ui/Button.svelte';
	import Card from '#lib/components/ui/Card.svelte';
	import CardContent from '#lib/components/ui/CardContent.svelte';
	import { appStore } from '#lib/stores/app';
	import AccountIcon from '#lib/components/AccountIcon.svelte';
	import { accountApi, txApi, statsApi } from '#lib/api/modules';
	import { formatMoney, getMonthRange, formatRelativeDate } from '#lib/utils/format';
	import { resolve } from '$app/paths';
	import type {
		CreditRepayItem,
		Transaction,
		AssetOverview,
		TransactionListData
	} from '#lib/types';
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

	// C20：把交易的 category_id / account_id 解析成可读名称，供列表副标题使用
	function categoryOf(tx: Transaction) {
		const all = [...(appStore.categories?.expense ?? []), ...(appStore.categories?.income ?? [])];
		return all.find((c: any) => c.id === tx.category_id);
	}
	function accountOf(tx: Transaction) {
		return (appStore.accounts ?? []).find((a: any) => a.id === tx.account_id);
	}

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
				const data = txs.value as TransactionListData;
				// flat_list 已被后端移除（与 grouped 重复序列化，原 P-02），从分组展开
				recentTxs = (data.grouped ?? []).flatMap((g) => g.transactions ?? []);
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
		{#each [1, 2, 3] as i (i)}
			<div class="h-28 animate-pulse rounded-xl bg-muted"></div>
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
				<div class="mt-2 text-lg font-bold text-primary tabular-nums md:text-2xl">
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
				<div class="mt-2 text-lg font-bold text-[var(--color-expense)] tabular-nums md:text-2xl">
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
				<div class="mt-2 text-lg font-bold tabular-nums md:text-2xl">
					{formatMoney(assetOverview?.net_asset || 0)}
				</div>
			</CardContent>
		</Card>
	</section>

	<!-- ============ 本月收支 ============ -->
	{#if assetOverview}
		<section class="mt-4 grid grid-cols-2 gap-3 md:gap-4">
			<Card>
				<CardContent class="flex items-center justify-between pt-4">
					<div>
						<div class="text-xs text-muted-foreground">本月收入</div>
						<div class="mt-1 text-xl font-bold text-[var(--color-income)] tabular-nums">
							{formatMoney(assetOverview.month_income)}
						</div>
					</div>
					<div class="grid h-10 w-10 place-items-center rounded-full bg-[var(--color-income)]/10">
						<ArrowUpRight class="text-[var(--color-income)]" size={20} />
					</div>
				</CardContent>
			</Card>
			<Card>
				<CardContent class="flex items-center justify-between pt-4">
					<div>
						<div class="text-xs text-muted-foreground">本月支出</div>
						<div class="mt-1 text-xl font-bold text-[var(--color-expense)] tabular-nums">
							{formatMoney(assetOverview.month_expense)}
						</div>
					</div>
					<div class="grid h-10 w-10 place-items-center rounded-full bg-[var(--color-expense)]/10">
						<ArrowDownRight class="text-[var(--color-expense)]" size={20} />
					</div>
				</CardContent>
			</Card>
		</section>
	{/if}

	<!-- ============ 快捷操作 ============ -->
	<section class="mt-6">
		<h2 class="mb-3 text-sm font-medium text-muted-foreground">快捷操作</h2>
		<div class="grid grid-cols-4 gap-2 md:grid-cols-5 md:gap-3">
			<button
				class="flex flex-col items-center gap-2 rounded-xl border p-3 transition hover:bg-accent"
				onclick={() => goto(resolve('/(auth)/transactions/add'))}
			>
				<div
					class="grid h-10 w-10 place-items-center rounded-xl bg-primary text-primary-foreground"
				>
					<Plus size={20} />
				</div>
				<span class="text-xs">记一笔</span>
			</button>
			<button
				class="flex flex-col items-center gap-2 rounded-xl border p-3 transition hover:bg-accent"
				onclick={() => goto(resolve('/(auth)/budgets'))}
			>
				<div class="grid h-10 w-10 place-items-center rounded-xl bg-muted">
					<Target size={20} />
				</div>
				<span class="text-xs">预算</span>
			</button>
			<button
				class="flex flex-col items-center gap-2 rounded-xl border p-3 transition hover:bg-accent"
				onclick={() => goto(resolve('/(auth)/statistics'))}
			>
				<div class="grid h-10 w-10 place-items-center rounded-xl bg-muted">
					<BarChart3 size={20} />
				</div>
				<span class="text-xs">统计</span>
			</button>
			<button
				class="flex flex-col items-center gap-2 rounded-xl border p-3 transition hover:bg-accent"
				onclick={() => goto(resolve('/(auth)/accounts'))}
			>
				<div class="grid h-10 w-10 place-items-center rounded-xl bg-muted">
					<Wallet size={20} />
				</div>
				<span class="text-xs">账户</span>
			</button>
			<button
				class="hidden flex-col items-center gap-2 rounded-xl border p-3 transition hover:bg-accent md:flex"
				onclick={() => goto(resolve('/(auth)/cards'))}
			>
				<div class="grid h-10 w-10 place-items-center rounded-xl bg-muted">
					<CreditCard size={20} />
				</div>
				<span class="text-xs">银行卡</span>
			</button>
		</div>
	</section>

	<!-- ============ 信用卡还款 ============ -->
	{#if creditItems.length > 0}
		<section class="mt-6">
			<div class="mb-3 flex items-center justify-between">
				<h2 class="text-sm font-medium">信用卡还款</h2>
				<a
					href={resolve('/(auth)/cards')}
					class="flex items-center text-xs text-primary hover:underline"
				>
					查看全部
					<ChevronRight size={14} />
				</a>
			</div>
			<div class="space-y-2">
				{#each creditItems.slice(0, 3) as item (item.id)}
					<Card class={item.overdue ? 'border-destructive/50' : ''}>
						<CardContent class="flex items-center gap-3 px-4 py-3">
							<AccountIcon bankName={item.bank_name} size={40} />
							<div class="min-w-0 flex-1">
								<div class="truncate text-sm font-medium">
									{item.bank_name} · •••• {item.card_no4}
								</div>
								<div class="text-xs text-muted-foreground">
									应还 {formatMoney(item.bill_amount)} · 还剩 {item.days_left} 天
								</div>
							</div>
							<div
								class={item.overdue
									? 'text-xs font-medium text-destructive'
									: 'text-xs text-muted-foreground'}
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
		<div class="mb-3 flex items-center justify-between">
			<h2 class="text-sm font-medium">最近交易</h2>
			<a
				href={resolve('/(auth)/transactions')}
				class="flex items-center text-xs text-primary hover:underline"
			>
				查看全部
				<ChevronRight size={14} />
			</a>
		</div>
		<Card>
			<CardContent class="divide-y p-0">
				{#if recentTxs.length === 0}
					<div class="py-12 text-center text-muted-foreground">
						<Receipt size={32} class="mx-auto mb-3 opacity-50" />
						<p class="text-sm">暂无交易记录</p>
						<Button
							class="mt-4"
							size="sm"
							onclick={() => goto(resolve('/(auth)/transactions/add'))}
						>
							<Plus size={16} />
							记一笔
						</Button>
					</div>
				{:else}
					{#each recentTxs as tx (tx.id)}
						<!-- 分类只查一次并收进 {@const}：原先在一行里连写三次
						        categoryOf(tx)，其中两次的 `.color` 直接访问会被 TS 判
						        「Object is possibly 'undefined'」 -->
						{@const cat = categoryOf(tx)}
						<button
							type="button"
							class="flex w-full cursor-pointer items-center gap-3 px-3 py-2.5 text-left transition hover:bg-accent/50"
							onclick={() => goto(resolve('/(auth)/transactions/edit/[id]', { id: String(tx.id) }))}
						>
							<!-- C19：class 属性为空（重构残留），图标没有底色 -->
							<div
								class="grid h-9 w-9 place-items-center rounded-lg bg-muted text-sm font-semibold"
								style={cat?.color ? `background:${cat.color}22;color:${cat.color}` : ''}
							>
								{cat?.icon || tx.description?.[0] || tx.merchant?.[0] || '¥'}
							</div>
							<div class="min-w-0 flex-1">
								<div class="truncate text-sm font-medium">
									{tx.description || tx.merchant || cat?.name || '未分类'}
								</div>
								<!-- C20：副标题补上「分类 · 账户」，与分类管理/账户资产的数据资产对齐 -->
								<div class="truncate text-xs text-muted-foreground">
									{cat?.name || '未分类'} · {accountOf(tx)?.name || '—'} · {formatRelativeDate(
										tx.tx_date
									)}
								</div>
							</div>
							<div class="text-sm font-semibold tabular-nums">
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
