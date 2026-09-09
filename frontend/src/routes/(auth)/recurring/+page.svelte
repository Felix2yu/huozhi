<script lang="ts">
	import { onMount } from 'svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Card from '$lib/components/ui/Card.svelte'
import CardContent from '$lib/components/ui/CardContent.svelte';
	import Badge from '$lib/components/ui/Badge.svelte';
	import { recurringApi } from '$lib/api/modules/recurring';
	import { formatMoney, formatRelativeDate } from '$lib/utils/format';
	import type { Recurring } from '$lib/types';
	import { Plus, Repeat } from '@lucide/svelte';

	let list = $state<Recurring[]>([]);

	onMount(async () => {
		try { list = await recurringApi.list(); } catch {}
	});
</script>

<svelte:head>
	<title>周期记账 · 货殖</title>
</svelte:head>

<div class="space-y-4">
	<div class="flex items-center justify-between">
		<h2 class="text-sm font-medium">周期任务</h2>
		<Button size="sm" onclick={() => alert('功能开发中')}>
			<Plus size={16} />
			新建周期
		</Button>
	</div>
	{#if list.length === 0}
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
					<div class="flex items-center gap-3 p-4">
						<div class="w-10 h-10 rounded-lg bg-muted grid place-items-center">
							<Repeat size={18} class="text-muted-foreground" />
						</div>
						<div class="flex-1 min-w-0">
							<div class="font-medium truncate">{item.description || item.name}</div>
							<div class="text-xs text-muted-foreground">
								{item.recurring_type} · 下次 {formatRelativeDate(item.next_run_at)}
							</div>
						</div>
						<div class="text-right">
							<div class="font-semibold tabular-nums"


							>
								{formatMoney(item.amount)}
							</div>
							<Badge variant={item.status === 'active' ? 'default' : 'secondary'} class="mt-1">
								{item.status === 'active' ? '运行中' : '暂停'}
							</Badge>
						</div>
					</div>
				{/each}
			</CardContent>
		</Card>
	{/if}
</div>
