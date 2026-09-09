<script lang="ts">
	import { onMount } from 'svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Card from '$lib/components/ui/Card.svelte';
	import CardContent from '$lib/components/ui/CardContent.svelte';
	import Progress from '$lib/components/ui/Progress.svelte';
	import Dialog from '$lib/components/ui/Dialog.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import Label from '$lib/components/ui/Label.svelte';
	import { installmentApi } from '$lib/api/modules/installments';
	import { appStore } from '$lib/stores/app';
	import { hzToast } from '$lib/components/ui/toast';
	import { formatMoney } from '$lib/utils/format';
	import type { Installment } from '$lib/types';
	import { Plus, CreditCard, Trash2 } from '@lucide/svelte';

	let list = $state<Installment[]>([]);
	let loading = $state(true);

	// Dialog state
	let showDialog = $state(false);
	let name = $state('');
	let totalAmount = $state('');
	let totalMonths = $state('');
	let monthlyAmount = $state('');
	let interestAmount = $state('');
	let categoryId = $state<number>(0);
	let accountId = $state<number>(0);
	let firstRepayDate = $state('');
	let saving = $state(false);

	async function loadData() {
		loading = true;
		try {
			list = await installmentApi.list();
		} catch {}
		loading = false;
	}

	onMount(loadData);

	function openNew() {
		name = '';
		totalAmount = '';
		totalMonths = '';
		monthlyAmount = '';
		interestAmount = '0';
		categoryId = appStore.categories.expense[0]?.id || 0;
		accountId = appStore.accounts[0]?.id || 0;
		firstRepayDate = new Date().toISOString().split('T')[0];
		showDialog = true;
	}

	async function handleSave() {
		const total = parseFloat(totalAmount);
		const months = parseInt(totalMonths);
		const monthly = parseFloat(monthlyAmount);

		if (!name.trim()) {
			hzToast.warning('请输入名称');
			return;
		}
		if (!total || total <= 0) {
			hzToast.warning('请输入总金额');
			return;
		}
		if (!months || months <= 0) {
			hzToast.warning('请输入分期期数');
			return;
		}

		saving = true;
		try {
			await installmentApi.create({
				name: name.trim(),
				total_amount: total,
				total_months: months,
				monthly_amount: monthly || total / months,
				interest_amount: parseFloat(interestAmount) || 0,
				category_id: categoryId,
				account_id: accountId,
				first_repay_date: firstRepayDate,
				book_id: appStore.currentBookId
			});
			hzToast.success('分期已创建');
			showDialog = false;
			await loadData();
		} catch (e: any) {
			hzToast.error(e.message || '创建失败');
		} finally {
			saving = false;
		}
	}

	async function handleDelete(item: Installment) {
		if (!confirm(`确定删除「${item.name}」？`)) return;
		try {
			await installmentApi.remove(item.id);
			hzToast.success('已删除');
			await loadData();
		} catch (e: any) {
			hzToast.error(e.message || '删除失败');
		}
	}
</script>

<svelte:head>
	<title>分期管理 · 货殖</title>
</svelte:head>

<div class="space-y-4">
	<div class="flex items-center justify-between">
		<h2 class="text-sm font-medium">进行中的分期</h2>
		<Button size="sm" onclick={openNew}>
			<Plus size={16} />
			新增分期
		</Button>
	</div>
	{#if loading}
		<div class="grid gap-3 md:grid-cols-2">
			{#each [1, 2] as i}
				<div class="h-32 rounded-xl animate-pulse bg-muted" ></div>
			{/each}
		</div>
	{:else if list.length === 0}
		<Card>
			<div class="py-16 text-center">
				<CreditCard size={32} class="mx-auto mb-3 text-muted-foreground opacity-50" />
				<p class="text-muted-foreground text-sm">暂无分期</p>
			</div>
		</Card>
	{:else}
		<div class="grid gap-3 md:grid-cols-2">
			{#each list as item (item.id)}
				<Card class="group">
					<CardContent class="p-4">
						<div class="flex items-center justify-between mb-2">
							<div class="font-medium truncate">{item.name}</div>
							<div class="flex items-center gap-1">
								<span class="text-xs text-muted-foreground">
									{item.status === 'active' ? '还款中' : '已结清'}
								</span>
								<button
									class="p-1 rounded hover:bg-destructive/10 opacity-0 group-hover:opacity-100 transition"
									onclick={() => handleDelete(item)}
								>
									<Trash2 size={14} class="text-destructive" />
								</button>
							</div>
						</div>
						<Progress value={(item.paid_months / item.total_months) * 100} />
						<div class="flex items-center justify-between mt-2 text-sm">
							<div>
								<span class="font-semibold">{item.paid_months}/{item.total_months}</span>
								<span class="text-muted-foreground ml-1">期</span>
							</div>
							<div class="font-semibold tabular-nums">
								月供 {formatMoney(item.monthly_amount)}
							</div>
						</div>
					</CardContent>
				</Card>
			{/each}
		</div>
	{/if}
</div>

<!-- 新建 Dialog -->
<Dialog bind:open={showDialog}>
	<div class="space-y-4">
		<h3 class="text-lg font-semibold">新增分期</h3>

		<div class="space-y-2">
			<Label>名称</Label>
			<Input bind:value={name} placeholder="例如: 手机分期" />
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
				<Label>分期期数</Label>
				<Input type="number" min={1} placeholder="12" bind:value={totalMonths} />
			</div>
		</div>

		<div class="grid grid-cols-2 gap-4">
			<div class="space-y-2">
				<Label>月供金额</Label>
				<div class="relative">
					<span class="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground">¥</span>
					<Input class="pl-8" type="number" step="0.01" placeholder="自动计算" bind:value={monthlyAmount} />
				</div>
			</div>
			<div class="space-y-2">
				<Label>利息总额</Label>
				<div class="relative">
					<span class="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground">¥</span>
					<Input class="pl-8" type="number" step="0.01" placeholder="0.00" bind:value={interestAmount} />
				</div>
			</div>
		</div>

		<div class="grid grid-cols-2 gap-4">
			<div class="space-y-2">
				<Label>分类</Label>
				<select
					class="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm focus:outline-none focus:ring-1 focus:ring-ring"
					bind:value={categoryId}
				>
					{#each appStore.categories.expense as cat}
						<option value={cat.id}>{cat.icon || '📁'} {cat.name}</option>
					{/each}
				</select>
			</div>
			<div class="space-y-2">
				<Label>账户</Label>
				<select
					class="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm focus:outline-none focus:ring-1 focus:ring-ring"
					bind:value={accountId}
				>
					{#each appStore.accounts as acc}
						<option value={acc.id}>{acc.name}</option>
					{/each}
				</select>
			</div>
		</div>

		<div class="space-y-2">
			<Label>首次还款日</Label>
			<Input type="date" bind:value={firstRepayDate} />
		</div>

		<div class="flex gap-2 justify-end pt-2">
			<Button variant="outline" onclick={() => (showDialog = false)}>取消</Button>
			<Button onclick={handleSave} disabled={saving}>
				{saving ? '保存中...' : '保存'}
			</Button>
		</div>
	</div>
</Dialog>
