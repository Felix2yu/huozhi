<script lang="ts">
	import { appStore } from '$lib/stores/app';
	import AccountIcon from './AccountIcon.svelte';
	import { ChevronsUpDown, Check } from '@lucide/svelte';

	let {
		value = $bindable<number>(),
		onChange = (_id: number) => {},
		exclude = 0,
		placeholder = '选择账户'
	}: {
		value?: number;
		onChange?: (id: number) => void;
		exclude?: number;
		placeholder?: string;
	} = $props();

	let open = $state(false);

	// 排除已归档账户与指定排除项（如转账的对方账户）
	const list = $derived(
		appStore.accounts.filter((a) => a.id !== exclude && !a.is_archived)
	);
	const selected = $derived(list.find((a) => a.id === value) || null);

	function pick(id: number) {
		value = id;
		onChange(id);
		open = false;
	}
</script>

<div class="relative">
	<button
		type="button"
		class="flex h-9 w-full items-center justify-between gap-2 rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm focus:outline-none focus:ring-1 focus:ring-ring"
		onclick={() => (open = !open)}
	>
		{#if selected}
			<span class="flex min-w-0 items-center gap-2">
				<AccountIcon account={selected} size={22} />
				<span class="truncate">{selected.name}</span>
				{#if selected.bank_name}
					<span class="truncate text-xs text-muted-foreground">{selected.bank_name}</span>
				{/if}
			</span>
		{:else}
			<span class="text-muted-foreground">{placeholder}</span>
		{/if}
		<ChevronsUpDown size={16} class="shrink-0 text-muted-foreground" />
	</button>

	{#if open}
		<!-- 点击空白关闭 -->
		<div class="fixed inset-0 z-40" onclick={() => (open = false)} aria-hidden="true"></div>
		<div
			class="absolute z-50 mt-1 max-h-72 w-full overflow-auto rounded-md border bg-card p-1 shadow-md"
		>
			{#each list as acc (acc.id)}
				<button
					type="button"
					class="flex w-full items-center gap-2 rounded-sm px-2 py-2 text-left hover:bg-accent"
					onclick={() => pick(acc.id)}
				>
					<AccountIcon account={acc} size={28} />
					<span class="flex min-w-0 flex-col">
						<span class="truncate text-sm">{acc.name}</span>
						{#if acc.bank_name || acc.card_no4}
							<span class="truncate text-xs text-muted-foreground">
								{acc.bank_name}{acc.card_no4 ? ' · •••• ' + acc.card_no4 : ''}
							</span>
						{/if}
					</span>
					{#if acc.id === value}
						<Check size={16} class="ml-auto shrink-0 text-primary" />
					{/if}
				</button>
			{/each}
			{#if list.length === 0}
				<div class="px-2 py-3 text-center text-xs text-muted-foreground">暂无账户</div>
			{/if}
		</div>
	{/if}
</div>
