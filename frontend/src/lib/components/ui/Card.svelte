<script lang="ts">
import { cn } from '$lib/utils/cn';
let { class: className = '', ...rest }: { class?: string } = $props();
</script>

<div
	class={cn('rounded-xl border bg-card text-card-foreground shadow', className)}
	{...rest}
>
	<slot />
</div>
