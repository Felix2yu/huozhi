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
		MinusCircle
	} from '@lucide/svelte';

	let { children } = $props();

	// ========= 认证检查 =========
	let checking = $state(true);

	async function init() {
		const ok = await appStore.checkAuth();
		if (!ok) {
			const path = $page?.url?.pathname || '/dashboard';
			goto(`/login?redirect=${encodeURIComponent(path)}`);
			return;
		}
		try {
			await appStore.loadBooks();
			await appStore.loadDictionaries();
		} catch (e) {
			console.warn('加载基础数据失败', e);
		} finally {
			checking = false;
		}

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

	function handleSync(msg: WsMessage) {
		if (msg.type === 'sync' && msg.table) {
			// 根据变更的表刷新对应数据
			switch (msg.table) {
				case 'transactions':
				case 'accounts':
				case 'categories':
				case 'tags':
				case 'budgets':
				case 'books':
				case 'recurring':
				case 'installments':
				case 'reimbursements':
				case 'saving_plans':
					appStore.loadDictionaries();
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

	// ========= 导航项 =========
	const navItems = [
		{ to: '/dashboard', label: '首页总览', icon: LayoutDashboard },
		{ to: '/transactions', label: '账单流水', icon: Receipt },
		{ to: '/accounts', label: '账户资产', icon: Wallet },
		{ to: '/cards', label: '我的银行卡', icon: CreditCard },
		{ to: '/categories', label: '分类管理', icon: PieChart },
		{ to: '/budgets', label: '预算管理', icon: Target },
		{ to: '/statistics', label: '统计分析', icon: BarChart3 },
		{ to: '/tags', label: '标签中心', icon: Tags },
		{ to: '/savings', label: '存钱计划', icon: TrendingUp },
		{ to: '/recurring', label: '周期记账', icon: Repeat },
		{ to: '/installments', label: '分期管理', icon: CreditCard },
		{ to: '/reimbursements', label: '报销管理', icon: FileText },
		{ to: '/shared-books', label: '共享账本', icon: Users },
		{ to: '/settings', label: '系统设置', icon: Settings }
	];

	const mobileTabs = [
		{ to: '/dashboard', label: '首页', icon: Home },
		{ to: '/transactions', label: '账单', icon: Receipt },
		{ to: '/transactions/add', label: '', icon: Plus },
		{ to: '/accounts', label: '资产', icon: Wallet },
		{ to: '/settings', label: '我的', icon: Settings }
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
		const bid = appStore.currentBookId;
		if (!bid) return;

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
			<nav class="flex-1 overflow-y-auto px-3 py-3 space-y-0.5">
				{#each navItems as item (item.to)}
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
				<nav class="flex-1 overflow-y-auto px-3 py-3 space-y-0.5">
					{#each navItems as item (item.to)}
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
