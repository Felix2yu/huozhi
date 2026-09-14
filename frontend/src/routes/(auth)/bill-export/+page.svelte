<script lang="ts">
	/**
	 * 账单导出（A8）
	 *
	 * 此前是「孤儿页面」：不在导航里，只能靠 URL 直达；
	 * 且「查看月度账单」用 window.open + document.write(JSON.stringify(data)) 裸显 JSON。
	 * 现在挂在 设置 → 数据 下，并把月度账单渲染成可读、可打印的 HTML 账单。
	 */
	import Card from '$lib/components/ui/Card.svelte';
	import CardContent from '$lib/components/ui/CardContent.svelte';
	import CardHeader from '$lib/components/ui/CardHeader.svelte';
	import CardTitle from '$lib/components/ui/CardTitle.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import Badge from '$lib/components/ui/Badge.svelte';
	import { ioApi } from '$lib/api/modules/io';
	import { appStore } from '$lib/stores/app';
	import { hzToast } from '$lib/components/ui/toast';
	import { http } from '$lib/api/http';
	import { formatMoney } from '$lib/utils/format';
	import dayjs from 'dayjs';
	import { Download, FileText, Printer } from '@lucide/svelte';

	let selectedMonth = $state(dayjs().format('YYYY-MM'));
	let bill = $state<any>(null);
	let loading = $state(false);

	async function loadBill() {
		loading = true;
		try {
			bill = await http.get(
				`/io/bill?month=${selectedMonth}&book_id=${appStore.currentBookId || 0}`
			);
		} catch (e: any) {
			hzToast.error(e.message || '获取账单失败');
			bill = null;
		} finally {
			loading = false;
		}
	}

	async function exportCSV() {
		const [year, month] = selectedMonth.split('-');
		await ioApi.exportCSV({
			book_id: appStore.currentBookId,
			start_date: `${year}-${month}-01`,
			end_date: dayjs(`${year}-${month}`).endOf('month').format('YYYY-MM-DD')
		});
		hzToast.success('CSV 已下载');
	}

	function printBill() {
		window.print();
	}
</script>

<svelte:head>
	<title>账单导出 · 货殖</title>
</svelte:head>

<div class="max-w-3xl mx-auto space-y-4">
	<Card class="print:hidden">
		<CardHeader>
			<CardTitle>导出账单</CardTitle>
		</CardHeader>
		<CardContent class="space-y-4">
			<div class="flex items-end gap-2">
				<div class="space-y-2 flex-1">
					<label for="export-month" class="text-sm font-medium">选择月份</label>
					<Input id="export-month" type="month" bind:value={selectedMonth} />
				</div>
				<Button onclick={loadBill} disabled={loading}>
					<FileText size={16} />
					{loading ? '生成中…' : '查看月度账单'}
				</Button>
			</div>
			<div class="flex gap-2">
				<Button class="flex-1" variant="outline" onclick={exportCSV}>
					<Download size={16} />
					导出 CSV
				</Button>
				{#if bill}
					<Button class="flex-1" variant="outline" onclick={printBill}>
						<Printer size={16} />
						打印 / 存为 PDF
					</Button>
				{/if}
			</div>
		</CardContent>
	</Card>

	{#if bill}
		<Card id="bill-print">
			<CardContent class="p-8 space-y-6">
				<!-- 账单抬头 -->
				<div class="text-center border-b pb-4">
					<div class="text-xl font-bold">{bill.meta?.month} 月度账单</div>
					<div class="text-xs text-muted-foreground mt-1">
						{bill.meta?.book_name || '全部账本'} · {bill.meta?.range_start} ~ {bill.meta?.range_end}
					</div>
					<div class="text-xs text-muted-foreground">
						{bill.meta?.user_name || ''} · 生成于 {bill.meta?.generated_at}
					</div>
				</div>

				<!-- 收支汇总 -->
				<section>
					<h3 class="text-sm font-medium mb-2">收支汇总</h3>
					<div class="grid grid-cols-4 gap-3 text-center">
						<div class="p-3 rounded-lg bg-muted/50">
							<div class="text-[11px] text-muted-foreground">收入</div>
							<div class="font-semibold tabular-nums text-[var(--color-income)]">
								{formatMoney(bill.summary?.total_income)}
							</div>
							<div class="text-[10px] text-muted-foreground">{bill.summary?.income_count} 笔</div>
						</div>
						<div class="p-3 rounded-lg bg-muted/50">
							<div class="text-[11px] text-muted-foreground">支出</div>
							<div class="font-semibold tabular-nums text-[var(--color-expense)]">
								{formatMoney(bill.summary?.total_expense)}
							</div>
							<div class="text-[10px] text-muted-foreground">{bill.summary?.expense_count} 笔</div>
						</div>
						<div class="p-3 rounded-lg bg-muted/50">
							<div class="text-[11px] text-muted-foreground">结余</div>
							<div class="font-semibold tabular-nums">{formatMoney(bill.summary?.net)}</div>
						</div>
						<div class="p-3 rounded-lg bg-muted/50">
							<div class="text-[11px] text-muted-foreground">日均支出</div>
							<div class="font-semibold tabular-nums">
								¥{(bill.summary?.avg_daily_expense ?? 0).toFixed(2)}
							</div>
						</div>
					</div>
				</section>

				<!-- 支出分类 -->
				{#if (bill.category_expense || []).length > 0}
					<section>
						<h3 class="text-sm font-medium mb-2">支出分类</h3>
						<table class="w-full text-sm">
							<thead class="text-xs text-muted-foreground border-b">
								<tr>
									<th class="text-left py-1">分类</th>
									<th class="text-right py-1">金额</th>
									<th class="text-right py-1">笔数</th>
									<th class="text-right py-1">占比</th>
								</tr>
							</thead>
							<tbody>
								{#each bill.category_expense as item (item.id)}
									<tr class="border-b last:border-0">
										<td class="py-1">{item.icon || '📁'} {item.name}</td>
										<td class="py-1 text-right tabular-nums">{formatMoney(item.amount)}</td>
										<td class="py-1 text-right tabular-nums">{item.count}</td>
										<td class="py-1 text-right tabular-nums">{item.percent?.toFixed(1)}%</td>
									</tr>
								{/each}
							</tbody>
						</table>
					</section>
				{/if}

				<!-- 预算执行 -->
				{#if (bill.budgets || []).length > 0}
					<section>
						<h3 class="text-sm font-medium mb-2">预算执行</h3>
						<div class="space-y-2">
							{#each bill.budgets as b (b.id)}
								<div class="flex items-center gap-3 text-sm">
									<span class="flex-1 truncate">{b.name}</span>
									<span class="tabular-nums text-muted-foreground">
										{formatMoney(b.used_amount)} / {formatMoney(b.amount)}
									</span>
									{#if b.is_over_budget}
										<Badge variant="destructive">超支</Badge>
									{:else}
										<span class="tabular-nums text-xs w-12 text-right">{b.usage_rate}%</span>
									{/if}
								</div>
							{/each}
						</div>
					</section>
				{/if}

				<!-- 资产 -->
				<section>
					<h3 class="text-sm font-medium mb-2">期末资产</h3>
					<div class="grid grid-cols-3 gap-3 text-center text-sm">
						<div class="p-3 rounded-lg bg-muted/50">
							<div class="text-[11px] text-muted-foreground">总资产</div>
							<div class="font-semibold tabular-nums">{formatMoney(bill.assets?.total_asset)}</div>
						</div>
						<div class="p-3 rounded-lg bg-muted/50">
							<div class="text-[11px] text-muted-foreground">总负债</div>
							<div class="font-semibold tabular-nums">{formatMoney(bill.assets?.total_debt)}</div>
						</div>
						<div class="p-3 rounded-lg bg-muted/50">
							<div class="text-[11px] text-muted-foreground">净资产</div>
							<div class="font-semibold tabular-nums">{formatMoney(bill.assets?.net_asset)}</div>
						</div>
					</div>
				</section>

				<div class="text-center text-[11px] text-muted-foreground pt-4 border-t">
					由「货殖」生成 · {bill.meta?.generated_at}
				</div>
			</CardContent>
		</Card>
	{:else if !loading}
		<Card>
			<div class="py-12 text-center text-sm text-muted-foreground">
				选择月份后点击「查看月度账单」
			</div>
		</Card>
	{/if}
</div>
