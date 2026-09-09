<script lang="ts">
	import type { Account } from '$lib/types';
	import { getBankBrand } from '$lib/utils/bank-themes';

	let {
		account = null,
		bankName = '',
		name = '',
		type = '',
		size = 40,
		class: className = ''
	}: {
		account?: Account | null;
		bankName?: string;
		name?: string;
		type?: string;
		size?: number;
		class?: string;
	} = $props();

	const brand = $derived.by(() => {
		if (type === 'cash' || account?.type === 'cash') {
			return { short: '¥', color: '#0ea5e9' };
		}
		const src = bankName || account?.bank_name || name || account?.name || '';
		return getBankBrand(src);
	});
</script>

<div
	class="grid shrink-0 place-items-center rounded-lg font-semibold text-white {className}"
	style="width:{size}px;height:{size}px;background:{brand.color};font-size:{Math.round(size * 0.42)}px"
	title={bankName || name || account?.bank_name || account?.name || ''}
>
	{brand.short}
</div>
