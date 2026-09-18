<script lang="ts">
	import type { Account } from '$lib/types';
	import { getBankBrand } from '$lib/utils/bank-themes';
	import { resolveAccountIcon } from '$lib/utils/bank-icons';

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

	// 图标解析统一走 resolveAccountIcon()：手动 icon → 自动识别 → 类型兜底。
	// 历史数据里 icon 可能是 emoji（如 💳/💵），按字面量渲染，不再静默退化成首字。
	const resolved = $derived.by(() =>
		resolveAccountIcon({
			icon: icon || account?.icon,
			bankName: bankName || account?.bank_name,
			name: name || account?.name,
			type: type || account?.type
		})
	);

	const brand = $derived.by(() => {
		if (type === 'cash' || account?.type === 'cash') {
			return { short: '¥', color: '#0ea5e9' };
		}
		const src = bankName || account?.bank_name || name || account?.name || '';
		return getBankBrand(src);
	});

	const title = $derived(bankName || name || account?.bank_name || account?.name || '');
</script>

{#if resolved?.kind === 'svg'}
	<img
		src={resolved.url}
		alt={title}
		class="shrink-0 rounded-lg {className}"
		style="width:{size}px;height:{size}px"
	/>
{:else}
	<div
		class="grid shrink-0 place-items-center rounded-lg font-semibold text-white {className}"
		style="width:{size}px;height:{size}px;background:{brand.color};font-size:{Math.round(size * 0.42)}px"
		{title}
	>
		{resolved?.kind === 'glyph' ? resolved.text : brand.short}
	</div>
{/if}
