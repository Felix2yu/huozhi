<script lang="ts">
	import { getBankIcon } from '$lib/utils/bank-themes';
	import { getIconName } from '$lib/utils/icon-names';
	import AccountIcon from './AccountIcon.svelte';
	import { Search, X } from '@lucide/svelte';

	const bankIcons = import.meta.glob('$lib/assets/bank-icons/*.svg', {
		eager: true,
		query: '?url',
		import: 'default'
	});

	let {
		value = $bindable(''),
		bankName = '',
		name = '',
		type = '',
		onchange = () => {}
	}: {
		value?: string;
		bankName?: string;
		name?: string;
		type?: string;
		onchange?: (icon: string) => void;
	} = $props();

	let open = $state(false);
	let search = $state('');

	const iconList = $derived(
		Object.entries(bankIcons).map(([key, url]) => {
			const filename = key.split('/').pop()?.replace('.svg', '') || '';
			return { id: filename, name: getIconName(filename), url: url as string };
		})
	);

	const filteredIcons = $derived.by(() => {
		const q = search.trim().toLowerCase();
		if (!q) return iconList;
		return iconList.filter(
			(icon) =>
				icon.name.toLowerCase().includes(q) ||
				icon.id.toLowerCase().includes(q)
		);
	});

	const autoIcon = $derived.by(() => {
		const src = bankName || name || '';
		return getBankIcon(src) || '';
	});

	const selectedIcon = $derived(value || autoIcon);

	function selectIcon(id: string) {
		value = id === autoIcon ? '' : id;
		onchange(value);
		open = false;
		search = '';
	}

	function toggleOpen() {
		open = !open;
		if (!open) search = '';
	}
</script>

<div class="space-y-2">
	<div class="flex items-center gap-3">
		<button
			type="button"
			data-testid="icon-trigger"
			class="flex items-center gap-2 px-3 py-2 border rounded-md hover:bg-accent transition text-sm"
			onclick={toggleOpen}
		>
			{#if selectedIcon}
				<img src={bankIcons[`/src/lib/assets/bank-icons/${selectedIcon}.svg`]} alt="" class="w-6 h-6" />
			{:else}
				<AccountIcon {bankName} {name} {type} size={24} />
			{/if}
			<span>{selectedIcon ? getIconName(selectedIcon) : '自动匹配'}</span>
		</button>
		{#if value}
			<button
				type="button"
				class="text-xs text-muted-foreground hover:text-foreground"
				onclick={() => selectIcon(autoIcon)}
			>
				恢复自动
			</button>
		{/if}
	</div>

	{#if open}
		<div class="border rounded-lg bg-card">
			<!-- 搜索框 -->
			<div class="p-2 border-b">
				<div class="relative">
					<Search size={14} class="absolute left-2 top-1/2 -translate-y-1/2 text-muted-foreground" />
					<input
						type="text"
						placeholder="搜索图标名称..."
						class="w-full h-8 pl-7 pr-7 text-sm border rounded-md bg-transparent focus:outline-none focus:ring-1 focus:ring-ring"
						bind:value={search}
					/>
					{#if search}
						<button
							type="button"
							class="absolute right-2 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
							onclick={() => (search = '')}
						>
							<X size={12} />
						</button>
					{/if}
				</div>
			</div>

			<!-- 图标列表 -->
			<div class="p-3 max-h-64 overflow-y-auto">
				<div class="grid grid-cols-6 gap-2">
					<button
						type="button"
						class="flex flex-col items-center gap-1 p-2 rounded hover:bg-accent transition"
						class:ring-2={selectedIcon === ''}
						class:ring-primary={selectedIcon === ''}
						onclick={() => selectIcon(autoIcon)}
					>
						<AccountIcon {bankName} {name} {type} size={32} />
						<span class="text-[10px] text-muted-foreground">自动</span>
					</button>
					{#each filteredIcons as icon}
						<button
							type="button"
							class="flex flex-col items-center gap-1 p-2 rounded hover:bg-accent transition"
							class:ring-2={selectedIcon === icon.id}
							class:ring-primary={selectedIcon === icon.id}
							onclick={() => selectIcon(icon.id)}
							title={icon.name}
						>
							<img src={icon.url} alt={icon.name} class="w-8 h-8" />
							<span class="text-[10px] text-muted-foreground truncate w-full text-center">{icon.name}</span>
						</button>
					{/each}
				</div>
				{#if filteredIcons.length === 0}
					<div class="text-center text-sm text-muted-foreground py-4">
						未找到匹配的图标
					</div>
				{/if}
			</div>
		</div>
	{/if}
</div>
