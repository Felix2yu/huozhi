<script lang="ts">
	import { goto } from '$app/navigation';
	import Button from '#lib/components/ui/Button.svelte';
	import Card from '#lib/components/ui/Card.svelte';
	import CardContent from '#lib/components/ui/CardContent.svelte';
	import Dialog from '#lib/components/ui/Dialog.svelte';
	import Input from '#lib/components/ui/Input.svelte';
	import Label from '#lib/components/ui/Label.svelte';
	import Badge from '#lib/components/ui/Badge.svelte';
	import { appStore } from '#lib/stores/app';
	import { themeStore, fontSizeStore, type FontSize } from '#lib/stores/theme';
	import { privacyStore } from '#lib/stores/privacy';
	import { ratesStore } from '#lib/stores/rates.svelte';
	import { authApi } from '#lib/api/modules/auth';
	import { uploadApi } from '#lib/api/modules/upload';
	import { ioApi } from '#lib/api/modules/io';
	import { http } from '#lib/api/http';
	import { hzToast } from '#lib/components/ui/toast';
	import { CURRENCIES } from '#lib/types';
	import { onMount } from 'svelte';
	import { browser } from '$app/env';
	import { resolve } from '$app/paths';
	import {
		User,
		Palette,
		LogOut,
		CloudOff,
		Moon,
		Sun,
		Monitor,
		Key,
		Plus,
		Copy,
		Eye,
		EyeOff,
		Lock,
		Heart,
		Bot,
		Coins,
		RefreshCw,
		Camera,
		Loader2,
		Trash2,
		BookOpen,
		ChevronDown,
		AlertTriangle
	} from '@lucide/svelte';

	let nickname = $state('');
	let email = $state('');
	let theme = $state(themeStore.value);
	let fontSize = $state<FontSize>(fontSizeStore.value);
	let loading = $state(false);

	// B9：User 模型已有这些字段，后端 UpdateMe 也支持，但设置页此前完全没有入口
	let monthStart = $state(1);
	let currency = $state('CNY');
	let timezone = $state('Asia/Shanghai');
	let locale = $state('zh-CN');

	// 基准货币与汇率配置
	let fxAutoRefresh = $state(true);
	let fxRefreshHours = $state(12);

	// 修改密码
	let showChangePwd = $state(false);
	let oldPwd = $state('');
	let newPwd = $state('');
	let confirmPwd = $state('');
	let pwdLoading = $state(false);

	// API 密钥（多把、按 scope 分权）
	// 只有一把密钥时权限只能取并集，只读消费方必然连带拿到写权限，
	// 所以这里按「名称 + 只读/读写」管理多把。
	type ApiKeyRow = {
		id: number;
		name: string;
		key: string;
		scope: 'read' | 'write';
		enabled: boolean;
		legacy: boolean;
		last_used_at: string;
		created_at: string;
	};
	let apiKeys = $state<ApiKeyRow[]>([]);
	let apiKeysLoading = $state(true);
	/** 当前展开明文的密钥 id；同一时间只展开一把 */
	let revealKeyId = $state<number | null>(null);
	/** 正在执行停用/吊销的密钥 id，用于按钮 loading */
	let keyBusyId = $state<number | null>(null);
	let newKeyOpen = $state(false);
	let newKeyName = $state('');
	let newKeyScope = $state<'read' | 'write'>('read');
	let keySaving = $state(false);

	// 头像上传
	let avatarUploading = $state(false);
	let avatarInput = $state<HTMLInputElement | null>(null);

	// 默认账本（打开应用时优先进入的账本）
	let defaultBookId = $state(0);

	// 汇率明细默认收起，点标题展开
	let showFxRows = $state(false);

	// 注销账号（危险操作）
	let showDeleteAcct = $state(false);
	let delPwd = $state('');
	let delConfirm = $state('');
	let delLoading = $state(false);

	// 清空全部数据（危险操作）
	let clearing = $state(false);

	// ===== 合并保存：所有表单字段统一进一个「未保存」条 =====
	// 个人信息与汇率此前各有一个保存按钮，但提交的是同一个 /auth/me 接口的不同子集，
	// 两处状态互相独立极易「改了没保存却看不到提示」。改为整页脏检测 + 底部保存条。
	interface SettingsSnap {
		nickname: string;
		email: string;
		monthStart: number;
		currency: string;
		timezone: string;
		locale: string;
		fxAutoRefresh: boolean;
		fxRefreshHours: number;
		defaultBookId: number;
	}

	function takeSnapshot(): SettingsSnap {
		return {
			nickname,
			email,
			monthStart,
			currency,
			timezone,
			locale,
			fxAutoRefresh,
			fxRefreshHours,
			defaultBookId
		};
	}

	let savedSnap = $state<SettingsSnap | null>(null);
	const dirty = $derived(
		savedSnap !== null && JSON.stringify(takeSnapshot()) !== JSON.stringify(savedSnap)
	);

	function discardChanges() {
		if (!savedSnap) return;
		nickname = savedSnap.nickname;
		email = savedSnap.email;
		monthStart = savedSnap.monthStart;
		currency = savedSnap.currency;
		timezone = savedSnap.timezone;
		locale = savedSnap.locale;
		fxAutoRefresh = savedSnap.fxAutoRefresh;
		fxRefreshHours = savedSnap.fxRefreshHours;
		defaultBookId = savedSnap.defaultBookId;
	}

	// ===== 时区可搜索下拉 =====
	const TZ_LIST: string[] = (() => {
		try {
			return (Intl as any).supportedValuesOf('timeZone') as string[];
		} catch {
			// 老浏览器不支持 supportedValuesOf 时退化为常用清单
			return [
				'Asia/Shanghai',
				'Asia/Hong_Kong',
				'Asia/Tokyo',
				'Asia/Singapore',
				'Asia/Seoul',
				'Europe/London',
				'Europe/Paris',
				'America/New_York',
				'America/Los_Angeles',
				'UTC'
			];
		}
	})();
	let tzOpen = $state(false);
	// 按当前输入过滤，最多展示 40 条，避免全量 400+ 时区撑爆 DOM
	const tzOptions = $derived.by(() => {
		const q = timezone.trim().toLowerCase();
		const list = q ? TZ_LIST.filter((t) => t.toLowerCase().includes(q)) : TZ_LIST;
		return list.slice(0, 40);
	});

	function tzOffset(tz: string): string {
		try {
			const part = new Intl.DateTimeFormat('en-US', {
				timeZone: tz,
				timeZoneName: 'longOffset'
			})
				.formatToParts(new Date())
				.find((p) => p.type === 'timeZoneName');
			// "GMT+08:00" → "UTC+08:00"，纯 UTC 显示 "GMT" → "UTC"
			return (part?.value || 'GMT').replace('GMT', 'UTC');
		} catch {
			return '';
		}
	}

	function pickTimezone(tz: string) {
		timezone = tz;
		tzOpen = false;
	}

	/**
	 * 头像上传：走凭证图同一套 uploadApi（后端校验类型/大小），拿到 URL 后
	 * 再写进 User.avatar。拆成两步是因为上传和存库是不同接口——
	 * 直接 updateMe 传二进制会 400。
	 */
	async function handleAvatarFile(e: Event) {
		const input = e.currentTarget as HTMLInputElement;
		const file = input.files?.[0];
		input.value = ''; // 允许连续选同一文件
		if (!file) return;

		if (!file.type.startsWith('image/')) {
			hzToast.warning('请选择图片文件');
			return;
		}
		avatarUploading = true;
		try {
			const { url } = await uploadApi.image(file);
			const updated = await authApi.updateMe({ avatar: url });
			if (appStore.user) {
				appStore.setTokenAndAuth(http.getToken()!, {
					...appStore.user,
					...updated,
					avatar: updated.avatar || url
				});
			}
			hzToast.success('头像已更新');
		} catch (err: any) {
			hzToast.error(err.message || '上传失败');
		} finally {
			avatarUploading = false;
		}
	}

	async function handleRemoveAvatar() {
		avatarUploading = true;
		try {
			await authApi.updateMe({ avatar: '' });
			if (appStore.user) {
				appStore.setTokenAndAuth(http.getToken()!, { ...appStore.user, avatar: '' });
			}
			hzToast.success('已移除头像');
		} catch (err: any) {
			hzToast.error(err.message || '操作失败');
		} finally {
			avatarUploading = false;
		}
	}

	onMount(async () => {
		if (appStore.user) {
			nickname = appStore.user.nickname;
			email = appStore.user.email || '';
			monthStart = appStore.user.month_start || 1;
			currency = appStore.user.currency || 'CNY';
			timezone = appStore.user.timezone || 'Asia/Shanghai';
			locale = appStore.user.locale || 'zh-CN';
			fxAutoRefresh = appStore.user.fx_auto_refresh !== false;
			fxRefreshHours = appStore.user.fx_refresh_hours || 12;
			defaultBookId = appStore.user.default_book_id || 0;
		}
		// 账本下拉的数据源（正常进设置页前 layout 已加载，这里兜底）
		if (!appStore.books.length) await appStore.loadBooks();
		// 记录初始快照，供整页脏检测使用
		savedSnap = takeSnapshot();
		// 汇率：进入设置页就确保有数据可展示（未过期直接复用缓存）
		ratesStore.ensure(currency);
		await loadApiKeys();
	});

	// ===== 基准货币与汇率 =====
	const fxUpdatedAt = $derived.by(() => {
		if (!ratesStore.fetchedAt) return '';
		const d = new Date(ratesStore.fetchedAt);
		const p = (n: number) => String(n).padStart(2, '0');
		return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`;
	});

	// 只展示「有汇率」的币种，且基准币自身不列（恒为 1:1，列出来是噪声）
	const fxRows = $derived(
		ratesStore.currencies
			.filter((c) => c !== currency && ratesStore.rate(c))
			.map((c) => ({ code: c, rate: ratesStore.rate(c) as number }))
	);

	// 个人信息 / 汇率 / 默认账本 合并为一次保存——它们本就是同一个 /auth/me 的不同字段
	async function handleSave() {
		if (!savedSnap) return;
		loading = true;
		try {
			const prevCurrency = savedSnap.currency;
			const updated = await authApi.updateMe({
				nickname,
				email,
				month_start: monthStart,
				currency,
				timezone,
				locale,
				fx_auto_refresh: fxAutoRefresh,
				fx_refresh_hours: fxRefreshHours,
				default_book_id: defaultBookId
			});
			if (appStore.user) {
				appStore.setTokenAndAuth(http.getToken()!, { ...appStore.user, ...updated });
			}
			// 基准货币可能已变：让汇率缓存失效并按新基准重新拉取
			if (currency !== prevCurrency) {
				ratesStore.invalidate();
				await ratesStore.ensure(currency, { force: true });
			}
			savedSnap = takeSnapshot();
			hzToast.success('设置已保存');
		} catch (e: any) {
			hzToast.error(e.message || '保存失败');
		} finally {
			loading = false;
		}
	}

	async function handleRefreshFx() {
		const ok = await ratesStore.refresh(currency);
		if (ok) hzToast.success('汇率已更新');
		else hzToast.warning(ratesStore.error || '刷新失败，当前展示的是上次成功获取的汇率');
	}

	// ===== API 密钥 =====
	async function loadApiKeys() {
		try {
			apiKeys = await http.get<ApiKeyRow[]>('/api-keys');
		} catch (e: any) {
			hzToast.error(e.message || '读取密钥失败');
		} finally {
			apiKeysLoading = false;
		}
	}

	function openNewKey() {
		newKeyName = '';
		// 缺省只读：新建密钥最常见的目的是接一个只取数的应用
		newKeyScope = 'read';
		newKeyOpen = true;
	}

	async function handleCreateKey() {
		keySaving = true;
		try {
			const row = await http.post<ApiKeyRow>('/api-keys', {
				name: newKeyName.trim() || undefined,
				scope: newKeyScope
			});
			apiKeys = [...apiKeys, row];
			// 新建后直接展开明文：这是唯一一次「你正想复制它」的时刻
			revealKeyId = row.id;
			newKeyOpen = false;
			hzToast.success(row.scope === 'write' ? '已生成读写密钥' : '已生成只读密钥');
		} catch (e: any) {
			hzToast.error(e.message || '生成失败');
		} finally {
			keySaving = false;
		}
	}

	async function patchKey(row: ApiKeyRow, body: Record<string, unknown>, okMsg: string) {
		keyBusyId = row.id;
		try {
			const next = await http.patch<ApiKeyRow>(`/api-keys/${row.id}`, body);
			apiKeys = apiKeys.map((k) => (k.id === row.id ? next : k));
			hzToast.success(okMsg);
		} catch (e: any) {
			hzToast.error(e.message || '操作失败');
		} finally {
			keyBusyId = null;
		}
	}

	async function handleDeleteKey(row: ApiKeyRow) {
		const ok = confirm(
			`吊销「${row.name}」？正在用这把密钥的客户端会立即失去访问权限，且无法恢复。`
		);
		if (!ok) return;
		keyBusyId = row.id;
		try {
			await http.delete(`/api-keys/${row.id}`);
			apiKeys = apiKeys.filter((k) => k.id !== row.id);
			if (revealKeyId === row.id) revealKeyId = null;
			hzToast.success('已吊销');
		} catch (e: any) {
			hzToast.error(e.message || '吊销失败');
		} finally {
			keyBusyId = null;
		}
	}

	function copyKey(key: string) {
		navigator.clipboard.writeText(key);
		hzToast.success('已复制到剪贴板');
	}

	/** 未展开时只露头尾，便于核对是哪把，又不至于被旁人一眼看全 */
	function maskKey(key: string) {
		if (!key) return '';
		return key.length > 16 ? `${key.slice(0, 8)}••••${key.slice(-4)}` : key;
	}

	function scopeLabel(scope: string) {
		return scope === 'write' ? '读写' : '只读';
	}

	function fmtKeyTime(v: string) {
		if (!v || v.startsWith('0001')) return '从未使用';
		const d = new Date(v);
		const p = (n: number) => String(n).padStart(2, '0');
		return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`;
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

	// MCP 片段用「读写」密钥：MCP 要能记账/改账/删账，
	// 只读密钥接上去会让写工具从 tools/list 里消失。
	const writeKeyRow = $derived(apiKeys.find((k) => k.enabled && k.scope === 'write'));
	const maskedKey = $derived(
		writeKeyRow && revealKeyId === writeKeyRow.id ? writeKeyRow.key : 'YOUR_API_KEY'
	);

	const mcpSnippet = $derived(
		mcpClient === 'claude'
			? JSON.stringify(
					{
						mcpServers: {
							huozhi: {
								command: 'npx',
								args: ['-y', 'mcp-remote', mcpEndpoint, '--header', `X-API-Key:${maskedKey}`]
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
			const res = await authApi.changePwd({ old_password: oldPwd, new_password: newPwd });
			// 改密码会让其它端 token 立即失效，换上后端签发的新 token，避免当前设备被自己踢下线
			if (res?.token && appStore.user) {
				appStore.setTokenAndAuth(res.token, appStore.user);
			}
			hzToast.success('密码已修改，其它设备已退出登录');
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

	// ===== 危险操作 =====
	// 清空全部业务数据：账号保留，服务端先落安全快照（与 /data 页同一接口）
	async function handleClearData() {
		const ok = confirm(
			'⚠️ 此操作非常危险，可能导致不可逆的数据丢失！\n\n' +
				'将删除全部交易、账户、分类、标签、预算、周期、分期、报销、存钱计划数据。\n' +
				'服务端会先保存一份安全快照，但恢复需要人工介入。\n\n' +
				'确定继续？'
		);
		if (!ok) return;
		const pwd = window.prompt('请输入登录密码以确认清空：');
		if (!pwd) return;

		clearing = true;
		try {
			await ioApi.reset(pwd);
			hzToast.success('已清空全部数据');
			await appStore.loadBooks();
			await appStore.loadDictionaries();
		} catch (e: any) {
			hzToast.error(e.message || '清空失败');
		} finally {
			clearing = false;
		}
	}

	// 注销账号：连用户记录一起永久删除（服务端会先落全量快照），需密码 + 显式确认串
	async function handleDeleteAccount() {
		if (delConfirm !== 'DELETE_ACCOUNT') {
			hzToast.warning('请输入 DELETE_ACCOUNT 以确认');
			return;
		}
		delLoading = true;
		try {
			await authApi.deleteAccount({ password: delPwd, confirm: delConfirm });
			appStore.logout();
			hzToast.success('账号已注销');
			goto(resolve('/login'));
		} catch (e: any) {
			hzToast.error(e.message || '注销失败');
		} finally {
			delLoading = false;
		}
	}

	function handleThemeChange(t: 'light' | 'dark' | 'system') {
		theme = t;
		themeStore.value = t;
	}

	function handleFontSizeChange(s: FontSize) {
		fontSize = s;
		fontSizeStore.value = s;
	}

	async function handleLogout() {
		try {
			await http.post<void>('/auth/logout');
		} catch {
			/* 登出接口失败也要清本地会话 */
		}
		appStore.logout();
		goto(resolve('/login'));
	}
</script>

<svelte:head>
	<title>系统设置 · 货殖</title>
</svelte:head>

<div class="max-w-2xl space-y-6">
	<!-- 个人信息 -->
	<section>
		<div class="mb-3 flex items-center gap-2">
			<User size={16} />
			<h2 class="text-sm font-medium">个人信息</h2>
		</div>
		<Card>
			<CardContent class="space-y-4 p-4">
				<div class="flex items-center gap-3">
					<button
						type="button"
						class="group relative grid h-16 w-16 shrink-0 place-items-center overflow-hidden rounded-full bg-primary/10 text-xl font-bold text-primary"
						onclick={() => avatarInput?.click()}
						disabled={avatarUploading}
						title="点击更换头像"
					>
						{#if appStore.user?.avatar}
							<img
								src={appStore.user.avatar}
								alt="头像"
								class="absolute inset-0 h-full w-full object-cover"
							/>
						{:else}
							{appStore.user?.nickname?.[0] || 'U'}
						{/if}
						<span
							class="absolute inset-0 grid place-items-center bg-black/45 text-white opacity-0 transition group-hover:opacity-100"
						>
							{#if avatarUploading}
								<Loader2 size={20} class="animate-spin" />
							{:else}
								<Camera size={20} />
							{/if}
						</span>
					</button>
					<div class="min-w-0 flex-1">
						<div class="font-medium">{appStore.user?.username}</div>
						<div class="text-xs text-muted-foreground">{appStore.user?.email || '未设置邮箱'}</div>
						<div class="mt-1.5 flex gap-3">
							<button
								type="button"
								class="text-xs text-primary hover:underline disabled:opacity-50"
								disabled={avatarUploading}
								onclick={() => avatarInput?.click()}
							>
								{avatarUploading ? '上传中…' : '更换头像'}
							</button>
							{#if appStore.user?.avatar}
								<button
									type="button"
									class="flex items-center gap-1 text-xs text-muted-foreground hover:text-destructive disabled:opacity-50"
									disabled={avatarUploading}
									onclick={handleRemoveAvatar}
								>
									<Trash2 size={12} />
									移除
								</button>
							{/if}
						</div>
					</div>
				</div>
				<!-- 隐藏的文件选择器：头像圆盘与「更换头像」都调它 click() -->
				<input
					bind:this={avatarInput}
					type="file"
					accept="image/*"
					class="hidden"
					onchange={handleAvatarFile}
				/>
				<div class="space-y-2">
					<Label>昵称</Label>
					<Input bind:value={nickname} />
				</div>
				<div class="space-y-2">
					<Label>邮箱</Label>
					<Input type="email" bind:value={email} />
				</div>

				<!-- B9：账期起始日 —— 后端模型早已支持，此前无处配置。
				     基准货币已移到下方「基准货币与汇率」专区（同一个 currency 字段，
				     两处都放会变成两个控件抢同一份状态） -->
				<div class="space-y-2">
					<Label>账期起始日</Label>
					<select
						class="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm focus:ring-1 focus:ring-ring focus:outline-none"
						bind:value={monthStart}
					>
						{#each Array.from({ length: 28 }, (_, i) => i + 1) as d (d)}
							<option value={d}>每月 {d} 日</option>
						{/each}
					</select>
					<p class="text-[11px] text-muted-foreground">影响月度预算与统计的周期划分</p>
				</div>

				<div class="grid grid-cols-2 gap-3">
					<div class="space-y-2">
						<Label>时区</Label>
						<!-- 可搜索下拉：直接绑 timezone 本身，既能点选也能继续手输自定义值 -->
						<div class="relative">
							<input
								class="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm focus:ring-1 focus:ring-ring focus:outline-none"
								bind:value={timezone}
								onfocus={() => (tzOpen = true)}
								onblur={() => (tzOpen = false)}
								placeholder="搜索时区，如 Shanghai"
							/>
							{#if tzOpen && tzOptions.length}
								<ul
									class="absolute top-full right-0 left-0 z-20 mt-1 max-h-52 overflow-y-auto rounded-md border bg-popover text-sm shadow-md"
								>
									{#each tzOptions as tz (tz)}
										<li>
											<!-- mousedown 先于 input 的 blur 触发，避免下拉先被收起导致点击落空 -->
											<button
												type="button"
												class="flex w-full items-center justify-between gap-2 px-3 py-1.5 text-left hover:bg-accent"
												onmousedown={() => pickTimezone(tz)}
											>
												<span class="truncate">{tz}</span>
												<span class="shrink-0 text-[11px] text-muted-foreground">
													{tzOffset(tz)}
												</span>
											</button>
										</li>
									{/each}
								</ul>
							{/if}
						</div>
					</div>
					<div class="space-y-2">
						<Label>语言</Label>
						<select
							class="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm focus:ring-1 focus:ring-ring focus:outline-none"
							bind:value={locale}
						>
							<option value="zh-CN">简体中文</option>
							<option value="en">English</option>
						</select>
					</div>
				</div>
			</CardContent>
		</Card>
	</section>

	<!-- 默认账本 -->
	<section>
		<div class="mb-3 flex items-center gap-2">
			<BookOpen size={16} />
			<h2 class="text-sm font-medium">账本</h2>
		</div>
		<Card>
			<CardContent class="p-4">
				<div class="space-y-2">
					<Label>默认账本</Label>
					<select
						class="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm focus:ring-1 focus:ring-ring focus:outline-none"
						bind:value={defaultBookId}
					>
						<option value={0}>跟随账本默认（不指定偏好）</option>
						{#each appStore.books.filter((b) => !b.is_archived) as b (b.id)}
							<option value={b.id}>{b.icon || '📘'} {b.name}</option>
						{/each}
					</select>
					<p class="text-[11px] text-muted-foreground">
						打开应用时优先进入该账本；未设置时使用账本自身的「默认」标记
					</p>
				</div>
			</CardContent>
		</Card>
	</section>

	<!-- 基准货币与汇率 -->
	<section>
		<div class="mb-3 flex items-center gap-2">
			<Coins size={16} />
			<h2 class="text-sm font-medium">基准货币与汇率</h2>
		</div>
		<Card>
			<CardContent class="space-y-4 p-4">
				<div class="grid grid-cols-2 gap-3">
					<div class="space-y-2">
						<Label>基准货币</Label>
						<select
							class="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm focus:ring-1 focus:ring-ring focus:outline-none"
							bind:value={currency}
						>
							{#each CURRENCIES as c (c.code)}
								<option value={c.code}>{c.code} {c.label}</option>
							{/each}
						</select>
						<p class="text-[11px] text-muted-foreground">外币账单按汇率折算为基准货币后参与统计</p>
					</div>
					<div class="space-y-2">
						<Label>刷新间隔</Label>
						<select
							class="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm focus:ring-1 focus:ring-ring focus:outline-none"
							bind:value={fxRefreshHours}
						>
							<option value={6}>每 6 小时</option>
							<option value={12}>每 12 小时</option>
							<option value={24}>每 24 小时</option>
						</select>
						<label class="flex items-center gap-2 pt-2 text-[11px] text-muted-foreground">
							<input type="checkbox" bind:checked={fxAutoRefresh} class="rounded border-input" />
							自动刷新汇率
						</label>
					</div>
				</div>

				<div
					class="flex items-center justify-between gap-2 rounded-lg border bg-muted/40 px-3 py-2"
				>
					<div class="space-y-0.5 text-xs text-muted-foreground">
						{#if ratesStore.error}
							<div class="text-amber-600 dark:text-amber-400">{ratesStore.error}</div>
						{:else if fxUpdatedAt}
							<div>
								更新于 {fxUpdatedAt}
								{#if ratesStore.stale}
									<span class="text-amber-600 dark:text-amber-400">（已过期）</span>
								{/if}
							</div>
						{:else}
							<div>尚未获取汇率</div>
						{/if}
						<div>
							数据源 {ratesStore.source || '—'}
							{#if !ratesStore.enabled}
								<span class="text-amber-600 dark:text-amber-400">（服务端已关闭）</span>
							{/if}
						</div>
					</div>
					<Button
						size="sm"
						variant="outline"
						disabled={ratesStore.refreshing || !ratesStore.enabled}
						onclick={handleRefreshFx}
					>
						<RefreshCw size={14} class={ratesStore.refreshing ? 'animate-spin' : ''} />
						{ratesStore.refreshing ? '刷新中…' : '立即刷新'}
					</Button>
				</div>

				{#if fxRows.length}
					<div class="space-y-2">
						<!-- 汇率明细默认收起：多数时候只看「更新时间 + 刷新」，长表格是次要信息 -->
						<button
							type="button"
							class="flex items-center gap-1.5 text-sm font-medium"
							onclick={() => (showFxRows = !showFxRows)}
							aria-expanded={showFxRows}
						>
							<ChevronDown
								size={14}
								class="transition-transform {showFxRows ? '' : '-rotate-90'}"
							/>
							当前汇率（1 外币 = ? {currency}）
							<span class="text-[11px] font-normal text-muted-foreground">{fxRows.length} 项</span>
						</button>
						{#if showFxRows}
							<div
								class="grid max-h-48 grid-cols-2 gap-x-4 gap-y-1 overflow-y-auto text-xs tabular-nums"
							>
								{#each fxRows as row (row.code)}
									<div class="flex justify-between border-b border-border/40 py-1">
										<span class="text-muted-foreground">{row.code}</span>
										<span>{row.rate.toFixed(4)}</span>
									</div>
								{/each}
							</div>
						{/if}
					</div>
				{/if}
			</CardContent>
		</Card>
	</section>

	<!-- 主题设置 -->
	<section>
		<div class="mb-3 flex items-center gap-2">
			<Palette size={16} />
			<h2 class="text-sm font-medium">外观与字体</h2>
		</div>
		<Card>
			<CardContent class="space-y-4 p-4">
				<div class="grid grid-cols-3 gap-2">
					<button
						class="flex flex-col items-center gap-2 rounded-lg border p-4 transition"
						class:border-primary={theme === 'light'}
						onclick={() => handleThemeChange('light')}
					>
						<Sun size={20} />
						<span class="text-xs">浅色</span>
					</button>
					<button
						class="flex flex-col items-center gap-2 rounded-lg border p-4 transition"
						class:border-primary={theme === 'dark'}
						onclick={() => handleThemeChange('dark')}
					>
						<Moon size={20} />
						<span class="text-xs">深色</span>
					</button>
					<button
						class="flex flex-col items-center gap-2 rounded-lg border p-4 transition"
						class:border-primary={theme === 'system'}
						onclick={() => handleThemeChange('system')}
					>
						<Monitor size={20} />
						<span class="text-xs">跟随系统</span>
					</button>
				</div>

				<div class="space-y-2">
					<Label>字体大小</Label>
					<div class="grid grid-cols-4 gap-2">
						{#each Object.entries( { sm: '小', md: '标准', lg: '大', xl: '特大' } ) as [key, label] (key)}
							<button
								class="flex flex-col items-center gap-1.5 rounded-lg border p-3 transition"
								class:border-primary={fontSize === key}
								onclick={() => handleFontSizeChange(key as FontSize)}
							>
								<!-- 用各自档位的绝对像素预览，所见即所得 -->
								<span
									class="leading-none font-medium"
									style="font-size: {key === 'sm'
										? 13
										: key === 'md'
											? 15
											: key === 'lg'
												? 17
												: 19}px">Aa</span
								>
								<span class="text-xs">{label}</span>
							</button>
						{/each}
					</div>
					<p class="text-[11px] text-muted-foreground">
						调整整个界面的文字与控件大小，仅影响当前设备
					</p>
				</div>
			</CardContent>
		</Card>
	</section>

	<!-- 安全 -->
	<section>
		<div class="mb-3 flex items-center gap-2">
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
							<Button size="sm" variant="outline" onclick={() => (showChangePwd = false)}
								>取消</Button
							>
						</div>
					</div>
				{:else}
					<button
						class="flex w-full items-center justify-between rounded-lg border p-3 transition hover:bg-accent"
						onclick={() => (showChangePwd = true)}
					>
						<span>修改密码</span>
						<span class="text-sm text-muted-foreground">→</span>
					</button>
					<p class="mt-2 text-[11px] text-muted-foreground">
						修改密码后，其它已登录设备会立即退出登录，需要重新输入新密码
					</p>
				{/if}

				<!-- 隐私模式：即时生效的显示偏好，与主题/字号同级，不进「未保存」条 -->
				<div class="mt-4 flex items-center justify-between gap-3 border-t pt-4">
					<div class="min-w-0">
						<div class="text-sm font-medium">隐藏金额（隐私模式）</div>
						<div class="text-xs text-muted-foreground">
							全站金额显示为 ••••，点击侧边栏眼睛可临时显示（刷新后恢复遮蔽）
						</div>
					</div>
					<button
						type="button"
						role="switch"
						aria-checked={privacyStore.hidden}
						title={privacyStore.hidden ? '关闭隐私模式' : '开启隐私模式'}
						class="relative h-6 w-11 shrink-0 rounded-full transition-colors {privacyStore.hidden
							? 'bg-primary'
							: 'bg-input'}"
						onclick={() => privacyStore.set(!privacyStore.hidden)}
					>
						<span
							class="absolute top-0.5 left-0.5 h-5 w-5 rounded-full bg-background shadow transition-transform {privacyStore.hidden
								? 'translate-x-5'
								: ''}"
						></span>
					</button>
				</div>
			</CardContent>
		</Card>
	</section>

	<!-- 离线数据 -->
	<section>
		<div class="mb-3 flex items-center gap-2">
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
							class="text-destructive hover:bg-destructive/10 hover:text-destructive"
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
		<div class="mb-3 flex items-center justify-between gap-2">
			<div class="flex items-center gap-2">
				<Key size={16} />
				<h2 class="text-sm font-medium">API 密钥</h2>
			</div>
			<Button size="sm" variant="outline" onclick={openNewKey}>
				<Plus size={14} />
				新建密钥
			</Button>
		</div>
		<Card>
			<CardContent class="space-y-3 p-4">
				<p class="text-xs text-muted-foreground">
					<span class="font-medium text-foreground">只读</span
					>密钥只能取数，给牵丝、幕间这类应用用；<span class="font-medium text-foreground"
						>读写</span
					>密钥还能通过 MCP 记账、改账、删账，给 AI 客户端用。
				</p>
				{#if apiKeysLoading}
					<div class="animate-pulse text-sm text-muted-foreground">加载中...</div>
				{:else if apiKeys.length === 0}
					<p class="text-sm text-muted-foreground">
						还没有密钥。新建一把只读密钥，就能让其它应用读取账单。
					</p>
				{:else}
					<div class="space-y-2">
						{#each apiKeys as k (k.id)}
							<div class="rounded-lg border border-border p-3">
								<div class="flex flex-wrap items-center gap-2">
									<span class="text-sm font-medium">{k.name}</span>
									<Badge variant={k.scope === 'write' ? 'default' : 'secondary'}
										>{scopeLabel(k.scope)}</Badge
									>
									{#if k.legacy}
										<Badge variant="secondary">历史密钥</Badge>
									{/if}
									{#if !k.enabled}
										<Badge variant="secondary">已停用</Badge>
									{/if}
									<span class="ml-auto text-xs text-muted-foreground">
										最后使用 {fmtKeyTime(k.last_used_at)}
									</span>
								</div>
								<div class="mt-2 flex items-center gap-2">
									<Input
										readonly
										value={revealKeyId === k.id ? k.key : maskKey(k.key)}
										class="font-mono text-xs"
									/>
									<Button
										size="icon"
										variant="outline"
										title={revealKeyId === k.id ? '隐藏' : '显示完整密钥'}
										onclick={() => (revealKeyId = revealKeyId === k.id ? null : k.id)}
									>
										{#if revealKeyId === k.id}<EyeOff size={14} />{:else}<Eye size={14} />{/if}
									</Button>
									<Button size="icon" variant="outline" title="复制" onclick={() => copyKey(k.key)}>
										<Copy size={14} />
									</Button>
								</div>
								<div class="mt-2 flex flex-wrap gap-2">
									<Button
										size="sm"
										variant="outline"
										disabled={keyBusyId === k.id}
										onclick={() =>
											patchKey(k, { enabled: !k.enabled }, k.enabled ? '已停用' : '已启用')}
									>
										{k.enabled ? '停用' : '启用'}
									</Button>
									<Button
										size="sm"
										variant="outline"
										disabled={keyBusyId === k.id}
										onclick={() =>
											patchKey(
												k,
												{ scope: k.scope === 'write' ? 'read' : 'write' },
												k.scope === 'write' ? '已改为只读' : '已改为读写'
											)}
									>
										{k.scope === 'write' ? '改为只读' : '改为读写'}
									</Button>
									<Button
										size="sm"
										variant="outline"
										class="text-destructive"
										disabled={keyBusyId === k.id}
										onclick={() => handleDeleteKey(k)}
									>
										吊销
									</Button>
								</div>
							</div>
						{/each}
					</div>
				{/if}
			</CardContent>
		</Card>
	</section>

	<Dialog bind:open={newKeyOpen}>
		<div class="space-y-4">
			<h3 class="text-sm font-medium">新建 API 密钥</h3>
			<div class="space-y-2">
				<Label>名称</Label>
				<Input
					bind:value={newKeyName}
					placeholder="例如：牵丝、幕间、Claude Desktop"
					maxlength={50}
				/>
				<p class="text-xs text-muted-foreground">只用于在列表里区分用途，可留空。</p>
			</div>
			<div class="space-y-2">
				<Label>权限</Label>
				<div class="grid grid-cols-2 gap-2">
					<button
						type="button"
						class="rounded-lg border p-3 text-left text-sm {newKeyScope === 'read'
							? 'border-primary bg-primary/5'
							: 'border-border'}"
						onclick={() => (newKeyScope = 'read')}
					>
						<div class="font-medium">只读</div>
						<div class="mt-1 text-xs text-muted-foreground">
							只能查询账单，不能改动。接牵丝、幕间选这个。
						</div>
					</button>
					<button
						type="button"
						class="rounded-lg border p-3 text-left text-sm {newKeyScope === 'write'
							? 'border-primary bg-primary/5'
							: 'border-border'}"
						onclick={() => (newKeyScope = 'write')}
					>
						<div class="font-medium">读写</div>
						<div class="mt-1 text-xs text-muted-foreground">
							可通过 MCP 记账、改账、删账。给 AI 客户端用。
						</div>
					</button>
				</div>
			</div>
			<div class="flex justify-end gap-2">
				<Button variant="outline" onclick={() => (newKeyOpen = false)}>取消</Button>
				<Button onclick={handleCreateKey} disabled={keySaving}>
					{#if keySaving}<Loader2 size={14} class="animate-spin" />{/if}
					生成
				</Button>
			</div>
		</div>
	</Dialog>

	<!-- AI 助手（MCP） -->
	<section>
		<div class="mb-3 flex items-center gap-2">
			<Bot size={16} />
			<h2 class="text-sm font-medium">AI 助手（MCP）</h2>
		</div>
		<Card>
			<CardContent class="space-y-3 p-4">
				<p class="text-sm text-muted-foreground">
					接入 MCP 后，AI
					助手可以用自然语言查询、分析和修改你的账单（搜索流水、统计消费趋势与分类占比、 记账 / 改账
					/ 删账）。数据只在你自己的服务器上流转。
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

				{#if !writeKeyRow}
					<p class="text-xs text-amber-600 dark:text-amber-400">
						还没有启用的「读写」密钥。MCP
						用只读密钥连上去也能查账，但记账、改账、删账这类工具不会出现在工具列表里；要写入请在上方新建一把读写密钥。
					</p>
				{:else if revealKeyId !== writeKeyRow.id}
					<p class="text-xs text-muted-foreground">
						下面的配置里密钥写作 YOUR_API_KEY。点上方「{writeKeyRow.name}」的眼睛图标展开明文后，这里会自动填上，可直接复制。
					</p>
				{/if}

				<div class="space-y-2">
					<Label>客户端配置</Label>
					<div class="flex flex-wrap gap-2">
						{#each mcpClients as c (c.key)}
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
						class="overflow-x-auto rounded-lg bg-muted p-3 font-mono text-xs leading-relaxed">{mcpSnippet}</pre>
					<div class="flex gap-2">
						<Button size="sm" variant="outline" onclick={() => copyText(mcpSnippet, '配置')}>
							<Copy size={14} />
							复制配置
						</Button>
					</div>
					<p class="text-xs text-muted-foreground">{currentMcpClient.hint}</p>
				</div>

				<div class="space-y-1 text-xs text-muted-foreground">
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
	<Button class="w-full" variant="destructive" onclick={handleLogout}>
		<LogOut size={16} />
		退出登录
	</Button>

	<!-- 危险操作 -->
	<section>
		<div class="mb-3 flex items-center gap-2 text-destructive">
			<AlertTriangle size={16} />
			<h2 class="text-sm font-medium">危险操作</h2>
		</div>
		<Card class="border-destructive/40">
			<CardContent class="space-y-4 p-4">
				<div class="flex items-start justify-between gap-3">
					<div class="min-w-0">
						<div class="text-sm font-medium">清空全部数据</div>
						<div class="text-xs text-muted-foreground">
							删除全部交易、账户、分类、预算等业务数据，账号保留。服务端会先落一份安全快照
						</div>
					</div>
					<Button
						size="sm"
						variant="outline"
						class="shrink-0 border-destructive/40 text-destructive hover:bg-destructive/10 hover:text-destructive"
						disabled={clearing}
						onclick={handleClearData}
					>
						{clearing ? '清空中…' : '清空数据'}
					</Button>
				</div>
				<div class="flex items-start justify-between gap-3 border-t pt-4">
					<div class="min-w-0">
						<div class="text-sm font-medium">注销账号</div>
						<div class="text-xs text-muted-foreground">
							连同账号本身一并永久删除，删除前服务端会保存快照。不可恢复
						</div>
					</div>
					<Button
						size="sm"
						variant="destructive"
						class="shrink-0"
						onclick={() => (showDeleteAcct = true)}
					>
						注销账号
					</Button>
				</div>
			</CardContent>
		</Card>
	</section>

	<Dialog bind:open={showDeleteAcct}>
		<div class="space-y-4">
			<h3 class="text-sm font-medium text-destructive">注销账号</h3>
			<p class="text-xs text-muted-foreground">
				将永久删除账号及全部数据（账本、流水、账户、预算等）。删除前服务端会保存一份快照，
				但恢复需要人工介入。此操作不可撤销。
			</p>
			<div class="space-y-2">
				<Label>登录密码</Label>
				<Input type="password" bind:value={delPwd} placeholder="请输入登录密码" />
			</div>
			<div class="space-y-2">
				<Label>输入 DELETE_ACCOUNT 确认</Label>
				<Input bind:value={delConfirm} placeholder="DELETE_ACCOUNT" class="font-mono" />
			</div>
			<div class="flex justify-end gap-2">
				<Button size="sm" variant="outline" onclick={() => (showDeleteAcct = false)}>取消</Button>
				<Button
					size="sm"
					variant="destructive"
					disabled={delLoading || !delPwd || delConfirm !== 'DELETE_ACCOUNT'}
					onclick={handleDeleteAccount}
				>
					{delLoading ? '注销中…' : '永久注销'}
				</Button>
			</div>
		</div>
	</Dialog>

	<!-- 致谢 -->
	<section>
		<div class="mb-3 flex items-center gap-2 text-sm font-medium text-muted-foreground">
			<Heart size={16} />
			致谢
		</div>
		<Card>
			<CardContent class="space-y-4 p-4 text-sm">
				<div>
					<div class="mb-2 font-medium">前端技术</div>
					<div class="space-y-1 text-muted-foreground">
						<div><span class="font-medium text-foreground">SvelteKit</span> — 新一代全栈框架</div>
						<div>
							<span class="font-medium text-foreground">TailwindCSS</span> — 原子化 CSS 框架
						</div>
						<div><span class="font-medium text-foreground">Lucide</span> — 精美 SVG 图标库</div>
						<div><span class="font-medium text-foreground">Chart.js</span> — 数据可视化图表</div>
						<div><span class="font-medium text-foreground">Vaul Svelte</span> — 抽屉组件</div>
						<div>
							<span class="font-medium text-foreground">svelte-sonner</span> — Toast 通知组件
						</div>
						<div><span class="font-medium text-foreground">Day.js</span> — 轻量日期处理库</div>
						<div>
							<span class="font-medium text-foreground">Bank Logos</span> — 银行图标库 (icongo)
						</div>
					</div>
				</div>
				<div>
					<div class="mb-2 font-medium">后端技术</div>
					<div class="space-y-1 text-muted-foreground">
						<div><span class="font-medium text-foreground">Go</span> — 高性能编程语言</div>
						<div><span class="font-medium text-foreground">Gin</span> — HTTP Web 框架</div>
						<div><span class="font-medium text-foreground">GORM</span> — ORM 框架</div>
						<div><span class="font-medium text-foreground">SQLite / PostgreSQL</span> — 数据库</div>
						<div><span class="font-medium text-foreground">JWT</span> — 身份认证</div>
						<div><span class="font-medium text-foreground">WebSocket</span> — 实时同步</div>
					</div>
				</div>
				<div>
					<div class="mb-2 font-medium">特别感谢</div>
					<div class="space-y-1 text-muted-foreground">
						<div>所有开源项目的贡献者</div>
						<div>每一位用户的反馈与支持</div>
					</div>
				</div>
			</CardContent>
		</Card>
	</section>

	<!-- 关于 -->
	<div class="pt-4 text-center text-xs text-muted-foreground">货殖 v0.1.0 · 简洁纯粹的记账本</div>

	<!-- 未保存提示：整页脏字段合并成一个保存条（替代原先分散的两个保存按钮） -->
	{#if dirty}
		<div class="sticky bottom-4 z-30">
			<div
				class="flex items-center justify-between gap-3 rounded-xl border bg-card px-4 py-3 shadow-lg"
			>
				<span class="text-sm text-muted-foreground">有未保存的修改</span>
				<div class="flex gap-2">
					<Button size="sm" variant="outline" onclick={discardChanges}>放弃修改</Button>
					<Button size="sm" onclick={handleSave} disabled={loading}>
						{loading ? '保存中…' : '保存全部修改'}
					</Button>
				</div>
			</div>
		</div>
	{/if}
</div>
