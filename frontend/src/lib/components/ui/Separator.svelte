<script lang="ts">
import { cn } from '$lib/utils/cn';

let {
	class: className = '',
	orientation = 'horizontal',
	decorative = true,
	...rest
}: {
	class?: string;
	orientation?: 'horizontal' | 'vertical';
	decorative?: boolean;
} = $props();

const classes = $derived(
	cn(
		'shrink-0 bg-border',
		orientation === 'horizontal' ? 'h-[1px] w-full' : 'h-full w-[1px]',
		className
	)
);
</script>

<div
	class={classes}
	role={decorative ? undefined : 'separator'}
	aria-orientation={orientation}
	{...rest}
/>
