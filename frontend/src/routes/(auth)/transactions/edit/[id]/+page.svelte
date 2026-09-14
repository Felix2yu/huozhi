<script lang="ts">
	// 编辑交易：与记一笔共用 TransactionForm，消除两份复制粘贴的实现（C16）
	import { page } from '$app/stores';
	import TransactionForm from '$lib/components/TransactionForm.svelte';

	const id = $derived(($page?.params as any)?.id);
</script>

<svelte:head>
	<title>编辑交易 · 货殖</title>
</svelte:head>

{#key id}
	<TransactionForm {id} />
{/key}
