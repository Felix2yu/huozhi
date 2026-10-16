<script lang="ts">
import type { HTMLLabelAttributes } from 'svelte/elements';
import { cn } from '#lib/utils/cn';

let { class: className = '', ...rest }: { class?: string } & HTMLLabelAttributes = $props();

const classes = $derived(
	cn(
		'text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70',
		className
	)
);
</script>

<label class={classes} {...rest}>
	<slot />
</label>
