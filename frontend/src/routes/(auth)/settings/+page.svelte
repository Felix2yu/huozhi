<script lang="ts">
	import { goto } from '$app/navigation';
	import Button from '$lib/components/ui/Button.svelte';
	import Card from '$lib/components/ui/Card.svelte';
	import CardContent from '$lib/components/ui/CardContent.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import Label from '$lib/components/ui/Label.svelte';
	import Badge from '$lib/components/ui/Badge.svelte';
	import { appStore } from '$lib/stores/app';
	import { themeStore } from '$lib/stores/theme';
	import { authApi } from '$lib/api/modules/auth';
	import { http } from '$lib/api/http';
	import { hzToast } from '$lib/components/ui/toast';
	import { onMount } from 'svelte';
	import { browser } from '$app/environment';
	import {
		User,
		Palette,
		LogOut,
		CloudOff,
		Moon,
		Sun,
		Monitor,
		Key,
		Copy,
		Eye,
		EyeOff,
		Lock,
		Heart,
		Bot
	} from '@lucide/svelte';

	let nickname = $state('');
	let email = $state('');
	let theme = $state(themeStore.value);
	let loading = $state(false);

	// B9：User 模型已有这些字段，后端 UpdateMe 也支持，但设置页此前完全没有入口
	let monthStart = $state(1);
	let currency = $state('CNY');
	let timezone = $state('Asia/Shanghai');
	let locale = $state('zh-CN');

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
			monthStart = appStore.user.month_start || 1;
			currency = appStore.user.currency || 'CNY';
			timezone = appStore.user.timezone || 'Asia/Shanghai';
			locale = appStore.user.locale || 'zh-CN';
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

	// ====== AI 助手（MCP）======
	// MCP 端点与 REST API 同前缀，直接由当前页面来源推导，
	// 避免用户在反向代理 / 自定义域名下还要手拼地址。
	let mcpClient = $state('generic');
	const mcpEndpoint = browser ? `${window.location.origin}/api/mcp` : '/api/mcp';

	const mcpClients = [
		{
			key: 'generic',
			label: '通用 / Cursor',
			hint: '把上面的 JSON 填进客户端的 MCP 配置（Cursor 为 ~/.cursor/mcp.json），重启客户端生效。'
		},
		{
			key: 'claude',
			label: 'Claude Desktop',
			hint: '写入 claude_desktop_config.json。Claude Desktop 走 stdio，用 mcp-remote 做桥接。'
		},
		{
			key: 'cherry',
			label: 'Cherry Studio',
			hint: '在「设置 → MCP 服务器」里添加，类型选「可流式传输的 HTTP」，URL 与请求头按上面填写。'
		}
	];

	const maskedKey = $derived(
		apiKeyInfo?.api_key && showApiKey ? apiKeyInfo.api_key : 'YOUR_API_KEY'
	);

	const mcpSnippet = $derived(
		mcpClient === 'claude'
			? JSON.stringify(
					{
						mcpServers: {
							huozhi: {
								command: 'npx',
								args: [
									'-y',
									'mcp-remote',
									mcpEndpoint,
									'--header',
									`X-API-Key:${maskedKey}`
								]
							}
						}
					},
					null,
					2
				)
			: JSON.stringify(
					{
						mcpServers: {
							huozhi: {
								url: mcpEndpoint,
								headers: { 'X-API-Key': maskedKey }
							}
						}
					},
					null,
					2
				)
	);

	const currentMcpClient = $derived(
		mcpClients.find((c) => c.key === mcpClient) ?? {
			key: 'generic',
			label: '通用',
			hint: '把上面的 JSON 填进客户端的 MCP 配置文件即可。'
		}
	);

	function copyText(text: string, label: string) {
		navigator.clipboard.writeText(text);
		hzToast.success(`已复制${label}`);
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
			await authApi.updateMe({
				nickname,
				email,
				// B9：账期起始日 / 币种 / 时区 / 语言
				month_start: monthStart,
				currency,
				timezone,
				locale
			});
			if (appStore.user) {
				appStore.setTokenAndAuth(http.getToken()!, {
					...appStore.user,
					nickname,
					email,
					month_start: monthStart,
					currency,
					timezone,
					locale
				});
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

				<!-- B9：账期起始日 / 币种 —— 后端模型早已支持，此前无处配置 -->
				<div class="grid grid-cols-2 gap-3">
					<div class="space-y-2">
						<Label>账期起始日</Label>
						<select
							class="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm focus:outline-none focus:ring-1 focus:ring-ring"
							bind:value={monthStart}
						>
							{#each Array.from({ length: 28 }, (_, i) => i + 1) as d}
								<option value={d}>每月 {d} 日</option>
							{/each}
						</select>
						<p class="text-[11px] text-muted-foreground">影响月度预算与统计的周期划分</p>
					</div>
					<div class="space-y-2">
						<Label>默认币种</Label>
						<select
							class="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm focus:outline-none focus:ring-1 focus:ring-ring"
							bind:value={currency}
						>
							<option value="CNY">CNY 人民币</option>
							<option value="USD">USD 美元</option>
							<option value="EUR">EUR 欧元</option>
							<option value="HKD">HKD 港币</option>
							<option value="JPY">JPY 日元</option>
						</select>
					</div>
				</div>

				<div class="grid grid-cols-2 gap-3">
					<div class="space-y-2">
						<Label>时区</Label>
						<Input bind:value={timezone} placeholder="Asia/Shanghai" />
					</div>
					<div class="space-y-2">
						<Label>语言</Label>
						<select
							class="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm focus:outline-none focus:ring-1 focus:ring-ring"
							bind:value={locale}
						>
							<option value="zh-CN">简体中文</option>
							<option value="en">English</option>
						</select>
					</div>
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

	<!-- AI 助手（MCP） -->
	<section>
		<div class="flex items-center gap-2 mb-3">
			<Bot size={16} />
			<h2 class="text-sm font-medium">AI 助手（MCP）</h2>
		</div>
		<Card>
			<CardContent class="p-4 space-y-3">
				<p class="text-sm text-muted-foreground">
					接入 MCP 后，AI 助手可以用自然语言查询、分析和修改你的账单（搜索流水、统计消费趋势与分类占比、
					记账 / 改账 / 删账）。数据只在你自己的服务器上流转。
				</p>

				<div class="space-y-2">
					<Label>MCP 服务地址</Label>
					<div class="flex items-center gap-2">
						<Input readonly value={mcpEndpoint} class="font-mono text-xs" />
						<Button size="icon" variant="outline" onclick={() => copyText(mcpEndpoint, '地址')}>
							<Copy size={14} />
						</Button>
					</div>
				</div>

				{#if !apiKeyInfo?.has_api_key}
					<p class="text-xs text-amber-600 dark:text-amber-400">
						请先在上方「API密钥」处生成并启用密钥，MCP 使用它作为鉴权凭据。
					</p>
				{:else if !apiKeyInfo.api_key_enabled}
					<p class="text-xs text-amber-600 dark:text-amber-400">API密钥当前已禁用，MCP 将无法连接。</p>
				{/if}

				<div class="space-y-2">
					<Label>客户端配置</Label>
					<div class="flex flex-wrap gap-2">
						{#each mcpClients as c}
							<Button
								size="sm"
								variant={mcpClient === c.key ? 'default' : 'outline'}
								onclick={() => (mcpClient = c.key)}
							>
								{c.label}
							</Button>
						{/each}
					</div>
					<pre
						class="bg-muted rounded-lg p-3 text-xs overflow-x-auto font-mono leading-relaxed">{mcpSnippet}</pre>
					<div class="flex gap-2">
						<Button size="sm" variant="outline" onclick={() => copyText(mcpSnippet, '配置')}>
							<Copy size={14} />
							复制配置
						</Button>
					</div>
					<p class="text-xs text-muted-foreground">{currentMcpClient.hint}</p>
				</div>

				<div class="text-xs text-muted-foreground space-y-1">
					<div class="font-medium text-foreground">可以这样对 AI 说：</div>
					<div>· 「上个月我在餐饮上花了多少？比前一个月涨了还是降了？」</div>
					<div>· 「查一下最近 30 天超过 200 元的支出」</div>
					<div>· 「记一笔：今天午饭 42.5 元，餐饮，现金」</div>
					<div>· 「把昨天那笔地铁改成 5 元」</div>
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

	<!-- 致谢 -->
	<section>
		<div class="flex items-center gap-2 mb-3 text-sm font-medium text-muted-foreground">
			<Heart size={16} />
			致谢
		</div>
		<Card>
			<CardContent class="p-4 space-y-4 text-sm">
				<div>
					<div class="font-medium mb-2">前端技术</div>
					<div class="text-muted-foreground space-y-1">
						<div><span class="font-medium text-foreground">SvelteKit</span> — 新一代全栈框架</div>
						<div><span class="font-medium text-foreground">TailwindCSS</span> — 原子化 CSS 框架</div>
						<div><span class="font-medium text-foreground">Lucide</span> — 精美 SVG 图标库</div>
						<div><span class="font-medium text-foreground">Chart.js</span> — 数据可视化图表</div>
						<div><span class="font-medium text-foreground">Vaul Svelte</span> — 抽屉组件</div>
						<div><span class="font-medium text-foreground">svelte-sonner</span> — Toast 通知组件</div>
						<div><span class="font-medium text-foreground">Day.js</span> — 轻量日期处理库</div>
						<div><span class="font-medium text-foreground">Bank Logos</span> — 银行图标库 (icongo)</div>
					</div>
				</div>
				<div>
					<div class="font-medium mb-2">后端技术</div>
					<div class="text-muted-foreground space-y-1">
						<div><span class="font-medium text-foreground">Go</span> — 高性能编程语言</div>
						<div><span class="font-medium text-foreground">Gin</span> — HTTP Web 框架</div>
						<div><span class="font-medium text-foreground">GORM</span> — ORM 框架</div>
						<div><span class="font-medium text-foreground">SQLite / PostgreSQL</span> — 数据库</div>
						<div><span class="font-medium text-foreground">JWT</span> — 身份认证</div>
						<div><span class="font-medium text-foreground">WebSocket</span> — 实时同步</div>
					</div>
				</div>
				<div>
					<div class="font-medium mb-2">特别感谢</div>
					<div class="text-muted-foreground space-y-1">
						<div>所有开源项目的贡献者</div>
						<div>每一位用户的反馈与支持</div>
					</div>
				</div>
			</CardContent>
		</Card>
	</section>

	<!-- 关于 -->
	<div class="text-center text-xs text-muted-foreground pt-4">
		货殖 v0.1.0 · 简洁纯粹的记账本
	</div>
</div>
