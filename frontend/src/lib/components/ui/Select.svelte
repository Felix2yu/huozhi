<script lang="ts">
import { cn } from '$lib/utils/cn';

let {
	class: className = '',
	value = $bindable(),
	children,
	...rest
}: {
	class?: string;
	value?: string;
	children?: any;
} = $props();

const classes = $derived(
	cn(
		'flex h-9 w-full items-center justify-between whitespace-nowrap rounded-md border border-input bg-transparent px-3 py-2 text-sm shadow-sm ring-offset-background placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-ring disabled:cursor-not-allowed disabled:opacity-50 [&>option]:bg-background',
		className
	)
);
</script>

<select class={classes} {...rest} bind:value>
	{@render children?.()}
</select>
