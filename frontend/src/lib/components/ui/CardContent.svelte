<script lang="ts">
import { cn } from '$lib/utils/cn';
let { class: className = '', ...rest }: { class?: string } = $props();
</script>

<div class={cn('p-6 pt-0', className)} {...rest}>
	<slot />
</div>
