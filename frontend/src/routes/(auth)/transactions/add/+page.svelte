<script lang="ts">
	// 记一笔：与编辑页共用 TransactionForm，消除两份复制粘贴的实现（C16）
	import TransactionForm from '$lib/components/TransactionForm.svelte';
</script>

<svelte:head>
	<title>记一笔 · 货殖</title>
</svelte:head>

<TransactionForm />
