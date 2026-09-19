<script lang="ts">
	import { onMount } from 'svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Card from '$lib/components/ui/Card.svelte';
	import CardContent from '$lib/components/ui/CardContent.svelte';
	import Badge from '$lib/components/ui/Badge.svelte';
	import Dialog from '$lib/components/ui/Dialog.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import Label from '$lib/components/ui/Label.svelte';
	import { loanApi } from '$lib/api/modules/loans';
	import { appStore } from '$lib/stores/app';
	import { hzToast } from '$lib/components/ui/toast';
	import { formatMoney, formatDate } from '$lib/utils/format';
	import type { Loan, LoanDirection, LoanInterestType, LoanStatus } from '$lib/types';
	import { Plus, HandCoins, Trash2, Pencil, Wallet, CheckCircle2 } from '@lucide/svelte';

	let list = $state<Loan[]>([]);
	let loading = $state(true);

	// 可用于「资金账户 / 收款账户」的账户：排除负债类与借贷联动的系统账户
	const sysAccountNames = ['借出·应收', '借款·应付'];
	let fundAccounts = $derived(
		appStore.accounts.filter(
			(a) => a.type !== 'liability' && !sysAccountNames.includes(a.name)
		)
	);
	const todayStr = () => formatDate(new Date(), 'YYYY-MM-DD');

	// ---------- 新建 / 编辑 ----------
	let showDialog = $state(false);
	let editingItem = $state<Loan | null>(null);
	let direction = $state<LoanDirection>('lend');
	let counterparty = $state('');
	let principalYuan = $state('');
	let accountId = $state<number>(0);
	let loanDate = $state(todayStr());
	let dueDate = $state('');
	let interestType = $state<LoanInterestType>('none');
	let interestRate = $state('');
	let note = $state('');
	let saving = $state(false);

	// ---------- 还款 ----------
	let showRepay = $state(false);
	let repayItem = $state<Loan | null>(null);
	let repayPrincipal = $state('');
	let repayInterest = $state('');
	let repayAccountId = $state<number>(0);
	let repayDate = $state(todayStr());
	let repayNote = $state('');
	let repaying = $state(false);

	function remaining(item: Loan): number {
		return Math.max(0, item.principal - item.repaid_principal);
	}
	function isOverdue(item: Loan): boolean {
		if (item.status !== 'active' || !item.due_date) return false;
		const d = item.due_date.slice(0, 10);
		if (d.startsWith('0001')) return false;
		return d < todayStr();
	}
	function dirLabel(d: LoanDirection): string {
		return d === 'lend' ? '借出' : '借入';
	}

	// 汇总
	let summary = $derived.by(() => {
		let lendOut = 0;
		let borrowOut = 0;
		let overdue = 0;
		for (const it of list) {
			if (it.status !== 'active') continue;
			const rem = remaining(it);
			if (it.direction === 'lend') lendOut += rem;
			else borrowOut += rem;
			if (isOverdue(it)) overdue += 1;
		}
		return { lendOut, borrowOut, overdue };
	});

	async function loadData() {
		loading = true;
		try {
			list = await loanApi.list();
		} catch {}
		loading = false;
	}
	onMount(loadData);

	function openNew() {
		editingItem = null;
		direction = 'lend';
		counterparty = '';
		principalYuan = '';
		accountId = fundAccounts[0]?.id || 0;
		loanDate = todayStr();
		dueDate = '';
		interestType = 'none';
		interestRate = '';
		note = '';
		showDialog = true;
	}

	function openEdit(item: Loan) {
		editingItem = item;
		direction = item.direction;
		counterparty = item.counterparty;
		principalYuan = String(item.principal);
		accountId = item.account_id;
		loanDate = (item.loan_date || '').slice(0, 10) || todayStr();
		const dd = (item.due_date || '').slice(0, 10);
		dueDate = dd && !dd.startsWith('0001') ? dd : '';
		interestType = (item.interest_type as LoanInterestType) || 'none';
		interestRate = item.interest_rate ? String(item.interest_rate) : '';
		note = item.note;
		showDialog = true;
	}

	async function handleSave() {
		if (!counterparty.trim()) {
			hzToast.warning('请输入对方姓名/备注');
			return;
		}
		const p = parseFloat(principalYuan);
		if (!p || p <= 0) {
			hzToast.warning('请输入本金金额');
			return;
		}
		if (!accountId) {
			hzToast.warning('请选择资金账户');
			return;
		}
		saving = true;
		try {
			if (editingItem) {
				// 创建后本金不可改；仅允许调整备注与状态
				await loanApi.update(editingItem.id, {
					status: (editingItem.status as LoanStatus) || 'active',
					note: note.trim()
				});
				hzToast.success('借贷信息已更新');
			} else {
				await loanApi.create({
					direction,
					counterparty: counterparty.trim(),
					principal: p,
					account_id: accountId,
					book_id: appStore.effectiveBookId(),
					loan_date: loanDate,
					due_date: dueDate || undefined,
					interest_type: interestType,
					interest_rate: parseFloat(interestRate) || 0,
					note: note.trim()
				});
				hzToast.success(
					direction === 'lend'
						? '已记录借出并联动账户（现金→应收）'
						: '已记录借入并联动账户（应付→现金）'
				);
			}
			showDialog = false;
			await loadData();
		} catch (e: any) {
			hzToast.error(e.message || '保存失败');
		} finally {
			saving = false;
		}
	}

	function openRepay(item: Loan) {
		repayItem = item;
		repayPrincipal = String(remaining(item));
		repayInterest = '';
		repayAccountId = fundAccounts[0]?.id || 0;
		repayDate = todayStr();
		repayNote = '';
		showRepay = true;
	}

	async function handleRepay() {
		if (!repayItem) return;
		const amt = parseFloat(repayPrincipal);
		if (!amt || amt <= 0) {
			hzToast.warning('请输入还款本金');
			return;
		}
		if (!repayAccountId) {
			hzToast.warning('请选择收款/还款账户');
			return;
		}
		repaying = true;
		try {
			await loanApi.repay(repayItem.id, {
				amount: amt,
				interest_amount: parseFloat(repayInterest) || 0,
				repay_account_id: repayAccountId,
				repaid_at: repayDate,
				note: repayNote.trim()
			});
			hzToast.success('还款已记录，账户余额已更新');
			showRepay = false;
			await loadData();
		} catch (e: any) {
			hzToast.error(e.message || '还款失败');
		} finally {
			repaying = false;
		}
	}

	async function handleDelete(item: Loan) {
		if (!confirm(`确定删除该借贷记录？关联的资金流水将一并回滚。`)) return;
		try {
			await loanApi.remove(item.id);
			hzToast.success('已删除');
			await loadData();
		} catch (e: any) {
			hzToast.error(e.message || '删除失败');
		}
	}
</script>

<svelte:head>
	<title>借贷管理 · 货殖</title>
</svelte:head>

<div class="space-y-4">
	<div class="flex items-center justify-between">
		<h2 class="text-sm font-medium">借贷管理</h2>
		<Button size="sm" onclick={openNew}>
			<Plus size={16} />
			新建借贷
		</Button>
	</div>

	<!-- 汇总卡片 -->
	<div class="grid grid-cols-3 gap-3">
		<Card>
			<CardContent class="p-3">
				<div class="text-xs text-muted-foreground">别人欠我（未收回）</div>
				<div class="font-semibold tabular-nums text-rose-600">
					{formatMoney(summary.lendOut)}
				</div>
			</CardContent>
		</Card>
		<Card>
			<CardContent class="p-3">
				<div class="text-xs text-muted-foreground">我欠别人（未偿还）</div>
				<div class="font-semibold tabular-nums text-emerald-600">
					{formatMoney(summary.borrowOut)}
				</div>
			</CardContent>
		</Card>
		<Card>
			<CardContent class="p-3">
				<div class="text-xs text-muted-foreground">已逾期</div>
				<div class="font-semibold tabular-nums {summary.overdue > 0 ? 'text-amber-600' : ''}">
					{summary.overdue} 笔
				</div>
			</CardContent>
		</Card>
	</div>

	{#if loading}
		<div class="space-y-2">
			{#each [1, 2, 3] as i}
				<div class="h-16 rounded-lg animate-pulse bg-muted"></div>
			{/each}
		</div>
	{:else if list.length === 0}
		<Card>
			<div class="py-16 text-center">
				<HandCoins size={32} class="mx-auto mb-3 text-muted-foreground opacity-50" />
				<p class="text-muted-foreground text-sm">暂无借贷记录</p>
			</div>
		</Card>
	{:else}
		<Card>
			<CardContent class="p-0 divide-y">
				{#each list as item (item.id)}
					<div class="flex items-center gap-3 p-4 group">
						<div class="flex-1 min-w-0">
							<div class="flex items-center gap-2">
								<span class="font-medium truncate">{item.counterparty}</span>
								<Badge variant={item.direction === 'lend' ? 'default' : 'secondary'}>
									{dirLabel(item.direction)}
								</Badge>
							</div>
							<div class="text-xs text-muted-foreground mt-0.5">
								已还 {formatMoney(item.repaid_principal)}
								<span class="mx-1">/</span>
								本金 {formatMoney(item.principal)}
								{#if item.due_date && !item.due_date.startsWith('0001')}
									<span class="mx-1">·</span>
									到期 {item.due_date.slice(0, 10)}
								{/if}
								{#if item.note}
									<span class="mx-1">·</span>{item.note}
								{/if}
							</div>
						</div>
						<div class="text-right flex items-center gap-1">
							<div>
								<div
									class="font-semibold tabular-nums {item.direction === 'lend'
										? 'text-rose-600'
										: 'text-emerald-600'}"
								>
									{formatMoney(remaining(item))}
									<span class="text-muted-foreground text-xs">未还</span>
								</div>
								{#if item.status === 'completed'}
									<Badge variant="outline" class="mt-1 flex items-center gap-1">
										<CheckCircle2 size={11} /> 已结清
									</Badge>
								{:else if isOverdue(item)}
									<Badge variant="destructive" class="mt-1">已逾期</Badge>
								{:else}
									<Badge variant="secondary" class="mt-1">进行中</Badge>
								{/if}
							</div>
							{#if item.status !== 'completed'}
								<button
									class="p-1 rounded hover:bg-accent opacity-0 group-hover:opacity-100 transition"
									title="还款"
									onclick={() => openRepay(item)}
								>
									<Wallet size={14} class="text-muted-foreground" />
								</button>
							{/if}
							<button
								class="p-1 rounded hover:bg-accent opacity-0 group-hover:opacity-100 transition"
								title="编辑"
								onclick={() => openEdit(item)}
							>
								<Pencil size={14} class="text-muted-foreground" />
							</button>
							<button
								class="p-1 rounded hover:bg-destructive/10 opacity-0 group-hover:opacity-100 transition"
								title="删除"
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

	<p class="text-[11px] text-muted-foreground leading-relaxed">
		借贷会联动真实账户：借出时现金转入「借出·应收」虚拟账户，借入时从「借款·应付」负债账户转入现金，净资产不变，且自动计入资产概览。删除会回滚全部关联交易流水。
	</p>
</div>

<!-- 新建 / 编辑 Dialog -->
<Dialog bind:open={showDialog}>
	<div class="space-y-4">
		<h3 class="text-lg font-semibold">{editingItem ? '编辑借贷' : '新建借贷'}</h3>

		{#if !editingItem}
			<div class="space-y-2">
				<Label>借贷方向</Label>
				<div class="grid grid-cols-2 gap-2">
					<button
						type="button"
						class="h-9 rounded-md border text-sm transition {direction === 'lend'
							? 'border-primary bg-primary/10 text-primary'
							: 'border-input'}"
						onclick={() => (direction = 'lend')}
					>
						借出（别人欠我）
					</button>
					<button
						type="button"
						class="h-9 rounded-md border text-sm transition {direction === 'borrow'
							? 'border-primary bg-primary/10 text-primary'
							: 'border-input'}"
						onclick={() => (direction = 'borrow')}
					>
						借入（我欠别人）
					</button>
				</div>
			</div>
		{/if}

		<div class="space-y-2">
			<Label>对方姓名 / 备注</Label>
			<Input bind:value={counterparty} placeholder="例如: 小王" />
		</div>

		<div class="space-y-2">
			<Label>{editingItem ? '本金（创建后不可修改）' : '本金'}</Label>
			<div class="relative">
				<span class="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground">¥</span>
				<Input
					class="pl-8"
					type="number"
					step="0.01"
					placeholder="0.00"
					bind:value={principalYuan}
					disabled={!!editingItem}
				/>
			</div>
		</div>

		<div class="grid grid-cols-2 gap-4">
			<div class="space-y-2">
				<Label>资金账户</Label>
				<select
					class="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm focus:outline-none focus:ring-1 focus:ring-ring"
					bind:value={accountId}
					disabled={!!editingItem}
				>
					{#each fundAccounts as a}
						<option value={a.id}>{a.name}</option>
					{/each}
				</select>
				<p class="text-[11px] text-muted-foreground">
					{direction === 'lend' ? '钱从这里借出' : '钱进到这个账户'}
				</p>
			</div>
			<div class="space-y-2">
				<Label>借款日期</Label>
				<Input type="date" bind:value={loanDate} disabled={!!editingItem} />
			</div>
		</div>

		<div class="grid grid-cols-2 gap-4">
			<div class="space-y-2">
				<Label>到期日期（可选）</Label>
				<Input type="date" bind:value={dueDate} disabled={!!editingItem} />
			</div>
			<div class="space-y-2">
				<Label>计息方式</Label>
				<select
					class="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm focus:outline-none focus:ring-1 focus:ring-ring"
					bind:value={interestType}
					disabled={!!editingItem}
				>
					<option value="none">不计息</option>
					<option value="simple">年化单利</option>
					<option value="monthly">月利率</option>
				</select>
			</div>
		</div>

		{#if interestType !== 'none'}
			<div class="space-y-2">
				<Label>利率（%）</Label>
				<Input
					type="number"
					step="0.01"
					placeholder="例如: 5"
					bind:value={interestRate}
					disabled={!!editingItem}
				/>
			</div>
		{/if}

		<div class="space-y-2">
			<Label>备注</Label>
			<Input bind:value={note} placeholder="可选" />
		</div>

		<div class="flex gap-2 justify-end pt-2">
			<Button variant="outline" onclick={() => (showDialog = false)}>取消</Button>
			<Button onclick={handleSave} disabled={saving}>
				{saving ? '保存中...' : '保存'}
			</Button>
		</div>
	</div>
</Dialog>

<!-- 还款 Dialog -->
<Dialog bind:open={showRepay}>
	<div class="space-y-4">
		<h3 class="text-lg font-semibold">
			记录还款 · {repayItem ? repayItem.counterparty : ''}
		</h3>
		{#if repayItem}
			<p class="text-xs text-muted-foreground">
				剩余未还本金 {formatMoney(remaining(repayItem))}
			</p>
		{/if}

		<div class="space-y-2">
			<Label>还本金</Label>
			<div class="relative">
				<span class="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground">¥</span>
				<Input class="pl-8" type="number" step="0.01" bind:value={repayPrincipal} />
			</div>
		</div>

		<div class="space-y-2">
			<Label>利息（可选）</Label>
			<div class="relative">
				<span class="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground">¥</span>
				<Input class="pl-8" type="number" step="0.01" placeholder="0.00" bind:value={repayInterest} />
			</div>
			<p class="text-[11px] text-muted-foreground">
				借出利息记为收入、借入利息记为支出，进入所选账户。
			</p>
		</div>

		<div class="grid grid-cols-2 gap-4">
			<div class="space-y-2">
				<Label>{repayItem?.direction === 'lend' ? '收款账户' : '还款账户'}</Label>
				<select
					class="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm focus:outline-none focus:ring-1 focus:ring-ring"
					bind:value={repayAccountId}
				>
					{#each fundAccounts as a}
						<option value={a.id}>{a.name}</option>
					{/each}
				</select>
			</div>
			<div class="space-y-2">
				<Label>还款日期</Label>
				<Input type="date" bind:value={repayDate} />
			</div>
		</div>

		<div class="space-y-2">
			<Label>备注</Label>
			<Input bind:value={repayNote} placeholder="可选" />
		</div>

		<div class="flex gap-2 justify-end pt-2">
			<Button variant="outline" onclick={() => (showRepay = false)}>取消</Button>
			<Button onclick={handleRepay} disabled={repaying}>
				{repaying ? '提交中...' : '确认还款'}
			</Button>
		</div>
	</div>
</Dialog>
