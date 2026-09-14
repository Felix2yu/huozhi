<script lang="ts">
	import { getBankIcon } from '$lib/utils/bank-themes';
	import AccountIcon from './AccountIcon.svelte';

	const bankIcons = import.meta.glob('$lib/assets/bank-icons/*.svg', {
		eager: true,
		query: '?url',
		import: 'default'
	});

	let {
		value = '',
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

	const iconList = $derived(
		Object.entries(bankIcons).map(([key, url]) => {
			const filename = key.split('/').pop()?.replace('.svg', '') || '';
			return { id: filename, url: url as string };
		})
	);

	const autoIcon = $derived.by(() => {
		const src = bankName || name || '';
		return getBankIcon(src) || '';
	});

	const selectedIcon = $derived(value || autoIcon);

	function selectIcon(id: string) {
		onchange(id === autoIcon ? '' : id);
		open = false;
	}
</script>

<div class="space-y-2">
	<div class="flex items-center gap-3">
		<button
			type="button"
			class="flex items-center gap-2 px-3 py-2 border rounded-md hover:bg-accent transition text-sm"
			onclick={() => (open = !open)}
		>
			{#if selectedIcon}
				<img src={bankIcons[`/src/lib/assets/bank-icons/${selectedIcon}.svg`]} alt="" class="w-6 h-6" />
			{:else}
				<AccountIcon {bankName} {name} {type} size={24} />
			{/if}
			<span>{selectedIcon ? '已选择图标' : '自动匹配'}</span>
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
		<div class="border rounded-lg p-3 bg-card max-h-64 overflow-y-auto">
			<div class="grid grid-cols-8 gap-2">
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
				{#each iconList as icon}
					<button
						type="button"
						class="flex flex-col items-center gap-1 p-2 rounded hover:bg-accent transition"
						class:ring-2={selectedIcon === icon.id}
						class:ring-primary={selectedIcon === icon.id}
						onclick={() => selectIcon(icon.id)}
					>
						<img src={icon.url} alt={icon.id} class="w-8 h-8" />
						<span class="text-[10px] text-muted-foreground truncate w-full text-center">{icon.id}</span>
					</button>
				{/each}
			</div>
		</div>
	{/if}
</div>
