<script lang="ts">
	/**
	 * 仅用于测试的宿主组件：用 `bind:value` 绑定 IconPicker，
	 * 并把绑定值渲染出来，以验证「选择图标后能回传到父组件」。
	 */
	import IconPicker from '$lib/components/IconPicker.svelte';

	let {
		bankName = '',
		name = '',
		type = ''
	}: { bankName?: string; name?: string; type?: string } = $props();

	let icon = $state('');
</script>

<IconPicker bind:value={icon} {bankName} {name} {type} />
<span data-testid="bound-icon">{icon}</span>
