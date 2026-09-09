<script lang="ts">
	import { onMount } from 'svelte';
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
	import { Plus, Pencil, Trash2, GripVertical, ChevronDown } from '@lucide/svelte';

	let kind = $state<CategoryKind>('expense');
	let showDialog = $state(false);
	let editingCategory = $state<Category | null>(null);
	let catName = $state('');
	let catIcon = $state('📁');
	let catParentId = $state<number | ''>('');
	let loading = $state(false);

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

	<!-- 分类列表 -->
	<Card>
		<CardContent class="p-4">
			{#if topCategories.length === 0}
				<div class="py-12 text-center text-muted-foreground">
					<p class="text-sm">暂无分类</p>
				</div>
			{:else}
				<div class="space-y-3">
					{#each topCategories as cat (cat.id)}
						<div class="border rounded-lg overflow-hidden bg-card">
							<!-- 顶级分类行 -->
							<button
								class="w-full flex items-center gap-3 p-3 hover:bg-accent/50 transition text-left group"
								onclick={() => openEdit(cat)}
							>
								<span class="text-xl">{cat.icon || '📁'}</span>
								<span class="text-sm font-medium truncate flex-1">{cat.name}</span>
								{#if cat.is_system}
									<Badge variant="outline" class="text-[9px]">系统</Badge>
								{/if}
								<ChevronDown size={14} class="text-muted-foreground transition-transform duration-200 group-hover:rotate-180" />
							</button>

							<!-- 子分类 -->
							{#if childrenMap.get(cat.id)?.length}
								<div class="pl-8 border-t bg-muted/30 divide-y divide-muted">
									{#each childrenMap.get(cat.id) as child (child.id)}
										<button
											class="w-full flex items-center gap-3 px-3 py-2 hover:bg-accent/50 transition text-left"
											onclick={() => openEdit(child)}
										>
											<span class="text-lg">{child.icon || '📁'}</span>
											<span class="text-sm truncate flex-1">{child.name}</span>
											{#if !child.is_system}
												<div class="hidden group-hover:flex gap-1">
													<button
														class="p-1 rounded hover:bg-accent"
														onclick={(e) => { e.stopPropagation(); openEdit(child); }}
													>
														<Pencil size={12} />
													</button>
													<button
														class="p-1 rounded hover:bg-destructive/10 text-destructive"
														onclick={(e) => { e.stopPropagation(); handleDelete(child); }}
													>
														<Trash2 size={12} />
													</button>
												</div>
											{/if}
										</button>
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