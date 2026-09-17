<script lang="ts">
	import { onMount } from 'svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Card from '$lib/components/ui/Card.svelte';
	import CardContent from '$lib/components/ui/CardContent.svelte';
	import Progress from '$lib/components/ui/Progress.svelte';
	import Dialog from '$lib/components/ui/Dialog.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import Label from '$lib/components/ui/Label.svelte';
	import AccountSelect from '$lib/components/AccountSelect.svelte';
	import { savingApi } from '$lib/api/modules/savings';
	import { appStore } from '$lib/stores/app';
	import { hzToast } from '$lib/components/ui/toast';
	import { formatMoney, formatDate } from '$lib/utils/format';
	import type { SavingPlan } from '$lib/types';
	import { Plus, PiggyBank, Pencil, Trash2, Coins, Wallet } from '@lucide/svelte';

	let plans = $state<SavingPlan[]>([]);
	let loading = $state(true);

	// Dialog state
	let showDialog = $state(false);
	let showRecordDialog = $state(false);
	let editingPlan = $state<SavingPlan | null>(null);
	let recordPlan = $state<SavingPlan | null>(null);
	let planName = $state('');
	let targetAmount = $state('');
	let currentAmount = $state('');
	let recordAmount = $state('');
	let startDate = $state('');
	let targetDate = $state('');
	let accountId = $state<number>(0);
	let saving = $state(false);

	// B6：存入资金必须指定来源账户与日期。后端 AddSavingRecordRequest 的
	// record_date 是 required，此前前端只发 { amount } → 必然 400；
	// 且不传 account_id 时后端只能退化成「取第一个非负债账户」，存入哪张卡由系统猜。
	let recordDate = $state(formatDate(new Date(), 'YYYY-MM-DD'));
	let recordAccountId = $state<number>(0);

	async function loadData() {
		loading = true;
		try {
			plans = await savingApi.list();
		} catch {}
		loading = false;
	}

	onMount(loadData);

	function openNew() {
		editingPlan = null;
		planName = '';
		targetAmount = '';
		currentAmount = '0';
		startDate = new Date().toISOString().split('T')[0];
		targetDate = '';
		accountId = appStore.accounts[0]?.id || 0;
		showDialog = true;
	}

	function openEdit(plan: SavingPlan) {
		editingPlan = plan;
		planName = plan.name;
		targetAmount = String(plan.target_amount);
		currentAmount = String(plan.current_amount);
		startDate = plan.start_date;
		targetDate = plan.target_date;
		accountId = plan.account_id;
		showDialog = true;
	}

	function openRecord(plan: SavingPlan) {
		recordPlan = plan;
		recordAmount = '';
		recordDate = formatDate(new Date(), 'YYYY-MM-DD');
		// 默认取计划绑定的账户，没有则取第一个资产账户
		recordAccountId = plan.account_id || appStore.accounts[0]?.id || 0;
		showRecordDialog = true;
	}

	async function handleSave() {
		const target = parseFloat(targetAmount);
		if (!planName.trim()) {
			hzToast.warning('请输入计划名称');
			return;
		}
		if (!target || target <= 0) {
			hzToast.warning('请输入目标金额');
			return;
		}

		saving = true;
		try {
			const data = {
				name: planName.trim(),
				target_amount: target,
				current_amount: parseFloat(currentAmount) || 0,
				start_date: startDate,
				target_date: targetDate,
				account_id: accountId,
				book_id: appStore.effectiveBookId()
			};

			if (editingPlan) {
				await savingApi.update(editingPlan.id, data);
				hzToast.success('计划已更新');
			} else {
				await savingApi.create(data);
				hzToast.success('计划已创建');
			}
			showDialog = false;
			await loadData();
		} catch (e: any) {
			hzToast.error(e.message || '保存失败');
		} finally {
			saving = false;
		}
	}

	async function handleAddRecord() {
		const amt = parseFloat(recordAmount);
		if (!amt || amt <= 0 || !recordPlan) {
			hzToast.warning('请输入有效金额');
			return;
		}
		if (!recordAccountId) {
			hzToast.warning('请选择资金来源账户');
			return;
		}

		saving = true;
		try {
			await savingApi.addRecord(recordPlan.id, {
				amount: amt,
				record_date: recordDate,
				account_id: recordAccountId
			});
			hzToast.success('已存入并从账户扣减');
			showRecordDialog = false;
			await loadData();
		} catch (e: any) {
			hzToast.error(e.message || '操作失败');
		} finally {
			saving = false;
		}
	}

	async function handleDelete(plan: SavingPlan) {
		if (!confirm(`确定删除计划「${plan.name}」？`)) return;
		try {
			await savingApi.remove(plan.id);
			hzToast.success('已删除');
			await loadData();
		} catch (e: any) {
			hzToast.error(e.message || '删除失败');
		}
	}
</script>

<svelte:head>
	<title>存钱计划 · 货殖</title>
</svelte:head>

<div class="space-y-4">
	<div class="flex items-center justify-between">
		<h2 class="text-sm font-medium">存钱计划</h2>
		<Button size="sm" onclick={openNew}>
			<Plus size={16} />
			新建计划
		</Button>
	</div>
	{#if loading}
		<div class="grid gap-3 md:grid-cols-2">
			{#each [1, 2] as i}
				<div class="h-32 rounded-xl animate-pulse bg-muted" ></div>
			{/each}
		</div>
	{:else if plans.length === 0}
		<Card>
			<div class="py-16 text-center">
				<PiggyBank size={32} class="mx-auto mb-3 text-muted-foreground opacity-50" />
				<p class="text-muted-foreground text-sm">暂无存钱计划</p>
			</div>
		</Card>
	{:else}
		<div class="grid gap-3 md:grid-cols-2">
			{#each plans as plan (plan.id)}
				<Card class="group">
					<CardContent class="p-4">
						<div class="flex items-center justify-between mb-2">
							<div class="font-medium">{plan.name}</div>
							<div class="flex items-center gap-1">
								<span class="text-xs text-muted-foreground">
									{plan.status === 'active' ? '进行中' : plan.status === 'done' ? '已完成' : '已暂停'}
								</span>
								<button
									class="p-1 rounded hover:bg-accent opacity-0 group-hover:opacity-100 transition"
									onclick={() => openRecord(plan)}
									title="存钱"
								>
									<Coins size={14} class="text-muted-foreground" />
								</button>
								<button
									class="p-1 rounded hover:bg-accent opacity-0 group-hover:opacity-100 transition"
									onclick={() => openEdit(plan)}
								>
									<Pencil size={14} class="text-muted-foreground" />
								</button>
								<button
									class="p-1 rounded hover:bg-destructive/10 opacity-0 group-hover:opacity-100 transition"
									onclick={() => handleDelete(plan)}
								>
									<Trash2 size={14} class="text-destructive" />
								</button>
							</div>
						</div>
						<Progress
							value={(plan.current_amount / plan.target_amount) * 100}
						/>
						<div class="flex items-center justify-between mt-2 text-sm">
							<span class="font-semibold tabular-nums">
								{formatMoney(plan.current_amount)}
							</span>
							<span class="text-muted-foreground">
								/ {formatMoney(plan.target_amount)}
							</span>
						</div>
					</CardContent>
				</Card>
			{/each}
		</div>
	{/if}
</div>

<!-- 新增/编辑 Dialog -->
<Dialog bind:open={showDialog}>
	<div class="space-y-4">
		<h3 class="text-lg font-semibold">
			{editingPlan ? '编辑计划' : '新建计划'}
		</h3>

		<div class="space-y-2">
			<Label>计划名称</Label>
			<Input bind:value={planName} placeholder="例如: 买车基金" />
		</div>

		<div class="space-y-2">
			<Label>目标金额</Label>
			<div class="relative">
				<span class="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground">¥</span>
				<Input class="pl-8" type="number" step="0.01" placeholder="0.00" bind:value={targetAmount} />
			</div>
		</div>

		<div class="space-y-2">
			<Label>已有金额</Label>
			<div class="relative">
				<span class="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground">¥</span>
				<Input class="pl-8" type="number" step="0.01" placeholder="0.00" bind:value={currentAmount} />
			</div>
		</div>

		<div class="space-y-2">
			<Label>账户</Label>
			<AccountSelect bind:value={accountId} includeArchived />
		</div>

		<div class="grid grid-cols-2 gap-4">
			<div class="space-y-2">
				<Label>开始日期</Label>
				<Input type="date" bind:value={startDate} />
			</div>
			<div class="space-y-2">
				<Label>目标日期</Label>
				<Input type="date" bind:value={targetDate} />
			</div>
		</div>

		<div class="flex gap-2 justify-end pt-2">
			<Button variant="outline" onclick={() => (showDialog = false)}>取消</Button>
			<Button onclick={handleSave} disabled={saving}>
				{saving ? '保存中...' : '保存'}
			</Button>
		</div>
	</div>
</Dialog>

<!-- 存钱 Dialog -->
<Dialog bind:open={showRecordDialog}>
	<div class="space-y-4">
		<h3 class="text-lg font-semibold">存钱 - {recordPlan?.name}</h3>
		<div class="space-y-2">
			<Label>存入金额</Label>
			<div class="relative">
				<span class="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground">¥</span>
				<Input class="pl-8" type="number" step="0.01" placeholder="0.00" bind:value={recordAmount} />
			</div>
		</div>
		<!-- B6：来源账户 + 日期，存入会真实扣减该账户余额 -->
		<div class="grid grid-cols-2 gap-4">
			<div class="space-y-2">
				<Label>资金来源</Label>
				<AccountSelect bind:value={recordAccountId} placeholder="选择账户" />
			</div>
			<div class="space-y-2">
				<Label>存入日期</Label>
				<Input type="date" bind:value={recordDate} />
			</div>
		</div>
		<p class="flex items-start gap-1.5 text-[11px] text-muted-foreground">
			<Wallet size={12} class="mt-0.5 shrink-0" />
			确认后将从所选账户扣减相应金额，并生成一笔转入存款的支出记录。
		</p>
		<div class="flex gap-2 justify-end pt-2">
			<Button variant="outline" onclick={() => (showRecordDialog = false)}>取消</Button>
			<Button onclick={handleAddRecord} disabled={saving}>
				{saving ? '保存中...' : '确认存入'}
			</Button>
		</div>
	</div>
</Dialog>
