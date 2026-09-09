<script lang="ts">
	import Button from '$lib/components/ui/Button.svelte';
	import Card from '$lib/components/ui/Card.svelte'
	import CardContent from '$lib/components/ui/CardContent.svelte'
	import CardHeader from '$lib/components/ui/CardHeader.svelte'
	import CardTitle from '$lib/components/ui/CardTitle.svelte';
	import Tabs from '$lib/components/ui/Tabs.svelte'
	import TabsTrigger from '$lib/components/ui/TabsTrigger.svelte';
	import Dialog from '$lib/components/ui/Dialog.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import Label from '$lib/components/ui/Label.svelte';
	import Badge from '$lib/components/ui/Badge.svelte';
	import { appStore } from '$lib/stores/app';
	import { categoryApi } from '$lib/api/modules/categories';
	import { hzToast } from '$lib/components/ui/toast';
	import type { Category, CategoryKind } from '$lib/types';
	import { Plus, Pencil, Trash2, ChevronDown } from '@lucide/svelte';

	let kind = $state<CategoryKind>('expense');
	let showDialog = $state(false);
	let editingCategory = $state<Category | null>(null);
	let catName = $state('');
	let catIcon = $state('📁');
	let catParentId = $state<number | ''>('');
	let loading = $state(false);
	let expanded = $state<Record<number, boolean>>({});

	function toggleExpand(id: number) {
		expanded[id] = !expanded[id];
	}

	let allCategories = $derived(
		kind === 'expense'
			? appStore.categories.expense
			: kind === 'income'
				? appStore.categories.income
				: appStore.categories.system
	);

	// 顶级分类（parent_id === 0）
	let topCategories = $derived(
		allCategories.filter((c) => c.parent_id === 0)
	);

	// 子分类按 parent_id 分组
	let childrenMap = $derived.by(() => {
		const map = new Map<number, Category[]>();
		for (const cat of allCategories) {
			if (cat.parent_id !== 0) {
				const arr = map.get(cat.parent_id) || [];
				arr.push(cat);
				map.set(cat.parent_id, arr);
			}
		}
		return map;
	});

	let parentOptions = $derived(
		topCategories.filter((c) => c.id !== (editingCategory?.id || 0))
	);

	function openNew() {
		editingCategory = null;
		catName = '';
		catIcon = '📁';
		catParentId = '';
		showDialog = true;
	}

	function openEdit(cat: Category) {
		editingCategory = cat;
		catName = cat.name;
		catIcon = cat.icon || '📁';
		catParentId = cat.parent_id || '';
		showDialog = true;
	}

	async function handleSave() {
		if (!catName.trim()) {
			hzToast.warning('请输入分类名称');
			return;
		}
		loading = true;
		try {
			if (editingCategory) {
				await categoryApi.update(editingCategory.id, {
					name: catName,
					icon: catIcon,
					parent_id: catParentId || 0
				});
				hzToast.success('更新成功');
			} else {
				await categoryApi.create({
					name: catName,
					icon: catIcon,
					kind,
					parent_id: catParentId || 0,
					book_id: appStore.currentBookId
				});
				hzToast.success('创建成功');
			}
			showDialog = false;
			await appStore.loadDictionaries();
		} catch (e: any) {
			hzToast.error(e.message || '保存失败');
		} finally {
			loading = false;
		}
	}

	async function handleDelete(cat: Category) {
		if (!confirm(`确定删除分类 "${cat.name}"？`)) return;
		try {
			await categoryApi.remove(cat.id);
			hzToast.success('已删除');
			await appStore.loadDictionaries();
		} catch (e: any) {
			hzToast.error(e.message || '删除失败');
		}
	}

	const iconOptions = ['📁', '🍔', '🚗', '🛒', '🏠', '💡', '🎮', '📱', '👕', '💼', '📚', '🏥', '🎁', '💳', '💰', '💵', '🏦', '📈', '🎓', '✈️', '🏋️', '🐶', '💄', '🎬', '🍳', '☕', '🚕', '🏧', '📦', '🔧'];
</script>

<svelte:head>
	<title>分类管理 · 货殖</title>
</svelte:head>

<div class="space-y-4">
	<div class="flex items-center justify-between">
		<Tabs bind:value={kind}>
			<TabsTrigger value="expense">支出</TabsTrigger>
			<TabsTrigger value="income">收入</TabsTrigger>
			<TabsTrigger value="system">系统</TabsTrigger>
		</Tabs>
		<Button size="sm" onclick={openNew}>
			<Plus size={16} />
			新增
		</Button>
	</div>

	<!-- 分类列表：紧凑换行排列 + 点击展开层级 -->
	<Card>
		<CardContent class="p-4">
			{#if topCategories.length === 0}
				<div class="py-12 text-center text-muted-foreground">
					<p class="text-sm">暂无分类</p>
				</div>
			{:else}
				<div class="flex flex-wrap gap-2">
					{#each topCategories as cat (cat.id)}
						{@const children = childrenMap.get(cat.id) || []}
						<!-- 顶层分类：按内容宽度的卡片，不再占满整行 -->
						<div class="group rounded-lg border bg-card overflow-hidden">
							<div class="flex items-center">
								<button
									class="flex items-center gap-2 px-3 py-2 hover:bg-accent/50 transition text-left min-w-0"
									onclick={() => (children.length ? toggleExpand(cat.id) : openEdit(cat))}
								>
									<span class="text-lg leading-none">{cat.icon || '📁'}</span>
									<span class="text-sm font-medium">{cat.name}</span>
									{#if cat.is_system}
										<Badge variant="outline" class="text-[9px]">系统</Badge>
									{/if}
									{#if children.length}
										<ChevronDown
											size={14}
											class="text-muted-foreground transition-transform duration-200 {expanded[cat.id] ? 'rotate-180' : ''}"
										/>
									{/if}
								</button>
								{#if !cat.is_system}
									<button
										class="p-2 text-muted-foreground hover:bg-accent/50 opacity-0 group-hover:opacity-100 shrink-0"
										onclick={(e) => { e.stopPropagation(); openEdit(cat); }}
										title="编辑"
									>
										<Pencil size={13} />
									</button>
								{/if}
							</div>

							<!-- 子分类：展开后内嵌显示，缩进 + 更小尺寸以区分层级 -->
							{#if children.length && expanded[cat.id]}
								<div class="flex flex-wrap gap-1.5 px-3 pb-2 pt-1 border-t bg-muted/30">
									{#each children as child (child.id)}
										<div class="group/chip flex items-center gap-1.5 pl-2 pr-1 py-1 rounded-md border bg-background hover:bg-accent/40 transition">
											<span class="text-sm leading-none">{child.icon || '📁'}</span>
											<span class="text-xs">{child.name}</span>
											{#if !child.is_system}
												<div class="hidden group-hover/chip:flex items-center gap-0.5 ml-0.5">
													<button
														class="p-0.5 rounded hover:bg-accent"
														onclick={(e) => { e.stopPropagation(); openEdit(child); }}
													>
														<Pencil size={11} />
													</button>
													<button
														class="p-0.5 rounded hover:bg-destructive/10 text-destructive"
														onclick={(e) => { e.stopPropagation(); handleDelete(child); }}
													>
														<Trash2 size={11} />
													</button>
												</div>
											{/if}
										</div>
									{/each}
								</div>
							{/if}
						</div>
					{/each}
				</div>
			{/if}
		</CardContent>
	</Card>

	<!-- 新增/编辑 Dialog -->
	<Dialog bind:open={showDialog}>
		<div class="space-y-4">
			<h3 class="text-lg font-semibold">
				{editingCategory ? '编辑分类' : '新增分类'}
			</h3>

			<div class="space-y-2">
				<Label>名称</Label>
				<Input bind:value={catName} placeholder="分类名称" />
			</div>

			<div class="space-y-2">
				<Label>图标</Label>
				<div class="grid grid-cols-10 gap-1 max-h-40 overflow-y-auto p-2 rounded-lg border">
					{#each iconOptions as icon}
						<button
							class="w-8 h-8 text-lg rounded hover:bg-accent transition"
							onclick={() => (catIcon = icon)}
						>
							{icon}
						</button>
					{/each}
				</div>
			</div>

			<div class="space-y-2">
				<Label>父分类</Label>
				<select
					class="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm focus:outline-none focus:ring-1 focus:ring-ring"
					bind:value={catParentId}
				>
					<option value="">顶级分类</option>
					{#each parentOptions as p}
						<option value={p.id}>{p.icon || '📁'} {p.name}</option>
					{/each}
				</select>
			</div>

			<div class="flex gap-2 justify-end pt-2">
				<Button variant="outline" onclick={() => (showDialog = false)}>取消</Button>
				<Button onclick={handleSave} disabled={loading}>
					{loading ? '保存中...' : '保存'}
				</Button>
			</div>
		</div>
	</Dialog>
</div>