<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import Button from '$lib/components/ui/Button.svelte';
	import Card from '$lib/components/ui/Card.svelte';
	import CardContent from '$lib/components/ui/CardContent.svelte';
	import Badge from '$lib/components/ui/Badge.svelte';
	import AccountIcon from '$lib/components/AccountIcon.svelte';
	import { appStore } from '$lib/stores/app';
	import { accountApi } from '$lib/api/modules/accounts';
	import { formatMoney } from '$lib/utils/format';
	import type { Account, AccountSummary } from '$lib/types';
	import {
		Plus,
		Wallet,
		CreditCard,
		Banknote,
		TrendingUp,
		MoreHorizontal,
		Pencil,
		Archive,
		ArchiveRestore,
		EyeOff
	} from '@lucide/svelte';

	let all = $state<Account[]>([]);
	let summary = $state<AccountSummary | null>(null);
	let loading = $state(true);
	let menuOpen = $state<number | null>(null);
	let busyId = $state<number | null>(null);

	const labels: Record<string, { label: string; icon: any }> = {
		cash: { label: '现金', icon: Banknote },
		bank: { label: '银行卡', icon: Wallet },
		credit: { label: '信用卡', icon: CreditCard },
		prepaid: { label: '预付卡', icon: Wallet },
		investment: { label: '投资', icon: TrendingUp },
		liability: { label: '负债', icon: CreditCard },
		virtual: { label: '虚拟账户', icon: Wallet }
	};

	const active = $derived(all.filter((a) => !a.is_archived));
	const archived = $derived(all.filter((a) => a.is_archived));
	const byType = $derived.by(() => {
		const groups: Record<string, Account[]> = {};
		for (const acc of active) {
			(groups[acc.type] ||= []).push(acc);
		}
		return groups;
	});

	async function load() {
		try {
			const res: any = await accountApi.list({
				book_id: appStore.currentBookId,
				include_archived: 1
			});
			all = res.accounts || [];
			summary = res.summary;
		} catch {
			// ignore
		}
		loading = false;
	}

	onMount(load);

	async function doArchive(id: number) {
		busyId = id;
		menuOpen = null;
		try {
			await accountApi.remove(id);
			await load();
			await appStore.loadDictionaries();
		} catch {
			// ignore
		}
		busyId = null;
	}

	async function doRestore(id: number) {
		busyId = id;
		try {
			// 发送完整账户对象，仅翻转 is_archived，避免更新接口把其它字段清空
			const acc = archived.find((a) => a.id === id);
			await accountApi.update(id, { ...acc, is_archived: false });
			await load();
			await appStore.loadDictionaries();
		} catch {
			// ignore
		}
		busyId = null;
	}
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

	{#if loading}
		<div class="py-16 text-center text-muted-foreground text-sm animate-pulse">加载中...</div>
	{:else if active.length === 0 && archived.length === 0}
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
	{:else}
		<!-- 活跃账户：按类型分组 -->
		{#each Object.entries(byType) as [type, accs]}
			{@const info = labels[type] || { label: type, icon: Wallet }}
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
						<div class="relative">
							<Card
								class="cursor-pointer hover:border-primary/50 transition"
								onclick={() => goto(`/accounts/${acc.id}`)}
							>
								<CardContent class="p-4">
									<div class="flex items-start justify-between">
										<div class="flex items-center gap-3 min-w-0">
											<AccountIcon account={acc} size={40} />
											<div class="min-w-0">
												<div class="font-medium truncate">{acc.name}</div>
												<div class="text-xs text-muted-foreground truncate">
													{acc.bank_name || info.label}
													{#if acc.card_no4} · •••• {acc.card_no4}{/if}
												</div>
											</div>
										</div>
										<button
											class="p-1 rounded hover:bg-accent shrink-0"
											onclick={(e) => {
												e.stopPropagation();
												menuOpen = menuOpen === acc.id ? null : acc.id;
											}}
										>
											<MoreHorizontal size={16} class="text-muted-foreground" />
										</button>
									</div>
									<div class="mt-3 flex items-end justify-between">
										<div class="text-xl font-bold tabular-nums">
											{formatMoney(acc.balance)}
										</div>
										{#if acc.type === 'credit' && acc.credit_limit}
											<Badge variant="outline">额度 {formatMoney(acc.credit_limit)}</Badge>
										{/if}
									</div>
								</CardContent>
							</Card>

							{#if menuOpen === acc.id}
								<div
									class="fixed inset-0 z-40"
									onclick={() => (menuOpen = null)}
									aria-hidden="true"
								></div>
								<div
									class="absolute z-50 right-2 top-12 w-36 rounded-md border bg-card p-1 shadow-md"
								>
									<button
										class="flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-sm hover:bg-accent"
										onclick={(e) => {
											e.stopPropagation();
											menuOpen = null;
											goto(`/accounts/${acc.id}`);
										}}
									>
										<Pencil size={14} /> 编辑
									</button>
									<button
										class="flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-sm text-destructive hover:bg-destructive/10 disabled:opacity-50"
										onclick={(e) => {
											e.stopPropagation();
											doArchive(acc.id);
										}}
										disabled={busyId === acc.id}
									>
										<Archive size={14} /> 归档
									</button>
								</div>
							{/if}
						</div>
					{/each}
				</div>
			</section>
		{/each}

		<!-- 已归档账户 -->
		{#if archived.length > 0}
			<section class="mt-2">
				<details>
					<summary
						class="flex cursor-pointer items-center gap-2 text-sm font-medium text-muted-foreground"
					>
						<EyeOff size={16} /> 已归档账户 ({archived.length})
					</summary>
					<div class="mt-3 grid gap-3 md:grid-cols-2 lg:grid-cols-3">
						{#each archived as acc (acc.id)}
							<Card class="opacity-70">
								<CardContent class="p-4">
									<div class="flex items-start justify-between">
										<div class="flex items-center gap-3 min-w-0">
											<AccountIcon account={acc} size={40} />
											<div class="min-w-0">
												<div class="font-medium truncate">{acc.name}</div>
												<div class="text-xs text-muted-foreground truncate">
													{acc.bank_name || labels[acc.type]?.label || ''}
												</div>
											</div>
										</div>
										<button
											class="flex shrink-0 items-center gap-1 rounded-md border px-2 py-1 text-xs hover:bg-accent disabled:opacity-50"
											onclick={() => doRestore(acc.id)}
											disabled={busyId === acc.id}
										>
											<ArchiveRestore size={13} /> 恢复
										</button>
									</div>
								</CardContent>
							</Card>
						{/each}
					</div>
				</details>
			</section>
		{/if}
	{/if}
</div>
