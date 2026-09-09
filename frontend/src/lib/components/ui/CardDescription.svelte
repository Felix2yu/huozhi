<script lang="ts">
import { cn } from '$lib/utils/cn';
let { class: className = '', ...rest }: { class?: string } = $props();
</script>

<p class={cn('text-sm text-muted-foreground', className)} {...rest}>
	<slot />
</p>
