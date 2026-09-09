<script lang="ts">
import { getContext } from 'svelte';
import { cn } from '$lib/utils/cn';

let {
	class: className = '',
	value,
	children
}: {
	class?: string;
	value: string;
	children?: any;
} = $props();

const ctx = getContext<{ value: string }>('tabs-current');
let selected = $derived(ctx.value === value);

const classes = $derived(
	cn(
		'inline-flex items-center justify-center whitespace-nowrap rounded-md px-3 py-1 text-sm font-medium ring-offset-background transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-50',
		selected
			? 'bg-background text-foreground shadow'
			: 'hover:text-foreground',
		className
	)
);

function handleClick() {
	ctx.value = value;
}
</script>

<button
	type="button"
	role="tab"
	data-state={selected ? 'active' : 'inactive'}
	value={value}
	class={classes}
	onclick={handleClick}
>
	{@render children?.()}
</button>
