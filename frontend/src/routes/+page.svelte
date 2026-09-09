<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { appStore } from '$lib/stores/app';

	onMount(async () => {
		const ok = await appStore.checkAuth();
		if (ok) {
			goto('/dashboard', { replaceState: true });
		} else {
			goto('/login', { replaceState: true });
		}
	});
</script>

<div class="min-h-screen grid place-items-center text-muted-foreground text-sm">
	<div class="animate-pulse">正在加载货殖...</div>
</div>
