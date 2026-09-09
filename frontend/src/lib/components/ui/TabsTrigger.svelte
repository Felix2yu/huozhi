<script lang="ts">
import { cn } from '$lib/utils/cn';

let {
	class: className = '',
	value,
	...rest
}: {
	class?: string;
	value: string;
} = $props();

let current = $state('');
let selected = $derived(current === value);

$effect(() => {
	const parent = document.querySelector('[role="tablist"]');
	if (parent) {
		// 简单的 context 传递
	}
});

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
	selected = true;
}
</script>

<button
	type="button"
	role="tab"
	data-state={selected ? 'active' : 'inactive'}
	value={value}
	class={classes}
	onclick={handleClick}
	{...rest}
>
	<slot />
</button>
