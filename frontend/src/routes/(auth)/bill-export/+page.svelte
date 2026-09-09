<script lang="ts">
	import { onMount } from 'svelte';
	import Card from '$lib/components/ui/Card.svelte'
	import CardContent from '$lib/components/ui/CardContent.svelte'
	import CardHeader from '$lib/components/ui/CardHeader.svelte'
	import CardTitle from '$lib/components/ui/CardTitle.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import { ioApi } from '$lib/api/modules/io';
	import { appStore } from '$lib/stores/app';
	import { hzToast } from '$lib/components/ui/toast';
	import { http } from '$lib/api/http';
	import dayjs from 'dayjs';
	import { Download, FileText } from '@lucide/svelte';

	let selectedMonth = $state(dayjs().format('YYYY-MM'));

	async function exportCSV() {
		const [year, month] = selectedMonth.split('-');
		await ioApi.exportCSV({
			book_id: appStore.currentBookId,
			start_date: `${year}-${month}-01`,
			end_date: dayjs(`${year}-${month}`).endOf('month').format('YYYY-MM-DD')
		});
		hzToast.success('CSV 已下载');
	}

	async function viewBill() {
		try {
			const data = await http.get(`/io/bill?month=${selectedMonth}&book_id=${appStore.currentBookId}`);
			const w = window.open('', '_blank');
			if (w) {
				w.document.write(`<pre style="font-family:monospace;padding:20px;">${JSON.stringify(data, null, 2)}</pre>`);
			}
		} catch (e) {
			hzToast.error('获取账单失败');
		}
	}
</script>

<svelte:head>
	<title>账单导出 · 货殖</title>
</svelte:head>

<div class="max-w-xl mx-auto space-y-4">
	<Card>
		<CardHeader>
			<CardTitle>导出账单</CardTitle>
		</CardHeader>
		<CardContent class="space-y-4">
			<div class="space-y-2">
				<label for="export-month" class="text-sm font-medium">选择月份</label>
				<Input id="export-month" type="month" bind:value={selectedMonth} />
			</div>
			<div class="flex gap-2">
				<Button class="flex-1" onclick={viewBill}>
					<FileText size={16} />
					查看月度账单
				</Button>
				<Button class="flex-1" variant="outline" onclick={exportCSV}>
					<Download size={16} />
					导出 CSV
				</Button>
			</div>
		</CardContent>
	</Card>
</div>
