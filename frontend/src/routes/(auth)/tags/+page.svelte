<script lang="ts">
	import { onMount } from 'svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Card from '$lib/components/ui/Card.svelte';
	import CardContent from '$lib/components/ui/CardContent.svelte';
	import Badge from '$lib/components/ui/Badge.svelte';
	import Dialog from '$lib/components/ui/Dialog.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import Label from '$lib/components/ui/Label.svelte';
	import { tagApi } from '$lib/api/modules/tags';
	import { appStore } from '$lib/stores/app';
	import { hzToast } from '$lib/components/ui/toast';
	import { Plus, Tag, Pencil, Trash2 } from '@lucide/svelte';

	let showDialog = $state(false);
	let editingTag = $state<{ id: number; name: string } | null>(null);
	let tagName = $state('');
	let saving = $state(false);

	onMount(() => appStore.loadDictionaries());

	function openNew() {
		editingTag = null;
		tagName = '';
		showDialog = true;
	}

	function openEdit(tag: { id: number; name: string }) {
		editingTag = tag;
		tagName = tag.name;
		showDialog = true;
	}

	async function handleSave() {
		if (!tagName.trim()) {
			hzToast.warning('请输入标签名称');
			return;
		}
		saving = true;
		try {
			if (editingTag) {
				await tagApi.update(editingTag.id, { name: tagName.trim() });
				hzToast.success('标签已更新');
			} else {
				await tagApi.create({ name: tagName.trim() });
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
		if (!confirm(`确定删除标签「${tag.name}」？`)) return;
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

<div class="space-y-4">
	<div class="flex items-center justify-between">
		<h2 class="text-sm font-medium">所有标签</h2>
		<Button size="sm" onclick={openNew}>
			<Plus size={16} />
			新增标签
		</Button>
	</div>
	<Card>
		<CardContent class="p-4">
			{#if appStore.tags.length === 0}
				<div class="py-12 text-center text-muted-foreground">
					<Tag size={32} class="mx-auto mb-3 opacity-50" />
					<p class="text-sm">暂无标签</p>
				</div>
			{:else}
				<div class="flex flex-wrap gap-2">
					{#each appStore.tags as tag (tag.id)}
						<div class="group relative inline-flex items-center">
							<Badge variant="secondary" class="cursor-pointer hover:bg-secondary/80 pr-1">
								{tag.name}
								<span class="ml-1 text-muted-foreground">({tag.count})</span>
								<button
									class="ml-1 p-0.5 rounded hover:bg-background/50 opacity-0 group-hover:opacity-100 transition"
									onclick={() => openEdit(tag)}
								>
									<Pencil size={10} />
								</button>
								<button
									class="p-0.5 rounded hover:bg-destructive/10 text-destructive opacity-0 group-hover:opacity-100 transition"
									onclick={() => handleDelete(tag)}
								>
									<Trash2 size={10} />
								</button>
							</Badge>
						</div>
					{/each}
				</div>
			{/if}
		</CardContent>
	</Card>
</div>

<!-- 新增/编辑 Dialog -->
<Dialog bind:open={showDialog}>
	<div class="space-y-4">
		<h3 class="text-lg font-semibold">
			{editingTag ? '编辑标签' : '新增标签'}
		</h3>
		<div class="space-y-2">
			<Label>标签名称</Label>
			<Input bind:value={tagName} placeholder="例如: 餐饮、交通" />
		</div>
		<div class="flex gap-2 justify-end pt-2">
			<Button variant="outline" onclick={() => (showDialog = false)}>取消</Button>
			<Button onclick={handleSave} disabled={saving}>
				{saving ? '保存中...' : '保存'}
			</Button>
		</div>
	</div>
</Dialog>
