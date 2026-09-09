<script lang="ts">
import { cn } from '$lib/utils/cn';
import { derived } from 'svelte/store';

let {
	class: className = '',
	value = 0,
	...rest
}: {
	class?: string;
	value?: number;
} = $props();

const clamped = $derived(Math.max(0, Math.min(100, value)));

const rootClasses = $derived(
	cn('relative h-2 w-full overflow-hidden rounded-full bg-secondary', className)
);

const indicatorStyle = $derived(`transform: translateX(-${100 - clamped}%);`);
</script>

<div class={rootClasses} {...rest}>
	<div
		class="h-full w-full flex-1 bg-primary transition-transform"
		style={indicatorStyle}
	></div>
</div>
