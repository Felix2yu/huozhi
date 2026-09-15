<script lang="ts">
	import { onMount } from 'svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Card from '$lib/components/ui/Card.svelte';
	import CardContent from '$lib/components/ui/CardContent.svelte';
	import Dialog from '$lib/components/ui/Dialog.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import Label from '$lib/components/ui/Label.svelte';
	import { tagApi } from '$lib/api/modules/tags';
	import { appStore } from '$lib/stores/app';
	import { hzToast } from '$lib/components/ui/toast';
	import { Plus, Tag, Pencil, Trash2, Hash } from '@lucide/svelte';

	// 预设配色（与标签 dot / 卡片强调色一致）
	const PALETTE = [
		'#ef4444', '#f97316', '#eab308', '#22c55e',
		'#14b8a6', '#06b6d4', '#3b82f6', '#6366f1',
		'#8b5cf6', '#ec4899', '#f43f5e', '#64748b'
	];

	let showDialog = $state(false);
	let editingTag = $state<{ id: number; name: string; color: string; sort: number } | null>(null);
	let tagName = $state('');
	let selectedColor = $state('');
	let saving = $state(false);

	const totalUsage = $derived(
		appStore.tags.reduce((sum, t) => sum + (t.count || 0), 0)
	);

	onMount(() => appStore.loadDictionaries());

	function openNew() {
		editingTag = null;
		tagName = '';
		selectedColor = PALETTE[0];
		showDialog = true;
	}

	function openEdit(tag: { id: number; name: string; color: string; sort: number }) {
		editingTag = tag;
		tagName = tag.name;
		selectedColor = tag.color || '';
		showDialog = true;
	}

	function pickColor(c: string) {
		selectedColor = c;
	}

	async function handleSave() {
		if (!tagName.trim()) {
			hzToast.warning('请输入标签名称');
			return;
		}
		saving = true;
		try {
			if (editingTag) {
				await tagApi.update(editingTag.id, {
					name: tagName.trim(),
					color: selectedColor,
					sort: editingTag.sort
				});
				hzToast.success('标签已更新');
			} else {
				await tagApi.create({
					name: tagName.trim(),
					color: selectedColor,
					sort: 0
				});
				hzToast.success('标签已创建');
			}
			showDialog = false;
			await appStore.loadDictionaries();
		} catch (e: any) {
			hzToast.error(e.message || '保存失败');
		} finally {
			saving = false;
		}
	}

	async function handleDelete(tag: { id: number; name: string }) {
		if (!confirm(`确定删除标签「${tag.name}」？\n删除后该标签将从所有交易中移除。`)) return;
		try {
			await tagApi.remove(tag.id);
			hzToast.success('已删除');
			await appStore.loadDictionaries();
		} catch (e: any) {
			hzToast.error(e.message || '删除失败');
		}
	}
</script>

<svelte:head>
	<title>标签中心 · 货殖</title>
</svelte:head>

<div class="space-y-6">
	<header class="flex flex-wrap items-end justify-between gap-3">
		<div>
			<h1 class="text-xl font-semibold tracking-tight">标签中心</h1>
			<p class="mt-1 text-sm text-muted-foreground">
				{#if appStore.tags.length}
					共 {appStore.tags.length} 个标签 · 累计使用 {totalUsage} 次
				{:else}
					用标签标记消费场景，方便日后筛选与统计
				{/if}
			</p>
		</div>
		<Button onclick={openNew}>
			<Plus size={16} />
			新增标签
		</Button>
	</header>

	{#if appStore.tags.length === 0}
		<Card>
			<CardContent class="py-16">
				<div class="flex flex-col items-center text-center text-muted-foreground">
					<Tag size={40} class="mb-3 opacity-40" />
					<p class="text-sm">还没有标签</p>
					<p class="mt-1 text-xs">点右上角「新增标签」，给高频商户（如拼多多、美团）建个标签吧</p>
				</div>
			</CardContent>
		</Card>
	{:else}
		<div class="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
			{#each appStore.tags as tag (tag.id)}
				<div
					class="group relative flex flex-col rounded-xl border bg-card p-3 shadow-sm transition-all hover:border-primary/40 hover:shadow-md"
				>
					<div class="flex items-center gap-2 pr-12">
						<span
							class="h-3 w-3 shrink-0 rounded-full ring-1 ring-black/5"
							style:background={tag.color || '#cbd5e1'}
						></span>
						<span class="truncate text-sm font-medium" title={tag.name}>{tag.name}</span>
					</div>

					<div class="mt-3 flex items-center gap-1 text-xs text-muted-foreground">
						<Hash size={12} />
						{tag.count || 0} 笔
					</div>

					<div
						class="absolute right-2 top-2 flex gap-1 opacity-0 transition-opacity group-hover:opacity-100 focus-within:opacity-100"
					>
						<button
							type="button"
							class="rounded-md p-1 text-muted-foreground hover:bg-accent hover:text-foreground"
							title="编辑"
							onclick={() => openEdit(tag)}
						>
							<Pencil size={13} />
						</button>
						<button
							type="button"
							class="rounded-md p-1 text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
							title="删除"
							onclick={() => handleDelete(tag)}
						>
							<Trash2 size={13} />
						</button>
					</div>
				</div>
			{/each}
		</div>
	{/if}
</div>

<!-- 新增/编辑 Dialog -->
<Dialog bind:open={showDialog}>
	<div class="space-y-5">
		<h3 class="text-lg font-semibold">{editingTag ? '编辑标签' : '新增标签'}</h3>

		<div class="space-y-2">
			<Label>标签名称</Label>
			<Input bind:value={tagName} placeholder="例如：餐饮、交通、拼多多" maxlength={50} />
		</div>

		<div class="space-y-2">
			<Label>颜色</Label>
			<div class="flex flex-wrap items-center gap-2">
				<!-- 无颜色 -->
				<button
					type="button"
					class="flex h-7 w-7 items-center justify-center rounded-full border ring-offset-2 transition
						{selectedColor === '' ? 'ring-2 ring-primary' : 'hover:scale-105'}"
					title="无颜色"
					onclick={() => pickColor('')}
				>
					<span class="h-4 w-4 rounded-full border border-dashed border-muted-foreground"></span>
				</button>
				{#each PALETTE as c (c)}
					<button
						type="button"
						class="h-7 w-7 rounded-full transition ring-offset-2
							{selectedColor === c ? 'ring-2 ring-primary' : 'hover:scale-105'}"
						style:background={c}
						title={c}
						onclick={() => pickColor(c)}
					></button>
				{/each}
			</div>
		</div>

		<div class="flex gap-2 justify-end pt-1">
			<Button variant="outline" onclick={() => (showDialog = false)}>取消</Button>
			<Button onclick={handleSave} disabled={saving}>
				{saving ? '保存中…' : '保存'}
			</Button>
		</div>
	</div>
</Dialog>
