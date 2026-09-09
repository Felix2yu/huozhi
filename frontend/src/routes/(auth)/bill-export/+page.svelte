<script lang="ts">
	import Card from '$lib/components/ui/Card.svelte'
import CardContent from '$lib/components/ui/CardContent.svelte'
import CardHeader from '$lib/components/ui/CardHeader.svelte'
import CardTitle from '$lib/components/ui/CardTitle.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import { ioApi } from '$lib/api/modules/io';
	import { appStore } from '$lib/stores/app';
	import { hzToast } from '$lib/components/ui/toast';
	import dayjs from 'dayjs';
	import { Download, FileText } from '@lucide/svelte';

	let selectedMonth = $state(dayjs().format('YYYY-MM'));

	function exportCSV() {
		const [year, month] = selectedMonth.split('-');
		ioApi.exportCSV({
			book_id: appStore.currentBookId,
			start_date: `${year}-${month}-01`,
			end_date: dayjs(`${year}-${month}`).endOf('month').format('YYYY-MM-DD')
		});
	}

	function viewBill() {
		window.open(`/api/io/bill?month=${selectedMonth}&book_id=${appStore.currentBookId}`);
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
				<label class="text-sm font-medium">选择月份</label>
				<Input type="month" bind:value={selectedMonth} />
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
