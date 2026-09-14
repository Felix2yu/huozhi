<script lang="ts">
	// 记一笔：与编辑页共用 TransactionForm，消除两份复制粘贴的实现（C16）
	import { page } from '$app/state';
	import TransactionForm from '$lib/components/TransactionForm.svelte';

	// 从 URL 参数读取克隆数据（复制账单时使用）
	const cloneData = $derived.by(() => {
		const sp = page.url.searchParams;
		const type = sp.get('type');
		if (!type) return null;
		return {
			type,
			amount: Number(sp.get('amount') || 0),
			description: sp.get('description') || '',
			category_id: Number(sp.get('category_id') || 0),
			account_id: Number(sp.get('account_id') || 0),
			to_account_id: Number(sp.get('to_account_id') || 0),
			merchant: sp.get('merchant') || '',
			location: sp.get('location') || '',
			remark: sp.get('remark') || '',
			images: sp.get('images') ? JSON.parse(sp.get('images')!) : [],
			include_in_budget: sp.get('include_in_budget') !== 'false',
			currency: sp.get('currency') || 'CNY',
			exchange_rate: Number(sp.get('exchange_rate') || 0)
		};
	});
</script>

<svelte:head>
	<title>记一笔 · 货殖</title>
</svelte:head>

<TransactionForm clone={cloneData} />
