<script lang="ts">
	import { slide } from 'svelte/transition';
	import { cubicOut } from 'svelte/easing';

	import Button from '$lib/components/ui/Button.svelte';
	import Card from '$lib/components/ui/Card.svelte';
	import Dialog from '$lib/components/ui/Dialog.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import Label from '$lib/components/ui/Label.svelte';
	import Badge from '$lib/components/ui/Badge.svelte';
	import Tabs from '$lib/components/ui/Tabs.svelte';
	import TabsTrigger from '$lib/components/ui/TabsTrigger.svelte';

	import { appStore } from '$lib/stores/app';
	import { categoryApi } from '$lib/api/modules/categories';
	import { hzToast } from '$lib/components/ui/toast';
	import { cn } from '$lib/utils/cn';

	import type { Category, CategoryKind } from '$lib/types';
	import {
		Plus,
		Pencil,
		Trash2,
		ChevronRight,
		CornerDownRight,
		Search,
		X,
		FolderTree,
		ListTree,
		Layers,
		AlertTriangle,
		ChevronsUpDown,
		ChevronsDownUp
	} from '@lucide/svelte';

	/** 「未归类」兜底分组的占位 ID：父分类被删除后遗留的二级分类 */
	const ORPHAN_ID = -1;

	/** 把十六进制色值转成低透明度的底色，非法值一律忽略，避免生成无效 CSS */
	function tint(color?: string): string | undefined {
		if (!color || !/^#([0-9a-f]{3}|[0-9a-f]{6})$/i.test(color)) return undefined;
		return `${color}1A`;
	}

	// ============ 状态 ============
	let kind = $state<CategoryKind>('expense');
	let keyword = $state('');
	let expanded = $state<Record<number, boolean>>({});

	let showDialog = $state(false);
	let editingCategory = $state<Category | null>(null);
	let catName = $state('');
	let catIcon = $state('📁');
	let catColor = $state('');
	let catParentId = $state<number | ''>('');
	let saving = $state(false);

	let deleteTarget = $state<Category | null>(null);
	let deleting = $state(false);

	// ============ 派生数据 ============
	/** 本页管理的是「具体账本」的分类。
	 *  currentBookId === 0（全部账本）时，后端 /categories 对 book_id=0 不做账本过滤，
	 *  会把所有账本的分类合并返回（同名一级/二级重复多份，本项目实测一级 153 行、
	 *  去重后仅 29 个名称）。而本页新建分类写入的是 appStore.effectiveBookId()，
	 *  因此列表同样按该账本收窄，保证「看到的 = 能编辑的 = 新建落到的」。 */
	let bookScope = $derived(appStore.effectiveBookId());
	const scopeOf = (list: Category[]): Category[] =>
		bookScope && list ? list.filter((c) => !c.book_id || c.book_id === bookScope) : list || [];

	let allCategories = $derived(
		kind === 'expense'
			? scopeOf(appStore.categories.expense)
			: kind === 'income'
				? scopeOf(appStore.categories.income)
				: scopeOf(appStore.categories.system)
	);

	let counts = $derived({
		expense: scopeOf(appStore.categories.expense).length,
		income: scopeOf(appStore.categories.income).length,
		system: scopeOf(appStore.categories.system).length
	});

	/** 按 parent_id 还原两级结构；同时收集父级已丢失的「孤儿」二级分类 */
	const model = $derived.by(() => {
		const byParent = new Map<number, Category[]>();
		for (const c of allCategories) {
			const pid = c.parent_id ?? 0;
			if (!byParent.has(pid)) byParent.set(pid, []);
			byParent.get(pid)!.push(c);
		}
		for (const arr of byParent.values())
			arr.sort((a, b) => (a.sort || 0) - (b.sort || 0) || a.id - b.id);

		const roots = byParent.get(0) || [];
		const rootIds = new Set(roots.map((r) => r.id));

		const orphans: Category[] = [];
		for (const [pid, arr] of byParent) {
			if (pid === 0) continue;
			if (!rootIds.has(pid)) orphans.push(...arr);
		}
		orphans.sort((a, b) => (a.sort || 0) - (b.sort || 0) || a.id - b.id);

		return { byParent, roots, orphans };
	});

	const childrenOf = (id: number): Category[] => model.byParent.get(id) || [];

	/** 一级分类网格：正常一级 + 末位的「未归类」兜底分组 */
	const rootsView = $derived.by<Category[]>(() => {
		const list = [...model.roots];
		if (model.orphans.length) {
			list.push({
				id: ORPHAN_ID,
				user_id: 0,
				book_id: 0,
				parent_id: 0,
				name: '未归类',
				kind,
				icon: '📦',
				sort: Number.MAX_SAFE_INTEGER,
				is_system: false,
				is_archived: false,
				need_tag: false
			});
		}
		return list;
	});

	const isOrphan = (id: number) => id === ORPHAN_ID;

	/** 搜索：跨层级平铺命中项，类目多时不必逐个展开翻找 */
	const searchResults = $derived.by<Category[]>(() => {
		const kw = keyword.trim().toLowerCase();
		if (!kw) return [];
		return allCategories.filter((c) => c.name.toLowerCase().includes(kw));
	});

	const parentNameOf = (c: Category): string | null => {
		if (!c.parent_id || c.parent_id === 0) return null;
		if (isOrphan(c.parent_id)) return '未归类';
		return allCategories.find((x) => x.id === c.parent_id)?.name ?? null;
	};

	let allExpanded = $derived(
		rootsView.length > 0 && rootsView.every((r) => !!expanded[r.id])
	);

	let totalSubCount = $derived(
		model.roots.reduce((sum, r) => sum + childrenOf(r.id).length, 0)
	);

	let overview = $derived([
		{
			label: '一级分类',
			value: model.roots.length,
			icon: FolderTree
		},
		{ label: '二级分类', value: totalSubCount, icon: ListTree },
		{ label: '当前类型合计', value: allCategories.length, icon: Layers }
	]);

	// ============ 交互 ============
	function toggleExpand(id: number) {
		expanded[id] = !expanded[id];
	}

	function toggleAll() {
		const next = !allExpanded;
		const map: Record<number, boolean> = {};
		for (const r of rootsView) map[r.id] = next;
		expanded = map;
	}

	let parentOptions = $derived(
		model.roots.filter((r) => r.id !== (editingCategory?.id || 0)).map((r) => r)
	);

	function openNew(parentId?: number) {
		editingCategory = null;
		catName = '';
		catIcon = '📁';
		catColor = '';
		catParentId = parentId ?? '';
		iconTab = 'common';
		showDialog = true;
	}

	function openEdit(cat: Category) {
		editingCategory = cat;
		catName = cat.name;
		catIcon = cat.icon || '📁';
		catColor = cat.color || '';
		catParentId =
			cat.parent_id && !isOrphan(cat.parent_id) ? cat.parent_id : '';
		iconTab = groupKeyOfIcon(catIcon);
		showDialog = true;
	}

	function askDelete(cat: Category) {
		deleteTarget = cat;
	}

	let deleteChildCount = $derived(
		deleteTarget ? childrenOf(deleteTarget.id).length : 0
	);

	async function handleSave() {
		const name = catName.trim();
		if (!name) {
			hzToast.warning('请输入分类名称');
			return;
		}
		saving = true;
		try {
			if (editingCategory) {
				const target = editingCategory;
				await categoryApi.update(target.id, {
					name,
					icon: catIcon,
					color: catColor,
					parent_id: catParentId === ORPHAN_ID ? 0 : catParentId || 0
				});
				hzToast.success('分类已更新');
			} else {
				await categoryApi.create({
					name,
					icon: catIcon,
					color: catColor,
					kind,
					parent_id: catParentId || 0,
					book_id: appStore.effectiveBookId()
				});
				hzToast.success('分类已创建');
				if (catParentId !== '') expanded[Number(catParentId)] = true;
			}
			showDialog = false;
			await appStore.loadDictionaries();
		} catch (e: any) {
			hzToast.error(e.message || '保存失败');
		} finally {
			saving = false;
		}
	}

	async function doDelete() {
		if (!deleteTarget) return;
		deleting = true;
		try {
			await categoryApi.remove(deleteTarget.id);
			hzToast.success('分类已删除');
			deleteTarget = null;
			await appStore.loadDictionaries();
		} catch (e: any) {
			hzToast.error(e.message || '删除失败');
		} finally {
			deleting = false;
		}
	}

	// 分类图标库：按主题分组（共 200+），覆盖常见记账场景并含现有分类已用的全部图标。
	// 分组便于在大量图标中快速定位，避免盲目滚动查找。
	const iconGroups: { key: string; label: string; icons: string[] }[] = [
		{
			key: 'common',
			label: '常用',
			icons: [
				'🍚', '🍔', '🍜', '☕', '🚌', '🚗', '🚕', '🛒', '🏠', '💡',
				'🎮', '📱', '👕', '💼', '📚', '🏥', '🎁', '💳', '💰', '💵',
				'🏦', '📈', '🎓', '✈️', '🏋️', '🐶', '💄', '🎬', '🍳', '📦',
				'🔧', '💊', '📞', '⚽', '🧧', '🎊', '💻', '↩️', '🔄', '⚙️',
				'🎫', '🏨', '🚇', '🚄', '🚲', '🍲', '🥡', '🧾', '💧', '⚡',
				'🌿', '🎒', '🧺', '♻️', '👶', '🖥️', '🥇', '🅿️', '🎉'
			]
		},
		{
			key: 'food',
			label: '饮食',
			icons: [
				'🍚', '🍔', '🍜', '☕', '🍲', '🍱', '🍛', '🍣', '🍝', '🍕',
				'🌭', '🍟', '🥗', '🍎', '🍊', '🍇', '🍉', '🍓', '🥑', '🥦',
				'🥕', '🥩', '🍗', '🥓', '🦐', '🐟', '🥚', '🧀', '🥛', '🍵',
				'🧋', '🍺', '🍷', '🍰', '🍩', '🍪', '🍫', '🍭', '🍽️', '🥡',
				'🍳', '🥢'
			]
		},
		{
			key: 'transport',
			label: '出行',
			icons: [
				'🚌', '🚗', '🚕', '🚙', '🏍️', '🚲', '🛵', '🚆', '🚄', '🚂',
				'🚇', '🛫', '✈️', '🚀', '⛴️', '🚢', '⛽', '🅿️', '🚐', '🚏',
				'🧳', '🗺️', '🏝️', '🏖️', '🏔️', '🏕️', '⛺', '🏨', '🎡', '🎢',
				'🎠', '🛤️'
			]
		},
		{
			key: 'shopping',
			label: '购物',
			icons: [
				'🛒', '🛍️', '👗', '👕', '👚', '👖', '👜', '👟', '👠', '🧥',
				'🧣', '🧤', '💍', '💄', '👓', '🕶️', '🎀', '🧸'
			]
		},
		{
			key: 'life',
			label: '生活健康',
			icons: [
				'🏠', '🏡', '🛋️', '🛏️', '🚿', '🧺', '🧹', '💡', '🔌', '🔧',
				'🔨', '🪛', '🧰', '🪟', '🪴', '🌿', '🌱', '🌳', '💐', '🌸',
				'🌻', '💊', '💉', '🩺', '🏥', '🦷', '🧠', '🤒'
			]
		},
		{
			key: 'learn',
			label: '学习教育',
			icons: [
				'📚', '📖', '✏️', '📝', '🎓', '🏫', '📐', '🧮', '📞', '📱',
				'📡', '💬', '📧', '✉️', '📻', '📺', '🎙️'
			]
		},
		{
			key: 'fun',
			label: '休闲运动',
			icons: [
				'🎮', '🕹️', '🎬', '🎭', '🎨', '🎯', '🎲', '🎸', '🎹', '🎺',
				'🎻', '🥁', '🎤', '🎧', '📷', '⚽', '🏀', '🏈', '⚾', '🎾',
				'🏐', '🏓', '🏸', '🏒', '🏏', '⛳', '🏊', '🏄', '🏂', '🏋️',
				'🥊', '🥋', '🎿', '🛹', '🚴', '🐶', '🐱', '🐰', '🐹', '🦊',
				'🐻', '🐼', '🐨', '🐯', '🦁', '🐮', '🐷', '🐸', '🐵', '🐔',
				'🐧', '🦄', '🐢', '🐍', '🦋', '🐝'
			]
		},
		{
			key: 'social',
			label: '人情节日',
			icons: ['🎁', '🧧', '💝', '💌', '🎉', '🎊', '🥳', '🎈', '👶', '👪', '💏']
		},
		{
			key: 'money',
			label: '财务工作',
			icons: [
				'💰', '💵', '💴', '💶', '💷', '💳', '💎', '🏦', '📈', '📉',
				'📊', '💼', '💻', '🖥️', '🧾', '🪙', '🤑', '💸', '💱', '🏧',
				'💹', '🛠️', '⚙️', '🔔', '⏰', '⌚', '🔋', '⌨️', '🖱️', '🖨️'
			]
		},
		{
			key: 'misc',
			label: '其他',
			icons: ['📦', '🗂️', '📁', '🏆', '🥇', '🔄', '↩️']
		}
	];

	/** 图标分组筛选：默认「常用」，编辑既有分类时自动跳到其图标所在分组。 */
	let iconTab = $state('common');
	const iconOptions = $derived(
		(iconGroups.find((g) => g.key === iconTab) ?? iconGroups[0]).icons
	);

	/** 定位某图标所属分组（用于编辑时自动切换分组）。 */
	function groupKeyOfIcon(icon: string): string {
		return iconGroups.find((g) => g.icons.includes(icon))?.key ?? 'common';
	}

	const colorOptions = [
		{ value: '', label: '默认' },
		{ value: '#EF4444', label: '红' },
		{ value: '#F97316', label: '橙' },
		{ value: '#F59E0B', label: '琥珀' },
		{ value: '#10B981', label: '绿' },
		{ value: '#06B6D4', label: '青' },
		{ value: '#3B82F6', label: '蓝' },
		{ value: '#8B5CF6', label: '紫' },
		{ value: '#EC4899', label: '粉' },
		{ value: '#64748B', label: '灰' }
	];
</script>

<svelte:head>
	<title>分类管理 · 货殖</title>
</svelte:head>

<!-- 通用图标操作按钮 -->
{#snippet actBtn(label: string, handler: () => void, Icon: any, danger?: boolean)}
	<button
		type="button"
		title={label}
		aria-label={label}
		onclick={(e) => {
			e.stopPropagation();
			handler();
		}}
		class={cn(
			'inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-md text-muted-foreground transition-colors',
			'hover:bg-accent hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
			danger && 'hover:bg-destructive/10 hover:text-destructive'
		)}
	>
		<Icon size={14} />
	</button>
{/snippet}

<div class="space-y-5">
	<!-- ============ 页头 ============ -->
	<header class="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
		<div class="min-w-0">
			<h1 class="text-lg font-semibold tracking-tight">分类管理</h1>
			<p class="mt-1 text-sm text-muted-foreground">
				一级分类平铺展示，点击即在同屏展开其下属的二级分类，便于统一维护
			</p>
		</div>
		<div class="flex shrink-0 items-center gap-2">
			<Button variant="outline" size="sm" onclick={toggleAll} disabled={rootsView.length === 0}>
				{#if allExpanded}
					<ChevronsDownUp size={15} />
					收起全部
				{:else}
					<ChevronsUpDown size={15} />
					展开全部
				{/if}
			</Button>
			<Button
				size="sm"
				onclick={() => openNew()}
				disabled={kind === 'system'}
				title={kind === 'system' ? '系统分类由系统预置，不支持新增' : ''}
			>
				<Plus size={15} />
				新增分类
			</Button>
		</div>
	</header>

	<!-- ============ 概览 ============ -->
	<div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
		{#each overview as item (item.label)}
			{@const Icon = item.icon}
			<div class="flex items-center gap-3 rounded-xl border bg-card px-4 py-3">
				<div
					class="grid h-9 w-9 shrink-0 place-items-center rounded-lg bg-primary/10 text-primary"
				>
					<Icon size={17} />
				</div>
				<div class="min-w-0">
					<div class="text-xs text-muted-foreground">{item.label}</div>
					<div class="text-lg font-semibold leading-tight tabular-nums">{item.value}</div>
				</div>
			</div>
		{/each}
	</div>

	<!-- ============ 工具条 ============ -->
	<div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
		<Tabs bind:value={kind}>
			<TabsTrigger value="expense">
				支出
				<span class="ml-1.5 text-xs opacity-70 tabular-nums">{counts.expense}</span>
			</TabsTrigger>
			<TabsTrigger value="income">
				收入
				<span class="ml-1.5 text-xs opacity-70 tabular-nums">{counts.income}</span>
			</TabsTrigger>
			<TabsTrigger value="system">
				系统
				<span class="ml-1.5 text-xs opacity-70 tabular-nums">{counts.system}</span>
			</TabsTrigger>
		</Tabs>

		<div class="relative w-full sm:w-64">
			<Search
				size={15}
				class="pointer-events-none absolute top-1/2 left-3 -translate-y-1/2 text-muted-foreground"
			/>
			<Input bind:value={keyword} placeholder="搜索分类名称…" class="pr-9 pl-9" />
			{#if keyword}
				<button
					type="button"
					aria-label="清空搜索"
					class="absolute top-1/2 right-2 -translate-y-1/2 rounded p-1 text-muted-foreground transition hover:bg-accent hover:text-foreground"
					onclick={() => (keyword = '')}
				>
					<X size={14} />
				</button>
			{/if}
		</div>
	</div>

	<!-- ============ 分类网格 ============ -->
	{#if keyword.trim()}
		<!-- 搜索：跨层级平铺命中项 -->
		{#if searchResults.length === 0}
			<Card>
				<div class="flex flex-col items-center justify-center py-16 text-center">
					<div class="mb-3 grid h-12 w-12 place-items-center rounded-xl bg-muted text-muted-foreground">
						<Search size={22} />
					</div>
					<p class="text-sm font-medium">没有匹配「{keyword.trim()}」的分类</p>
					<p class="mt-1 text-xs text-muted-foreground">试试其他关键词，或清空搜索查看全部分类</p>
				</div>
			</Card>
		{:else}
			<div
				class="grid grid-cols-2 gap-2 sm:grid-cols-3 lg:grid-cols-4"
				data-testid="category-search-results"
			>
				{#each searchResults as c (c.id)}
					{@const pName = parentNameOf(c)}
					<button
						type="button"
						data-testid="category-search-item"
						data-category-id={c.id}
						class="group flex items-center gap-2 rounded-lg border bg-card px-2.5 py-2 text-left transition hover:border-primary/40 hover:shadow-sm"
						onclick={() => openEdit(c)}
					>
						<div
							class="grid h-8 w-8 shrink-0 place-items-center rounded-md text-base"
							style={tint(c.color) ? `background-color:${tint(c.color)}` : ''}
						>
							{c.icon || '📁'}
						</div>
						<div class="min-w-0 flex-1">
							<div class="flex items-center gap-1.5">
								<span class="truncate text-sm">{c.name}</span>
								{#if c.is_system}
									<Badge variant="outline" class="shrink-0 px-1 py-0 text-[10px]">系统</Badge>
								{/if}
							</div>
							{#if pName}
								<div class="truncate text-[11px] text-muted-foreground">· {pName}</div>
							{:else}
								<div class="text-[11px] text-muted-foreground">一级分类</div>
							{/if}
						</div>
						<Pencil
							size={13}
							class="shrink-0 text-muted-foreground opacity-0 transition group-hover:opacity-100"
						/>
					</button>
				{/each}
			</div>
		{/if}
	{:else if rootsView.length === 0}
		<Card>
			<div class="flex flex-col items-center justify-center py-16 text-center">
				<div class="mb-3 grid h-12 w-12 place-items-center rounded-xl bg-muted text-muted-foreground">
					<FolderTree size={22} />
				</div>
				<p class="text-sm font-medium">还没有任何分类</p>
				<p class="mt-1 text-xs text-muted-foreground">创建第一个分类，开始归集你的账单</p>
				{#if kind !== 'system'}
					<Button size="sm" class="mt-4" onclick={() => openNew()}>
						<Plus size={15} />
						新增分类
					</Button>
				{/if}
			</div>
		</Card>
	{:else}
		<!-- 一级分类：全部平铺一屏可见，点击即在同屏展开二级 -->
		<div class="grid grid-cols-3 gap-2 sm:grid-cols-4 lg:grid-cols-5" data-testid="category-roots">
			{#each rootsView as r (r.id)}
				{@const kids = childrenOf(r.id)}
				{@const open = !!expanded[r.id]}
				{@const orphan = isOrphan(r.id)}
				<button
					type="button"
					data-testid="category-group"
					data-parent-id={r.id}
					class={cn(
						'flex flex-col items-center gap-1.5 rounded-xl border px-1 py-3 transition',
						open
							? 'border-primary/40 bg-primary/5 ring-1 ring-primary/30'
							: 'hover:border-primary/30 hover:bg-accent/50',
						orphan && 'border-dashed'
					)}
					onclick={() => toggleExpand(r.id)}
				>
					<span
						class="relative grid h-11 w-11 place-items-center rounded-full text-xl"
						style={tint(r.color) ? `background-color:${tint(r.color)}` : ''}
					>
						{r.icon || '📁'}
						<!-- 展开指示 / 含子级角标 -->
						<span
							data-testid="group-toggle"
							class={cn(
								'absolute -bottom-0.5 -right-0.5 grid h-4 w-4 place-items-center rounded-full bg-card text-muted-foreground ring-1 ring-border transition-transform',
								open && 'rotate-90 bg-primary text-primary-foreground ring-primary'
							)}
						>
							<ChevronRight size={11} strokeWidth={2.5} />
						</span>
					</span>
					<span class="w-full truncate text-center text-xs font-medium leading-tight">
						{r.name}
					</span>
					<span class="text-[10px] leading-tight text-muted-foreground">
						{kids.length ? `${kids.length} 个二级` : '无二级'}
					</span>
				</button>

				{#if open}
					{@const system = r.is_system}
					<!-- 二级面板紧跟在所属一级的同一行下方（占满整行），不跳到网格底部 -->
					<div
						class="col-span-full rounded-xl border border-border/70 bg-muted/40 p-3"
						data-testid="category-children-panel"
						data-parent-id={r.id}
						transition:slide={{ duration: 200, easing: cubicOut }}
					>
						<div class="mb-2 flex items-center gap-2">
							<div
								class="grid h-7 w-7 shrink-0 place-items-center rounded-md text-sm"
								style={tint(r.color) ? `background-color:${tint(r.color)}` : ''}
							>
								{r.icon || '📁'}
							</div>
							<div class="flex min-w-0 items-center gap-1.5">
								<span class="truncate text-sm font-medium">{r.name}</span>
								{#if system}
									<Badge variant="outline" class="shrink-0 px-1.5 py-0 text-[10px]">系统</Badge>
								{/if}
								<span class="shrink-0 text-[11px] text-muted-foreground">
									· {kids.length} 个二级分类
								</span>
							</div>

							<div class="ml-auto flex shrink-0 items-center gap-0.5">
								{#if orphan}
									<span class="pr-1 text-[11px] text-muted-foreground">父分类已删除</span>
								{:else if system}
									<span class="pr-1 text-[11px] text-muted-foreground">只读</span>
								{:else}
									{@render actBtn('添加二级分类', () => openNew(r.id), Plus)}
									{@render actBtn('编辑', () => openEdit(r), Pencil)}
									{@render actBtn('删除', () => askDelete(r), Trash2, true)}
								{/if}
							</div>
						</div>

						<div class="grid grid-cols-2 gap-1.5 sm:grid-cols-3 sm:gap-2 lg:grid-cols-4">
							{#each kids as child (child.id)}
								<div
									class="group/child flex min-w-0 items-center gap-1.5 rounded-lg border bg-card py-2 pr-2 pl-2.5 transition hover:border-primary/40 hover:shadow-sm sm:gap-2 sm:pr-1.5"
									data-testid="category-child"
									data-child-id={child.id}
									role="button"
									tabindex={0}
									onclick={() => openEdit(child)}
									onkeydown={(e: KeyboardEvent) =>
										(e.key === 'Enter' || e.key === ' ') && openEdit(child)}
								>
									<div
										class="grid h-7 w-7 shrink-0 place-items-center rounded-md text-sm"
										style={tint(child.color) ? `background-color:${tint(child.color)}` : ''}
									>
										{child.icon || '📁'}
									</div>
									<div class="flex min-w-0 flex-1 items-center gap-1.5">
										<span class="min-w-0 flex-1 truncate text-sm">{child.name}</span>
										{#if child.is_system}
											<Badge variant="outline" class="shrink-0 px-1 py-0 text-[10px]">系统</Badge>
										{/if}
									</div>
									<!-- 悬浮操作仅在具备 hover 的宽屏显示；触屏（<sm）不再为其预留 58px 宽度，
									     改为点击瓦片进入编辑弹窗，并在弹窗内提供「删除分类」。 -->
									{#if !child.is_system}
										<div
											class="hidden shrink-0 items-center gap-0.5 opacity-0 transition-opacity group-hover/child:opacity-100 focus-within:opacity-100 sm:flex"
										>
											{@render actBtn('编辑', () => openEdit(child), Pencil)}
											{@render actBtn('删除', () => askDelete(child), Trash2, true)}
										</div>
									{/if}
								</div>
							{/each}

							{#if !orphan && !system}
								<button
									type="button"
									class="flex items-center justify-center gap-1.5 rounded-lg border border-dashed py-2.5 text-xs text-muted-foreground transition hover:border-primary/50 hover:bg-primary/5 hover:text-primary"
									onclick={() => openNew(r.id)}
								>
									<Plus size={13} />
									添加二级分类
								</button>
							{/if}
						</div>
					</div>
				{/if}
			{/each}
		</div>
	{/if}
</div>

<!-- ============ 新增 / 编辑 ============ -->
<Dialog bind:open={showDialog}>
	<div class="space-y-5">
		<div>
			<h3 class="text-base font-semibold">{editingCategory ? '编辑分类' : '新增分类'}</h3>
			<p class="mt-1 text-xs text-muted-foreground">
				{editingCategory
					? '修改后点击保存生效'
					: kind === 'system'
						? '系统分类不支持新增'
						: '选择「顶级分类」创建一级分类，或选择某个一级分类创建二级分类'}
			</p>
		</div>

		<div class="space-y-2">
			<Label for="cat-name">名称</Label>
			<!-- svelte-ignore a11y_autofocus -->
			<Input
				id="cat-name"
				bind:value={catName}
				placeholder="例如：餐饮、交通、房租"
				maxlength={50}
				autofocus
				onkeydown={(e: KeyboardEvent) => e.key === 'Enter' && handleSave()}
			/>
		</div>

		<div class="space-y-2">
			<Label>图标</Label>
			<div class="flex items-start gap-3">
				<div
					class="grid h-11 w-11 shrink-0 place-items-center rounded-xl border text-xl"
					style={tint(catColor) ? `background-color:${tint(catColor)}` : ''}
				>
					{catIcon}
				</div>
				<div class="min-w-0 flex-1 space-y-1.5">
					<!-- 主题分组：图标量大时快速定位，无需盲目滚动 -->
					<div class="flex flex-wrap gap-1">
						{#each iconGroups as g (g.key)}
							<button
								type="button"
								data-testid="icon-group-tab"
								class={cn(
									'rounded-full px-2 py-0.5 text-[11px] transition',
									iconTab === g.key
										? 'bg-primary text-primary-foreground'
										: 'bg-muted text-muted-foreground hover:bg-accent'
								)}
								onclick={() => (iconTab = g.key)}
							>
								{g.label}
							</button>
						{/each}
					</div>
					<div
						class="grid max-h-48 grid-cols-8 gap-1 overflow-y-auto rounded-lg border p-2 sm:grid-cols-10"
						data-testid="icon-grid"
					>
						{#each iconOptions as icon (icon)}
							<button
								type="button"
								class={cn(
									'grid h-8 w-8 place-items-center rounded-md text-lg transition',
									catIcon === icon ? 'bg-primary/15 ring-2 ring-primary' : 'hover:bg-accent'
								)}
								onclick={() => (catIcon = icon)}
							>
								{icon}
							</button>
						{/each}
					</div>
				</div>
			</div>
		</div>

		<div class="space-y-2">
			<Label>标识色（用于统计图表）</Label>
			<div class="flex flex-wrap items-center gap-2">
				{#each colorOptions as c (c.value)}
					<button
						type="button"
						title={c.label}
						class={cn(
							'h-7 w-7 rounded-full border transition',
							catColor === c.value && 'ring-2 ring-primary ring-offset-2'
						)}
						style={c.value ? `background-color:${c.value}` : ''}
						onclick={() => (catColor = c.value)}
					>
						{#if !c.value}
							<span class="text-[10px] text-muted-foreground">无</span>
						{/if}
					</button>
				{/each}
			</div>
		</div>

		<div class="space-y-2">
			<Label>归属层级</Label>
			<div class="flex flex-wrap gap-2">
				<button
					type="button"
					class={cn(
						'inline-flex items-center gap-1.5 rounded-full border px-3 py-1.5 text-xs transition',
						catParentId === ''
							? 'border-primary bg-primary/10 font-medium text-primary'
							: 'text-muted-foreground hover:bg-accent'
					)}
					onclick={() => (catParentId = '')}
				>
					作为一级分类
				</button>
				{#each parentOptions as p (p.id)}
					<button
						type="button"
						class={cn(
							'inline-flex items-center gap-1.5 rounded-full border px-3 py-1.5 text-xs transition',
							catParentId === p.id
								? 'border-primary bg-primary/10 font-medium text-primary'
								: 'text-muted-foreground hover:bg-accent'
						)}
						onclick={() => (catParentId = p.id)}
					>
						<span>{p.icon || '📁'}</span>
						<span>{p.name}</span>
						<CornerDownRight size={11} class="opacity-60" />
					</button>
				{/each}
			</div>
			{#if catParentId !== ''}
				<p class="text-[11px] text-muted-foreground">
					将创建为二级分类，归属于「{parentOptions.find((p) => p.id === catParentId)?.name}」
				</p>
			{/if}
		</div>

		<div class="flex items-center justify-between gap-2 pt-1">
			<!-- 触屏设备无 hover，悬浮删除按钮不可用；在弹窗内提供等价入口 -->
			{#if editingCategory && !editingCategory.is_system}
				<button
					type="button"
					data-testid="dialog-delete"
					class="inline-flex shrink-0 items-center gap-1.5 rounded-md px-2 py-1.5 text-sm text-destructive transition-colors hover:bg-destructive/10"
					onclick={() => {
						const target = editingCategory;
						showDialog = false;
						if (target) askDelete(target);
					}}
				>
					<Trash2 size={14} />
					删除分类
				</button>
			{:else}
				<span aria-hidden="true"></span>
			{/if}
			<div class="flex shrink-0 gap-2">
				<Button variant="outline" onclick={() => (showDialog = false)}>取消</Button>
				<Button onclick={handleSave} disabled={saving || kind === 'system'}>
					{saving ? '保存中…' : '保存'}
				</Button>
			</div>
		</div>
	</div>
</Dialog>

<!-- ============ 删除确认 ============ -->
<Dialog bind:open={() => !!deleteTarget, (v) => !v && (deleteTarget = null)}>
	{#if deleteTarget}
		<div class="space-y-5">
			<div class="flex items-start gap-3">
				<div class="grid h-10 w-10 shrink-0 place-items-center rounded-full bg-destructive/10 text-destructive">
					<Trash2 size={18} />
				</div>
				<div class="min-w-0 flex-1">
					<h3 class="text-base font-semibold">删除分类</h3>
					<p class="mt-1 text-sm text-muted-foreground">
						确定删除「{deleteTarget.icon || '📁'}
						{deleteTarget.name}」吗？删除后该分类不再出现在记账与统计的选择列表中。
					</p>
					{#if deleteChildCount > 0}
						<p class="mt-3 flex items-start gap-2 rounded-lg bg-amber-500/10 px-3 py-2 text-xs text-amber-700 dark:text-amber-400">
							<AlertTriangle size={14} class="mt-0.5 shrink-0" />
							<span>
								该分类下还有 {deleteChildCount} 个二级分类，删除后它们会变为「未归类」，可编辑后重新指定归属。
							</span>
						</p>
					{/if}
				</div>
			</div>
			<div class="flex justify-end gap-2">
				<Button variant="outline" onclick={() => (deleteTarget = null)}>取消</Button>
				<Button variant="destructive" onclick={doDelete} disabled={deleting}>
					{deleting ? '删除中…' : '确认删除'}
				</Button>
			</div>
		</div>
	{/if}
</Dialog>
