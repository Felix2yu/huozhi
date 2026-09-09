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
	import { Plus, Pencil, Trash2, GripVertical } from '@lucide/svelte';

	let kind = $state<CategoryKind>('expense');
	let showDialog = $state(false);
	let editingCategory = $state<Category | null>(null);
	let catName = $state('');
	let catIcon = $state('📁');
	let loading = $state(false);

	let currentList = $derived(
		kind === 'expense'
			? appStore.categories.expense
			: kind === 'income'
				? appStore.categories.income
				: appStore.categories.system
	);

	function openNew() {
		editingCategory = null;
		catName = '';
		catIcon = '📁';
		showDialog = true;
	}

	function openEdit(cat: Category) {
		editingCategory = cat;
		catName = cat.name;
		catIcon = cat.icon || '📁';
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
					icon: catIcon
				});
				hzToast.success('更新成功');
			} else {
				await categoryApi.create({
					name: catName,
					icon: catIcon,
					kind,
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
			<div class="grid grid-cols-3 sm:grid-cols-4 md:grid-cols-5 lg:grid-cols-6 gap-3">
				{#each currentList as cat (cat.id)}
					<div class="relative group">
						<button
							class="w-full flex flex-col items-center gap-2 p-3 rounded-lg border hover:bg-accent transition"

						>
							<span class="text-2xl">{cat.icon || '📁'}</span>
							<span class="text-xs text-center truncate w-full">{cat.name}</span>
							{#if cat.is_system}
								<Badge variant="outline" class="absolute top-1 right-1 text-[9px]">系统</Badge>
							{/if}
						</button>
						{#if !cat.is_system}
							<div class="absolute inset-0 hidden group-hover:flex items-center justify-center gap-1 bg-background/80 rounded-lg">
								<button
									class="p-1.5 rounded hover:bg-accent"
									onclick={() => openEdit(cat)}
								>
									<Pencil size={14} />
								</button>
								<button
									class="p-1.5 rounded hover:bg-destructive/10 text-destructive"
									onclick={() => handleDelete(cat)}
								>
									<Trash2 size={14} />
								</button>
							</div>
						{/if}
					</div>
				{/each}
			</div>

			{#if currentList.length === 0}
				<div class="py-12 text-center text-muted-foreground">
					<p class="text-sm">暂无分类</p>
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

			<div class="flex gap-2 justify-end pt-2">
				<Button variant="outline" onclick={() => (showDialog = false)}>取消</Button>
				<Button onclick={handleSave} disabled={loading}>
					{loading ? '保存中...' : '保存'}
				</Button>
			</div>
		</div>
	</Dialog>
</div>
