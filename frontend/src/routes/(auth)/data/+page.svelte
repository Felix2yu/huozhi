<script lang="ts">
	import { goto } from '$app/navigation';
	import Button from '$lib/components/ui/Button.svelte';
	import Card from '$lib/components/ui/Card.svelte';
	import CardContent from '$lib/components/ui/CardContent.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import Label from '$lib/components/ui/Label.svelte';
	import Badge from '$lib/components/ui/Badge.svelte';
	import { appStore } from '$lib/stores/app';
	import { ioApi } from '$lib/api/modules/io';
	import { authApi } from '$lib/api/modules/auth';
	import { accountApi } from '$lib/api/modules/accounts';
	import { hzToast } from '$lib/components/ui/toast';
	import {
		Download,
		Upload,
		Database,
		ShieldCheck,
		FileText,
		Loader2,
		RefreshCw,
		AlertTriangle,
		Trash2,
		DatabaseZap,
		Clock,
		CalendarClock,
		Save
	} from '@lucide/svelte';

	// 备份 / 恢复
	let backupLoading = $state(false);
	let restoreLoading = $state(false);

	// 清空数据（危险操作）
	let clearing = $state(false);

	// 数据体检（账户余额 vs 流水重算）
	let auditRows = $state<any[]>([]);
	let auditLoading = $state(false);
	let auditOpen = $state(false);

	// 导入
	let importLoading = $state(false);
	let importSource = $state<'qianji' | 'alipay' | 'wechat'>('qianji');

	// 自动备份设置
	let autoBackupEnabled = $state(false);
	let autoBackupFrequency = $state<'daily' | 'weekly' | 'monthly'>('daily');
	let autoBackupTime = $state('03:00');
	let autoBackupKeepCount = $state(7);
	let autoBackupSaving = $state(false);
	let autoBackupList = $state<any[]>([]);
	let autoBackupListLoading = $state(false);

	// 初始化自动备份设置（从用户信息加载）
	$effect(() => {
		const user = appStore.user;
		if (user) {
			autoBackupEnabled = (user as any).auto_backup_enabled ?? false;
			autoBackupFrequency = (user as any).auto_backup_frequency ?? 'daily';
			autoBackupTime = (user as any).auto_backup_time ?? '03:00';
			autoBackupKeepCount = (user as any).auto_backup_keep_count ?? 7;
		}
	});

	// 保存自动备份设置
	async function saveAutoBackupSettings() {
		autoBackupSaving = true;
		try {
			await authApi.updateMe({
				auto_backup_enabled: autoBackupEnabled,
				auto_backup_frequency: autoBackupFrequency,
				auto_backup_time: autoBackupTime,
				auto_backup_keep_count: autoBackupKeepCount
			} as any);
			hzToast.success('自动备份设置已保存');
		} catch (e: any) {
			hzToast.error(e.message || '保存失败');
		} finally {
			autoBackupSaving = false;
		}
	}

	// 加载自动备份列表
	async function loadAutoBackupList() {
		autoBackupListLoading = true;
		try {
			autoBackupList = await ioApi.listAutoBackups();
		} catch {
			autoBackupList = [];
		} finally {
			autoBackupListLoading = false;
		}
	}

	function formatFileSize(bytes: number) {
		if (bytes < 1024) return bytes + ' B';
		if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
		return (bytes / (1024 * 1024)).toFixed(1) + ' MB';
	}

	// B8：全量备份（ZIP 快照，含 backup.json + 图片）
	async function handleBackup() {
		backupLoading = true;
		try {
			await ioApi.backup();
			hzToast.success('备份已下载');
		} catch (e: any) {
			hzToast.error(e.message || '备份失败');
		} finally {
			backupLoading = false;
		}
	}

	function triggerRestore() {
		const input = document.createElement('input');
		input.type = 'file';
		input.accept = '.zip,application/zip,.json,application/json';
		input.onchange = async (e) => {
			const file = (e.target as HTMLInputElement).files?.[0];
			if (!file) return;
			if (!confirm('恢复将覆盖当前全部账本与流水（replace 模式），确定继续？')) return;
			restoreLoading = true;
			try {
				const res = await ioApi.restore(file, 'replace');
				const summary = Object.entries(res?.imported ?? {})
					.map(([k, v]) => `${k} ${v}`)
					.join('、');
				const imgCount = res?.images_restored ?? 0;
				const msg = summary || '完成';
				hzToast.success(
					`已恢复：${msg}${imgCount > 0 ? `，${imgCount} 张图片` : ''}`
				);
				await appStore.loadBooks();
				await appStore.loadDictionaries();
			} catch (err: any) {
				hzToast.error(err.message || '恢复失败');
			} finally {
				restoreLoading = false;
			}
		};
		input.click();
	}

	// B7：数据体检
	async function runAudit() {
		auditOpen = true;
		auditLoading = true;
		try {
			const res = await accountApi.audit();
			auditRows = res?.accounts ?? [];
		} catch (e: any) {
			hzToast.error(e.message || '体检失败');
		} finally {
			auditLoading = false;
		}
	}

	async function fixAccount(id: number) {
		try {
			const res: any = await accountApi.recalc(id);
			hzToast.success(res?.changed ? `已修正差额 ${res.diff.toFixed(2)} 元` : '该账户无需修正');
			await runAudit();
			await appStore.loadDictionaries();
		} catch (e: any) {
			hzToast.error(e.message || '修复失败');
		}
	}

	// B9：清空全部业务数据（需二次输入密码，服务端先落安全快照）
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

	function handleExport() {
		ioApi.exportCSV({ book_id: appStore.currentBookId });
		hzToast.success('导出已开始');
	}

	function handleDownloadTemplate() {
		ioApi.template();
	}

	function triggerImport() {
		const input = document.createElement('input');
		input.type = 'file';
		input.accept = '.csv,.xlsx,.xls';
		input.onchange = (e) => {
			const file = (e.target as HTMLInputElement).files?.[0];
			if (file) doImport(file);
		};
		input.click();
	}

	async function doImport(file: File) {
		importLoading = true;
		try {
			const res = await ioApi.import(importSource, appStore.effectiveBookId(), file);
			if (res.ok) {
				hzToast.success(`成功导入 ${res.count || 0} 笔交易`);
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
</script>

<svelte:head>
	<title>数据管理 · 货殖</title>
</svelte:head>

<div class="space-y-6 max-w-2xl">
	<!-- 备份与恢复 -->
	<section>
		<div class="flex items-center gap-2 mb-3">
			<Database size={16} />
			<h2 class="text-sm font-medium">备份与恢复</h2>
		</div>
		<Card>
			<CardContent class="p-4 space-y-4">
				<div class="grid grid-cols-2 gap-3">
					<button
						class="flex items-center justify-center gap-2 p-4 rounded-lg border hover:bg-accent transition text-sm"
						onclick={handleBackup}
						disabled={backupLoading}
					>
						<Database size={18} />
						{backupLoading ? '导出中…' : '导出全量备份'}
					</button>
					<button
						class="flex items-center justify-center gap-2 p-4 rounded-lg border hover:bg-accent transition text-sm"
						onclick={triggerRestore}
						disabled={restoreLoading}
					>
						<Upload size={18} />
						{restoreLoading ? '恢复中…' : '从备份恢复'}
					</button>
				</div>
				<p class="text-[11px] text-muted-foreground">
					全量备份为 ZIP 压缩包，包含快照数据（backup.json）和所有交易凭证图片。
					恢复时支持 ZIP 和旧版 JSON 格式。
				</p>
			</CardContent>
		</Card>
	</section>

	<!-- 自动备份 -->
	<section>
		<div class="flex items-center gap-2 mb-3">
			<CalendarClock size={16} />
			<h2 class="text-sm font-medium">自动备份</h2>
		</div>
		<Card>
			<CardContent class="p-4 space-y-4">
				<div class="flex items-center justify-between">
					<div>
						<p class="text-sm font-medium">启用自动备份</p>
						<p class="text-xs text-muted-foreground">定期自动保存全量数据到服务器</p>
					</div>
					<button
						class="relative inline-flex h-5 w-9 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
						class:bg-primary={autoBackupEnabled}
						class:bg-input={!autoBackupEnabled}
						onclick={() => (autoBackupEnabled = !autoBackupEnabled)}
						aria-label={autoBackupEnabled ? '关闭自动备份' : '开启自动备份'}
					>
						<span
							class="pointer-events-none block h-4 w-4 rounded-full bg-background shadow-lg ring-0 transition-transform"
							class:translate-x-4={autoBackupEnabled}
							class:translate-x-0={!autoBackupEnabled}
						></span>
					</button>
				</div>

				{#if autoBackupEnabled}
					<div class="grid grid-cols-2 gap-3">
						<div class="space-y-1.5">
							<Label class="text-xs">备份频率</Label>
							<select
								class="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm focus:outline-none focus:ring-1 focus:ring-ring"
								bind:value={autoBackupFrequency}
							>
								<option value="daily">每天</option>
								<option value="weekly">每周日</option>
								<option value="monthly">每月1号</option>
							</select>
						</div>
						<div class="space-y-1.5">
							<Label class="text-xs">备份时间</Label>
							<Input
								type="time"
								bind:value={autoBackupTime}
								class="h-9"
							/>
						</div>
					</div>

					<div class="space-y-1.5">
						<Label class="text-xs">保留份数</Label>
						<Input
							type="number"
							min={1}
							max={30}
							bind:value={autoBackupKeepCount}
							class="h-9 w-24"
						/>
						<p class="text-[11px] text-muted-foreground">超出份数的旧备份将自动删除</p>
					</div>

					{#if autoBackupList.length > 0}
						<div class="space-y-1.5">
							<div class="flex items-center justify-between">
								<Label class="text-xs">历史备份</Label>
								<button
									class="text-xs text-muted-foreground hover:text-foreground transition"
									onclick={loadAutoBackupList}
									disabled={autoBackupListLoading}
									aria-label="刷新备份列表"
								>
									<RefreshCw size={12} class={autoBackupListLoading ? 'animate-spin' : ''} />
								</button>
							</div>
							<div class="max-h-32 overflow-y-auto space-y-1">
								{#each autoBackupList as b (b.name)}
									<div class="flex items-center justify-between text-xs py-1 px-2 rounded bg-muted/50">
										<span class="truncate">{b.name}</span>
										<span class="text-muted-foreground ml-2 shrink-0">{formatFileSize(b.size)}</span>
									</div>
								{/each}
							</div>
						</div>
					{/if}

					<Button
						size="sm"
						onclick={saveAutoBackupSettings}
						disabled={autoBackupSaving}
					>
						<Save size={14} />
						{autoBackupSaving ? '保存中...' : '保存设置'}
					</Button>
				{/if}
			</CardContent>
		</Card>
	</section>

	<!-- 数据导入导出 -->
	<section>
		<div class="flex items-center gap-2 mb-3">
			<FileText size={16} />
			<h2 class="text-sm font-medium">导入导出</h2>
		</div>
		<Card>
			<CardContent class="p-4 space-y-3">
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

				<button
					class="w-full flex items-center justify-between p-3 rounded-lg border hover:bg-accent transition"
					onclick={() => goto('/bill-export')}
				>
					<span class="flex items-center gap-2">
						<FileText size={16} />
						月度账单导出
					</span>
					<span class="text-muted-foreground text-sm">→</span>
				</button>
			</CardContent>
		</Card>
	</section>

	<!-- 数据体检 -->
	<section>
		<div class="flex items-center gap-2 mb-3">
			<ShieldCheck size={16} />
			<h2 class="text-sm font-medium">数据体检</h2>
		</div>
		<Card>
			<CardContent class="p-4 space-y-3">
				<button
					class="w-full flex items-center justify-between p-3 rounded-lg border hover:bg-accent transition"
					onclick={runAudit}
				>
					<span class="flex items-center gap-2">
						<DatabaseZap size={16} />
						检查账户余额 vs 流水
					</span>
					<span class="text-muted-foreground text-sm">→</span>
				</button>

				{#if auditOpen}
					{#if auditLoading}
						<div class="py-6 grid place-items-center text-sm text-muted-foreground">
							<Loader2 size={16} class="animate-spin" />
						</div>
					{:else if auditRows.length === 0}
						<p class="text-sm text-muted-foreground">没有账户</p>
					{:else}
						<div class="space-y-2">
							{#each auditRows as row (row.account_id)}
								<div class="flex items-center gap-3 p-2 rounded border text-sm">
									<div class="flex-1 min-w-0">
										<div class="truncate font-medium">{row.name}</div>
										<div class="text-xs text-muted-foreground">
											当前 {row.balance.toFixed(2)} · 按流水 {row.computed.toFixed(2)}
											· {row.tx_count} 笔
										</div>
									</div>
									{#if row.need_fix}
										<Badge variant="destructive">差 {row.diff.toFixed(2)}</Badge>
										<Button size="sm" onclick={() => fixAccount(row.account_id)}>修复</Button>
									{:else}
										<Badge variant="secondary">一致</Badge>
									{/if}
								</div>
							{/each}
						</div>
						<Button size="sm" variant="outline" onclick={runAudit} disabled={auditLoading}>
							<RefreshCw size={14} class={auditLoading ? 'animate-spin' : ''} />
							重新检查
						</Button>
					{/if}
				{/if}
			</CardContent>
		</Card>
	</section>

	<!-- 危险操作 -->
	<section>
		<div class="flex items-center gap-2 mb-3">
			<AlertTriangle size={16} class="text-destructive" />
			<h2 class="text-sm font-medium text-destructive">危险操作</h2>
		</div>
		<Card class="border-destructive/40">
			<CardContent class="p-4 space-y-3">
				<div>
					<p class="text-sm text-muted-foreground">
						清空全部业务数据（交易、账户、分类、预算、标签、周期、分期、报销、存钱计划）
						并重建默认账本与内置分类。此操作不可撤销，请先导出全量备份。
					</p>
				</div>
				<Button
					variant="outline"
					class="w-full border-destructive/50 text-destructive hover:bg-destructive/10"
					onclick={handleClearData}
					disabled={clearing}
				>
					<Trash2 size={16} />
					{clearing ? '清空中…' : '清空全部数据'}
				</Button>
			</CardContent>
		</Card>
	</section>
</div>
