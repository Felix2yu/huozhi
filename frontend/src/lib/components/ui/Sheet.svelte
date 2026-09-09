<script lang="ts">
import { cn } from '$lib/utils/cn';

let {
	open = $bindable(false),
	onOpenChange,
	side = 'right',
	children,
	class: className = ''
}: {
	open?: boolean;
	onOpenChange?: (open: boolean) => void;
	side?: 'top' | 'bottom' | 'left' | 'right';
	children?: any;
	class?: string;
} = $props();

function close() {
	open = false;
	onOpenChange?.(false);
}

function onKeydown(e: KeyboardEvent) {
	if (e.key === 'Escape') close();
}

const panelClasses = $derived(cn(
	'fixed z-50 bg-background shadow-lg transition-transform duration-200',
	side === 'right' && 'right-0 top-0 h-full w-full max-w-md border-l',
	side === 'left' && 'left-0 top-0 h-full w-full max-w-md border-r',
	side === 'top' && 'top-0 left-0 w-full max-h-[80vh] border-b rounded-b-xl',
	side === 'bottom' && 'bottom-0 left-0 w-full max-h-[80vh] border-t rounded-t-xl',
	className
));

const hiddenClasses = $derived(cn(
	side === 'right' && 'translate-x-full',
	side === 'left' && '-translate-x-full',
	side === 'top' && '-translate-y-full',
	side === 'bottom' && 'translate-y-full'
));

$effect(() => {
	if (open) {
		document.body.style.overflow = 'hidden';
	} else {
		document.body.style.overflow = '';
	}
	return () => {
		document.body.style.overflow = '';
	};
});
</script>

<svelte:window on:keydown={onKeydown} />

{#if open}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<div
		class="fixed inset-0 z-40 bg-black/50"
		onclick={close}
	></div>
	<div class={panelClasses} role="dialog" aria-modal="true">
		{@render children?.({ close })}
	</div>
{/if}
