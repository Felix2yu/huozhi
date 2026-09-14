<script lang="ts">
	import { goto } from '$app/navigation';
	import Button from '$lib/components/ui/Button.svelte';
	import Card from '$lib/components/ui/Card.svelte';
	import Dialog from '$lib/components/ui/Dialog.svelte';
	import Tabs from '$lib/components/ui/Tabs.svelte';
	import TabsTrigger from '$lib/components/ui/TabsTrigger.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import Label from '$lib/components/ui/Label.svelte';
	import { txApi } from '$lib/api/modules/transactions';
	import { appStore } from '$lib/stores/app';
	import { hzToast } from '$lib/components/ui/toast';
	import { formatMoney, formatRelativeDate } from '$lib/utils/format';
	import type { Transaction, DayGroup, TransactionListData } from '$lib/types';
	import { Plus, Search, Filter, X, Trash2, CheckSquare, Loader2 } from '@lucide/svelte';

	let type = $state<'all' | 'expense' | 'income' | 'transfer'>('all');
	let grouped = $state<DayGroup[]>([]);
	let summary = $state<{ total_income: number; total_expense: number; net: number }>({
		total_income: 0,
		total_expense: 0,
		net: 0
	});
	let loading = $state(true);

	// A1：分页状态 —— 此前从不传 page/page_size，用户最多只能看到 20 条流水
	let page = $state(1);
	const pageSize = 30;
	let total = $state(0);
	let flatCount = $state(0);
	let loadingMore = $state(false);
	const hasMore = $derived(grouped.length > 0 && flatCount < total);

	// Filter state
	let showFilters = $state(false);
	let keyword = $state('');
	let startDate = $state('');
	let endDate = $state('');
	let categoryId = $state<number | ''>('');
	let accountId = $state<number | ''>('');
	// A7：后端已支持但前端未暴露的筛选条件
	let minAmount = $state('');
	let maxAmount = $state('');
	let tagId = $state<number | ''>('');
	let reimburseStatus = $state('');

	// Preview state
	let previewTx = $state<Transaction | null>(null);
	let previewOpen = $state(false);

	// B3：批量删除
	let selectMode = $state(false);
	let selectedIds = $state<number[]>([]);

	function buildParams(targetPage: number): any {
		const params: any = { book_id: appStore.currentBookId || 0, page: targetPage, page_size: pageSize };
		if (type !== 'all') params.type = type;
		if (keyword.trim()) params.keyword = keyword.trim();
		if (startDate) params.start_date = startDate;
		if (endDate) params.end_date = endDate;
		if (categoryId !== '') params.category_id = categoryId;
		if (accountId !== '') params.account_id = accountId;
		if (minAmount) params.min_amount = Number(minAmount);
		if (maxAmount) params.max_amount = Number(maxAmount);
		if (tagId !== '') params.tag_id = tagId;
		if (reimburseStatus) params.reimburse_status = reimburseStatus;
		return params;
	}

	// mergeGroups：把新一页的按日分组并入已有分组（同日合并）
	function mergeGroups(existing: DayGroup[], incoming: DayGroup[]): DayGroup[] {
		const map = new Map<string, DayGroup>();
		for (const g of existing) map.set(g.date, { ...g, transactions: [...g.transactions] });
		for (const g of incoming) {
			const found = map.get(g.date);
			if (found) {
				const seen = new Set(found.transactions.map((t) => t.id));
				found.transactions.push(...g.transactions.filter((t) => !seen.has(t.id)));
				found.day_income += g.day_income;
				found.day_expense += g.day_expense;
				found.day_balance = found.day_income - found.day_expense;
			} else {
				map.set(g.date, { ...g, transactions: [...g.transactions] });
			}
		}
		return [...map.values()].sort((a, b) => (a.date < b.date ? 1 : -1));
	}

	async function loadData(append = false) {
		if (append) loadingMore = true;
		else loading = true;
		try {
			const data = (await txApi.list(buildParams(page))) as TransactionListData;
			const incoming = data.grouped || [];
			grouped = append ? mergeGroups(grouped, incoming) : incoming;
			summary = data.summary || { total_income: 0, total_expense: 0, net: 0 };
			total = data.pagination?.total ?? 0;
			flatCount = grouped.reduce((n, g) => n + g.transactions.length, 0);
		} catch (e) {
			console.warn(e);
		} finally {
			loading = false;
			loadingMore = false;
		}
	}

	async function loadMore() {
		if (loadingMore || !hasMore) return;
		page += 1;
		await loadData(true);
	}

	function resetAndLoad() {
		page = 1;
		selectedIds = [];
		loadData(false);
	}

	function clearFilters() {
		keyword = '';
		startDate = '';
		endDate = '';
		categoryId = '';
		accountId = '';
		minAmount = '';
		maxAmount = '';
		tagId = '';
		reimburseStatus = '';
		resetAndLoad();
	}

	function showPreview(tx: Transaction) {
		if (selectMode) {
			toggleSelect(tx.id);
			return;
		}
		previewTx = tx;
		previewOpen = true;
	}

	function toggleSelect(id: number) {
		selectedIds = selectedIds.includes(id)
			? selectedIds.filter((i) => i !== id)
			: [...selectedIds, id];
	}

	async function handleDelete(tx: Transaction) {
		if (!confirm(`确定删除「${tx.description || tx.merchant || '该笔'}」？删除后可在回收站恢复。`)) return;
		try {
			await txApi.remove(tx.id);
			hzToastSuccess('已删除');
			previewOpen = false;
			resetAndLoad();
		} catch (e: any) {
			hzToastError(e.message || '删除失败');
		}
	}

	async function handleBatchDelete() {
		if (selectedIds.length === 0) return;
		if (!confirm(`确定删除选中的 ${selectedIds.length} 笔交易？`)) return;
		try {
			const res: any = await txApi.batchRemove(selectedIds);
			hzToastSuccess(`已删除 ${res?.deleted_count ?? selectedIds.length} 笔`);
			selectMode = false;
			resetAndLoad();
		} catch (e: any) {
			hzToastError(e.message || '批量删除失败');
		}
	}

	function hzToastSuccess(m: string) { hzToast.success(m); }
	function hzToastError(m: string) { hzToast.error(m); }

	const hasFilters = $derived(
		keyword ||
			startDate ||
			endDate ||
			categoryId !== '' ||
			accountId !== '' ||
			minAmount ||
			maxAmount ||
			tagId !== '' ||
			reimburseStatus
	);

	// 唯一的取数入口：账本切换（含切到「全部账本」=0）与任一筛选条件变化都会重新拉取，
	// 关键词输入走 300ms 防抖。合并为一个 effect，避免挂载时重复请求。
	$effect(() => {
		void appStore.currentBookId;
		void type;
		void keyword;
		void startDate;
		void endDate;
		void categoryId;
		void accountId;
		void minAmount;
		void maxAmount;
		void tagId;
		void reimburseStatus;

		const timer = setTimeout(() => {
			page = 1;
			loadData(false);
		}, 300);
		return () => clearTimeout(timer);
	});

	function getCategoryName(tx: Transaction): string {
		return (
			tx.category_name ||
			[...appStore.categories.expense, ...appStore.categories.income].find(
				(c) => c.id === tx.category_id
			)?.name ||
			'未分类'
		);
	}

	function getAccountName(tx: Transaction): string {
		return (
			tx.account_name ||
			appStore.accounts.find((a) => a.id === tx.account_id)?.name ||
			'—'
		);
	}

	function getTypeLabel(t: string): string {
		switch (t) {
			case 'income': return '收入';
			case 'expense': return '支出';
			case 'transfer': return '转账';
			case 'refund': return '退款';
			case 'reimburse': return '报销';
			case 'adjust': return '余额调整';
			default: return t;
		}
	}
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
	<div class="flex items-center gap-2 flex-wrap">
		<Tabs bind:value={type}>
			<TabsTrigger value="all">全部</TabsTrigger>
			<TabsTrigger value="expense">支出</TabsTrigger>
			<TabsTrigger value="income">收入</TabsTrigger>
			<TabsTrigger value="transfer">转账</TabsTrigger>
		</Tabs>
		<div class="flex-1"></div>
		<!-- B3：批量删除入口 -->
		<Button
			size="sm"
			variant={selectMode ? 'default' : 'outline'}
			onclick={() => {
				selectMode = !selectMode;
				selectedIds = [];
			}}
		>
			<CheckSquare size={14} />
			{selectMode ? '退出多选' : '多选'}
		</Button>
		<Button
			size="sm"
			variant={showFilters ? 'default' : 'outline'}
			onclick={() => (showFilters = !showFilters)}
		>
			<Filter size={14} />
			筛选
			{#if hasFilters}
				<span class="w-1.5 h-1.5 rounded-full bg-primary"></span>
			{/if}
		</Button>
	</div>

	{#if selectMode}
		<Card>
			<div class="p-3 flex items-center gap-3 text-sm">
				<span class="text-muted-foreground">已选 {selectedIds.length} 笔</span>
				<div class="flex-1"></div>
				<Button size="sm" variant="destructive" onclick={handleBatchDelete} disabled={selectedIds.length === 0}>
					<Trash2 size={14} />
					删除所选
				</Button>
			</div>
		</Card>
	{/if}

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

				<!-- A7：后端已实现但前端未暴露 -->
				<div class="grid grid-cols-2 gap-4">
					<div class="space-y-2">
						<Label>最小金额</Label>
						<Input type="number" step="0.01" placeholder="0.00" bind:value={minAmount} />
					</div>
					<div class="space-y-2">
						<Label>最大金额</Label>
						<Input type="number" step="0.01" placeholder="0.00" bind:value={maxAmount} />
					</div>
				</div>

				<div class="grid grid-cols-2 gap-4">
					<div class="space-y-2">
						<Label>标签</Label>
						<select
							class="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm focus:outline-none focus:ring-1 focus:ring-ring"
							bind:value={tagId}
						>
							<option value="">全部标签</option>
							{#each appStore.tags as tag}
								<option value={tag.id}>{tag.name}</option>
							{/each}
						</select>
					</div>
					<div class="space-y-2">
						<Label>报销状态</Label>
						<select
							class="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm focus:outline-none focus:ring-1 focus:ring-ring"
							bind:value={reimburseStatus}
						>
							<option value="">不限</option>
							<option value="none">未报销</option>
							<option value="pending">报销中</option>
							<option value="done">已完成</option>
						</select>
					</div>
				</div>

				<div class="flex gap-2 pt-2">
					<Button size="sm" onclick={resetAndLoad}>应用筛选</Button>
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
		<div class="space-y-2" data-testid="tx-skeleton">
			{#each [1, 2, 3, 4] as i}
				<div class="h-16 rounded-lg animate-pulse bg-muted"></div>
			{/each}
		</div>
	{:else if grouped.length === 0}
		<Card>
			<div class="py-16 text-center" data-testid="tx-empty">
				<div class="text-4xl mb-4 opacity-50">📋</div>
				<p class="text-muted-foreground mb-4">还没有交易记录</p>
				<Button onclick={() => goto('/transactions/add')}>
					<Plus size={16} />
					记一笔
				</Button>
			</div>
		</Card>
	{:else}
		<div class="space-y-4" data-testid="tx-list">
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
							<div class="w-full flex items-center gap-3 p-3 hover:bg-accent/50 transition">
								{#if selectMode}
									<input
										type="checkbox"
										class="rounded border-input"
										checked={selectedIds.includes(tx.id)}
										onchange={() => toggleSelect(tx.id)}
									/>
								{/if}
								<button
									class="flex items-center gap-3 flex-1 min-w-0 text-left"
									onclick={() => showPreview(tx)}
								>
									<div class="w-10 h-10 rounded-lg bg-muted grid place-items-center text-sm font-semibold">
										{(tx.description || tx.merchant || '¥')[0]}
									</div>
									<div class="flex-1 min-w-0">
										<div class="text-sm font-medium truncate">
											{tx.description || tx.merchant || '未分类'}
										</div>
										<!-- C20：副标题改为「分类 · 账户」，与分类管理/账户资产的数据资产对齐 -->
										<div class="text-xs text-muted-foreground truncate">
											{getCategoryName(tx)} · {getAccountName(tx)}
											{#if tx.merchant}· {tx.merchant}{/if}
										</div>
									</div>
									<div class="font-semibold tabular-nums text-sm">
										{tx.type === 'income' ? '+' : tx.type === 'expense' ? '-' : ''}
										{formatMoney(tx.amount)}
									</div>
								</button>
								<!-- B3：单笔删除入口（后端 API 早已实现，前端此前零调用） -->
								{#if !selectMode}
									<button
										class="p-1.5 rounded hover:bg-destructive/10 text-muted-foreground hover:text-destructive"
										onclick={() => handleDelete(tx)}
										title="删除"
									>
										<Trash2 size={15} />
									</button>
								{/if}
							</div>
						{/each}
					</Card>
				</div>
			{/each}

			<!-- A1：分页 -->
			<div class="text-center py-2">
				{#if hasMore}
					<Button variant="outline" onclick={loadMore} disabled={loadingMore}>
						{#if loadingMore}
							<Loader2 size={14} class="animate-spin" />
							加载中…
						{:else}
							加载更多（已显示 {flatCount} / 共 {total} 笔）
						{/if}
					</Button>
				{:else}
					<p class="text-xs text-muted-foreground">共 {total} 笔，已全部显示</p>
				{/if}
			</div>
		</div>
	{/if}
</div>

<!-- 预览弹窗 -->
{#if previewTx}
	<Dialog bind:open={previewOpen}>
		<div class="space-y-4">
			<div class="text-center">
				<div class="text-3xl font-bold tabular-nums {previewTx.type === 'income' ? 'text-[var(--color-income)]' : previewTx.type === 'expense' ? 'text-[var(--color-expense)]' : ''}">
					{previewTx.type === 'income' ? '+' : previewTx.type === 'expense' ? '-' : ''}
					{formatMoney(previewTx.amount)}
				</div>
				<div class="text-sm text-muted-foreground mt-1">
					{getTypeLabel(previewTx.type)}
				</div>
			</div>

			<div class="space-y-3 text-sm">
				<div class="flex justify-between">
					<span class="text-muted-foreground">描述</span>
					<span>{previewTx.description || previewTx.merchant || '—'}</span>
				</div>
				{#if previewTx.merchant}
					<div class="flex justify-between">
						<span class="text-muted-foreground">商户</span>
						<span>{previewTx.merchant}</span>
					</div>
				{/if}
				<div class="flex justify-between">
					<span class="text-muted-foreground">日期</span>
					<span>{previewTx.tx_date?.split('T')[0] || previewTx.tx_date}</span>
				</div>
				<div class="flex justify-between">
					<span class="text-muted-foreground">分类</span>
					<span>{getCategoryName(previewTx)}</span>
				</div>
				<div class="flex justify-between">
					<span class="text-muted-foreground">账户</span>
					<span>{getAccountName(previewTx)}</span>
				</div>
				{#if (previewTx.tags || []).length}
					<div class="flex justify-between">
						<span class="text-muted-foreground">标签</span>
						<span>{previewTx.tags!.map((t: any) => t.name).join('、')}</span>
					</div>
				{/if}
				{#if (previewTx.images || []).length}
					<div class="flex gap-2 flex-wrap">
						{#each previewTx.images as url}
							<img src={url} alt="凭证" class="w-16 h-16 object-cover rounded border" />
						{/each}
					</div>
				{/if}
				{#if previewTx.location}
					<div class="flex justify-between">
						<span class="text-muted-foreground">地点</span>
						<span>{previewTx.location}</span>
					</div>
				{/if}
				{#if previewTx.remark}
					<div class="flex justify-between">
						<span class="text-muted-foreground">备注</span>
						<span>{previewTx.remark}</span>
					</div>
				{/if}
			</div>

			<div class="flex gap-2 pt-2">
				<Button class="flex-1" onclick={() => goto(`/transactions/edit/${previewTx.id}`)}>
					编辑
				</Button>
				<!-- B3：预览弹窗内直接删除 -->
				<Button
					variant="outline"
					onclick={() => previewTx && handleDelete(previewTx)}
					title="删除（可在回收站恢复）"
				>
					<Trash2 size={16} class="text-destructive" />
				</Button>
				<Button variant="outline" onclick={() => { previewOpen = false; }}>关闭</Button>
			</div>
		</div>
	</Dialog>
{/if}
