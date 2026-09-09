<script lang="ts">
	import './app.css';
	import ToastProvider from '$lib/components/ui/ToastProvider.svelte';

	let { children } = $props();
</script>

<svelte:head>
	<title>货殖 · 简洁纯粹的记账本</title>
	<meta name="description" content="货殖 - 一个简洁纯粹的个人记账系统" />
	<meta name="theme-color" content="#10B981" />
	<!-- 主题初始化脚本，避免 FOUC -->
	<script>
		(function () {
			try {
				const stored = localStorage.getItem('hz_theme');
				let theme = stored || 'system';
				if (theme === 'system') {
					theme = window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
				}
				if (theme === 'dark') {
					document.documentElement.classList.add('dark');
				}
			} catch (e) {}
		})();
	</script>
</svelte:head>

{@render children()}
<ToastProvider />
