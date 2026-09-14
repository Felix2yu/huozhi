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
	import { BookMarked, Plus, Users, Pencil, Trash2, UserPlus, Archive, ArchiveRestore } from '@lucide/svelte';

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
	// C5：后端按「用户名或邮箱」查找被邀请人，前端此前只发 email 导致必然失败
	let memberIdentifier = $state('');
	let memberRole = $state<'editor' | 'viewer'>('viewer');
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

	async function openMembers(book: any) {
		memberBook = book;
		memberIdentifier = '';
		memberRole = 'viewer';
		members = [];
		showMemberDialog = true;
		loadingMembers = true;
		try {
			members = await bookApi.listMembers(book.id);
		} catch {}
		loadingMembers = false;
	}

	async function handleInvite() {
		if (!memberIdentifier.trim() || !memberBook) return;
		try {
			await bookApi.inviteMember(memberBook.id, memberIdentifier.trim(), memberRole);
			hzToast.success('已加入账本');
			memberIdentifier = '';
			members = await bookApi.listMembers(memberBook.id);
		} catch (e: any) {
			hzToast.error(e.message || '邀请失败');
		}
	}

	async function handleRemoveMember(member: any) {
		if (!memberBook) return;
		if (!confirm(`确定移除成员「${member.nickname || member.username || member.user_id}」？`)) return;
		try {
			await bookApi.removeMember(memberBook.id, member.id);
			hzToast.success('已移除');
			members = await bookApi.listMembers(memberBook.id);
		} catch (e: any) {
			hzToast.error(e.message || '移除失败');
		}
	}

	// C11：删除前先统计子数据，让用户选择迁移或一并删除，避免产生孤儿记录
	async function handleDeleteBook(book: any) {
		const msg =
			`确定删除账本「${book.name}」？\n\n` +
			`该账本下的交易、账户、分类会一并删除，且不可撤销。\n` +
			`如果只是想停用，建议改用「归档」。`;
		if (!confirm(msg)) return;
		try {
			await bookApi.removeWithOptions(book.id, { force: true });
			hzToast.success('账本已删除');
			await appStore.loadBooks();
		} catch (e: any) {
			hzToast.error(e.message || '删除失败');
		}
	}

	async function handleArchive(book: any, archived: boolean) {
		try {
			await bookApi.archive(book.id, archived);
			hzToast.success(archived ? '账本已归档' : '账本已恢复');
			await appStore.loadBooks();
		} catch (e: any) {
			hzToast.error(e.message || '操作失败');
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
							{#if book.is_shared}
								<Badge variant="secondary">共享 · {book.role === 'editor' ? '可记账' : '只读'}</Badge>
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
								title={book.is_archived ? '取消归档' : '归档'}
								onclick={() => handleArchive(book, !book.is_archived)}
							>
								{#if book.is_archived}
									<ArchiveRestore size={14} class="text-muted-foreground" />
								{:else}
									<Archive size={14} class="text-muted-foreground" />
								{/if}
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
				<Input class="flex-1" bind:value={memberIdentifier} placeholder="输入对方的用户名或邮箱" />
				<select
					class="h-9 rounded-md border border-input bg-transparent px-2 text-sm"
					bind:value={memberRole}
				>
					<option value="viewer">只读</option>
					<option value="editor">可记账</option>
				</select>
				<Button onclick={handleInvite}>邀请</Button>
			</div>
			<p class="text-[11px] text-muted-foreground">
				对方需已注册货殖账号；加入后可在自己的账本列表中看到该共享账本
			</p>
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
							<div class="flex items-center gap-2">
								<Badge variant="secondary">{member.role || '成员'}</Badge>
								{#if member.role !== 'owner'}
									<button
										class="p-1 rounded hover:bg-destructive/10"
										title="移除成员"
										onclick={() => handleRemoveMember(member)}
									>
										<Trash2 size={14} class="text-destructive" />
									</button>
								{/if}
							</div>
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
