<script lang="ts">
import type { HTMLAttributes } from 'svelte/elements';
import { cn } from '#lib/utils/cn';
let { class: className = '', ...rest }: { class?: string } & HTMLAttributes<HTMLDivElement> = $props();
</script>

<div
	class={cn('rounded-xl border bg-card text-card-foreground shadow', className)}
	{...rest}
>
	<slot />
</div>
