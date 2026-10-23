<script lang="ts">
	import { cn } from '#lib/utils/cn';
	let { class: className = '', ...rest }: { class?: string } = $props();
</script>

<h3 class={cn('text-lg leading-none font-semibold tracking-tight', className)} {...rest}>
	<slot />
</h3>
