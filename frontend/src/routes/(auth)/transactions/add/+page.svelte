<script lang="ts">
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import Card from '$lib/components/ui/Card.svelte'
import CardContent from '$lib/components/ui/CardContent.svelte'
import CardHeader from '$lib/components/ui/CardHeader.svelte'
import CardTitle from '$lib/components/ui/CardTitle.svelte';
	import Tabs from '$lib/components/ui/Tabs.svelte'
import TabsTrigger from '$lib/components/ui/TabsTrigger.svelte';
	import Dialog from '$lib/components/ui/Dialog.svelte';
	import Label from '$lib/components/ui/Label.svelte';
	import { txApi } from '$lib/api/modules/transactions';
	import { appStore } from '$lib/stores/app';
	import { hzToast } from '$lib/components/ui/toast';
	import { aiApi } from '$lib/api/modules/ai';
	import { formatDate } from '$lib/utils/format';
	import type { Transaction, Category } from '$lib/types';
	import { Sparkles, Save, X } from '@lucide/svelte';

	let isEdit = $derived(currentId !== undefined);
	let currentId = $derived(($page?.params as any)?.id);

	let type = $state<'expense' | 'income' | 'transfer'>('expense');
	let amount = $state('');
	let description = $state('');
	let categoryId = $state<number>(0);
	let accountId = $state<number>(0);
	let toAccountId = $state<number>(0);
	let txDate = $state(formatDate(new Date(), 'YYYY-MM-DD'));
	let loading = $state(false);
	let aiLoading = $state(false);

	let showCategoryPicker = $state(false);

	let categories = $derived.by(() => {
		if (type === 'expense') return appStore.categories.expense;
		if (type === 'income') return appStore.categories.income;
		return appStore.categories.expense;
	});

	onMount(async () => {
		// 设置默认账户
		if (appStore.accounts.length > 0) {
			accountId = appStore.accounts[0].id;
			if (type === 'transfer' && appStore.accounts.length > 1) {
				toAccountId = appStore.accounts[1].id;
			}
		}
		// 设置默认分类
		const firstCat = categories[0];
		if (firstCat) categoryId = firstCat.id;

		// 如果是编辑模式，加载交易
		if (isEdit && currentId) {
			try {
				const tx = await txApi.get(Number(currentId));
				type = tx.type as any;
				amount = String(tx.amount);
				description = tx.description || '';
				categoryId = tx.category_id;
				accountId = tx.account_id;
				toAccountId = tx.to_account_id || 0;
				txDate = tx.tx_date;
			} catch (e) {
				hzToast.error('加载交易失败');
				goto('/transactions');
			}
		}
	});

	$effect(() => {
		if (!isEdit) {
			const firstCat = categories[0];
			if (firstCat && !categories.find((c) => c.id === categoryId)) {
				categoryId = firstCat.id;
			}
		}
	});

	async function handleSave() {
		const amt = parseFloat(amount);
		if (!amt || amt <= 0) {
			hzToast.warning('请输入有效金额');
			return;
		}
		if (!accountId) {
			hzToast.warning('请选择账户');
			return;
		}
		if (type !== 'transfer' && !categoryId) {
			hzToast.warning('请选择分类');
			return;
		}
		if (type === 'transfer' && accountId === toAccountId) {
			hzToast.warning('转账账户不能相同');
			return;
		}

		loading = true;
		try {
			const data: any = {
				type,
				amount: amt,
				category_id: categoryId,
				account_id: accountId,
				tx_date: txDate,
				description: description || '',
				book_id: appStore.currentBookId
			};
			if (type === 'transfer') {
				data.to_account_id = toAccountId;
			}
			if (isEdit && currentId) {
				await txApi.update(Number(currentId), data);
				hzToast.success('更新成功');
			} else {
				await txApi.create(data);
				hzToast.success('记账成功');
			}
			goto('/transactions');
		} catch (e: any) {
			hzToast.error(e.message || '保存失败');
		} finally {
			loading = false;
		}
	}

	async function handleAI() {
		if (!description.trim()) {
			hzToast.warning('请先输入描述');
			return;
		}
		aiLoading = true;
		try {
			const res = await aiApi.classify({ description });
			if (res.category_id) {
				categoryId = res.category_id;
			}
			if (res.confidence > 0.7) {
				hzToast.success(`AI 推荐: ${res.category}`);
			} else {
				hzToast.info(`AI 推荐: ${res.category}（置信度较低）`);
			}
		} catch (e) {
			hzToast.error('AI 识别失败');
		} finally {
			aiLoading = false;
		}
	}
</script>

<svelte:head>
	<title>{isEdit ? '编辑交易' : '记一笔'} · 货殖</title>
</svelte:head>

<div class="max-w-xl mx-auto">
	<Card>
		<CardHeader>
			<CardTitle>{isEdit ? '编辑交易' : '记一笔'}</CardTitle>
		</CardHeader>
		<CardContent class="space-y-5">
			<!-- 类型切换 -->
			<Tabs>
				<TabsTrigger value="expense" bind:group={type}>支出</TabsTrigger>
				<TabsTrigger value="income" bind:group={type}>收入</TabsTrigger>
				<TabsTrigger value="transfer" bind:group={type}>转账</TabsTrigger>
			</Tabs>

			<!-- 金额 -->
			<div class="space-y-2">
				<Label>金额</Label>
				<div class="relative">
					<span class="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground">¥</span>
					<Input
						class="text-xl h-12 pl-8 font-semibold tabular-nums"
						type="number"
						step="0.01"
						placeholder="0.00"
						bind:value={amount}
					/>
				</div>
			</div>

			<!-- 分类 / 选择器 -->
			{#if type !== 'transfer'}
				<div class="space-y-2">
					<Label>分类</Label>
					<button
						class="w-full flex items-center gap-2 h-10 rounded-md border px-3 text-left hover:bg-accent transition"
						onclick={() => (showCategoryPicker = true)}
					>
						<span class="w-6 h-6 rounded bg-muted grid place-items-center text-xs">
							{categories.find((c) => c.id === categoryId)?.icon || '📁'}
						</span>
						<span class="flex-1 text-sm">
							{categories.find((c) => c.id === categoryId)?.name || '选择分类'}
						</span>
						<span class="text-muted-foreground text-sm">点击选择</span>
					</button>
				</div>
			{/if}

			<!-- 账户 -->
			<div class="space-y-2">
				<Label>{type === 'transfer' ? '从账户' : '账户'}</Label>
				<select
					class="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm focus:outline-none focus:ring-1 focus:ring-ring"
					bind:value={accountId}
					onchange={(e) => (accountId = Number((e.target as HTMLSelectElement).value))}
				>
					{#each appStore.accounts as acc}
						<option value={acc.id}>{acc.name}</option>
					{/each}
				</select>
			</div>

			{#if type === 'transfer'}
				<div class="space-y-2">
					<Label>到账户</Label>
					<select
						class="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm focus:outline-none focus:ring-1 focus:ring-ring"
						bind:value={toAccountId}
						onchange={(e) => (toAccountId = Number((e.target as HTMLSelectElement).value))}
					>
						{#each appStore.accounts as acc}
							<option value={acc.id}>{acc.name}</option>
						{/each}
					</select>
				</div>
			{/if}

			<!-- 日期 -->
			<div class="space-y-2">
				<Label>日期</Label>
				<Input type="date" bind:value={txDate} />
			</div>

			<!-- 描述 -->
			<div class="space-y-2">
				<Label>描述</Label>
				<div class="flex gap-2">
					<Input
						class="flex-1"
						placeholder="例如: 午餐、打车..."
						bind:value={description}
					/>
					<Button
						type="button"
						variant="outline"
						onclick={handleAI}
						disabled={aiLoading}
					>
						{#if aiLoading}...{:else}<Sparkles size={16} />AI{/if}
					</Button>
				</div>
			</div>

			<!-- 保存按钮 -->
			<div class="flex gap-2 pt-2">
				<Button class="flex-1" onclick={handleSave} disabled={loading}>
					<Save size={16} />
					{isEdit ? '保存修改' : '保存'}
				</Button>
				<Button
					variant="outline"
					onclick={() => goto('/transactions')}
				>
					<X size={16} />
					取消
				</Button>
			</div>
		</CardContent>
	</Card>

	<!-- 分类选择器 -->
	<Dialog bind:open={showCategoryPicker}>
		<div class="flex items-center justify-between mb-4">
			<h3 class="text-lg font-semibold">选择分类</h3>
		</div>
		<div class="grid grid-cols-4 gap-2 max-h-64 overflow-y-auto">
			{#each categories as cat (cat.id)}
				<button
					class="flex flex-col items-center gap-1 p-3 rounded-lg border hover:bg-accent transition"


					onclick={() => {
						categoryId = cat.id;
						showCategoryPicker = false;
					}}
				>
					<span class="text-2xl">{cat.icon || '📁'}</span>
					<span class="text-xs truncate w-full text-center">{cat.name}</span>
				</button>
			{/each}
		</div>
	</Dialog>
</div>
