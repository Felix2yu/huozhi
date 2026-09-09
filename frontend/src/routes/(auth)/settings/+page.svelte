<script lang="ts">
	import { goto } from '$app/navigation';
	import Button from '$lib/components/ui/Button.svelte';
	import Card from '$lib/components/ui/Card.svelte'
import CardContent from '$lib/components/ui/CardContent.svelte'
import CardHeader from '$lib/components/ui/CardHeader.svelte'
import CardTitle from '$lib/components/ui/CardTitle.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import Label from '$lib/components/ui/Label.svelte';
	import { appStore } from '$lib/stores/app';
	import { themeStore } from '$lib/stores/theme';
	import { authApi } from '$lib/api/modules/auth';
	import { http } from '$lib/api/http';
	import { hzToast } from '$lib/components/ui/toast';
	import { onMount } from 'svelte';
	import { User, Palette, LogOut, CloudOff, Moon, Sun, Monitor, CreditCard } from '@lucide/svelte';

	let nickname = $state('');
	let email = $state('');
	let theme = $state(themeStore.value);
	let loading = $state(false);

	onMount(() => {
		if (appStore.user) {
			nickname = appStore.user.nickname;
			email = appStore.user.email || '';
		}
	});

	async function handleSaveProfile() {
		loading = true;
		try {
			await authApi.updateMe({ nickname, email });
			if (appStore.user) {
				appStore.setTokenAndAuth(http.getToken()!, { ...appStore.user, nickname, email });
			}
			hzToast.success('保存成功');
		} catch (e: any) {
			hzToast.error(e.message || '保存失败');
		} finally {
			loading = false;
		}
	}

	function handleThemeChange(t: 'light' | 'dark' | 'system') {
		theme = t;
		themeStore.value = t;
	}

	async function handleLogout() {
		try { await http.post<void>('/auth/logout'); } catch {}
		appStore.logout();
		goto('/login');
	}
</script>

<svelte:head>
	<title>系统设置 · 货殖</title>
</svelte:head>

<div class="space-y-6 max-w-2xl">
	<!-- 个人信息 -->
	<section>
		<div class="flex items-center gap-2 mb-3">
			<User size={16} />
			<h2 class="text-sm font-medium">个人信息</h2>
		</div>
		<Card>
			<CardContent class="p-4 space-y-4">
				<div class="flex items-center gap-3">
					<div class="w-12 h-12 rounded-full bg-primary/10 text-primary grid place-items-center text-lg font-bold">
						{appStore.user?.nickname?.[0] || 'U'}
					</div>
					<div>
						<div class="font-medium">{appStore.user?.username}</div>
						<div class="text-xs text-muted-foreground">{appStore.user?.email || '未设置邮箱'}</div>
					</div>
				</div>
				<div class="space-y-2">
					<Label>昵称</Label>
					<Input bind:value={nickname} />
				</div>
				<div class="space-y-2">
					<Label>邮箱</Label>
					<Input type="email" bind:value={email} />
				</div>
				<Button onclick={handleSaveProfile} disabled={loading}>
					{loading ? '保存中...' : '保存修改'}
				</Button>
			</CardContent>
		</Card>
	</section>

	<!-- 主题设置 -->
	<section>
		<div class="flex items-center gap-2 mb-3">
			<Palette size={16} />
			<h2 class="text-sm font-medium">外观主题</h2>
		</div>
		<Card>
			<CardContent class="p-4">
				<div class="grid grid-cols-3 gap-2">
					<button
						class="flex flex-col items-center gap-2 p-4 rounded-lg border transition"


						onclick={() => handleThemeChange('light')}
					>
						<Sun size={20} />
						<span class="text-xs">浅色</span>
					</button>
					<button
						class="flex flex-col items-center gap-2 p-4 rounded-lg border transition"


						onclick={() => handleThemeChange('dark')}
					>
						<Moon size={20} />
						<span class="text-xs">深色</span>
					</button>
					<button
						class="flex flex-col items-center gap-2 p-4 rounded-lg border transition"


						onclick={() => handleThemeChange('system')}
					>
						<Monitor size={20} />
						<span class="text-xs">跟随系统</span>
					</button>
				</div>
			</CardContent>
		</Card>
	</section>

	<!-- 数据 -->
	<section>
		<div class="flex items-center gap-2 mb-3">
			<CreditCard size={16} />
			<h2 class="text-sm font-medium">数据</h2>
		</div>
		<Card>
			<CardContent class="p-4 space-y-2">
				<button
					class="w-full flex items-center justify-between p-3 rounded-lg border hover:bg-accent transition"
					onclick={() => alert('功能开发中')}
				>
					<span>导出账单</span>
					<span class="text-muted-foreground text-sm">→</span>
				</button>
				<button
					class="w-full flex items-center justify-between p-3 rounded-lg border hover:bg-accent transition"
					onclick={() => alert('功能开发中')}
				>
					<span>导入数据</span>
					<span class="text-muted-foreground text-sm">→</span>
				</button>
			</CardContent>
		</Card>
	</section>

	<!-- 离线数据 -->
	<section>
		<div class="flex items-center gap-2 mb-3">
			<CloudOff size={16} />
			<h2 class="text-sm font-medium">离线</h2>
		</div>
		<Card>
			<CardContent class="p-4">
				<div class="flex items-center justify-between">
					<div>
						<div class="font-medium">离线请求队列</div>
						<div class="text-xs text-muted-foreground">
							当前有 {http.queueCount()} 条待同步
						</div>
					</div>
					<div class="flex gap-2">
						<Button
							size="sm"
							variant="outline"
							onclick={async () => {
								const r = await http.replayQueue();
								hzToast.success(`同步 ${r.ok} 条，剩余 ${r.remaining} 条`);
							}}
						>
							立即同步
						</Button>
						<Button
							size="sm"
							variant="ghost"
							class="text-destructive hover:text-destructive hover:bg-destructive/10"
							onclick={() => {
								http.clearQueue();
								hzToast.success('已清空');
							}}
						>
							清空
						</Button>
					</div>
				</div>
			</CardContent>
		</Card>
	</section>

	<!-- 退出登录 -->
	<Button
		class="w-full"
		variant="destructive"
		onclick={handleLogout}
	>
		<LogOut size={16} />
		退出登录
	</Button>

	<!-- 关于 -->
	<div class="text-center text-xs text-muted-foreground pt-4">
		货殖 v0.1.0 · 简洁纯粹的记账本
	</div>
</div>
