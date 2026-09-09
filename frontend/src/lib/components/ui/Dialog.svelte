<script lang="ts">
import { cn } from '$lib/utils/cn';

let {
	open = $bindable(false),
	onOpenChange,
	children,
	class: className = '',
	...rest
}: {
	open?: boolean;
	onOpenChange?: (open: boolean) => void;
	children?: any;
	class?: string;
} = $props();

function close() {
	open = false;
	onOpenChange?.(false);
}

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

{#if open}
	<button
		type="button"
		class="fixed inset-0 z-50 flex items-center justify-center"
		onclick={(e) => {
			if (e.target === e.currentTarget) close();
		}}
	>
		<!-- Backdrop -->
		<div class="absolute inset-0 bg-black/50 backdrop-blur-sm"></div>

		<!-- Content -->
		<div
			class={cn(
				'relative z-10 w-full max-w-lg rounded-xl border bg-card p-6 shadow-lg',
				className
			)}
			role="dialog"
			aria-modal="true"
			{...rest}
		>
			<button
				class="absolute right-4 top-4 rounded-sm opacity-70 ring-offset-background transition-opacity hover:opacity-100 focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2"
				onclick={close}
			>
				<svg
					xmlns="http://www.w3.org/2000/svg"
					width="16"
					height="16"
					viewBox="0 0 24 24"
					fill="none"
					stroke="currentColor"
					stroke-width="2"
					stroke-linecap="round"
					stroke-linejoin="round"
				>
					<path d="M18 6 6 18" />
					<path d="m6 6 12 12" />
				</svg>
				<span class="sr-only">Close</span>
			</button>
			{@render children?.({ close })}
		</div>
	</button>
{/if}
