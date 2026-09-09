<script lang="ts">
	import { onMount } from 'svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Card from '$lib/components/ui/Card.svelte'
import CardContent from '$lib/components/ui/CardContent.svelte';
	import Progress from '$lib/components/ui/Progress.svelte';
	import { savingApi } from '$lib/api/modules/savings';
	import { formatMoney } from '$lib/utils/format';
	import type { SavingPlan } from '$lib/types';
	import { Plus, PiggyBank } from '@lucide/svelte';

	let plans = $state<SavingPlan[]>([]);

	onMount(async () => {
		try { plans = await savingApi.list(); } catch {}
	});
</script>

<svelte:head>
	<title>存钱计划 · 货殖</title>
</svelte:head>

<div class="space-y-4">
	<div class="flex items-center justify-between">
		<h2 class="text-sm font-medium">存钱计划</h2>
		<Button size="sm" onclick={() => alert('功能开发中')}>
			<Plus size={16} />
			新建计划
		</Button>
	</div>
	{#if plans.length === 0}
		<Card>
			<div class="py-16 text-center">
				<PiggyBank size={32} class="mx-auto mb-3 text-muted-foreground opacity-50" />
				<p class="text-muted-foreground text-sm">暂无存钱计划</p>
			</div>
		</Card>
	{:else}
		<div class="grid gap-3 md:grid-cols-2">
			{#each plans as plan (plan.id)}
				<Card>
					<CardContent class="p-4">
						<div class="flex items-center justify-between mb-2">
							<div class="font-medium">{plan.name}</div>
							<span
								class="text-xs"


							>
								{plan.status === 'active' ? '进行中' : plan.status === 'done' ? '已完成' : '已暂停'}
							</span>
						</div>
						<Progress
							value={(plan.current_amount / plan.target_amount) * 100}
						/>
						<div class="flex items-center justify-between mt-2 text-sm">
							<span class="font-semibold tabular-nums">
								{formatMoney(plan.current_amount)}
							</span>
							<span class="text-muted-foreground">
								/ {formatMoney(plan.target_amount)}
							</span>
						</div>
					</CardContent>
				</Card>
			{/each}
		</div>
	{/if}
</div>
