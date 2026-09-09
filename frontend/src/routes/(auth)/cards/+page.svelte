<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import Button from '$lib/components/ui/Button.svelte';
	import Card from '$lib/components/ui/Card.svelte'
	import CardContent from '$lib/components/ui/CardContent.svelte'
	import Badge from '$lib/components/ui/Badge.svelte';
	import { accountApi } from '$lib/api/modules/accounts';
	import { appStore } from '$lib/stores/app';
	import { formatMoney } from '$lib/utils/format';
	import { getBankTheme, getCardGradient } from '$lib/utils/bank-themes';
	import type { Account, CreditRepayItem } from '$lib/types';
	import { Plus, CreditCard, Wallet } from '@lucide/svelte';

	let creditCards = $state<Account[]>([]);
	let bankCards = $state<Account[]>([]);
	let repayItems = $state<CreditRepayItem[]>([]);
	let loading = $state(true);

	onMount(async () => {
		try {
			const allAccounts = appStore.accounts;
			creditCards = allAccounts.filter((a) => a.type === 'credit');
			bankCards = allAccounts.filter((a) => a.type === 'bank');
			if (creditCards.length > 0) {
				repayItems = await accountApi.creditSummary();
			}
		} catch {
			// ignore
		} finally {
			loading = false;
		}
	});

	function formatCardNumber(card: Account): string {
		if (card.card_no4) {
			return `•••• ${card.card_no4}`;
		}
		return '•••• ••••';
	}
</script>

<svelte:head>
	<title>我的银行卡 · 货殖</title>
</svelte:head>

<div class="space-y-6">
	<!-- 信用卡列表 -->
	{#if creditCards.length > 0}
		<section>
			<div class="flex justify-between items-center mb-4">
				<h2 class="text-sm font-medium">我的信用卡</h2>
				<Button size="sm" onclick={() => goto('/accounts/new')}>
					<Plus size={16} />
					添加卡
				</Button>
			</div>

			<div class="grid gap-4 md:grid-cols-2">
				{#each creditCards as card (card.id)}
					{@const theme = getBankTheme(card.bank_name)}
					<button
						type="button"
						class="relative h-56 rounded-2xl p-5 text-white overflow-hidden cursor-pointer transition-transform hover:scale-[1.02] text-left"
						style="background: {getCardGradient(card.bank_name)};"
						onclick={() => goto(`/accounts/${card.id}`)}
					>
						<!-- 装饰圆 -->
						<div class="absolute -top-10 -right-10 w-40 h-40 rounded-full opacity-20"
							style="background: rgba(255,255,255,0.3);"></div>
						<div class="absolute -bottom-16 -left-16 w-48 h-48 rounded-full opacity-10"
							style="background: rgba(255,255,255,0.4);"></div>

						<!-- 银行名 -->
						<div class="relative flex items-center justify-between">
							<span class="font-semibold text-lg">{card.bank_name || card.name}</span>
						</div>

						<!-- 卡号 -->
						<div class="relative mt-6 font-mono text-xl tracking-wider">
							{formatCardNumber(card)}
						</div>

						<!-- 底部信息 -->
						<div class="relative mt-6 flex items-end justify-between text-xs">
							<div>
								<div class="opacity-75">持卡人</div>
								<div class="font-medium uppercase">{card.name}</div>
							</div>
							{#if card.credit_limit}
								<div class="text-right">
									<div class="opacity-75">额度</div>
									<div class="font-medium">{formatMoney(card.credit_limit)}</div>
								</div>
							{/if}
						</div>

						<!-- 余额角标 -->
						<div class="absolute bottom-3 right-4 text-right text-[11px] opacity-75">
							<div>欠款</div>
							<div class="font-medium text-sm">{formatMoney(card.balance)}</div>
						</div>
					</button>
				{/each}
			</div>
		</section>
	{/if}

	<!-- 储蓄卡列表 -->
	{#if bankCards.length > 0}
		<section>
			<div class="flex justify-between items-center mb-4">
				<h2 class="text-sm font-medium">我的储蓄卡</h2>
				<Button size="sm" onclick={() => goto('/accounts/new')}>
					<Plus size={16} />
					添加卡
				</Button>
			</div>

			<div class="grid gap-4 md:grid-cols-2">
				{#each bankCards as card (card.id)}
					{@const theme = getBankTheme(card.bank_name)}
					<button
						type="button"
						class="relative h-48 rounded-2xl p-5 text-white overflow-hidden cursor-pointer transition-transform hover:scale-[1.02] text-left"
						style="background: {getCardGradient(card.bank_name)};"
						onclick={() => goto(`/accounts/${card.id}`)}
					>
						<!-- 装饰圆 -->
						<div class="absolute -top-10 -right-10 w-40 h-40 rounded-full opacity-20"
							style="background: rgba(255,255,255,0.3);"></div>
						<div class="absolute -bottom-16 -left-16 w-48 h-48 rounded-full opacity-10"
							style="background: rgba(255,255,255,0.4);"></div>

						<!-- 银行名 -->
						<div class="relative flex items-center justify-between">
							<span class="font-semibold text-lg">{card.bank_name || card.name}</span>
						</div>

						<!-- 卡号 -->
						<div class="relative mt-4 font-mono text-lg tracking-wider">
							{formatCardNumber(card)}
						</div>

						<!-- 底部信息 -->
						<div class="relative mt-4 flex items-end justify-between text-xs">
							<div>
								<div class="opacity-75">持卡人</div>
								<div class="font-medium uppercase">{card.name}</div>
							</div>
							<div class="text-right">
								<div class="opacity-75">余额</div>
								<div class="font-medium">{formatMoney(card.balance)}</div>
							</div>
						</div>
					</button>
				{/each}
			</div>
		</section>
	{/if}

	<!-- 空状态 -->
	{#if !loading && creditCards.length === 0 && bankCards.length === 0}
		<section>
			<div class="flex justify-between items-center mb-4">
				<h2 class="text-sm font-medium">我的银行卡</h2>
				<Button size="sm" onclick={() => goto('/accounts/new')}>
					<Plus size={16} />
					添加卡
				</Button>
			</div>
			<Card>
				<div class="py-12 text-center">
					<CreditCard size={32} class="mx-auto mb-3 text-muted-foreground" />
					<p class="text-muted-foreground text-sm">还没有添加银行卡</p>
					<p class="text-xs text-muted-foreground mt-1">添加信用卡或储蓄卡来管理你的账户</p>
				</div>
			</Card>
		</section>
	{/if}

	<!-- 还款提醒 -->
	{#if repayItems.length > 0}
		<section>
			<h2 class="text-sm font-medium mb-3">还款倒计时</h2>
			<Card>
				<CardContent class="p-0 divide-y">
					{#each repayItems as item (item.id)}
						{@const theme = getBankTheme(item.bank_name)}
						<div class="flex items-center gap-4 p-4">
						<div class="w-12 h-12 rounded-xl grid place-items-center font-bold text-lg"
							style="background: {theme.color};">
							{(card.bank_name || card.name || '?')[0]}
						</div>
							<div class="flex-1 min-w-0">
								<div class="font-medium truncate">
									{item.bank_name} · {formatCardNumber({ card_no4: item.card_no4 })}
								</div>
								<div class="text-xs text-muted-foreground">
									应还 {formatMoney(item.bill_amount)} · 到期 {item.repay_date}
								</div>
							</div>
							<Badge variant={item.overdue ? 'destructive' : 'outline'}>
								{item.overdue ? '逾期' : '还剩'} {item.days_left} 天
							</Badge>
						</div>
					{/each}
				</CardContent>
			</Card>
		</section>
	{/if}
</div>
