<script lang="ts">
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
	import { hzToast } from '$lib/components/ui/toast';

	let username = $state('');
	let email = $state('');
	let nickname = $state('');
	let password = $state('');
	let confirm = $state('');
	let loading = $state(false);
	let error = $state('');

	function validate(): boolean {
		if (!username || username.length < 2) {
			error = '用户名至少 2 个字符';
			return false;
		}
		if (email && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) {
			error = '邮箱格式不正确';
			return false;
		}
		if (!password || password.length < 6) {
			error = '密码至少 6 个字符';
			return false;
		}
		if (password !== confirm) {
			error = '两次输入的密码不一致';
			return false;
		}
		return true;
	}

	async function handleRegister() {
		error = '';
		if (!validate()) return;
		loading = true;
		try {
			await authApi.register({
				username,
				email: email || undefined,
				nickname: nickname || username,
				password
			});
			hzToast.success('注册成功，请登录');
			goto('/login');
		} catch (e: any) {
			error = e.message || '注册失败';
			hzToast.error(error);
		} finally {
			loading = false;
		}
	}
</script>

<svelte:head>
	<title>注册 · 货殖</title>
</svelte:head>

<div class="min-h-screen flex items-center justify-center bg-background p-4">
	<Card class="w-full max-w-md">
		<CardHeader class="text-center">
			<div class="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-xl bg-primary text-primary-foreground font-bold text-lg">
				账
			</div>
			<CardTitle class="text-2xl">创建账号</CardTitle>
			<CardDescription>开始你的记账之旅</CardDescription>
		</CardHeader>
		<CardContent>
			<form class="space-y-4" onsubmit={(e) => { e.preventDefault(); handleRegister(); }}>
				<div class="space-y-2">
					<Label for="username">用户名 *</Label>
					<Input id="username" placeholder="2-20 个字符" bind:value={username} />
				</div>
				<div class="space-y-2">
					<Label for="nickname">昵称</Label>
					<Input id="nickname" placeholder="选填，显示给别人看的名字" bind:value={nickname} />
				</div>
				<div class="space-y-2">
					<Label for="email">邮箱</Label>
					<Input id="email" type="email" placeholder="选填" bind:value={email} />
				</div>
				<div class="space-y-2">
					<Label for="password">密码 *</Label>
					<Input id="password" type="password" placeholder="至少 6 位" bind:value={password} />
				</div>
				<div class="space-y-2">
					<Label for="confirm">确认密码 *</Label>
					<Input id="confirm" type="password" placeholder="再次输入密码" bind:value={confirm} />
				</div>
				{#if error}
					<p class="text-sm text-destructive">{error}</p>
				{/if}
				<Button class="w-full" type="submit" disabled={loading}>
					{#if loading}注册中...{:else}创建账号{/if}
				</Button>
			</form>
			<div class="mt-6 text-center text-sm text-muted-foreground">
				已有账号？
				<a href="/login" class="text-primary hover:underline">立即登录</a>
			</div>
		</CardContent>
	</Card>
</div>
