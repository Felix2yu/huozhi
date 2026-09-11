<script lang="ts">
	import { goto } from '$app/navigation';
	import Button from '$lib/components/ui/Button.svelte';
	import Card from '$lib/components/ui/Card.svelte'
import CardContent from '$lib/components/ui/CardContent.svelte'
import CardHeader from '$lib/components/ui/CardHeader.svelte'
import CardTitle from '$lib/components/ui/CardTitle.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import Label from '$lib/components/ui/Label.svelte';
	import Badge from '$lib/components/ui/Badge.svelte';
	import { appStore } from '$lib/stores/app';
	import { themeStore } from '$lib/stores/theme';
	import { authApi } from '$lib/api/modules/auth';
	import { ioApi } from '$lib/api/modules/io';
	import { http } from '$lib/api/http';
	import { hzToast } from '$lib/components/ui/toast';
	import { onMount } from 'svelte';
	import { User, Palette, LogOut, CloudOff, Moon, Sun, Monitor, CreditCard, Key, Copy, Eye, EyeOff, Lock, Download, Upload } from '@lucide/svelte';

	let nickname = $state('');
	let email = $state('');
	let theme = $state(themeStore.value);
	let loading = $state(false);

	// 修改密码
	let showChangePwd = $state(false);
	let oldPwd = $state('');
	let newPwd = $state('');
	let confirmPwd = $state('');
	let pwdLoading = $state(false);

	// API Key 状态
	let apiKeyInfo = $state<{ api_key: string; api_key_enabled: boolean; has_api_key: boolean } | null>(null);
	let apiKeyLoading = $state(true);
	let showApiKey = $state(false);
	let toggleLoading = $state(false);

	onMount(async () => {
		if (appStore.user) {
			nickname = appStore.user.nickname;
			email = appStore.user.email || '';
		}
		try {
			apiKeyInfo = await http.get('/api-key');
		} catch {}
		apiKeyLoading = false;
	});

	async function handleGenerateApiKey() {
		try {
			const res = await http.post<{ api_key: string; api_key_enabled: boolean }>('/api-key/generate');
			apiKeyInfo = { ...apiKeyInfo, ...res, has_api_key: true };
			showApiKey = true;
			hzToast.success('API密钥已生成');
		} catch (e: any) {
			hzToast.error(e.message || '生成失败');
		}
	}

	async function handleToggleApiKey() {
		toggleLoading = true;
		try {
			const res = await http.post<{ api_key_enabled: boolean }>('/api-key/toggle');
			if (apiKeyInfo) apiKeyInfo.api_key_enabled = res.api_key_enabled;
			hzToast.success(res.api_key_enabled ? 'API密钥已启用' : 'API密钥已禁用');
		} catch (e: any) {
			hzToast.error(e.message || '操作失败');
		} finally {
			toggleLoading = false;
		}
	}

	function copyApiKey() {
		if (apiKeyInfo?.api_key) {
			navigator.clipboard.writeText(apiKeyInfo.api_key);
			hzToast.success('已复制到剪贴板');
		}
	}

	function handleExport() {
		ioApi.exportCSV({ book_id: appStore.currentBookId });
		hzToast.success('导出已开始');
	}

	function handleDownloadTemplate() {
		ioApi.template();
	}

	let importFile = $state<File | null>(null);
	let importLoading = $state(false);
	let importSource = $state<'qianji' | 'alipay' | 'wechat'>('qianji');

	function triggerImport() {
		const input = document.createElement('input');
		input.type = 'file';
		input.accept = '.csv,.xlsx,.xls';
		input.onchange = (e) => {
			const file = (e.target as HTMLInputElement).files?.[0];
			if (file) {
				importFile = file;
				doImport();
			}
		};
		input.click();
	}

	async function doImport() {
		if (!importFile) return;
		importLoading = true;
		try {
			const res = await ioApi.import(importSource, appStore.effectiveBookId(), importFile);
			if (res.ok) {
				hzToast.success(`成功导入 ${res.count || 0} 笔交易`);
				importFile = null;
				await appStore.loadDictionaries();
			} else {
				hzToast.error(res.message || '导入失败');
			}
		} catch (e: any) {
			hzToast.error(e.message || '导入失败');
		} finally {
			importLoading = false;
		}
	}

	async function handleChangePassword() {
		if (!oldPwd || !newPwd) {
			hzToast.warning('请填写完整');
			return;
		}
		if (newPwd !== confirmPwd) {
			hzToast.warning('两次密码不一致');
			return;
		}
		if (newPwd.length < 6) {
			hzToast.warning('密码至少6位');
			return;
		}
		pwdLoading = true;
		try {
			await authApi.changePwd({ old_password: oldPwd, new_password: newPwd });
			hzToast.success('密码修改成功');
			showChangePwd = false;
			oldPwd = '';
			newPwd = '';
			confirmPwd = '';
		} catch (e: any) {
			hzToast.error(e.message || '修改失败');
		} finally {
			pwdLoading = false;
		}
	}

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
						class:border-primary={theme === 'light'}
						onclick={() => handleThemeChange('light')}
					>
						<Sun size={20} />
						<span class="text-xs">浅色</span>
					</button>
					<button
						class="flex flex-col items-center gap-2 p-4 rounded-lg border transition"
						class:border-primary={theme === 'dark'}
						onclick={() => handleThemeChange('dark')}
					>
						<Moon size={20} />
						<span class="text-xs">深色</span>
					</button>
					<button
						class="flex flex-col items-center gap-2 p-4 rounded-lg border transition"
						class:border-primary={theme === 'system'}
						onclick={() => handleThemeChange('system')}
					>
						<Monitor size={20} />
						<span class="text-xs">跟随系统</span>
					</button>
				</div>
			</CardContent>
		</Card>
	</section>

	<!-- 安全 -->
	<section>
		<div class="flex items-center gap-2 mb-3">
			<Lock size={16} />
			<h2 class="text-sm font-medium">安全</h2>
		</div>
		<Card>
			<CardContent class="p-4">
				{#if showChangePwd}
					<div class="space-y-3">
						<div class="space-y-2">
							<Label>当前密码</Label>
							<Input type="password" bind:value={oldPwd} />
						</div>
						<div class="space-y-2">
							<Label>新密码</Label>
							<Input type="password" bind:value={newPwd} />
						</div>
						<div class="space-y-2">
							<Label>确认新密码</Label>
							<Input type="password" bind:value={confirmPwd} />
						</div>
						<div class="flex gap-2">
							<Button size="sm" onclick={handleChangePassword} disabled={pwdLoading}>
								{pwdLoading ? '保存中...' : '保存'}
							</Button>
							<Button size="sm" variant="outline" onclick={() => (showChangePwd = false)}>取消</Button>
						</div>
					</div>
				{:else}
					<button
						class="w-full flex items-center justify-between p-3 rounded-lg border hover:bg-accent transition"
						onclick={() => (showChangePwd = true)}
					>
						<span>修改密码</span>
						<span class="text-muted-foreground text-sm">→</span>
					</button>
				{/if}
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
					onclick={handleExport}
				>
					<span class="flex items-center gap-2">
						<Download size={16} />
						导出账单 (CSV)
					</span>
					<span class="text-muted-foreground text-sm">→</span>
				</button>
				<div class="flex items-center gap-2">
					<select
						class="flex h-9 rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm focus:outline-none focus:ring-1 focus:ring-ring"
						bind:value={importSource}
					>
						<option value="qianji">钱迹</option>
						<option value="alipay">支付宝</option>
						<option value="wechat">微信</option>
					</select>
					<button
						class="flex-1 flex items-center justify-between p-3 rounded-lg border hover:bg-accent transition"
						onclick={triggerImport}
					>
						<span class="flex items-center gap-2">
							<Upload size={16} />
							{importLoading ? '导入中...' : '导入数据'}
						</span>
						<span class="text-muted-foreground text-sm">→</span>
					</button>
				</div>
				<button
					class="w-full flex items-center justify-between p-3 rounded-lg border hover:bg-accent transition text-sm text-muted-foreground"
					onclick={handleDownloadTemplate}
				>
					<span>下载导入模板</span>
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

	<!-- API密钥 -->
	<section>
		<div class="flex items-center gap-2 mb-3">
			<Key size={16} />
			<h2 class="text-sm font-medium">API密钥</h2>
		</div>
		<Card>
			<CardContent class="p-4 space-y-4">
				{#if apiKeyLoading}
					<div class="text-sm text-muted-foreground animate-pulse">加载中...</div>
				{:else if !apiKeyInfo?.has_api_key}
					<p class="text-sm text-muted-foreground">尚未生成API密钥，生成后可通过密钥访问公开账单接口。</p>
					<Button size="sm" onclick={handleGenerateApiKey}>
						<Key size={14} />
						生成API密钥
					</Button>
				{:else}
					<div class="space-y-3">
						<div class="flex items-center gap-2">
							<span class="text-sm">状态:</span>
							<Badge variant={apiKeyInfo.api_key_enabled ? 'default' : 'secondary'}>
								{apiKeyInfo.api_key_enabled ? '已启用' : '已禁用'}
							</Badge>
						</div>
						<div class="space-y-2">
							<Label>API密钥</Label>
							<div class="flex items-center gap-2">
								<Input
									readonly
									value={showApiKey ? (apiKeyInfo.api_key || '') : '••••••••••••••••'}
									class="font-mono text-xs"
								/>
								<Button size="icon" variant="outline" onclick={() => (showApiKey = !showApiKey)}>
									{#if showApiKey}<EyeOff size={14} />{:else}<Eye size={14} />{/if}
								</Button>
								<Button size="icon" variant="outline" onclick={copyApiKey}>
									<Copy size={14} />
								</Button>
							</div>
						</div>
						<div class="flex gap-2">
							<Button size="sm" variant="outline" onclick={handleToggleApiKey} disabled={toggleLoading}>
								{apiKeyInfo.api_key_enabled ? '禁用' : '启用'}
							</Button>
							<Button size="sm" variant="outline" onclick={handleGenerateApiKey}>
								重新生成
							</Button>
						</div>
						<div class="text-xs text-muted-foreground">
							<p>使用方式: <code class="bg-muted px-1 py-0.5 rounded">Authorization: Bearer {apiKeyInfo.api_key_enabled ? '[API_KEY]' : '[已禁用]'}</code></p>
						</div>
					</div>
				{/if}
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
