<script lang="ts">
	import { onMount } from 'svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Card from '$lib/components/ui/Card.svelte';
	import CardContent from '$lib/components/ui/CardContent.svelte';
	import Badge from '$lib/components/ui/Badge.svelte';
	import Dialog from '$lib/components/ui/Dialog.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import Label from '$lib/components/ui/Label.svelte';
	import { reimbApi } from '$lib/api/modules/reimbursements';
	import { appStore } from '$lib/stores/app';
	import { hzToast } from '$lib/components/ui/toast';
	import { formatMoney } from '$lib/utils/format';
	import type { Reimbursement } from '$lib/types';
	import { Plus, FileText, Pencil, Trash2 } from '@lucide/svelte';

	let list = $state<Reimbursement[]>([]);
	let loading = $state(true);

	// Dialog state
	let showDialog = $state(false);
	let editingItem = $state<Reimbursement | null>(null);
	let reimbName = $state('');
	let totalAmount = $state('');
	let receivedAmount = $state('');
	let remark = $state('');
	let saving = $state(false);

	async function loadData() {
		loading = true;
		try {
			list = await reimbApi.list();
		} catch {}
		loading = false;
	}

	onMount(loadData);

	function openNew() {
		editingItem = null;
		reimbName = '';
		totalAmount = '';
		receivedAmount = '0';
		remark = '';
		showDialog = true;
	}

	function openEdit(item: Reimbursement) {
		editingItem = item;
		reimbName = item.name;
		totalAmount = String(item.total_amount);
		receivedAmount = String(item.received_amount);
		remark = item.remark;
		showDialog = true;
	}

	async function handleSave() {
		const total = parseFloat(totalAmount);
		if (!reimbName.trim()) {
			hzToast.warning('请输入报销名称');
			return;
		}
		if (!total || total <= 0) {
			hzToast.warning('请输入总金额');
			return;
		}

		saving = true;
		try {
			const data = {
				name: reimbName.trim(),
				total_amount: total,
				received_amount: parseFloat(receivedAmount) || 0,
				remark: remark.trim(),
				book_id: appStore.currentBookId
			};

			if (editingItem) {
				await reimbApi.update(editingItem.id, data);
				hzToast.success('报销单已更新');
			} else {
				await reimbApi.create(data);
				hzToast.success('报销单已创建');
			}
			showDialog = false;
			await loadData();
		} catch (e: any) {
			hzToast.error(e.message || '保存失败');
		} finally {
			saving = false;
		}
	}

	async function handleDelete(item: Reimbursement) {
		if (!confirm(`确定删除「${item.name}」？`)) return;
		try {
			await reimbApi.remove(item.id);
			hzToast.success('已删除');
			await loadData();
		} catch (e: any) {
			hzToast.error(e.message || '删除失败');
		}
	}
</script>

<svelte:head>
	<title>报销管理 · 货殖</title>
</svelte:head>

<div class="space-y-4">
	<div class="flex items-center justify-between">
		<h2 class="text-sm font-medium">报销单</h2>
		<Button size="sm" onclick={openNew}>
			<Plus size={16} />
			新建报销
		</Button>
	</div>
	{#if loading}
		<div class="space-y-2">
			{#each [1, 2, 3] as i}
				<div class="h-16 rounded-lg animate-pulse bg-muted" />
			{/each}
		</div>
	{:else if list.length === 0}
		<Card>
			<div class="py-16 text-center">
				<FileText size={32} class="mx-auto mb-3 text-muted-foreground opacity-50" />
				<p class="text-muted-foreground text-sm">暂无报销单</p>
			</div>
		</Card>
	{:else}
		<Card>
			<CardContent class="p-0 divide-y">
				{#each list as item (item.id)}
					<div class="flex items-center gap-3 p-4 group">
						<div class="flex-1 min-w-0">
							<div class="font-medium truncate">{item.name}</div>
							<div class="text-xs text-muted-foreground">
								共 {item.transaction_ids.length} 笔交易
								{#if item.remark}
									· {item.remark}
								{/if}
							</div>
						</div>
						<div class="text-right flex items-center gap-1">
							<div>
								<div class="font-semibold tabular-nums">
									{formatMoney(item.received_amount)}
									<span class="text-muted-foreground text-sm">
										/ {formatMoney(item.total_amount)}
									</span>
								</div>
								<Badge
									variant={
										item.status === 'done'
											? 'default'
											: item.status === 'partial'
												? 'secondary'
												: 'outline'
									}
									class="mt-1"
								>
									{item.status === 'done' ? '已收齐' : item.status === 'partial' ? '部分' : '待处理'}
								</Badge>
							</div>
							<button
								class="p-1 rounded hover:bg-accent opacity-0 group-hover:opacity-100 transition"
								onclick={() => openEdit(item)}
							>
								<Pencil size={14} class="text-muted-foreground" />
							</button>
							<button
								class="p-1 rounded hover:bg-destructive/10 opacity-0 group-hover:opacity-100 transition"
								onclick={() => handleDelete(item)}
							>
								<Trash2 size={14} class="text-destructive" />
							</button>
						</div>
					</div>
				{/each}
			</CardContent>
		</Card>
	{/if}
</div>

<!-- 新建/编辑 Dialog -->
<Dialog bind:open={showDialog}>
	<div class="space-y-4">
		<h3 class="text-lg font-semibold">
			{editingItem ? '编辑报销单' : '新建报销单'}
		</h3>

		<div class="space-y-2">
			<Label>报销名称</Label>
			<Input bind:value={reimbName} placeholder="例如: 差旅费报销" />
		</div>

		<div class="grid grid-cols-2 gap-4">
			<div class="space-y-2">
				<Label>总金额</Label>
				<div class="relative">
					<span class="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground">¥</span>
					<Input class="pl-8" type="number" step="0.01" placeholder="0.00" bind:value={totalAmount} />
				</div>
			</div>
			<div class="space-y-2">
				<Label>已收金额</Label>
				<div class="relative">
					<span class="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground">¥</span>
					<Input class="pl-8" type="number" step="0.01" placeholder="0.00" bind:value={receivedAmount} />
				</div>
			</div>
		</div>

		<div class="space-y-2">
			<Label>备注</Label>
			<Input bind:value={remark} placeholder="可选" />
		</div>

		<div class="flex gap-2 justify-end pt-2">
			<Button variant="outline" onclick={() => (showDialog = false)}>取消</Button>
			<Button onclick={handleSave} disabled={saving}>
				{saving ? '保存中...' : '保存'}
			</Button>
		</div>
	</div>
</Dialog>
