<script lang="ts">
import { setContext, getContext } from 'svelte';
import { cn } from '$lib/utils/cn';

let {
	class: className = '',
	value = $bindable(''),
	children
}: {
	class?: string;
	value?: string;
	children?: any;
} = $props();

let current = $state(value || '');

setContext('tabs-current', {
	get value() { return current; },
	set value(v: string) { current = v; }
});

const classes = $derived(cn('inline-flex h-9 items-center justify-center rounded-lg bg-muted p-1 text-muted-foreground', className));

$effect(() => {
	value = current;
});
</script>

<div role="tablist" class={classes}>
	{@render children?.()}
</div>
