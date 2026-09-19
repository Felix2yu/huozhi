<script lang="ts">
	import { onMount } from 'svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Card from '$lib/components/ui/Card.svelte';
	import CardContent from '$lib/components/ui/CardContent.svelte';
	import Badge from '$lib/components/ui/Badge.svelte';
	import Dialog from '$lib/components/ui/Dialog.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import Label from '$lib/components/ui/Label.svelte';
	import AccountSelect from '$lib/components/AccountSelect.svelte';
	import { reimbApi } from '$lib/api/modules/reimbursements';
	import { appStore } from '$lib/stores/app';
	import { hzToast } from '$lib/components/ui/toast';
	import { formatMoney, formatDate } from '$lib/utils/format';
	import type { Reimbursement } from '$lib/types';
	import { Plus, FileText, Pencil, Trash2, Wallet } from '@lucide/svelte';

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

	// B5：报销到账此前只改 status，不生成收款交易、不进账户 —— 钱在资产里凭空消失。
	// 后端 UpdateReimbursement 要求 status（required, oneof=pending received partial），
	// 前端此前根本不传 status，更新请求必然 400。这里补齐三个字段：
	//   status  —— 待处理 / 部分到账 / 已收齐
	//   account —— 收款账户，到账金额按此账户生成一条收入交易
	//   date    —— 到账日期
	let status = $state<'pending' | 'received' | 'partial'>('pending');
	let accountId = $state<number>(0);
	let receivedDate = $state(formatDate(new Date(), 'YYYY-MM-DD'));

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
		status = 'pending';
		accountId = appStore.accounts[0]?.id || 0;
		receivedDate = formatDate(new Date(), 'YYYY-MM-DD');
		showDialog = true;
	}

	function openEdit(item: Reimbursement) {
		editingItem = item;
		reimbName = item.name;
		totalAmount = String(item.total_amount);
		receivedAmount = String(item.received_amount);
		remark = item.remark;
		status = (item.status as any) || 'pending';
		accountId = appStore.accounts[0]?.id || 0;
		// 未到账时后端返回零值时间（0001-01-01），不能直接展示
		const ra = (item.received_at || '').slice(0, 10);
		receivedDate = ra && !ra.startsWith('0001') ? ra : formatDate(new Date(), 'YYYY-MM-DD');
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
			if (editingItem) {
				// 更新：后端要求 status 必填；已到账/部分到账必须指定收款账户，否则无法入账
				if (status !== 'pending' && !accountId) {
					hzToast.warning('请选择收款账户');
					saving = false;
					return;
				}
				await reimbApi.update(editingItem.id, {
					status,
					received_amount: parseFloat(receivedAmount) || 0,
					remark: remark.trim(),
					account_id: accountId,
					received_date: receivedDate
				});
				hzToast.success(status === 'pending' ? '报销单已更新' : '已登记到账并生成收入交易');
			} else {
				await reimbApi.create({
					name: reimbName.trim(),
					total_amount: total,
					remark: remark.trim(),
					book_id: appStore.effectiveBookId()
				});
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
				<div class="h-16 rounded-lg animate-pulse bg-muted" ></div>
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
								<!-- transaction_ids 可能是 null（未关联交易的报销单），
								     直接取 .length 会抛 TypeError 让整页白屏 -->
								共 {item.transaction_ids?.length ?? 0} 笔交易
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
										item.status === 'received'
											? 'default'
											: item.status === 'partial'
												? 'secondary'
												: 'outline'
									}
									class="mt-1"
								>
									<!-- 后端 status 取值是 pending / received / partial，
									     此前前端判断 'done'，永远匹配不上，已收齐也显示「待处理」 -->
									{item.status === 'received'
										? '已收齐'
										: item.status === 'partial'
											? '部分到账'
											: '待处理'}
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

		<div class="space-y-2">
			<Label>总金额</Label>
			<div class="relative">
				<span class="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground">¥</span>
				<Input
					class="pl-8"
					type="number"
					step="0.01"
					placeholder="0.00"
					bind:value={totalAmount}
					disabled={!!editingItem}
				/>
			</div>
			{#if editingItem}
				<p class="text-[11px] text-muted-foreground">总金额创建后不可修改</p>
			{/if}
		</div>

		{#if editingItem}
			<!-- B5：到账登记。填写后会在所选账户生成一条「报销到账」收入交易 -->
			<div class="grid grid-cols-2 gap-4">
				<div class="space-y-2">
					<Label>报销状态</Label>
					<select
						class="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm focus:outline-none focus:ring-1 focus:ring-ring"
						bind:value={status}
					>
						<option value="pending">待处理</option>
						<option value="partial">部分到账</option>
						<option value="received">已收齐</option>
					</select>
				</div>
				<div class="space-y-2">
					<Label>已收金额</Label>
					<div class="relative">
						<span class="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground">¥</span>
						<Input
							class="pl-8"
							type="number"
							step="0.01"
							placeholder="0.00"
							bind:value={receivedAmount}
						/>
					</div>
				</div>
			</div>

			{#if status !== 'pending'}
				<div class="grid grid-cols-2 gap-4">
					<div class="space-y-2">
						<Label>收款账户</Label>
						<AccountSelect bind:value={accountId} placeholder="选择收款账户" />
					</div>
					<div class="space-y-2">
						<Label>到账日期</Label>
						<Input type="date" bind:value={receivedDate} />
					</div>
				</div>
				<p class="flex items-start gap-1.5 text-[11px] text-muted-foreground">
					<Wallet size={12} class="mt-0.5 shrink-0" />
					确认后将按「已收金额」在所选账户生成一笔收入交易；未填则按总金额入账。
				</p>
			{/if}
		{/if}

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
