<script lang="ts">
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import Button from '$lib/components/ui/Button.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import Label from '$lib/components/ui/Label.svelte';
	import Card from '$lib/components/ui/Card.svelte'
import CardContent from '$lib/components/ui/CardContent.svelte'
import CardDescription from '$lib/components/ui/CardDescription.svelte'
import CardHeader from '$lib/components/ui/CardHeader.svelte'
import CardTitle from '$lib/components/ui/CardTitle.svelte';
	import { authApi } from '$lib/api/modules/auth';
	import { appStore } from '$lib/stores/app';
	import { hzToast } from '$lib/components/ui/toast';
	import { Store } from '@lucide/svelte';

	let username = $state('');
	let password = $state('');
	let loading = $state(false);
	let error = $state('');

	async function handleLogin() {
		error = '';
		if (!username || !password) {
			error = '请填写用户名和密码';
			return;
		}
		loading = true;
		try {
			const res = await authApi.login({ username, password });
			appStore.setTokenAndAuth(res.token, res.user);
			hzToast.success(`欢迎回来，${res.user.nickname}`);
			// 加载基础数据
			await appStore.loadBooks();
			await appStore.loadDictionaries();
			const redirect = $page?.url?.searchParams?.get('redirect') || '/dashboard';
			goto(redirect);
		} catch (e: any) {
			error = e.message || '登录失败';
			hzToast.error(error);
		} finally {
			loading = false;
		}
	}

	let token = $derived(typeof localStorage !== 'undefined' ? localStorage.getItem('hz_token') : null);
	import { onMount } from 'svelte';
	onMount(() => {
		if (token) goto('/dashboard');
	});
</script>

<svelte:head>
	<title>登录 · 货殖</title>
</svelte:head>

<div class="min-h-screen flex items-center justify-center bg-background p-4">
	<Card class="w-full max-w-md">
		<CardHeader class="text-center">
			<div class="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-xl bg-primary text-primary-foreground font-bold text-lg">
				账
			</div>
			<CardTitle class="text-2xl">货殖</CardTitle>
			<CardDescription>简洁纯粹的记账本</CardDescription>
		</CardHeader>
		<CardContent>
			<form class="space-y-4" onsubmit={(e) => { e.preventDefault(); handleLogin(); }}>
				<div class="space-y-2">
					<Label for="username">用户名 / 邮箱</Label>
					<Input
						id="username"
						type="text"
						placeholder="请输入用户名或邮箱"
						bind:value={username}
						autocomplete="username"
					/>
				</div>
				<div class="space-y-2">
					<Label for="password">密码</Label>
					<Input
						id="password"
						type="password"
						placeholder="请输入密码"
						bind:value={password}
						autocomplete="current-password"
					/>
				</div>
				{#if error}
					<p class="text-sm text-destructive">{error}</p>
				{/if}
				<Button class="w-full" type="submit" disabled={loading}>
					{#if loading}
						<span class="animate-pulse">登录中...</span>
					{:else}
						登录
					{/if}
				</Button>
			</form>
			<div class="mt-6 text-center text-sm text-muted-foreground">
				还没有账号？
				<a href="/register" class="text-primary hover:underline">立即注册</a>
			</div>
		</CardContent>
	</Card>
</div>
