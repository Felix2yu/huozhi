<script lang="ts">
import { cn } from '$lib/utils/cn';

let { class: className = '', ...rest }: { class?: string } = $props();

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
