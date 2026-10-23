<script lang="ts">
	import { Toaster } from 'svelte-sonner';
</script>

<Toaster
	position="top-right"
	richColors
	closeButton
	toastOptions={{
		// svelte-sonner v1 的 style 是 CSS 字符串（对象会被拼成 "[object Object]" 而整条失效）
		style:
			'background: var(--color-card); color: var(--color-foreground); border: 1px solid var(--color-border)'
	}}
/>
