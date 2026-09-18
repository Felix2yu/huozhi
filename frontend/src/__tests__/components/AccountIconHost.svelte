<script lang="ts">
	/** AccountIcon 测试宿主：按传入的账户对象渲染图标 */
	import AccountIcon from '$lib/components/AccountIcon.svelte';

	let { account }: { account: any } = $props();
</script>

<div data-testid="icon">
	<AccountIcon {account} size={40} />
</div>
