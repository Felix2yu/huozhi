<script lang="ts">
	import { appStore } from '$lib/stores/app';
	import type { Category, CategoryKind } from '$lib/types';
	import { Check, ChevronsUpDown, CornerDownRight, Search } from '@lucide/svelte';

	let {
		value = $bindable(0),
		kind = 'expense' as CategoryKind,
		placeholder = '选择分类',
		disabled = false,
		onchange
	}: {
		value?: number;
		kind?: CategoryKind;
		placeholder?: string;
		disabled?: boolean;
		onchange?: (id: number) => void;
	} = $props();

	/* ------------------------------------------------------------------
	 * 分类池：只取「本次写入目标账本」的分类。
	 *
	 * currentBookId === 0 表示「全部账本」，后端 /categories 对 book_id=0
	 * 不做账本过滤，会把所有账本的分类合并返回——同名一级/二级会重复很多份
	 * （例如 16 个账本合并后一级有 153 行、去重后仅 29 个名称）。
	 * 但记账写入的是具体账本（appStore.effectiveBookId()），因此选择器
	 * 只应呈现该账本的分类，否则列表被重复项撑长、还可能选到别的账本的分类。
	 * ------------------------------------------------------------------ */
	const scopeBookId = $derived(appStore.effectiveBookId());
	const pool = $derived.by(() => {
		const all = appStore.categories[kind] || [];
		if (!scopeBookId) return all;
		return all.filter((c) => !c.book_id || c.book_id === scopeBookId);
	});

	/* ------------------------------------------------------------------
	 * 层级重建：后端返回的是「一级 + 其子级」的扁平数组，
	 * 这里按 parent_id 还原成两级结构，供「一级平铺 + 子级展开」使用。
	 * ------------------------------------------------------------------ */
	const tree = $derived.by(() => {
		const byParent = new Map<number, Category[]>();
		for (const c of pool) {
			const pid = c.parent_id ?? 0;
			if (!byParent.has(pid)) byParent.set(pid, []);
			byParent.get(pid)!.push(c);
		}
		for (const arr of byParent.values()) arr.sort((a, b) => a.sort - b.sort);
		return byParent;
	});

	const roots = $derived(tree.get(0) || []);
	const childrenOf = (id: number): Category[] => tree.get(id) || [];

	const selected = $derived((appStore.categories[kind] || []).find((c) => c.id === value) || null);

	function parentOf(c: Category | null): Category | null {
		if (!c || !c.parent_id) return null;
		return (appStore.categories[kind] || []).find((x) => x.id === c.parent_id) || null;
	}
	const selectedParent = $derived(parentOf(selected));

	let open = $state(false);
	let activeRoot = $state<number | null>(null);
	let query = $state('');
	/** 中部一级分类滚动容器：分类很多时只有它滚动，底部二级区固定可见 */
	let rootsScrollEl = $state<HTMLDivElement | null>(null);

	/* 打开时把浏览位置定位到当前选中项所属的一级分类（别无脑从第一个开始翻）。
	 * 注意依赖里不含 activeRoot 本身，否则用户点击切换一级会被这里重置。
	 * 定位前校验该一级确实在当前账本的分类池内，否则回落到第一个一级。 */
	$effect(() => {
		if (!open) return;
		const sp = selectedParent;
		if (sp && roots.some((r) => r.id === sp.id)) activeRoot = sp.id;
		else if (selected && selected.parent_id === 0 && roots.some((r) => r.id === selected.id))
			activeRoot = selected.id;
		else activeRoot = roots[0]?.id ?? null;
	});

	/* 一级分类很多、中部区域需要滚动时：点击后把当前一级滚入可视区，
	 * 否则用户点了列表深处的一级，看不出高亮跳到了哪里。 */
	$effect(() => {
		if (!open || activeRoot == null || !rootsScrollEl) return;
		const el = rootsScrollEl.querySelector<HTMLElement>(`[data-root-id="${activeRoot}"]`);
		// scrollIntoView 在测试环境（jsdom）不存在，做能力检测
		if (el && typeof el.scrollIntoView === 'function') el.scrollIntoView({ block: 'nearest' });
	});

	const activeRootCat = $derived(roots.find((r) => r.id === activeRoot) || null);
	const kids = $derived(activeRoot != null ? childrenOf(activeRoot) : []);

	/* 关键词搜索：跨层级平铺过滤，类目多时不必逐个展开翻找 */
	const results = $derived.by(() => {
		const q = query.trim().toLowerCase();
		if (!q) return [] as Category[];
		return pool.filter((c) => c.name.toLowerCase().includes(q));
	});

	function select(id: number) {
		value = id;
		open = false;
		query = '';
		onchange?.(id);
	}

	/* 有子级的一级分类：点击展开二级面板；没有子级的：点击即选中 */
	function onPickRoot(r: Category) {
		if (childrenOf(r.id).length > 0) {
			activeRoot = r.id;
		} else {
			select(r.id);
		}
	}

	function toggle() {
		if (disabled) return;
		open = !open;
		if (open) {
			query = '';
			placePanel();
		}
	}

	/* ------------------------------------------------------------------
	 * 弹层定位：把面板高度限制在「触发器下方（或上方）实际可用的高度」内。
	 *
	 * 只用 max-h-[72vh] 是不够的：当触发器本身位于页面偏下位置时，
	 * 72vh 高的面板底部——也就是二级分类区——会落到屏幕之外，
	 * 用户必须先滚动页面才能看到二级分类。这里按实测剩余空间收紧，
	 * 空间不足时改为向上展开。
	 * ------------------------------------------------------------------ */
	let triggerEl = $state<HTMLButtonElement | null>(null);
	let panelStyle = $state('');
	const PANEL_GAP = 4;

	function placePanel() {
		if (!triggerEl || typeof window === 'undefined') return;
		const r = triggerEl.getBoundingClientRect();
		const below = window.innerHeight - r.bottom - 12;
		const above = r.top - 12;
		const openUp = below < 300 && above > below;
		const maxH = Math.max(
			Math.min(window.innerHeight * 0.72, openUp ? above : below),
			200
		);
		panelStyle = openUp
			? `bottom: calc(100% + ${PANEL_GAP}px); max-height: ${Math.round(maxH)}px;`
			: `top: calc(100% + ${PANEL_GAP}px); max-height: ${Math.round(maxH)}px;`;
	}

	function onKeydown(e: KeyboardEvent) {
		if (open && e.key === 'Escape') open = false;
	}

	/* 图标底色：分类自带颜色时取该色 18%/20% 透明度；否则用主色/中性底 */
	function iconStyle(c: Category, active: boolean): string {
		if (c.color) return `background-color:${c.color}${active ? '33' : '1F'}`;
		return active ? 'background-color:var(--color-primary)' : 'background-color:var(--color-muted)';
	}
</script>

<svelte:window on:keydown={onKeydown} on:resize={() => open && placePanel()} />

<div class="relative">
	<!-- 触发器 -->
	<button
		type="button"
		data-testid="category-trigger"
		bind:this={triggerEl}
		aria-expanded={open}
		aria-haspopup="listbox"
		class="flex h-10 w-full items-center gap-2 rounded-md border border-input bg-transparent px-3 text-left text-sm shadow-sm transition hover:bg-accent/50 focus:outline-none focus:ring-1 focus:ring-ring disabled:cursor-not-allowed disabled:opacity-60"
		{disabled}
		onclick={toggle}
	>
		<span
			class="grid h-6 w-6 shrink-0 place-items-center rounded-md text-sm"
			style={selected ? iconStyle(selected, false) : 'background-color:var(--color-muted)'}
		>
			{selected?.icon || '📁'}
		</span>
		<span class="min-w-0 flex-1 truncate">
			{#if selected}
				{#if selectedParent}
					<span class="text-muted-foreground">{selectedParent.name}</span>
					<span class="mx-1 text-muted-foreground/60">/</span>
					<span class="font-medium">{selected.name}</span>
				{:else}
					<span class="font-medium">{selected.name}</span>
				{/if}
			{:else}
				<span class="text-muted-foreground">{placeholder}</span>
			{/if}
		</span>
		<ChevronsUpDown size={16} class="shrink-0 text-muted-foreground" />
	</button>

	{#if open}
		<!-- 点击空白关闭 -->
		<div class="fixed inset-0 z-40" onclick={() => (open = false)} aria-hidden="true"></div>

		<!-- 面板结构：顶部固定搜索栏 + 中部可滚动的一级分类网格 + 底部固定的二级分类区。
		     要点：二级区不参与滚动（shrink-0），因此无论一级分类有多少、
		     中部滚到哪里，二级分类都始终与一级同屏可见，绝不会被推到列表底部。
		     （历史 bug：整个面板是一块 overflow-y-auto，二级区被追加在整张
		      一级网格之后，一级一多就落到滚动区底部。） -->
		<div
			class="absolute z-50 flex w-full flex-col rounded-xl border bg-card p-3 shadow-lg"
			style={panelStyle}
		>
			<div class="relative shrink-0">
				<Search size={14} class="absolute left-2.5 top-1/2 -translate-y-1/2 text-muted-foreground" />
				<input
					class="h-8 w-full rounded-md border bg-transparent pl-8 pr-2 text-xs outline-none focus:ring-1 focus:ring-ring"
					placeholder="搜索分类…"
					bind:value={query}
				/>
			</div>

			{#snippet tile(c: Category, active: boolean, hint?: string)}
				<button
					type="button"
					class="relative flex flex-col items-center gap-1 rounded-xl px-1 py-1.5 transition
						{active ? 'bg-primary/10 ring-1 ring-primary/30' : 'hover:bg-accent'}"
					onclick={() => select(c.id)}
				>
					<span
						class="relative grid h-11 w-11 place-items-center rounded-full text-xl"
						style={iconStyle(c, active)}
					>
						{c.icon || '📁'}
						{#if value === c.id}
							<span
								class="absolute -bottom-0.5 -right-0.5 grid h-4 w-4 place-items-center rounded-full bg-primary text-primary-foreground ring-2 ring-card"
							>
								<Check size={10} strokeWidth={3} />
							</span>
						{/if}
					</span>
					<span
						class="w-full truncate text-center text-[11px] leading-tight
							{active ? 'font-medium text-primary' : 'text-foreground'}"
					>
						{c.name}
					</span>
					{#if hint}
						<span class="w-full truncate text-center text-[10px] leading-tight text-muted-foreground">
							{hint}
						</span>
					{/if}
				</button>
			{/snippet}

			{#if roots.length === 0}
				<div class="py-8 text-center text-sm text-muted-foreground">
					暂无分类，请先到「分类管理」创建
				</div>
			{:else if query.trim()}
				<!-- 搜索结果：跨层级平铺（中部可滚动） -->
				<div class="mt-2.5 min-h-0 flex-1 overflow-y-auto">
					{#if results.length === 0}
						<div class="py-8 text-center text-xs text-muted-foreground">
							没有匹配「{query.trim()}」的分类
						</div>
					{:else}
						<div class="grid grid-cols-5 gap-1" data-testid="category-search-results">
							{#each results as c (c.id)}
								{@const p = parentOf(c)}
								{@render tile(c, false, p ? p.name : '一级分类')}
							{/each}
						</div>
					{/if}
				</div>
			{:else}
				<!-- 一级分类：中部可滚动区。分类多时只有这里滚动，不牵动底部二级区 -->
				<div class="mt-2.5 min-h-0 flex-1 overflow-y-auto" bind:this={rootsScrollEl}>
					<div class="grid grid-cols-5 gap-1" data-testid="category-roots">
						{#each roots as r (r.id)}
							{@const hasChildren = childrenOf(r.id).length > 0}
							<button
								type="button"
								data-root-id={r.id}
								data-active={activeRoot === r.id}
								class="relative flex flex-col items-center gap-1 rounded-xl px-1 py-1.5 transition
									{activeRoot === r.id ? 'bg-primary/10 ring-1 ring-primary/30' : 'hover:bg-accent'}"
								onclick={() => onPickRoot(r)}
							>
								<span
									class="relative grid h-11 w-11 place-items-center rounded-full text-xl"
									style={iconStyle(r, activeRoot === r.id)}
								>
									{r.icon || '📁'}
									{#if value === r.id}
										<span
											class="absolute -bottom-0.5 -right-0.5 grid h-4 w-4 place-items-center rounded-full bg-primary text-primary-foreground ring-2 ring-card"
										>
											<Check size={10} strokeWidth={3} />
										</span>
									{:else if hasChildren}
										<!-- 有下级的角标提示，可展开 -->
										<span class="absolute -bottom-0.5 -right-0.5 h-1.5 w-1.5 rounded-full bg-primary/70">
										</span>
									{/if}
								</span>
								<span
									class="w-full truncate text-center text-[11px] leading-tight
										{activeRoot === r.id ? 'font-medium text-primary' : 'text-foreground'}"
								>
									{r.name}
								</span>
							</button>
						{/each}
					</div>
				</div>

				<!-- 二级分类区：固定在面板底部（不参与滚动），与一级始终同屏 -->
				{#if activeRootCat && kids.length > 0}
					<div class="mt-2.5 shrink-0 rounded-xl border border-border/70 bg-muted/40 p-2">
						<div class="mb-1.5 flex items-center justify-between gap-2 px-0.5">
							<span class="flex min-w-0 items-center gap-1 text-[11px] text-muted-foreground">
								<CornerDownRight size={12} class="shrink-0" />
								<span class="truncate">{activeRootCat.name} · {kids.length} 个二级分类</span>
							</span>
							<button
								type="button"
								class="shrink-0 text-[11px] text-primary hover:underline"
								onclick={() => select(activeRootCat.id)}
							>
								直接选「{activeRootCat.name}」
							</button>
						</div>
						<!-- 二级分类很多时在自身区域内滚动，仍不把面板撑破 -->
						<div
							class="grid max-h-[34vh] grid-cols-5 gap-1 overflow-y-auto"
							data-testid="category-children"
						>
							{#each kids as ch (ch.id)}
								{@render tile(ch, value === ch.id)}
							{/each}
						</div>
					</div>
				{:else if activeRootCat}
					<div
						class="mt-2.5 shrink-0 rounded-xl border border-dashed border-border bg-muted/30 px-3 py-6 text-center text-xs text-muted-foreground"
					>
						「{activeRootCat.name}」没有二级分类，点击即可直接选用
					</div>
				{/if}
			{/if}
		</div>
	{/if}
</div>
