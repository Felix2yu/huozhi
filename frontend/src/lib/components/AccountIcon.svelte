<script lang="ts">
	import type { Account } from '$lib/types';
	import { getBankBrand, getBankIcon } from '$lib/utils/bank-themes';

	// 使用 Vite 的 import.meta.glob 批量导入所有 SVG 文件
	const bankIcons = import.meta.glob<string>('$lib/assets/bank-icons/*.svg', {
		eager: true,
		query: '?url',
		import: 'default'
	});

	let {
		account = null,
		bankName = '',
		name = '',
		type = '',
		icon = '',
		size = 40,
		class: className = ''
	}: {
		account?: Account | null;
		bankName?: string;
		name?: string;
		type?: string;
		icon?: string;
		size?: number;
		class?: string;
	} = $props();

	const iconPath = $derived.by(() => {
		// 优先使用手动选择的图标
		const manualIcon = icon || account?.icon;
		if (manualIcon) {
			const key = `/src/lib/assets/bank-icons/${manualIcon}.svg`;
			if (bankIcons[key]) return bankIcons[key];
		}
		if (type === 'cash' || account?.type === 'cash') return null;
		const src = bankName || account?.bank_name || name || account?.name || '';
		const iconId = getBankIcon(src);
		if (iconId) {
			// 从导入的 SVG 中查找对应的文件
			const key = `/src/lib/assets/bank-icons/${iconId}.svg`;
			return bankIcons[key] || null;
		}
		return null;
	});

	const brand = $derived.by(() => {
		if (type === 'cash' || account?.type === 'cash') {
			return { short: '¥', color: '#0ea5e9' };
		}
		const src = bankName || account?.bank_name || name || account?.name || '';
		return getBankBrand(src);
	});
</script>

{#if iconPath}
	<img
		src={iconPath}
		alt={bankName || name || account?.bank_name || account?.name || ''}
		class="shrink-0 rounded-lg {className}"
		style="width:{size}px;height:{size}px"
	/>
{:else}
	<div
		class="grid shrink-0 place-items-center rounded-lg font-semibold text-white {className}"
		style="width:{size}px;height:{size}px;background:{brand.color};font-size:{Math.round(size * 0.42)}px"
		title={bankName || name || account?.bank_name || account?.name || ''}
	>
		{brand.short}
	</div>
{/if}
