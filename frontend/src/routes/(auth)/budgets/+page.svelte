<script lang="ts">
	import { onMount } from 'svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Card from '$lib/components/ui/Card.svelte';
	import CardContent from '$lib/components/ui/CardContent.svelte';
	import Badge from '$lib/components/ui/Badge.svelte';
	import Progress from '$lib/components/ui/Progress.svelte';
	import Dialog from '$lib/components/ui/Dialog.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import Label from '$lib/components/ui/Label.svelte';
	import { budgetApi } from '$lib/api/modules/budgets';
	import { appStore } from '$lib/stores/app';
	import { hzToast } from '$lib/components/ui/toast';
	import { formatMoney } from '$lib/utils/format';
	import type { BudgetView } from '$lib/types';
	import { Plus, Target, AlertTriangle, Pencil, Trash2 } from '@lucide/svelte';

	let budgets = $state<BudgetView[]>([]);
	let loading = $state(true);

	// Dialog state
	let showDialog = $state(false);
	let editingBudget = $state<BudgetView | null>(null);
	let categoryId = $state<number>(0);
	let amount = $state('');
	let periodType = $state<'monthly' | 'yearly'>('monthly');
	let alertRate = $state('80');
	let saving = $state(false);

	async function loadData() {
		loading = true;
		try {
			budgets = await budgetApi.list({ book_id: appStore.currentBookId });
		} catch {}
		loading = false;
	}

	onMount(loadData);
	$effect(() => {
		if (appStore.currentBookId) loadData();
	});

	const overBudgetCount = $derived(budgets.filter((b) => b.is_over_budget).length);

	function openNew() {
		editingBudget = null;
		categoryId = appStore.categories.expense[0]?.id || 0;
		amount = '';
		periodType = 'monthly';
		alertRate = '80';
		showDialog = true;
	}

	function openEdit(budget: BudgetView) {
		editingBudget = budget;
		categoryId = budget.category_id;
		amount = String(budget.amount);
		periodType = budget.period_type as 'monthly' | 'yearly';
		alertRate = String(Math.round(budget.alert_rate * 100));
		showDialog = true;
	}

	async function handleSave() {
		const amt = parseFloat(amount);
		if (!amt || amt <= 0) {
			hzToast.warning('请输入有效金额');
			return;
		}
		if (!categoryId) {
			hzToast.warning('请选择分类');
			return;
		}

		saving = true;
		try {
			const data = {
				category_id: categoryId,
				amount: amt,
				period_type: periodType,
				alert_rate: parseInt(alertRate) / 100,
				book_id: appStore.currentBookId
			};

			if (editingBudget) {
				await budgetApi.update(editingBudget.id, data);
				hzToast.success('预算已更新');
			} else {
				await budgetApi.create(data);
				hzToast.success('预算已创建');
			}
			showDialog = false;
			await loadData();
		} catch (e: any) {
			hzToast.error(e.message || '保存失败');
		} finally {
			saving = false;
		}
	}

	async function handleDelete(budget: BudgetView) {
		const catName = appStore.categories.expense.find((c) => c.id === budget.category_id)?.name || '全部分类';
		if (!confirm(`确定删除「${catName}」的预算？`)) return;
		try {
			await budgetApi.remove(budget.id);
			hzToast.success('已删除');
			await loadData();
		} catch (e: any) {
			hzToast.error(e.message || '删除失败');
		}
	}
</script>

<svelte:head>
	<title>预算管理 · 货殖</title>
</svelte:head>

<div class="space-y-6">
	<!-- 汇总 -->
	{#if overBudgetCount > 0}
		<Card class="border-destructive/50 bg-destructive/5">
			<CardContent class="py-3 px-4 flex items-center gap-3">
				<AlertTriangle class="text-destructive" size={20} />
				<div class="flex-1 text-sm">
					{overBudgetCount} 个预算已超额，注意控制支出！
				</div>
			</CardContent>
		</Card>
	{/if}

	<div class="flex items-center justify-between">
		<h2 class="text-sm font-medium">月度预算</h2>
		<Button size="sm" onclick={openNew}>
			<Plus size={16} />
			新增预算
		</Button>
	</div>

	{#if loading}
		<div class="grid gap-3 md:grid-cols-2">
			{#each [1, 2] as i}
				<div class="h-32 rounded-xl animate-pulse bg-muted"></div>
			{/each}
		</div>
	{:else if budgets.length === 0}
		<Card>
			<div class="py-16 text-center">
				<Target size={32} class="mx-auto mb-3 text-muted-foreground opacity-50" />
				<p class="text-muted-foreground text-sm">还没有设置任何预算</p>
				<p class="text-xs text-muted-foreground mt-1">设置预算帮助你控制支出</p>
			</div>
		</Card>
	{:else}
		<div class="grid gap-3 md:grid-cols-2">
			{#each budgets as budget (budget.id)}
				<Card class="transition group">
					<CardContent class="p-4">
						<div class="flex items-start justify-between mb-2">
							<div class="min-w-0">
								<div class="font-medium truncate">
									{appStore.categories.expense.find((c) => c.id === budget.category_id)?.name || '全部分类'}
								</div>
								<div class="text-xs text-muted-foreground">
									{budget.period_type === 'monthly' ? '月度' : '年度'}预算
								</div>
							</div>
							<div class="flex items-center gap-1">
								{#if budget.is_over_budget}
									<Badge variant="destructive">已超额</Badge>
								{:else if budget.usage_rate >= budget.alert_rate}
									<Badge variant="secondary">接近超限</Badge>
								{/if}
								<button
									class="p-1 rounded hover:bg-accent opacity-0 group-hover:opacity-100 transition"
									onclick={() => openEdit(budget)}
								>
									<Pencil size={14} class="text-muted-foreground" />
								</button>
								<button
									class="p-1 rounded hover:bg-destructive/10 opacity-0 group-hover:opacity-100 transition"
									onclick={() => handleDelete(budget)}
								>
									<Trash2 size={14} class="text-destructive" />
								</button>
							</div>
						</div>

						<Progress value={Math.min(budget.usage_rate, 100)} />

						<div class="flex items-end justify-between mt-3">
							<div class="text-sm">
								<span class="font-semibold tabular-nums">
									{formatMoney(budget.used_amount)}
								</span>
								<span class="text-muted-foreground"> / {formatMoney(budget.amount)}</span>
							</div>
							<div class="text-xs text-muted-foreground">
								剩余 {formatMoney(Math.max(0, budget.remaining))}
							</div>
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
			{editingBudget ? '编辑预算' : '新增预算'}
		</h3>

		<div class="space-y-2">
			<Label>分类</Label>
			<select
				class="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm focus:outline-none focus:ring-1 focus:ring-ring"
				bind:value={categoryId}
			>
				<option value={0}>全部分类</option>
				{#each appStore.categories.expense as cat}
					<option value={cat.id}>{cat.icon || '📁'} {cat.name}</option>
				{/each}
			</select>
		</div>

		<div class="space-y-2">
			<Label>预算金额</Label>
			<div class="relative">
				<span class="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground">¥</span>
				<Input class="pl-8" type="number" step="0.01" placeholder="0.00" bind:value={amount} />
			</div>
		</div>

		<div class="grid grid-cols-2 gap-4">
			<div class="space-y-2">
				<Label>周期</Label>
				<select
					class="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm focus:outline-none focus:ring-1 focus:ring-ring"
					bind:value={periodType}
				>
					<option value="monthly">月度</option>
					<option value="yearly">年度</option>
				</select>
			</div>
			<div class="space-y-2">
				<Label>告警阈值 (%)</Label>
				<Input type="number" min={1} max={100} bind:value={alertRate} />
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
