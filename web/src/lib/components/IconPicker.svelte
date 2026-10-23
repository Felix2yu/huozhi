<script lang="ts">
	import { getBankIcon } from '#lib/utils/bank-themes';
	import { getIconName } from '#lib/utils/icon-names';
	import { bankIconUrls, resolveAccountIcon } from '#lib/utils/bank-icons';
	import AccountIcon from './AccountIcon.svelte';
	import { Search, X } from '@lucide/svelte';

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
		Object.entries(bankIconUrls).map(([id, url]) => ({
			id,
			name: getIconName(id),
			url
		}))
	);

	const filteredIcons = $derived.by(() => {
		const q = search.trim().toLowerCase();
		if (!q) return iconList;
		return iconList.filter(
			(icon) => icon.name.toLowerCase().includes(q) || icon.id.toLowerCase().includes(q)
		);
	});

	const autoIcon = $derived.by(() => {
		const src = bankName || name || '';
		return getBankIcon(src) || '';
	});

	const selectedIcon = $derived(value || autoIcon);

	// 预览与 AccountIcon 走同一套解析：value 为 emoji 等历史值时显示字面量，
	// 避免出现 src 为空的破图。
	const preview = $derived.by(() =>
		resolveAccountIcon({ icon: selectedIcon, bankName, name, type })
	);

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
			class="flex items-center gap-2 rounded-md border px-3 py-2 text-sm transition hover:bg-accent"
			onclick={toggleOpen}
		>
			{#if preview?.kind === 'svg'}
				<img src={preview.url} alt="" class="h-6 w-6" />
			{:else if preview?.kind === 'glyph'}
				<span class="text-base leading-none">{preview.text}</span>
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
		<div class="rounded-lg border bg-card">
			<!-- 搜索框 -->
			<div class="border-b p-2">
				<div class="relative">
					<Search
						size={14}
						class="absolute top-1/2 left-2 -translate-y-1/2 text-muted-foreground"
					/>
					<input
						type="text"
						placeholder="搜索图标名称..."
						class="h-8 w-full rounded-md border bg-transparent pr-7 pl-7 text-sm focus:ring-1 focus:ring-ring focus:outline-none"
						bind:value={search}
					/>
					{#if search}
						<button
							type="button"
							class="absolute top-1/2 right-2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
							onclick={() => (search = '')}
						>
							<X size={12} />
						</button>
					{/if}
				</div>
			</div>

			<!-- 图标列表 -->
			<div class="max-h-64 overflow-y-auto p-3">
				<div class="grid grid-cols-6 gap-2">
					<button
						type="button"
						class="flex flex-col items-center gap-1 rounded p-2 transition hover:bg-accent"
						class:ring-2={selectedIcon === ''}
						class:ring-primary={selectedIcon === ''}
						onclick={() => selectIcon(autoIcon)}
					>
						<AccountIcon {bankName} {name} {type} size={32} />
						<span class="text-[10px] text-muted-foreground">自动</span>
					</button>
					{#each filteredIcons as icon (icon.id)}
						<button
							type="button"
							class="flex flex-col items-center gap-1 rounded p-2 transition hover:bg-accent"
							class:ring-2={selectedIcon === icon.id}
							class:ring-primary={selectedIcon === icon.id}
							onclick={() => selectIcon(icon.id)}
							title={icon.name}
						>
							<img src={icon.url} alt={icon.name} class="h-8 w-8" />
							<span class="w-full truncate text-center text-[10px] text-muted-foreground"
								>{icon.name}</span
							>
						</button>
					{/each}
				</div>
				{#if filteredIcons.length === 0}
					<div class="py-4 text-center text-sm text-muted-foreground">未找到匹配的图标</div>
				{/if}
			</div>
		</div>
	{/if}
</div>
