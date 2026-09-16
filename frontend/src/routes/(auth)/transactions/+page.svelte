<script lang="ts">
	import { goto } from '$app/navigation';
	import { get } from 'svelte/store';
	import { createWindowVirtualizer } from '@tanstack/svelte-virtual';
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
	import {
		amountDisplay,
		baseAmount,
		highlightSegments,
		recomputeDaySubtotal,
		toneClass,
		typeLabel
	} from '$lib/utils/tx';
	import type { Category, DayGroup, Transaction, TransactionListData } from '$lib/types';
	import { Plus, Search, Filter, X, Trash2, CheckSquare, Loader2, Copy } from '@lucide/svelte';

	type TabType = 'all' | 'expense' | 'income' | 'transfer';
	let type = $state<TabType>('all');
	let grouped = $state<DayGroup[]>([]);
	let summary = $state<{ total_income: number; total_expense: number; net: number }>({
		total_income: 0,
		total_expense: 0,
		net: 0
	});
	let loading = $state(true);

	// A1：分页状态。改为「游标分页」—— 以本批最后一条的 (tx_date, id) 作为下一次的起点。
	// offset 分页在两页之间发生数据插入时会整页错位，前端去重后 flatCount 不再增长，
	// 「加载更多」按钮永远可点却永远加载不到新数据（原 F-04）。
	const pageSize = 30;
	let total = $state(0);
	let cursorId = $state(0);
	let cursorDate = $state('');
	let loadingMore = $state(false);
	// 兜底：本批追加后一条新数据都没带来，说明已经到底（或游标无法前进），停止再拉
	let endReached = $state(false);

	const flatCount = $derived(
		grouped.reduce((n, g) => n + (g.transactions?.length ?? 0), 0)
	);
	const hasMore = $derived(!endReached && grouped.length > 0 && flatCount < total);
	const lastTx = $derived.by(() => {
		for (let i = grouped.length - 1; i >= 0; i--) {
			const txs = grouped[i]?.transactions ?? [];
			if (txs.length) return txs[txs.length - 1];
		}
		return null;
	});

	// Filter state
	let showFilters = $state(false);
	let keyword = $state('');
	let startDate = $state('');
	let endDate = $state('');
	let categoryId = $state<number | ''>('');
	let accountId = $state<number | ''>('');
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

	const hasFilters = $derived(
		!!(
			keyword ||
			startDate ||
			endDate ||
			categoryId !== '' ||
			accountId !== '' ||
			minAmount ||
			maxAmount ||
			tagId !== '' ||
			reimburseStatus ||
			type !== 'all'
		)
	);

	// F-07：分类筛选下拉必须跟随当前 tab。
	// 此前无论切到哪个 tab 都只列支出分类，切到「收入」后筛选结果必为空集，
	// 用户会误判为「没有收入记录」。
	const categoryOptions = $derived.by(() => {
		const cats = appStore.categories;
		if (type === 'income') return cats.income ?? [];
		if (type === 'expense') return cats.expense ?? [];
		return [...(cats.expense ?? []), ...(cats.income ?? [])];
	});

	function buildParams(append: boolean): any {
		const params: any = { book_id: appStore.currentBookId || 0, page_size: pageSize };
		if (append && cursorId > 0 && cursorDate) {
			params.cursor_id = cursorId;
			params.cursor_date = cursorDate;
		}
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

	// mergeGroups：把新一批按日分组并入已有分组（同日合并 + 按 id 去重）。
	// 小计一律由「去重后的条目」重算，绝不累加服务端返回的小计 —— 否则
	// 跨页重复条目会被重复计入，日小计与明细对不上（原 F-04 场景 2）。
	function mergeGroups(existing: DayGroup[], incoming: DayGroup[]): DayGroup[] {
		const map = new Map<string, DayGroup>();
		const seen = new Set<number>();
		for (const g of existing) {
			map.set(g.date, { ...g, transactions: [...(g.transactions ?? [])] });
			for (const t of g.transactions ?? []) seen.add(t.id);
		}
		for (const g of incoming) {
			const found = map.get(g.date);
			if (found) {
				for (const t of g.transactions ?? []) {
					if (seen.has(t.id)) continue;
					seen.add(t.id);
					found.transactions.push(t);
				}
				recomputeDaySubtotal(found);
			} else {
				const copy: DayGroup = { ...g, transactions: [...(g.transactions ?? [])] };
				for (const t of copy.transactions) seen.add(t.id);
				map.set(g.date, copy);
			}
		}
		return [...map.values()].sort((a, b) => (a.date < b.date ? 1 : -1));
	}

	// F-03：请求序号 + 主动取消在途请求。
	// 只 clearTimeout 无法处理「已发出但未返回」的请求 —— 网络抖动会让旧响应
	// 后到并覆盖新条件的结果，界面停在旧数据上且不会自动纠正。
	let reqSeq = 0;
	let inflight: AbortController | null = null;

	async function loadData(append = false) {
		inflight?.abort();
		const ctrl = new AbortController();
		inflight = ctrl;
		const seq = ++reqSeq;

		if (append) loadingMore = true;
		else loading = true;
		try {
			const data = (await txApi.list(buildParams(append), ctrl.signal)) as TransactionListData;
			if (seq !== reqSeq) return; // 已发出更新的请求，丢弃本次结果
			const incoming = data.grouped ?? [];
			const before = flatCount;
			grouped = append ? mergeGroups(grouped, incoming) : incoming;
			// 汇总只在首页返回（后端为避免每翻一页重复全表聚合而裁剪，原 P-01）
			if (!append) {
				summary = data.summary ?? { total_income: 0, total_expense: 0, net: 0 };
				endReached = false;
			} else if (flatCount === before) {
				endReached = true;
			}
			total = data.pagination?.total ?? 0;
			const last = lastTx;
			cursorId = last?.id ?? 0;
			cursorDate = last ? String(last.tx_date ?? '').slice(0, 10) : '';
		} catch (e: any) {
			// -2 = 被主动取消，静默忽略；其余错误保留日志
			if (e?.code !== -2 && e?.name !== 'AbortError') console.warn(e);
		} finally {
			if (seq === reqSeq) {
				loading = false;
				loadingMore = false;
			}
		}
	}

	async function loadMore() {
		if (loadingMore || !hasMore) return;
		await loadData(true);
	}

	function resetAndLoad() {
		cursorId = 0;
		cursorDate = '';
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

	// F-09：删除后从本地剔除，不再强制回到第一页。
	// 此前调用 resetAndLoad() 会把第 8 页的用户打回顶部并重新加载全部数据。
	function removeLocal(ids: number[]) {
		const drop = new Set(ids);
		grouped = grouped
			.map((g) => ({
				...g,
				transactions: (g.transactions ?? []).filter((t) => !drop.has(t.id))
			}))
			.filter((g) => (g.transactions?.length ?? 0) > 0);
		total = Math.max(0, total - ids.length);
	}

	async function handleDelete(tx: Transaction) {
		if (!confirm(`确定删除「${tx.description || tx.merchant || '该笔'}」？删除后可在回收站恢复。`))
			return;
		try {
			await txApi.remove(tx.id);
			hzToastSuccess('已删除');
			previewOpen = false;
			removeLocal([tx.id]);
		} catch (e: any) {
			hzToastError(e.message || '删除失败');
		}
	}

	function handleCopy(tx: Transaction) {
		const params = new URLSearchParams();
		params.set('type', tx.type);
		params.set('amount', String(tx.amount));
		if (tx.description) params.set('description', tx.description);
		params.set('category_id', String(tx.category_id));
		params.set('account_id', String(tx.account_id));
		if (tx.to_account_id) params.set('to_account_id', String(tx.to_account_id));
		if (tx.merchant) params.set('merchant', tx.merchant);
		if (tx.location) params.set('location', tx.location);
		if (tx.remark) params.set('remark', tx.remark);
		if (tx.images?.length) params.set('images', JSON.stringify(tx.images));
		params.set('include_in_budget', String(tx.include_in_budget));
		if (tx.currency) params.set('currency', tx.currency);
		if ((tx as any).exchange_rate) params.set('exchange_rate', String((tx as any).exchange_rate));
		previewOpen = false;
		goto(`/transactions/add?${params.toString()}`);
	}

	async function handleBatchDelete() {
		if (selectedIds.length === 0) return;
		if (!confirm(`确定删除选中的 ${selectedIds.length} 笔交易？`)) return;
		try {
			const res: any = await txApi.batchRemove(selectedIds);
			hzToastSuccess(`已删除 ${res?.deleted_count ?? selectedIds.length} 笔`);
			removeLocal(selectedIds);
			selectMode = false;
			selectedIds = [];
		} catch (e: any) {
			hzToastError(e.message || '批量删除失败');
		}
	}

	function hzToastSuccess(m: string) {
		hzToast.success(m);
	}
	function hzToastError(m: string) {
		hzToast.error(m);
	}

	// A：立即生效的筛选（Tabs / 下拉 / 账本切换 / 服务端变更通知）—— 不走防抖，
	// 手感不再迟滞；依赖合并成一个 effect，挂载时只发一次请求。
	$effect(() => {
		void appStore.currentBookId;
		void appStore.listVersion; // P-04：WS 收到变更通知后刷新列表
		void type;
		void categoryId;
		void accountId;
		void tagId;
		void reimburseStatus;
		resetAndLoad();
	});

	// B：文本框类输入走 300ms 防抖（跳过首次挂载，避免与 effect A 重复请求）
	let debounceFirst = true;
	$effect(() => {
		void keyword;
		void startDate;
		void endDate;
		void minAmount;
		void maxAmount;
		if (debounceFirst) {
			debounceFirst = false;
			return;
		}
		const timer = setTimeout(() => {
			resetAndLoad();
		}, 300);
		return () => clearTimeout(timer);
	});

	function getCategory(tx: Transaction): Category | undefined {
		return [...appStore.categories.expense, ...appStore.categories.income].find(
			(c) => c.id === tx.category_id
		);
	}

	function getCategoryName(tx: Transaction): string {
		return tx.category_name || getCategory(tx)?.name || '未分类';
	}

	function getAccountName(tx: Transaction): string {
		return (
			tx.account_name || appStore.accounts.find((a) => a.id === tx.account_id)?.name || '—'
		);
	}

	function getAccountDisplay(tx: Transaction): string {
		if (tx.type === 'transfer' && tx.to_account_id) {
			const to =
				tx.to_account_name ||
				appStore.accounts.find((a) => a.id === tx.to_account_id)?.name ||
				'—';
			return `${getAccountName(tx)} → ${to}`;
		}
		return getAccountName(tx);
	}

	// ========== 虚拟滚动（原 P-05） ==========
	// 列表按需加载后 DOM 会线性膨胀（300 条 ≈ 4500 节点），滚动明显掉帧。
	// 这里以「一天」为虚拟项单位，离屏的日期分组完全不进入 DOM；
	// 组内条目通常很少，保留原有卡片结构，避免为虚拟化重写整套布局。
	const virtualizer = createWindowVirtualizer({
		count: 0,
		estimateSize: () => 240,
		overscan: 4
	});

	// 用 get() 读取实例而不是 $virtualizer：后者会让本 effect 依赖 store，
	// 而 setOptions 本身会写 store → 形成自触发循环。
	$effect(() => {
		const v = get(virtualizer);
		v?.setOptions({ count: grouped.length });
	});

	const totalHeight = $derived($virtualizer ? $virtualizer.getTotalSize() : 0);

	/** 让虚拟器测量真实高度（含 emoji / 长文本导致的换行） */
	function measureRow(node: HTMLElement, v: any) {
		v?.measureElement(node);
		return {
			update(next: any) {
				next?.measureElement(node);
			}
		};
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
				<div
					class="mt-1 font-bold tabular-nums"
					class:text-primary={summary.net >= 0}
					class:text-[var(--color-expense)]={summary.net < 0}
				>
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
		<div class="relative flex-1 min-w-[140px]">
			<Search size={14} class="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground" />
			<Input class="pl-9 h-8" placeholder="搜索..." bind:value={keyword} />
		</div>
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
				<Button
					size="sm"
					variant="destructive"
					onclick={handleBatchDelete}
					disabled={selectedIds.length === 0}
				>
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
							{#each categoryOptions as cat (cat.id)}
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
							{#each appStore.accounts as acc (acc.id)}
								<option value={acc.id}>{acc.name}</option>
							{/each}
						</select>
					</div>
				</div>

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
							{#each appStore.tags as tag (tag.id)}
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
		<!-- F-08：区分「从未记过账」与「筛选无结果」 -->
		<Card>
			<div class="py-16 text-center" data-testid="tx-empty">
				<div class="text-4xl mb-4 opacity-50">{hasFilters ? '🔍' : '📋'}</div>
				<p class="text-muted-foreground mb-4">
					{hasFilters ? '没有符合筛选条件的记录' : '还没有交易记录'}
				</p>
				{#if hasFilters}
					<Button variant="outline" onclick={clearFilters}>
						<X size={16} />
						清除筛选条件
					</Button>
				{:else}
					<Button onclick={() => goto('/transactions/add')}>
						<Plus size={16} />
						记一笔
					</Button>
				{/if}
			</div>
		</Card>
	{:else}
		<div class="space-y-4" data-testid="tx-list">
			<div class="relative w-full" style="height: {totalHeight}px;">
				{#each $virtualizer.getVirtualItems() as row (row.key)}
					{@const dayGroup = grouped[row.index]}
					{#if dayGroup}
						<div
							class="absolute left-0 top-0 w-full"
							style="transform: translateY({row.start}px);"
							data-index={row.index}
							use:measureRow={$virtualizer}
						>
							<div class="pb-4">
								<div class="flex items-center justify-between mb-2 px-1">
									<span class="text-xs text-muted-foreground">
										{formatRelativeDate(dayGroup.date)}
									</span>
									<div class="text-xs text-muted-foreground tabular-nums">
										<span class="text-[var(--color-income)]"
											>+{formatMoney(dayGroup.day_income).replace('¥', '')}</span
										>
										<span class="mx-1">/</span>
										<span class="text-[var(--color-expense)]"
											>-{formatMoney(dayGroup.day_expense).replace('¥', '')}</span
										>
									</div>
								</div>
								<Card class="divide-y">
									{#each dayGroup.transactions as tx (tx.id)}
										{@const disp = amountDisplay(tx)}
										{@const category = getCategory(tx)}
										{@const note = tx.description || tx.merchant || tx.remark}
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
												<div
													class="w-2 h-2 rounded-full shrink-0"
													class:bg-muted-foreground={!category?.color}
													style={category?.color ? `background-color: ${category.color}` : ''}
												></div>
												<div class="flex-1 min-w-0">
													<div class="text-sm font-medium truncate">
														{#each highlightSegments(getCategoryName(tx), keyword) as seg}
															{#if seg.hit}<mark
																	class="bg-yellow-200 dark:bg-yellow-800 rounded px-0.5"
																	>{seg.text}</mark
																>{:else}{seg.text}{/if}
														{/each}
													</div>
													{#if note}
														<div class="text-xs text-muted-foreground truncate">
															{#each highlightSegments(note, keyword) as seg}
																{#if seg.hit}<mark
																		class="bg-yellow-200 dark:bg-yellow-800 rounded px-0.5"
																		>{seg.text}</mark
																	>{:else}{seg.text}{/if}
															{/each}
														</div>
													{/if}
												</div>
												<!-- F-02：与日小计、详情弹窗共用同一套符号/颜色口径 -->
												<div class="text-right shrink-0 ml-2">
													<div class="font-semibold tabular-nums text-sm {toneClass(disp.tone)}">
														{disp.sign}{formatMoney(disp.abs)}
													</div>
													<div class="text-xs text-muted-foreground truncate max-w-[120px]">
														{#each highlightSegments(getAccountDisplay(tx), keyword) as seg}
															{#if seg.hit}<mark
																	class="bg-yellow-200 dark:bg-yellow-800 rounded px-0.5"
																	>{seg.text}</mark
																>{:else}{seg.text}{/if}
														{/each}
													</div>
												</div>
											</button>
											<!-- B3：单笔删除入口 -->
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
						</div>
					{/if}
				{/each}
			</div>

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
	<Dialog
		bind:open={previewOpen}
		onOpenChange={(o) => {
			// F-10：关闭时清空数据，避免弹窗实例与 window 监听长期挂着
			if (!o) previewTx = null;
		}}
	>
		<div class="space-y-4">
			<div class="text-center">
				{#if previewTx}
					{@const disp = amountDisplay(previewTx)}
					<div class="text-3xl font-bold tabular-nums {toneClass(disp.tone)}">
						{disp.sign}{formatMoney(disp.abs)}
					</div>
					{#if baseAmount(previewTx) !== previewTx.amount && previewTx.exchange_rate}
						<div class="text-xs text-muted-foreground mt-1 tabular-nums">
							原币 {previewTx.currency}
							{formatMoney(previewTx.amount)} × {previewTx.exchange_rate}
						</div>
					{/if}
				{/if}
				<div class="text-sm text-muted-foreground mt-1">
					{typeLabel(previewTx.type)}
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
				{#if previewTx.to_account_id}
					<div class="flex justify-between">
						<span class="text-muted-foreground">转入账户</span>
						<span
							>{previewTx.to_account_name ||
								appStore.accounts.find((a) => a.id === previewTx.to_account_id)?.name ||
								'—'}</span
						>
					</div>
				{/if}
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
				<Button
					variant="outline"
					onclick={() => previewTx && handleCopy(previewTx)}
					title="复制账单"
				>
					<Copy size={16} />
				</Button>
				<!-- B3：预览弹窗内直接删除 -->
				<Button
					variant="outline"
					onclick={() => previewTx && handleDelete(previewTx)}
					title="删除（可在回收站恢复）"
				>
					<Trash2 size={16} class="text-destructive" />
				</Button>
				<Button variant="outline" onclick={() => (previewOpen = false)}>关闭</Button>
			</div>
		</div>
	</Dialog>
{/if}
