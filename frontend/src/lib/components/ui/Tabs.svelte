<script lang="ts">
import { cn } from '$lib/utils/cn';

let {
	class: className = '',
	value = '',
	...rest
}: {
	class?: string;
	value?: string;
} = $props();

let current = $state(value || '');

const classes = $derived(cn('inline-flex h-9 items-center justify-center rounded-lg bg-muted p-1 text-muted-foreground', className));

function selectTab(v: string) {
	current = v;
}

defineExpose({ selectTab });
</script>

<div role="tablist" class={classes} {...rest}>
	<slot selectTab={selectTab} current={current} />
</div>
