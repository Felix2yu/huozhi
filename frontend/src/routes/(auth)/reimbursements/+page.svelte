<script lang="ts">
	import { onMount } from 'svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Card from '$lib/components/ui/Card.svelte'
import CardContent from '$lib/components/ui/CardContent.svelte';
	import Badge from '$lib/components/ui/Badge.svelte';
	import { reimbApi } from '$lib/api/modules/reimbursements';
	import { formatMoney } from '$lib/utils/format';
	import type { Reimbursement } from '$lib/types';
	import { Plus, FileText } from '@lucide/svelte';

	let list = $state<Reimbursement[]>([]);

	onMount(async () => {
		try { list = await reimbApi.list(); } catch {}
	});
</script>

<svelte:head>
	<title>报销管理 · 货殖</title>
</svelte:head>

<div class="space-y-4">
	<div class="flex items-center justify-between">
		<h2 class="text-sm font-medium">报销单</h2>
		<Button size="sm" onclick={() => alert('功能开发中')}>
			<Plus size={16} />
			新建报销
		</Button>
	</div>
	{#if list.length === 0}
		<Card>
			<div class="py-16 text-center">
				<FileText size={32} class="mx-auto mb-3 text-muted-foreground opacity-50" />
				<p class="text-muted-foreground text-sm">暂无报销单</p>
			</div>
		</Card>
	{:else}
		<Card>
			<CardContent class="p-0 divide-y">
				{#each list as item (item.id)}
					<div class="flex items-center gap-3 p-4">
						<div class="flex-1 min-w-0">
							<div class="font-medium truncate">{item.name}</div>
							<div class="text-xs text-muted-foreground">
								共 {item.transaction_ids.length} 笔交易
							</div>
						</div>
						<div class="text-right">
							<div class="font-semibold tabular-nums">
								{formatMoney(item.received_amount)}
								<span class="text-muted-foreground text-sm">
									/ {formatMoney(item.total_amount)}
								</span>
							</div>
							<Badge
								variant={
									item.status === 'done'
										? 'default'
										: item.status === 'partial'
											? 'secondary'
											: 'outline'
								}
								class="mt-1"
							>
								{item.status === 'done' ? '已收齐' : item.status === 'partial' ? '部分' : '待处理'}
							</Badge>
						</div>
					</div>
				{/each}
			</CardContent>
		</Card>
	{/if}
</div>
