<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { appStore } from '#lib/stores/app';

	onMount(async () => {
		try {
			const ok = await appStore.checkAuth();
			if (ok) {
				goto('/dashboard', { replaceState: true });
			} else {
				goto('/login', { replaceState: true });
			}
		} catch (e) {
			console.error('Auth check failed', e);
			goto('/login', { replaceState: true });
		}
	});
</script>

<div class="grid min-h-screen place-items-center text-sm text-muted-foreground">
	<div class="animate-pulse">正在加载货殖...</div>
</div>
