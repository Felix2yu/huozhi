<script lang="ts">
	import { onMount } from 'svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Card from '$lib/components/ui/Card.svelte';
	import CardContent from '$lib/components/ui/CardContent.svelte';
	import Badge from '$lib/components/ui/Badge.svelte';
	import Dialog from '$lib/components/ui/Dialog.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import Label from '$lib/components/ui/Label.svelte';
	import AccountSelect from '$lib/components/AccountSelect.svelte';
	import { recurringApi } from '$lib/api/modules/recurring';
	import { appStore } from '$lib/stores/app';
	import { hzToast } from '$lib/components/ui/toast';
	import { formatMoney, formatRelativeDate } from '$lib/utils/format';
	import type { Recurring, RecurringType } from '$lib/types';
	import { Plus, Repeat, Trash2, Pause, Play } from '@lucide/svelte';

	let list = $state<Recurring[]>([]);
	let loading = $state(true);

	// Dialog state
	let showDialog = $state(false);
	let name = $state('');
	let type = $state<'expense' | 'income'>('expense');
	let amount = $state('');
	let categoryId = $state<number>(0);
	let accountId = $state<number>(0);
	let description = $state('');
	let recurringType = $state<RecurringType>('monthly');
	let interval = $state('1');
	let startDate = $state('');
	// monthly 的「每月几号」与 weekly 的「星期几」此前根本没有表单项，
	// 后端收到的恒为 0，只能退化成「按 start_date 的日期/星期」推算 ——
	// 用户选了「每月」却无法指定几号，周期语义被静默改写。
	let monthDay = $state('1');
	let weekday = $state('1');
	let saving = $state(false);

	const TYPE_LABELS: Record<string, string> = {
		daily: '每天',
		weekly: '每周',
		biweekly: '每两周',
		monthly: '每月',
		yearly: '每年',
		custom: '自定义'
	};
	const WEEKDAYS = ['一', '二', '三', '四', '五', '六', '日'];

	/** 周期文案：每月 25 号 / 每周三 / 每 3 天 */
	function describeCycle(item: Recurring): string {
		const base = TYPE_LABELS[item.recurring_type] ?? item.recurring_type;
		if (item.recurring_type === 'monthly' && item.month_day > 0) {
			return `${base} ${item.month_day} 号`;
		}
		if (item.recurring_type === 'weekly' && item.weekday >= 1 && item.weekday <= 7) {
			return `每周${WEEKDAYS[item.weekday - 1]}`;
		}
		if ((item.recurring_type === 'daily' || item.recurring_type === 'custom') && item.interval > 1) {
			return `每 ${item.interval} 天`;
		}
		return base;
	}

	/** next_run_at 可能是 null（已达最大次数/已结束被自动暂停），直接格式化会得到 Invalid Date */
	function nextRunLabel(v?: string | null): string {
		if (!v || v.startsWith('0001')) return '已停止';
		return formatRelativeDate(v);
	}

	async function loadData() {
		loading = true;
		try {
			list = await recurringApi.list();
		} catch {}
		loading = false;
	}

	onMount(loadData);

	function openNew() {
		name = '';
		type = 'expense';
		amount = '';
		categoryId = appStore.categories.expense[0]?.id || 0;
		accountId = appStore.accounts[0]?.id || 0;
		description = '';
		recurringType = 'monthly';
		interval = '1';
		monthDay = String(new Date().getDate());
		weekday = String(((new Date().getDay() + 6) % 7) + 1); // JS 周日=0 → 业务口径 7
		startDate = new Date().toISOString().split('T')[0];
		showDialog = true;
	}

	async function handleSave() {
		const amt = parseFloat(amount);
		if (!name.trim()) {
			hzToast.warning('请输入名称');
			return;
		}
		if (!amt || amt <= 0) {
			hzToast.warning('请输入有效金额');
			return;
		}

		saving = true;
		try {
			await recurringApi.create({
				name: name.trim(),
				type,
				amount: amt,
				category_id: categoryId,
				account_id: accountId,
				description: description.trim(),
				recurring_type: recurringType,
				interval: parseInt(interval) || 1,
				// 只有对应周期类型才下发，避免给后端留下相互矛盾的排期参数
				month_day: recurringType === 'monthly' ? parseInt(monthDay) : 0,
				weekday: recurringType === 'weekly' ? parseInt(weekday) : 0,
				start_date: startDate,
				book_id: appStore.effectiveBookId()
			});
			hzToast.success('周期任务已创建');
			showDialog = false;
			await loadData();
		} catch (e: any) {
			hzToast.error(e.message || '创建失败');
		} finally {
			saving = false;
		}
	}

	async function handleToggle(item: Recurring) {
		try {
			await recurringApi.toggle(item.id);
			hzToast.success(item.status === 'active' ? '已暂停' : '已启用');
			await loadData();
		} catch (e: any) {
			hzToast.error(e.message || '操作失败');
		}
	}

	async function handleDelete(item: Recurring) {
		if (!confirm(`确定删除「${item.name || item.description}」？`)) return;
		try {
			await recurringApi.remove(item.id);
			hzToast.success('已删除');
			await loadData();
		} catch (e: any) {
			hzToast.error(e.message || '删除失败');
		}
	}
</script>

<svelte:head>
	<title>周期记账 · 货殖</title>
</svelte:head>

<div class="space-y-4">
	<div class="flex items-center justify-between">
		<h2 class="text-sm font-medium">周期任务</h2>
		<Button size="sm" onclick={openNew}>
			<Plus size={16} />
			新建周期
		</Button>
	</div>
	{#if loading}
		<div class="space-y-2">
			{#each [1, 2, 3] as i}
				<div class="h-16 rounded-lg animate-pulse bg-muted" ></div>
			{/each}
		</div>
	{:else if list.length === 0}
		<Card>
			<div class="py-16 text-center">
				<Repeat size={32} class="mx-auto mb-3 text-muted-foreground opacity-50" />
				<p class="text-muted-foreground text-sm">暂无周期任务</p>
			</div>
		</Card>
	{:else}
		<Card>
			<CardContent class="p-0 divide-y">
				{#each list as item (item.id)}
					<div class="flex items-center gap-3 p-4 group">
						<div class="w-10 h-10 rounded-lg bg-muted grid place-items-center">
							<Repeat size={18} class="text-muted-foreground" />
						</div>
						<div class="flex-1 min-w-0">
							<div class="font-medium truncate">{item.description || item.name}</div>
							<div class="text-xs text-muted-foreground">
								{describeCycle(item)} · 下次 {nextRunLabel(item.next_run_at)}
								{#if item.max_times > 0}
									· 已执行 {item.run_count}/{item.max_times} 次
								{/if}
							</div>
						</div>
						<div class="text-right flex items-center gap-1">
							<div class="font-semibold tabular-nums">
								{item.type === 'expense' ? '-' : '+'}{formatMoney(item.amount)}
							</div>
							<button
								class="p-1 rounded hover:bg-accent transition"
								title={item.status === 'active' ? '暂停' : '启用'}
								onclick={() => handleToggle(item)}
							>
								{#if item.status === 'active'}
									<Pause size={14} class="text-muted-foreground" />
								{:else}
									<Play size={14} class="text-green-500" />
								{/if}
							</button>
							<button
								class="p-1 rounded hover:bg-destructive/10 opacity-0 group-hover:opacity-100 transition"
								onclick={() => handleDelete(item)}
							>
								<Trash2 size={14} class="text-destructive" />
							</button>
						</div>
					</div>
				{/each}
			</CardContent>
		</Card>
	{/if}
</div>

<!-- 新建 Dialog -->
<Dialog bind:open={showDialog}>
	<div class="space-y-4">
		<h3 class="text-lg font-semibold">新建周期任务</h3>

		<div class="space-y-2">
			<Label>名称</Label>
			<Input bind:value={name} placeholder="例如: 房租、工资" />
		</div>

		<div class="grid grid-cols-2 gap-4">
			<div class="space-y-2">
				<Label>类型</Label>
				<select
					class="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm focus:outline-none focus:ring-1 focus:ring-ring"
					bind:value={type}
				>
					<option value="expense">支出</option>
					<option value="income">收入</option>
				</select>
			</div>
			<div class="space-y-2">
				<Label>金额</Label>
				<div class="relative">
					<span class="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground">¥</span>
					<Input class="pl-8" type="number" step="0.01" placeholder="0.00" bind:value={amount} />
				</div>
			</div>
		</div>

		<div class="grid grid-cols-2 gap-4">
			<div class="space-y-2">
				<Label>分类</Label>
				<select
					class="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm focus:outline-none focus:ring-1 focus:ring-ring"
					bind:value={categoryId}
				>
					{#each (type === 'expense' ? appStore.categories.expense : appStore.categories.income) as cat}
						<option value={cat.id}>{cat.icon || '📁'} {cat.name}</option>
					{/each}
				</select>
			</div>
			<div class="space-y-2">
				<Label>账户</Label>
				<AccountSelect bind:value={accountId} includeArchived />
			</div>
		</div>

		<div class="space-y-2">
			<Label>描述</Label>
			<Input bind:value={description} placeholder="可选" />
		</div>

		<div class="grid grid-cols-2 gap-4">
			<div class="space-y-2">
				<Label>周期</Label>
				<select
					class="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm focus:outline-none focus:ring-1 focus:ring-ring"
					bind:value={recurringType}
				>
					{#each Object.entries(TYPE_LABELS) as [value, label]}
						<option {value}>{label}</option>
					{/each}
				</select>
			</div>
			<div class="space-y-2">
				<Label>开始日期</Label>
				<Input type="date" bind:value={startDate} />
			</div>
		</div>

		<!-- 每月几号 / 每周星期几：此前缺失，周期语义只能靠 start_date 反推 -->
		{#if recurringType === 'monthly'}
			<div class="space-y-2">
				<Label>每月几号</Label>
				<select
					class="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm focus:outline-none focus:ring-1 focus:ring-ring"
					bind:value={monthDay}
				>
					{#each Array.from({ length: 31 }, (_, i) => i + 1) as d}
						<option value={String(d)}>每月 {d} 号</option>
					{/each}
				</select>
				<p class="text-[11px] text-muted-foreground">
					目标月不足该日期时自动收敛到当月最后一天（如 2 月 31 号 → 2 月 28/29 号）
				</p>
			</div>
		{:else if recurringType === 'weekly'}
			<div class="space-y-2">
				<Label>每周星期几</Label>
				<select
					class="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm focus:outline-none focus:ring-1 focus:ring-ring"
					bind:value={weekday}
				>
					{#each WEEKDAYS as w, i}
						<option value={String(i + 1)}>每周{w}</option>
					{/each}
				</select>
			</div>
		{:else}
			<div class="space-y-2">
				<Label>间隔天数</Label>
				<Input type="number" min={1} bind:value={interval} />
			</div>
		{/if}

		<div class="flex gap-2 justify-end pt-2">
			<Button variant="outline" onclick={() => (showDialog = false)}>取消</Button>
			<Button onclick={handleSave} disabled={saving}>
				{saving ? '保存中...' : '保存'}
			</Button>
		</div>
	</div>
</Dialog>
