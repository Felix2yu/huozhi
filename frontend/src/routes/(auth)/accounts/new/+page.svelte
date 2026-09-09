<script lang="ts">
	import { goto } from '$app/navigation';
	import Button from '$lib/components/ui/Button.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import Label from '$lib/components/ui/Label.svelte';
	import Card from '$lib/components/ui/Card.svelte';
	import CardContent from '$lib/components/ui/CardContent.svelte';
	import CardHeader from '$lib/components/ui/CardHeader.svelte';
	import CardTitle from '$lib/components/ui/CardTitle.svelte';
	import { accountApi } from '$lib/api/modules/accounts';
	import { appStore } from '$lib/stores/app';
	import { hzToast } from '$lib/components/ui/toast';
	import { detectBankName, detectAccountType } from '$lib/utils/bank-themes';
	import AccountIcon from '$lib/components/AccountIcon.svelte';
	import type { AccountType } from '$lib/types';
	import { Save, X } from '@lucide/svelte';

	let name = $state('');
	let type = $state<AccountType>('bank');
	let typeTouched = $state(false);
	let balance = $state('');
	let initialAmount = $state('');
	let bankName = $state('');
	let fullCardNo = $state('');
	let creditLimit = $state('');
	let billDay = $state('');
	let repayDay = $state('');
	let includeInTotal = $state(true);
	let includeInBudget = $state(false);
	let remark = $state('');
	let loading = $state(false);

	const accountTypes: { value: AccountType; label: string }[] = [
		{ value: 'cash', label: '现金' },
		{ value: 'bank', label: '银行卡' },
		{ value: 'credit', label: '信用卡' },
		{ value: 'prepaid', label: '预付卡' },
		{ value: 'investment', label: '投资' },
		{ value: 'liability', label: '负债' },
		{ value: 'virtual', label: '虚拟账户' }
	];

	const isCredit = $derived(type === 'credit');

	// 输入名称时，自动从名称推断银行/机构名（仅在用户尚未手动填写时），并适度推断类型
	function onNameInput() {
		if (!bankName.trim()) {
			const bn = detectBankName(name);
			if (bn) bankName = bn;
		}
		if (!typeTouched) {
			const t = detectAccountType(name);
			if (t === 'virtual' || t === 'credit') type = t;
		}
	}

	async function handleSave() {
		if (!name.trim()) {
			hzToast.warning('请输入账户名称');
			return;
		}

		loading = true;
		try {
			const data: any = {
				name: name.trim(),
				type,
				balance: parseFloat(balance) || 0,
				initial_amount: parseFloat(initialAmount) || 0,
				include_in_total: includeInTotal,
				include_in_budget: includeInBudget,
				book_id: appStore.currentBookId
			};
			if (bankName.trim()) data.bank_name = bankName.trim();
			if (fullCardNo.trim()) data.full_card_no = fullCardNo.replace(/\s/g, '');
			if (isCredit && creditLimit) data.credit_limit = parseFloat(creditLimit);
			if (isCredit && billDay) data.bill_day = parseInt(billDay);
			if (isCredit && repayDay) data.repay_day = parseInt(repayDay);
			if (remark.trim()) data.remark = remark.trim();

			await accountApi.create(data);
			hzToast.success('账户创建成功');
			await appStore.loadDictionaries();
			goto('/accounts');
		} catch (e: any) {
			hzToast.error(e.message || '创建失败');
		} finally {
			loading = false;
		}
	}
</script>

<svelte:head>
	<title>新增账户 · 货殖</title>
</svelte:head>

<div class="max-w-xl mx-auto">
	<Card>
		<CardHeader>
			<CardTitle>新增账户</CardTitle>
		</CardHeader>
		<CardContent class="space-y-5">
			<!-- 账户名称 -->
			<div class="space-y-2">
				<Label>账户名称</Label>
				<div class="flex items-center gap-3">
					<AccountIcon bankName={bankName} name={name} type={type} size={44} />
					<Input
						placeholder="例如: 招商银行信用卡、微信钱包"
						bind:value={name}
						oninput={onNameInput}
					/>
				</div>
				<p class="text-xs text-muted-foreground">输入名称后会自动识别银行/机构（如「招商银行信用卡」→ 招商银行）</p>
			</div>

			<!-- 账户类型 -->
			<div class="space-y-2">
				<Label>账户类型</Label>
				<select
					class="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm focus:outline-none focus:ring-1 focus:ring-ring"
					bind:value={type}
					onchange={() => (typeTouched = true)}
				>
					{#each accountTypes as t}
						<option value={t.value}>{t.label}</option>
					{/each}
				</select>
			</div>

			<!-- 余额 -->
			<div class="space-y-2">
				<Label>当前余额</Label>
				<div class="relative">
					<span class="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground">¥</span>
					<Input
						class="pl-8"
						type="number"
						step="0.01"
						placeholder="0.00"
						bind:value={balance}
					/>
				</div>
			</div>

			<!-- 初始金额 -->
			<div class="space-y-2">
				<Label>初始金额（用于统计）</Label>
				<div class="relative">
					<span class="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground">¥</span>
					<Input
						class="pl-8"
						type="number"
						step="0.01"
						placeholder="0.00"
						bind:value={initialAmount}
					/>
				</div>
			</div>

			<!-- 银行名称 -->
			<div class="space-y-2">
				<Label>银行/机构名称</Label>
				<Input placeholder="例如: 工商银行、支付宝" bind:value={bankName} />
			</div>

			<!-- 银行卡号 -->
			<div class="space-y-2">
				<Label>银行卡号</Label>
				<Input
					placeholder="选填，仅本地展示末四位，完整卡号加密存储"
					inputmode="numeric"
					maxlength={23}
					bind:value={fullCardNo}
				/>
			</div>

			<!-- 信用卡专属字段 -->
			{#if isCredit}
				<div class="space-y-4 p-4 rounded-lg border border-dashed">
					<p class="text-sm font-medium text-muted-foreground">信用卡信息</p>

					<div class="space-y-2">
						<Label>信用额度</Label>
						<div class="relative">
							<span class="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground">¥</span>
							<Input
								class="pl-8"
								type="number"
								step="0.01"
								placeholder="0.00"
								bind:value={creditLimit}
							/>
						</div>
					</div>

					<div class="grid grid-cols-2 gap-4">
						<div class="space-y-2">
							<Label>账单日</Label>
							<Input type="number" min={1} max={31} placeholder="1-31" bind:value={billDay} />
						</div>
						<div class="space-y-2">
							<Label>还款日</Label>
							<Input type="number" min={1} max={31} placeholder="1-31" bind:value={repayDay} />
						</div>
					</div>
				</div>
			{/if}

			<!-- 开关选项 -->
			<div class="space-y-3">
				<label class="flex items-center gap-3 cursor-pointer">
					<input type="checkbox" bind:checked={includeInTotal} class="rounded border-input" />
					<span class="text-sm">纳入总资产统计</span>
				</label>
				<label class="flex items-center gap-3 cursor-pointer">
					<input type="checkbox" bind:checked={includeInBudget} class="rounded border-input" />
					<span class="text-sm">纳入预算统计</span>
				</label>
			</div>

			<!-- 备注 -->
			<div class="space-y-2">
				<Label>备注</Label>
				<Input placeholder="可选" bind:value={remark} />
			</div>

			<!-- 按钮 -->
			<div class="flex gap-2 pt-2">
				<Button class="flex-1" onclick={handleSave} disabled={loading}>
					<Save size={16} />
					{loading ? '保存中...' : '保存'}
				</Button>
				<Button variant="outline" onclick={() => goto('/accounts')}>
					<X size={16} />
					取消
				</Button>
			</div>
		</CardContent>
	</Card>
</div>
