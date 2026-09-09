<script lang="ts">
	import Button from '$lib/components/ui/Button.svelte';
	import Card from '$lib/components/ui/Card.svelte'
import CardContent from '$lib/components/ui/CardContent.svelte';
	import { appStore } from '$lib/stores/app';
	import { hzToast } from '$lib/components/ui/toast';
	import { goto } from '$app/navigation';
	import { BookMarked, Plus, Users } from '@lucide/svelte';
</script>

<svelte:head>
	<title>共享账本 · 货殖</title>
</svelte:head>

<div class="space-y-4">
	<div class="flex items-center justify-between">
		<h2 class="text-sm font-medium">我的账本</h2>
		<Button size="sm" onclick={() => hzToast.info('功能开发中')}>
			<Plus size={16} />
			新建账本
		</Button>
	</div>
	<Card>
		<CardContent class="p-0 divide-y">
			{#if appStore.books.length === 0}
				<div class="py-16 text-center">
					<BookMarked size={32} class="mx-auto mb-3 text-muted-foreground opacity-50" />
					<p class="text-muted-foreground text-sm">暂无账本</p>
				</div>
			{:else}
				{#each appStore.books as book (book.id)}
					<button
						class="w-full flex items-center gap-3 p-4 text-left hover:bg-accent/50 transition"
						onclick={() => {
							appStore.setCurrentBook(book.id);
							goto('/dashboard');
						}}
					>
						<span class="w-10 h-10 rounded-lg bg-muted grid place-items-center text-xl">
							{book.icon || '📘'}
						</span>
						<div class="flex-1 min-w-0">
							<div class="font-medium truncate">
								{book.name}
								{#if book.is_default}
									<span class="ml-1 text-[10px] text-muted-foreground bg-muted px-1.5 py-0.5 rounded">默认</span>
								{/if}
							</div>
							<div class="text-xs text-muted-foreground truncate">
								{book.description || '个人账本'}
							</div>
						</div>
						{#if book.id === appStore.currentBookId}
							<span class="text-xs text-primary font-medium">当前</span>
						{/if}
					</button>
				{/each}
			{/if}
		</CardContent>
	</Card>
</div>
