<script lang="ts">
import { Toaster } from 'svelte-sonner';
</script>

<Toaster
	position="top-right"
	richColors
	closeButton
	toastOptions={{
		style: {
			background: 'var(--color-card)',
			color: 'var(--color-foreground)',
			border: '1px solid var(--color-border)'
		}
	}}
/>
