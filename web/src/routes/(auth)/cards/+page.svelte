<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import Button from '#lib/components/ui/Button.svelte';
	import Card from '#lib/components/ui/Card.svelte';
	import CardContent from '#lib/components/ui/CardContent.svelte';
	import Badge from '#lib/components/ui/Badge.svelte';
	import { accountApi } from '#lib/api/modules/accounts';
	import { appStore } from '#lib/stores/app';
	import { formatMoney } from '#lib/utils/format';
	import { getBankTheme, getCardGradient } from '#lib/utils/bank-themes';
	import { resolveAccountIcon } from '#lib/utils/bank-icons';
	import type { Account, CreditRepayItem } from '#lib/types';
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

	// 只用到卡末四位：还款提醒里的条目不是 Account（是账单视图），
	// 参数按实际用到的字段收窄，避免调用方被迫构造一个假的 Account。
	function formatCardNumber(card: Pick<Account, 'card_no4'>): string {
		if (card.card_no4) {
			return `•••• ${card.card_no4}`;
		}
		return '•••• ••••';
	}

	// 手动 icon 优先，其次按 bank_name/name 自动识别，最后按账户类型兜底。
	// 旧实现只看 bank_name，导致账户里手动选的图标在这页永远不生效。
	function resolveCardIcon(card: Account) {
		return resolveAccountIcon({
			icon: card.icon,
			bankName: card.bank_name,
			name: card.name,
			type: card.type
		});
	}
</script>

<svelte:head>
	<title>我的银行卡 · 货殖</title>
</svelte:head>

<div class="space-y-6">
	<!-- 信用卡列表 -->
	{#if creditCards.length > 0}
		<section>
			<div class="mb-4 flex items-center justify-between">
				<h2 class="text-sm font-medium">我的信用卡</h2>
				<Button size="sm" onclick={() => goto('/accounts/new')}>
					<Plus size={16} />
					添加卡
				</Button>
			</div>

			<div class="grid gap-4 md:grid-cols-2">
				{#each creditCards as card (card.id)}
					{@const theme = getBankTheme(card.bank_name)}
					{@const ic = resolveCardIcon(card)}
					<button
						type="button"
						class="relative h-56 cursor-pointer overflow-hidden rounded-2xl p-5 text-left text-white transition-transform hover:scale-[1.02]"
						style="background: {getCardGradient(card.bank_name)};"
						onclick={() => goto(`/accounts/${card.id}`)}
					>
						<!-- 装饰圆 -->
						<div
							class="absolute -top-10 -right-10 h-40 w-40 rounded-full opacity-20"
							style="background: rgba(255,255,255,0.3);"
						></div>
						<div
							class="absolute -bottom-16 -left-16 h-48 w-48 rounded-full opacity-10"
							style="background: rgba(255,255,255,0.4);"
						></div>

						<!-- 银行名和Logo -->
						<div class="relative flex items-center justify-between">
							<div class="flex items-center gap-2">
								{#if ic?.kind === 'svg'}
									<img src={ic.url} alt={card.bank_name || card.name} class="h-8 w-8" />
								{:else if ic?.kind === 'glyph'}
									<span class="text-xl leading-none">{ic.text}</span>
								{/if}
								<span class="text-lg font-semibold">{card.bank_name || card.name}</span>
							</div>
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
						<div class="absolute right-4 bottom-3 text-right text-[11px] opacity-75">
							<div>欠款</div>
							<div class="text-sm font-medium">{formatMoney(card.balance)}</div>
						</div>
					</button>
				{/each}
			</div>
		</section>
	{/if}

	<!-- 储蓄卡列表 -->
	{#if bankCards.length > 0}
		<section>
			<div class="mb-4 flex items-center justify-between">
				<h2 class="text-sm font-medium">我的储蓄卡</h2>
				<Button size="sm" onclick={() => goto('/accounts/new')}>
					<Plus size={16} />
					添加卡
				</Button>
			</div>

			<div class="grid gap-4 md:grid-cols-2">
				{#each bankCards as card (card.id)}
					{@const theme = getBankTheme(card.bank_name)}
					{@const ic = resolveCardIcon(card)}
					<button
						type="button"
						class="relative h-48 cursor-pointer overflow-hidden rounded-2xl p-5 text-left text-white transition-transform hover:scale-[1.02]"
						style="background: {getCardGradient(card.bank_name)};"
						onclick={() => goto(`/accounts/${card.id}`)}
					>
						<!-- 装饰圆 -->
						<div
							class="absolute -top-10 -right-10 h-40 w-40 rounded-full opacity-20"
							style="background: rgba(255,255,255,0.3);"
						></div>
						<div
							class="absolute -bottom-16 -left-16 h-48 w-48 rounded-full opacity-10"
							style="background: rgba(255,255,255,0.4);"
						></div>

						<!-- 银行名和Logo -->
						<div class="relative flex items-center justify-between">
							<div class="flex items-center gap-2">
								{#if ic?.kind === 'svg'}
									<img src={ic.url} alt={card.bank_name || card.name} class="h-8 w-8" />
								{:else if ic?.kind === 'glyph'}
									<span class="text-xl leading-none">{ic.text}</span>
								{/if}
								<span class="text-lg font-semibold">{card.bank_name || card.name}</span>
							</div>
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
			<div class="mb-4 flex items-center justify-between">
				<h2 class="text-sm font-medium">我的银行卡</h2>
				<Button size="sm" onclick={() => goto('/accounts/new')}>
					<Plus size={16} />
					添加卡
				</Button>
			</div>
			<Card>
				<div class="py-12 text-center">
					<CreditCard size={32} class="mx-auto mb-3 text-muted-foreground" />
					<p class="text-sm text-muted-foreground">还没有添加银行卡</p>
					<p class="mt-1 text-xs text-muted-foreground">添加信用卡或储蓄卡来管理你的账户</p>
				</div>
			</Card>
		</section>
	{/if}

	<!-- 还款提醒 -->
	{#if repayItems.length > 0}
		<section>
			<h2 class="mb-3 text-sm font-medium">还款倒计时</h2>
			<Card>
				<CardContent class="divide-y p-0">
					{#each repayItems as item (item.id)}
						{@const theme = getBankTheme(item.bank_name)}
						{@const ic = resolveAccountIcon({ bankName: item.bank_name, name: item.name })}
						<div class="flex items-center gap-4 p-4">
							<div
								class="grid h-12 w-12 place-items-center overflow-hidden rounded-xl text-lg font-bold"
								style="background: {theme.color};"
							>
								{#if ic?.kind === 'svg'}
									<img src={ic.url} alt={item.bank_name} class="h-10 w-10" />
								{:else}
									{(item.bank_name || item.name || '?')[0]}
								{/if}
							</div>
							<div class="min-w-0 flex-1">
								<div class="truncate font-medium">
									{item.bank_name} · {formatCardNumber({ card_no4: item.card_no4 })}
								</div>
								<div class="text-xs text-muted-foreground">
									应还 {formatMoney(item.bill_amount)} · 到期 {item.repay_date}
								</div>
							</div>
							<Badge variant={item.overdue ? 'destructive' : 'outline'}>
								<!-- C17：后端已把逾期天数改为正数，文案需随之调整，
								     否则会出现「还剩 -3 天」 -->
								{#if item.overdue}
									已逾期 {Math.abs(item.days_left)} 天
								{:else if item.days_left === 0}
									今天还款
								{:else}
									还剩 {item.days_left} 天
								{/if}
							</Badge>
						</div>
					{/each}
				</CardContent>
			</Card>
		</section>
	{/if}
</div>
