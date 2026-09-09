<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import Button from '$lib/components/ui/Button.svelte';
	import Card from '$lib/components/ui/Card.svelte'
import CardContent from '$lib/components/ui/CardContent.svelte'
import CardHeader from '$lib/components/ui/CardHeader.svelte'
import CardTitle from '$lib/components/ui/CardTitle.svelte';
	import Badge from '$lib/components/ui/Badge.svelte';
	import { appStore } from '$lib/stores/app';
	import { accountApi } from '$lib/api/modules/accounts';
	import { formatMoney } from '$lib/utils/format';
	import type { Account, AccountSummary } from '$lib/types';
	import { Plus, Wallet, CreditCard, Banknote, PiggyBank, TrendingUp, MoreHorizontal } from '@lucide/svelte';

	let summary = $state<AccountSummary | null>(null);
	let loading = $state(true);

	const accountsByType = $derived.by(() => {
		const groups: Record<string, Account[]> = {};
		const labels: Record<string, { label: string; icon: any }> = {
			cash: { label: '现金', icon: Banknote },
			bank: { label: '银行卡', icon: Wallet },
			credit: { label: '信用卡', icon: CreditCard },
			prepaid: { label: '预付卡', icon: Wallet },
			investment: { label: '投资', icon: TrendingUp },
			liability: { label: '负债', icon: CreditCard },
			virtual: { label: '虚拟账户', icon: Wallet }
		};
		for (const acc of appStore.accounts) {
			if (!groups[acc.type]) groups[acc.type] = [];
			groups[acc.type].push(acc);
		}
		return { groups, labels };
	});

	onMount(async () => {
		try {
			const res = await accountApi.list({ book_id: appStore.currentBookId });
			summary = (res as any).summary;
		} catch {}
		loading = false;
	});
</script>

<svelte:head>
	<title>账户资产 · 货殖</title>
</svelte:head>

<div class="space-y-6">
	<!-- 资产汇总 -->
	{#if summary}
		<section class="grid grid-cols-3 gap-3">
			<Card>
				<CardContent class="pt-4 text-center">
					<div class="text-xs text-muted-foreground">总资产</div>
					<div class="mt-2 text-lg md:text-2xl font-bold tabular-nums text-primary">
						{formatMoney(summary.total_asset)}
					</div>
				</CardContent>
			</Card>
			<Card>
				<CardContent class="pt-4 text-center">
					<div class="text-xs text-muted-foreground">总负债</div>
					<div class="mt-2 text-lg md:text-2xl font-bold tabular-nums text-[var(--color-expense)]">
						{formatMoney(summary.total_debt)}
					</div>
				</CardContent>
			</Card>
			<Card>
				<CardContent class="pt-4 text-center">
					<div class="text-xs text-muted-foreground">净资产</div>
					<div class="mt-2 text-lg md:text-2xl font-bold tabular-nums">
						{formatMoney(summary.net_asset)}
					</div>
				</CardContent>
			</Card>
		</section>
	{/if}

	<!-- 新增按钮 -->
	<div class="flex justify-between items-center">
		<h2 class="text-sm font-medium">账户列表</h2>
		<Button size="sm" onclick={() => goto('/accounts/new')}>
			<Plus size={16} />
			新增账户
		</Button>
	</div>

	<!-- 账户列表 -->
	{#each Object.entries(accountsByType.groups) as [type, accs]}
		{@const info = accountsByType.labels[type]}
		<section>
			<div class="flex items-center gap-2 mb-3">
				<div class="flex items-center gap-2 text-sm font-medium text-muted-foreground">
					<info.icon size={16} />
					{info.label}
					<span class="text-xs text-muted-foreground">({accs.length})</span>
				</div>
			</div>
			<div class="grid gap-3 md:grid-cols-2 lg:grid-cols-3">
				{#each accs as acc (acc.id)}
					<Card
						class="cursor-pointer hover:border-primary/50 transition"
						onclick={() => goto(`/accounts/${acc.id}`)}
					>
						<CardContent class="p-4">
							<div class="flex items-start justify-between">
								<div class="flex items-center gap-3">
									<div
										class="w-10 h-10 rounded-lg grid place-items-center"


									>
										<info.icon
											size={20}


										/>
									</div>
									<div class="min-w-0">
										<div class="font-medium truncate">{acc.name}</div>
										<div class="text-xs text-muted-foreground truncate">
											{acc.bank_name || info.label}
											{#if acc.card_no4} · •••• {acc.card_no4}{/if}
										</div>
									</div>
								</div>
								<button
									class="p-1 rounded hover:bg-accent"
									onclick={(e) => e.stopPropagation()}
								>
									<MoreHorizontal size={16} class="text-muted-foreground" />
								</button>
							</div>
							<div class="mt-3 flex items-end justify-between">
								<div
									class="text-xl font-bold tabular-nums"

								>
									{formatMoney(acc.balance)}
								</div>
								{#if acc.type === 'credit' && acc.credit_limit}
									<Badge variant="outline">
										额度 {formatMoney(acc.credit_limit)}
									</Badge>
								{/if}
							</div>
						</CardContent>
					</Card>
				{/each}
			</div>
		</section>
	{/each}

	{#if Object.keys(accountsByType.groups).length === 0}
		<Card>
			<div class="py-16 text-center">
				<div class="text-4xl mb-4 opacity-50">💳</div>
				<p class="text-muted-foreground mb-4">还没有添加任何账户</p>
				<Button onclick={() => goto('/accounts/new')}>
					<Plus size={16} />
					立即添加
				</Button>
			</div>
		</Card>
	{/if}
</div>
