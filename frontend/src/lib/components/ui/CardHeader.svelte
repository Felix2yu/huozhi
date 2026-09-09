<script lang="ts">
import { cn } from '$lib/utils/cn';
let { class: className = '', ...rest }: { class?: string } = $props();
</script>

<div class={cn('flex flex-col space-y-1.5 p-6', className)} {...rest}>
	<slot />
</div>
