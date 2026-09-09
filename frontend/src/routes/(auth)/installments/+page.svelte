<script lang="ts">
	import { onMount } from 'svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Card from '$lib/components/ui/Card.svelte'
import CardContent from '$lib/components/ui/CardContent.svelte';
	import Progress from '$lib/components/ui/Progress.svelte';
	import { installmentApi } from '$lib/api/modules/installments';
	import { formatMoney } from '$lib/utils/format';
	import type { Installment } from '$lib/types';
	import { Plus, CreditCard } from '@lucide/svelte';

	let list = $state<Installment[]>([]);

	onMount(async () => {
		try { list = await installmentApi.list(); } catch {}
	});
</script>

<svelte:head>
	<title>分期管理 · 货殖</title>
</svelte:head>

<div class="space-y-4">
	<div class="flex items-center justify-between">
		<h2 class="text-sm font-medium">进行中的分期</h2>
		<Button size="sm" onclick={() => alert('功能开发中')}>
			<Plus size={16} />
			新增分期
		</Button>
	</div>
	{#if list.length === 0}
		<Card>
			<div class="py-16 text-center">
				<CreditCard size={32} class="mx-auto mb-3 text-muted-foreground opacity-50" />
				<p class="text-muted-foreground text-sm">暂无分期</p>
			</div>
		</Card>
	{:else}
		<div class="grid gap-3 md:grid-cols-2">
			{#each list as item (item.id)}
				<Card>
					<CardContent class="p-4">
						<div class="flex items-center justify-between mb-2">
							<div class="font-medium truncate">{item.name}</div>
							<span
								class="text-xs"

							>
								{item.status === 'active' ? '还款中' : '已结清'}
							</span>
						</div>
						<Progress value={(item.paid_months / item.total_months) * 100} />
						<div class="flex items-center justify-between mt-2 text-sm">
							<div>
								<span class="font-semibold">{item.paid_months}/{item.total_months}</span>
								<span class="text-muted-foreground ml-1">期</span>
							</div>
							<div class="font-semibold tabular-nums">
								月供 {formatMoney(item.monthly_amount)}
							</div>
						</div>
					</CardContent>
				</Card>
			{/each}
		</div>
	{/if}
</div>
