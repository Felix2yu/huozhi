<script lang="ts">
	import { onMount } from 'svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Card from '$lib/components/ui/Card.svelte'
import CardContent from '$lib/components/ui/CardContent.svelte';
	import Badge from '$lib/components/ui/Badge.svelte';
	import { appStore } from '$lib/stores/app';
	import { Plus, Tag } from '@lucide/svelte';

	onMount(() => appStore.loadDictionaries());
</script>

<svelte:head>
	<title>标签中心 · 货殖</title>
</svelte:head>

<div class="space-y-4">
	<div class="flex items-center justify-between">
		<h2 class="text-sm font-medium">所有标签</h2>
		<Button size="sm" onclick={() => alert('功能开发中')}>
			<Plus size={16} />
			新增标签
		</Button>
	</div>
	<Card>
		<CardContent class="p-4">
			{#if appStore.tags.length === 0}
				<div class="py-12 text-center text-muted-foreground">
					<Tag size={32} class="mx-auto mb-3 opacity-50" />
					<p class="text-sm">暂无标签</p>
				</div>
			{:else}
				<div class="flex flex-wrap gap-2">
					{#each appStore.tags as tag (tag.id)}
						<Badge variant="secondary" class="cursor-pointer hover:bg-secondary/80">
							{tag.name}
							<span class="ml-1 text-muted-foreground">({tag.count})</span>
						</Badge>
					{/each}
				</div>
			{/if}
		</CardContent>
	</Card>
</div>
