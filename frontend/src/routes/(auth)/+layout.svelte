<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { cn } from '$lib/utils/cn';
	import Button from '$lib/components/ui/Button.svelte';
	import Sheet from '$lib/components/ui/Sheet.svelte';
	import Badge from '$lib/components/ui/Badge.svelte';
	import { appStore } from '$lib/stores/app';
	import { hzToast } from '$lib/components/ui/toast';
	import { http, replayQueue, subscribeQueue, queueCount } from '$lib/api/http';
	import { accountApi, txApi } from '$lib/api/modules';
	import { getMonthRange, formatMoney } from '$lib/utils/format';
	import { connectWs, disconnectWs, onSync } from '$lib/ws';
	import type { WsMessage } from '$lib/ws';
	import {
		LayoutDashboard,
		Receipt,
		Wallet,
		CreditCard,
		PieChart,
		Target,
		BarChart3,
		Tags,
		TrendingUp,
		Repeat,
		FileText,
		Users,
		Settings,
		Plus,
		LogOut,
		BookMarked,
		Menu,
		Home,
		CloudOff,
		ChevronDown,
		Check,
		MinusCircle,
		Database
	} from '@lucide/svelte';

	let { children } = $props();

	// ========= 认证检查 =========
	let checking = $state(true);

	async function init() {
		try {
			const ok = await appStore.checkAuth();
			if (!ok) {
				const path = $page?.url?.pathname || '/dashboard';
				goto(`/login?redirect=${encodeURIComponent(path)}`);
				return;
			}
			await appStore.loadBooks();
			await appStore.loadDictionaries();
		} catch (e) {
			console.warn('初始化失败', e);
			// 认证失败（token 失效 / 接口异常）时跳登录，避免卡在加载屏
			if (!appStore.isAuth) goto('/login');
		} finally {
			// 关键：无论成功 / 失败 / 未登录，都必须结束加载态，
			// 否则 checking 永远为 true，整页停留在「正在加载货殖...」且点击无反应
			checking = false;
		}

		if (!appStore.isAuth) return;

		// 尝试重放离线队列
		if (queueCount() > 0) {
			replayQueue().then((r) => {
				if (r.ok > 0) hzToast.success(`已同步 ${r.ok} 条离线请求`);
			});
		}

		// 连接 WebSocket 实时同步
		connectWs();
		unsubSync = onSync(handleSync);
	}

	let unsubSync: (() => void) | null = null;

	/**
	 * WS 变更通知的合流器（原 P-04）。
	 * 10 种表此前共用一条分支、每次都发 3 个请求：批量导入 500 条交易会产生
	 * 500 次 Broadcast → 最多 1500 次请求。改为按表分流 + 300ms 尾部防抖，
	 * 同表的一批变更只触发一次取数。
	 */
	const syncTimers = new Map<string, ReturnType<typeof setTimeout>>();
	function scheduleSync(key: string, fn: () => void, delay = 300) {
		const prev = syncTimers.get(key);
		if (prev) clearTimeout(prev);
		syncTimers.set(
			key,
			setTimeout(() => {
				syncTimers.delete(key);
				fn();
			}, delay)
		);
	}
	function clearSyncTimers() {
		for (const t of syncTimers.values()) clearTimeout(t);
		syncTimers.clear();
	}

	function handleSync(msg: WsMessage) {
		if (msg.type === 'sync' && msg.table) {
			// 只刷新与该表真正相关的资源，避免无关请求
			switch (msg.table) {
				case 'transactions':
					// 流水列表此前完全不响应 WS：周期记账自动入账、移动端记一笔之后
					// PC 端列表不会更新。bumpListVersion 早就存在却从未被调用。
					scheduleSync('transactions', () => {
						appStore.bumpListVersion();
						appStore.loadDictionaries();
					});
					break;
				case 'accounts':
				case 'categories':
				case 'tags':
					scheduleSync(msg.table, () => appStore.loadDictionaries());
					break;
				case 'books':
					scheduleSync('books', () => appStore.loadBooks());
					break;
				default:
					// budgets / recurring / installments / reimbursements / saving_plans
					// 由各自页面按需取数，全局布局不做无差别重刷
					break;
			}
		} else if (msg.type === 'alert') {
			hzToast.info(msg.data?.message || '有新通知');
		}
	}

	onMount(init);

	onDestroy(() => {
		disconnectWs();
		unsubSync?.();
		clearSyncTimers();
	});

	// ========= 移动端侧边栏 =========
	let sidebarOpen = $state(false);

	function toggleSidebar() {
		sidebarOpen = !sidebarOpen;
	}

	// 路由变化后自动收起
	$effect(() => {
		if ($page?.url) {
			sidebarOpen = false;
		}
	});

	// ========= 导航项（A4：按信息架构分组，取代 14 项平铺） =========
	const navGroups = [
		{
			title: '记账',
			items: [
				{ to: '/dashboard', label: '首页总览', icon: LayoutDashboard },
				{ to: '/transactions', label: '账单流水', icon: Receipt },
				{ to: '/categories', label: '分类管理', icon: PieChart }
			]
		},
		{
			title: '资产',
			items: [
				{ to: '/accounts', label: '账户资产', icon: Wallet },
				{ to: '/cards', label: '我的银行卡', icon: CreditCard },
				{ to: '/savings', label: '存钱计划', icon: TrendingUp },
				{ to: '/installments', label: '分期管理', icon: CreditCard }
			]
		},
		{
			title: '规则',
			items: [
				{ to: '/budgets', label: '预算管理', icon: Target },
				{ to: '/recurring', label: '周期记账', icon: Repeat },
				{ to: '/reimbursements', label: '报销管理', icon: FileText },
				{ to: '/tags', label: '标签中心', icon: Tags }
			]
		},
		{
			title: '分析与协作',
			items: [
				{ to: '/statistics', label: '统计分析', icon: BarChart3 },
				{ to: '/bill-export', label: '账单导出', icon: FileText },
				{ to: '/shared-books', label: '共享账本', icon: Users }
			]
		},
	{
		title: '系统',
		items: [
			{ to: '/data', label: '数据管理', icon: Database },
			{ to: '/settings', label: '系统设置', icon: Settings }
		]
	}
	];

	// 兼容旧引用（pageTitle 等处使用）
	const navItems = navGroups.flatMap((g) => g.items);

	// A3：底部原来只覆盖 5/14 功能。改为 4 个高频入口 + 1 个「更多」抽屉，
	// 把使用频次最高的「统计」提上来，其余 9 个功能从抽屉一键直达。
	const mobileTabs = [
		{ to: '/dashboard', label: '首页', icon: Home },
		{ to: '/transactions', label: '账单', icon: Receipt },
		{ to: '/transactions/add', label: '', icon: Plus },
		{ to: '/statistics', label: '统计', icon: BarChart3 },
		{ to: '', label: '更多', icon: Menu, more: true }
	];

	let currentPath = $derived($page?.url?.pathname || '');

	function isActive(to: string): boolean {
		if (to === '/dashboard') return currentPath === '/dashboard';
		return currentPath.startsWith(to);
	}

	// ========= 账本切换 =========
	let bookPickerOpen = $state(false);

	// ========= 迷你统计 =========
	let miniStats = $state<{ in: number; out: number; net: number; asset: number } | null>(null);

	$effect(() => {
		if (checking) return;
		// book_id 传 0 表示「全部账本」，后端会按用户聚合所有账本。
		// 不能因为 bid 为 0 就提前 return，否则切到「全部账本」后迷你统计永远不更新。
		const bid = appStore.currentBookId || 0;
		// 跟随 WS 变更通知刷新（原 P-04：此前顶部迷你统计对交易变更无反应）
		void appStore.listVersion;

		(async () => {
			try {
				const { start, end } = getMonthRange();
				const [txRes, accRes] = await Promise.all([
					txApi.list({
						book_id: bid,
						start_date: start,
						end_date: end,
						page_size: 1
					}),
					accountApi.list({ book_id: bid })
				]);
				const accounts = (accRes as any).accounts || accRes;
				const asset = accounts.reduce(
					(s: number, a: any) => s + (a.include_in_total ? a.balance : 0),
					0
				);
				miniStats = {
					in: (txRes as any).summary?.total_income || 0,
					out: (txRes as any).summary?.total_expense || 0,
					net: (txRes as any).summary?.net || 0,
					asset
				};
			} catch {
				// ignore
			}
		})();
	});

	// ========= 离线队列 =========
	let offlineCount = $state(queueCount());
	$effect(() => {
		const unsub = subscribeQueue(() => {
			offlineCount = queueCount();
		});
		return () => unsub();
	});

	// ========= 页面标题 =========
	const pageTitle = $derived.by(() => {
		if (currentPath === '/transactions/add') return '记一笔';
		if (currentPath.startsWith('/bill-export')) return '账单导出';
		if (currentPath.startsWith('/data')) return '数据管理';
		const found = navItems.find((it) => currentPath.startsWith(it.to));
		return found?.label || '货殖';
	});

	async function handleLogout() {
		try {
			await http.post<void>('/auth/logout');
		} catch {
			// ignore
		}
		appStore.logout();
		goto('/login');
	}
</script>

{#if checking}
	<div class="min-h-screen grid place-items-center text-muted-foreground text-sm">
		<div class="animate-pulse">正在加载货殖...</div>
	</div>
{:else}
	<div class="min-h-screen bg-background text-foreground flex">
		<!-- ========== 桌面侧边栏 ========== -->
		<aside class="hidden md:flex fixed md:sticky md:top-0 inset-y-0 left-0 z-40 w-64 h-screen border-r bg-card flex-col shrink-0">
			<!-- Logo -->
			<div class="px-5 py-4 border-b flex items-center gap-2">
				<div class="w-9 h-9 rounded-xl bg-primary grid place-items-center text-primary-foreground font-bold">
					账
				</div>
				<div class="flex-1 min-w-0">
					<div class="font-semibold leading-tight">货殖</div>
					<div class="text-xs text-muted-foreground">简洁纯粹的记账本</div>
				</div>
			</div>

			<!-- 账本切换器 -->
			<div class="px-3 py-3 border-b">
				<button
					class="w-full flex items-center gap-2.5 rounded-xl border px-2.5 py-2 text-left hover:border-primary/50 hover:bg-accent transition"
					onclick={() => (bookPickerOpen = !bookPickerOpen)}
				>
					<span class="w-8 h-8 rounded-lg bg-muted grid place-items-center text-base shrink-0">
						{appStore.currentBookId === 0
							? '📚'
							: (appStore.books.find((b) => b.id === appStore.currentBookId)?.icon || '📘')}
					</span>
					<span class="flex-1 min-w-0">
						<span class="block text-sm font-medium truncate">
							{appStore.currentBookId === 0
								? '全部账本'
								: (appStore.books.find((b) => b.id === appStore.currentBookId)?.name || '选择账本')}
						</span>
						<span class="block text-[11px] text-muted-foreground truncate">
							{appStore.currentBookId === 0
								? '聚合所有账本收支'
								: `共 ${appStore.books.length} 个账本`}
						</span>
					</span>
					<ChevronDown
						size={15}
						class={cn('shrink-0 text-muted-foreground transition-transform', bookPickerOpen && 'rotate-180')}
					/>
				</button>

				{#if bookPickerOpen}
					<div class="relative mt-2 w-full max-h-60 overflow-y-auto rounded-lg border bg-card shadow-lg z-50">
						<button
							class={cn(
								'w-full flex items-center gap-2 px-3 py-2 text-left text-sm hover:bg-accent transition',
								appStore.currentBookId === 0 && 'bg-primary/10 text-primary font-medium'
							)}
							onclick={() => {
								appStore.setCurrentBook(0);
								bookPickerOpen = false;
							}}
						>
							<span>📚</span>
							<span class="flex-1">全部账本</span>
							{#if appStore.currentBookId === 0}
								<Check size={14} />
							{/if}
						</button>
						{#each appStore.books.filter((b) => !b.is_archived) as book (book.id)}
							<button
								class={cn(
									'w-full flex items-center gap-2 px-3 py-2 text-left text-sm hover:bg-accent transition',
									appStore.currentBookId === book.id && 'bg-primary/10 text-primary font-medium'
								)}
								onclick={() => {
									appStore.setCurrentBook(book.id);
									bookPickerOpen = false;
								}}
							>
								<span>{book.icon || '📘'}</span>
								<span class="flex-1 truncate">{book.name}</span>
								{#if appStore.currentBookId === book.id}
									<Check size={14} />
								{/if}
							</button>
						{/each}
					</div>
				{/if}
			</div>

			<!-- 导航菜单 -->
			<nav class="flex-1 overflow-y-auto px-3 py-3 space-y-3">
				{#each navGroups as group (group.title)}
					<div>
						<div class="px-3 pb-1 text-[11px] font-medium uppercase tracking-wide text-muted-foreground/70">
							{group.title}
						</div>
						<div class="space-y-0.5">
							{#each group.items as item (item.to)}
								<a
									href={item.to}
									class={cn(
										'flex items-center gap-2 px-3 py-2 rounded-lg text-sm transition',
										isActive(item.to)
											? 'bg-primary/10 text-primary font-medium'
											: 'text-muted-foreground hover:bg-accent hover:text-foreground'
									)}
								>
									<item.icon size={18} />
									<span>{item.label}</span>
								</a>
							{/each}
						</div>
					</div>
				{/each}
			</nav>

			<!-- 底部操作区 -->
			<div class="px-3 py-3 border-t space-y-2">
				<Button
					class="w-full"
					onclick={() => goto('/transactions/add')}
				>
					<Plus size={16} />
					记一笔
				</Button>
				<div class="flex items-center justify-between text-xs text-muted-foreground">
					<div class="flex items-center gap-2 min-w-0">
						<div class="w-7 h-7 rounded-full bg-primary/10 text-primary grid place-items-center font-semibold truncate">
							{appStore.user?.nickname?.[0] || 'U'}
						</div>
						<div class="truncate">
							<div class="text-foreground font-medium truncate text-sm">
								{appStore.user?.nickname}
							</div>
							<div class="truncate text-[11px]">
								{appStore.user?.email || appStore.user?.username}
							</div>
						</div>
					</div>
					<button
						title="退出登录"
						class="p-2 rounded-lg text-muted-foreground hover:text-destructive hover:bg-destructive/10 transition"
						onclick={handleLogout}
					>
						<LogOut size={16} />
					</button>
				</div>
			</div>
		</aside>

		<!-- ========== 主内容区 ========== -->
		<div class="flex-1 flex flex-col min-w-0">
			<!-- 顶部栏 -->
			<header class="sticky top-0 z-20 bg-background/80 backdrop-blur border-b">
				<div class="px-4 md:px-8 py-3 flex items-center gap-3">
					<!-- 移动端 -->
					<div class="md:hidden w-8 h-8 rounded-lg bg-primary grid place-items-center text-primary-foreground text-sm font-bold shrink-0">
						账
					</div>
					<h1 class="md:hidden flex-1 min-w-0 text-base font-semibold truncate">
						{pageTitle}
					</h1>
					<button
						class="md:hidden inline-flex items-center justify-center rounded-md p-2 hover:bg-accent shrink-0"
						onclick={toggleSidebar}
					>
						<Menu size={20} />
					</button>

					<!-- 桌面端迷你统计 -->
					<div class="hidden md:flex flex-1 items-center gap-4 text-xs justify-end">
						{#if offlineCount > 0}
							<button
								class="flex items-center gap-1 px-2.5 py-1 rounded-lg bg-amber-50 border border-amber-200 text-amber-700 hover:bg-amber-100 transition dark:bg-amber-950 dark:border-amber-800 dark:text-amber-400"
								onclick={async () => {
									const r = await replayQueue();
									if (r.ok > 0) hzToast.success(`已同步 ${r.ok} 条请求`);
									if (r.remaining > 0)
										hzToast.warning(`${r.remaining} 条仍未同步`);
								}}
							>
								<CloudOff size={14} />
								<span>{offlineCount} 条待同步</span>
							</button>
						{/if}
						{#if miniStats}
							<div class="flex items-center gap-4">
								<div class="text-right leading-tight">
									<div class="text-muted-foreground text-[10px]">本月收入</div>
									<div class="font-semibold tabular-nums text-sm text-[var(--color-income)]">
										{formatMoney(miniStats.in)}
									</div>
								</div>
								<div class="text-right leading-tight">
									<div class="text-muted-foreground text-[10px]">本月支出</div>
									<div class="font-semibold tabular-nums text-sm text-[var(--color-expense)]">
										{formatMoney(miniStats.out)}
									</div>
								</div>
								<div class="text-right leading-tight">
									<div class="text-muted-foreground text-[10px]">总资产</div>
									<div class="font-semibold tabular-nums text-sm text-primary">
										{formatMoney(miniStats.asset)}
									</div>
								</div>
							</div>
						{/if}
					</div>
				</div>
			</header>

			<!-- 页面内容 -->
			<main class="flex-1 px-4 md:px-8 pt-4 md:pt-6 pb-24 md:pb-8 max-w-[1400px] w-full mx-auto">
				{@render children()}
			</main>
		</div>

		<!-- ========== 移动端侧边栏抽屉 ========== -->
		<Sheet bind:open={sidebarOpen} side="left">
			<div class="flex flex-col h-full">
				<div class="px-5 py-4 border-b flex items-center gap-2">
					<div class="w-9 h-9 rounded-xl bg-primary grid place-items-center text-primary-foreground font-bold">
						账
					</div>
					<div>
						<div class="font-semibold">货殖</div>
						<div class="text-xs text-muted-foreground">简洁纯粹的记账本</div>
					</div>
				</div>
				<nav class="flex-1 overflow-y-auto px-3 py-3 space-y-3">
					{#each navGroups as group (group.title)}
						<div>
							<div class="px-3 pb-1 text-[11px] font-medium uppercase tracking-wide text-muted-foreground/70">
								{group.title}
							</div>
							<div class="space-y-0.5">
								{#each group.items as item (item.to)}
									<a
										href={item.to}
										class={cn(
											'flex items-center gap-2 px-3 py-2 rounded-lg text-sm transition',
											isActive(item.to)
												? 'bg-primary/10 text-primary font-medium'
												: 'text-muted-foreground hover:bg-accent hover:text-foreground'
										)}
									>
										<item.icon size={18} />
										<span>{item.label}</span>
									</a>
								{/each}
							</div>
						</div>
					{/each}
				</nav>
				<div class="px-3 py-3 border-t">
					<button
						class="flex items-center gap-2 w-full px-3 py-2 rounded-lg text-sm text-destructive hover:bg-destructive/10"
						onclick={async () => {
							await handleLogout();
						}}
					>
						<LogOut size={18} />
						退出登录
					</button>
				</div>
			</div>
		</Sheet>

		<!-- ========== 移动端底部 Tab Bar ========== -->
		<nav class="md:hidden fixed bottom-0 inset-x-0 z-20 bg-background border-t pb-[env(safe-area-inset-bottom)]">
			<div class="grid grid-cols-5">
				{#each mobileTabs as tab (tab.to)}
					{#if tab.to === '/transactions/add'}
						<button
							class="relative flex items-center justify-center"
							onclick={() => goto('/transactions/add')}
						>
							<div class="absolute -top-5 w-14 h-14 rounded-full bg-primary text-primary-foreground grid place-items-center shadow-lg border-4 border-background">
								<Plus size={24} />
							</div>
						</button>
					{:else if tab.more}
						<!-- A3：「更多」打开完整导航抽屉，9 个低频功能不再无处可去 -->
						<button
							class="flex flex-col items-center justify-center gap-0.5 py-2.5 text-[11px] transition text-muted-foreground"
							onclick={() => (sidebarOpen = true)}
						>
							<tab.icon size={20} />
							<span>{tab.label}</span>
						</button>
					{:else}
						<button
							class={cn(
								'flex flex-col items-center justify-center gap-0.5 py-2.5 text-[11px] transition',
								isActive(tab.to) ? 'text-primary' : 'text-muted-foreground'
							)}
							onclick={() => goto(tab.to)}
						>
							<tab.icon size={20} />
							<span>{tab.label}</span>
						</button>
					{/if}
				{/each}
			</div>
		</nav>
	</div>
{/if}
