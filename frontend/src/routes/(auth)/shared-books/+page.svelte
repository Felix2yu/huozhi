<script lang="ts">
	import { goto } from '$app/navigation';
	import Button from '$lib/components/ui/Button.svelte';
	import Card from '$lib/components/ui/Card.svelte';
	import CardContent from '$lib/components/ui/CardContent.svelte';
	import Dialog from '$lib/components/ui/Dialog.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import Label from '$lib/components/ui/Label.svelte';
	import Badge from '$lib/components/ui/Badge.svelte';
	import { bookApi } from '$lib/api/modules/books';
	import { appStore } from '$lib/stores/app';
	import { hzToast } from '$lib/components/ui/toast';
	import { BookMarked, Plus, Users, Pencil, Trash2, UserPlus } from '@lucide/svelte';

	// Dialog state
	let showBookDialog = $state(false);
	let showMemberDialog = $state(false);
	let editingBook = $state<{ id: number; name: string; icon?: string; description?: string } | null>(null);
	let bookName = $state('');
	let bookIcon = $state('📘');
	let bookDesc = $state('');
	let saving = $state(false);

	// Member dialog
	let memberBook = $state<{ id: number; name: string } | null>(null);
	let memberEmail = $state('');
	let members = $state<any[]>([]);
	let loadingMembers = $state(false);

	const iconOptions = ['📘', '📕', '📗', '📙', '📓', '📒', '📔', '💰', '💵', '🏦', '📊', '📈', '💎', '🎯', '🏆', '💼', '🏠', '🚗', '✈️', '🎓'];

	function openNewBook() {
		editingBook = null;
		bookName = '';
		bookIcon = '📘';
		bookDesc = '';
		showBookDialog = true;
	}

	function openEditBook(book: any) {
		editingBook = book;
		bookName = book.name;
		bookIcon = book.icon || '📘';
		bookDesc = book.description || '';
		showBookDialog = true;
	}

	async function handleSaveBook() {
		if (!bookName.trim()) {
			hzToast.warning('请输入账本名称');
			return;
		}

		saving = true;
		try {
			const data = { name: bookName.trim(), icon: bookIcon, description: bookDesc.trim() };
			if (editingBook) {
				await bookApi.update(editingBook.id, data);
				hzToast.success('账本已更新');
			} else {
				await bookApi.create(data);
				hzToast.success('账本已创建');
			}
			showBookDialog = false;
			await appStore.loadBooks();
		} catch (e: any) {
			hzToast.error(e.message || '保存失败');
		} finally {
			saving = false;
		}
	}

	async function handleDeleteBook(book: any) {
		if (!confirm(`确定删除账本「${book.name}」？此操作不可撤销！`)) return;
		try {
			await bookApi.remove(book.id);
			hzToast.success('账本已删除');
			await appStore.loadBooks();
		} catch (e: any) {
			hzToast.error(e.message || '删除失败');
		}
	}

	async function openMembers(book: any) {
		memberBook = book;
		memberEmail = '';
		members = [];
		showMemberDialog = true;
		loadingMembers = true;
		try {
			members = await bookApi.listMembers(book.id);
		} catch {}
		loadingMembers = false;
	}

	async function handleInvite() {
		if (!memberEmail.trim() || !memberBook) return;
		try {
			await bookApi.inviteMember(memberBook.id, { email: memberEmail.trim() });
			hzToast.success('邀请已发送');
			memberEmail = '';
			members = await bookApi.listMembers(memberBook.id);
		} catch (e: any) {
			hzToast.error(e.message || '邀请失败');
		}
	}
</script>

<svelte:head>
	<title>共享账本 · 货殖</title>
</svelte:head>

<div class="space-y-4">
	<div class="flex items-center justify-between">
		<h2 class="text-sm font-medium">我的账本</h2>
		<Button size="sm" onclick={openNewBook}>
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
					<div class="flex items-center gap-3 p-4 group">
						<button
							class="flex-1 flex items-center gap-3 text-left hover:bg-accent/50 transition"
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
						<div class="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition">
							<button
								class="p-1.5 rounded hover:bg-accent"
								title="成员管理"
								onclick={() => openMembers(book)}
							>
								<UserPlus size={14} class="text-muted-foreground" />
							</button>
							<button
								class="p-1.5 rounded hover:bg-accent"
								title="编辑"
								onclick={() => openEditBook(book)}
							>
								<Pencil size={14} class="text-muted-foreground" />
							</button>
							{#if !book.is_default}
								<button
									class="p-1.5 rounded hover:bg-destructive/10"
									title="删除"
									onclick={() => handleDeleteBook(book)}
								>
									<Trash2 size={14} class="text-destructive" />
								</button>
							{/if}
						</div>
					</div>
				{/each}
			{/if}
		</CardContent>
	</Card>
</div>

<!-- 新建/编辑账本 Dialog -->
<Dialog bind:open={showBookDialog}>
	<div class="space-y-4">
		<h3 class="text-lg font-semibold">
			{editingBook ? '编辑账本' : '新建账本'}
		</h3>

		<div class="space-y-2">
			<Label>图标</Label>
			<div class="flex flex-wrap gap-1">
				{#each iconOptions as icon}
					<button
						class="w-8 h-8 text-lg rounded hover:bg-accent transition {bookIcon === icon ? 'bg-primary/10' : ''}"
						onclick={() => (bookIcon = icon)}
					>
						{icon}
					</button>
				{/each}
			</div>
		</div>

		<div class="space-y-2">
			<Label>账本名称</Label>
			<Input bind:value={bookName} placeholder="例如: 家庭账本" />
		</div>

		<div class="space-y-2">
			<Label>描述</Label>
			<Input bind:value={bookDesc} placeholder="可选" />
		</div>

		<div class="flex gap-2 justify-end pt-2">
			<Button variant="outline" onclick={() => (showBookDialog = false)}>取消</Button>
			<Button onclick={handleSaveBook} disabled={saving}>
				{saving ? '保存中...' : '保存'}
			</Button>
		</div>
	</div>
</Dialog>

<!-- 成员管理 Dialog -->
<Dialog bind:open={showMemberDialog}>
	<div class="space-y-4">
		<h3 class="text-lg font-semibold">成员管理 - {memberBook?.name}</h3>

		<div class="space-y-2">
			<Label>邀请成员</Label>
			<div class="flex gap-2">
				<Input class="flex-1" bind:value={memberEmail} placeholder="输入邮箱" type="email" />
				<Button onclick={handleInvite}>邀请</Button>
			</div>
		</div>

		<div class="space-y-2">
			<Label>当前成员</Label>
			{#if loadingMembers}
				<div class="text-sm text-muted-foreground animate-pulse">加载中...</div>
			{:else if members.length === 0}
				<div class="text-sm text-muted-foreground">暂无成员</div>
			{:else}
				<div class="space-y-2">
					{#each members as member}
						<div class="flex items-center justify-between p-2 rounded-lg border">
							<div class="flex items-center gap-2">
								<div class="w-8 h-8 rounded-full bg-muted grid place-items-center text-xs">
									{member.nickname?.[0] || member.username?.[0] || '?'}
								</div>
								<div>
									<div class="text-sm font-medium">{member.nickname || member.username}</div>
									<div class="text-xs text-muted-foreground">{member.email}</div>
								</div>
							</div>
							<Badge variant="secondary">{member.role || '成员'}</Badge>
						</div>
					{/each}
				</div>
			{/if}
		</div>

		<div class="flex justify-end pt-2">
			<Button variant="outline" onclick={() => (showMemberDialog = false)}>关闭</Button>
		</div>
	</div>
</Dialog>
