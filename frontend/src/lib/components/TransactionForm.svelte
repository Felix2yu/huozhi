<script lang="ts">
	/**
	 * 统一的交易表单：记一笔（新增）与编辑共用同一份实现。
	 *
	 * 此前 add/+page.svelte 与 edit/[id]/+page.svelte 是复制粘贴的两份（各约 250 行），
	 * 且 add 页里残留了恒为 false 的 isEdit 分支（C16）。合并后消除分叉风险。
	 *
	 * 同时补齐后端早已支持但前端未暴露的字段（A5）：
	 * 标签、图片凭证、商户、地点、备注、是否计入预算；
	 * 并接入 AI 智能记账（A6）。
	 */
	import { goto } from '$app/navigation';
	import Button from '$lib/components/ui/Button.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import Card from '$lib/components/ui/Card.svelte';
	import CardContent from '$lib/components/ui/CardContent.svelte';
	import CardHeader from '$lib/components/ui/CardHeader.svelte';
	import CardTitle from '$lib/components/ui/CardTitle.svelte';
	import Tabs from '$lib/components/ui/Tabs.svelte';
	import TabsTrigger from '$lib/components/ui/TabsTrigger.svelte';
	import Label from '$lib/components/ui/Label.svelte';
	import Badge from '$lib/components/ui/Badge.svelte';
	import AccountSelect from '$lib/components/AccountSelect.svelte';
	import CategoryPicker from '$lib/components/CategoryPicker.svelte';
	import { txApi } from '$lib/api/modules/transactions';
	import { uploadApi } from '$lib/api/modules/upload';
	import { appStore } from '$lib/stores/app';
	import { hzToast } from '$lib/components/ui/toast';
	import { aiApi } from '$lib/api/modules/ai';
	import { formatDate } from '$lib/utils/format';
	import {
		Sparkles, Save, X, ChevronDown, ChevronUp, ImagePlus, Trash2, Wand2
	} from '@lucide/svelte';

	interface Props {
		id?: string | number | null;
		clone?: any | null;
	}
	let { id = null, clone = null }: Props = $props();

	const isEdit = $derived(!!id);

	let type = $state<'expense' | 'income' | 'transfer'>('expense');
	let amount = $state('');
	let description = $state('');
	let categoryId = $state<number>(0);
	let accountId = $state<number>(0);
	let toAccountId = $state<number>(0);
	let txDate = $state(formatDate(new Date(), 'YYYY-MM-DD'));
	let loading = $state(false);
	let aiLoading = $state(false);

	// A5：后端已支持但此前完全没有入口的字段
	let merchant = $state('');
	let location = $state('');
	let remark = $state('');
	let tagIds = $state<number[]>([]);
	let images = $state<string[]>([]);
	let includeInBudget = $state(true);
	let showMore = $state(false);
	let uploading = $state(false);

	// A6：智能记账（粘贴一段文本自动解析金额/分类/商户）
	let showSmartInput = $state(false);
	let smartText = $state('');
	let smartLoading = $state(false);

	// B10：连续记账
	let keepGoing = $state(false);

	// B11：多币种。后端 Transaction 支持 currency / exchange_rate，
	// 但此前没有任何录入入口，外币记账无法正确折算。
	let currency = $state('CNY');
	let exchangeRate = $state('');

	const baseCurrency = $derived((appStore.user as any)?.currency || 'CNY');
	// 账户币种优先，其次用户基准币种
	const effectiveCurrency = $derived(
		appStore.accounts.find((a) => a.id === accountId)?.currency || baseCurrency
	);
	const isForeign = $derived(currency !== baseCurrency);

	let categories = $derived.by(() => {
		if (type === 'income') return appStore.categories.income;
		return appStore.categories.expense;
	});

	// 默认值初始化（新增模式）
	$effect(() => {
		if (isEdit) return;
		if (!accountId && appStore.accounts.length > 0) {
			accountId = appStore.accounts[0].id;
			if (appStore.accounts.length > 1 && !toAccountId) {
				toAccountId = appStore.accounts[1].id;
			}
		}
		const firstLeaf = categories.find((c) => c.parent_id) || categories[0];
		if (firstLeaf && !categories.find((c) => c.id === categoryId)) {
			categoryId = firstLeaf.id;
		}
		// B11：币种跟随所选账户，用户可手动改为其他币种
		currency = effectiveCurrency;
	});

	// 编辑模式：加载原交易
	$effect(() => {
		if (!isEdit || !id) return;
		txApi
			.get(Number(id))
			.then((tx) => {
				type = tx.type as any;
				// 后端 Money 类型 JSON 序列化已是「元」，直接使用，勿再 /100
				amount = String(tx.amount ?? 0);
				description = tx.description || '';
				categoryId = tx.category_id;
				accountId = tx.account_id;
				toAccountId = tx.to_account_id || 0;
				txDate = (tx.tx_date || '').slice(0, 10);
				merchant = tx.merchant || '';
				location = tx.location || '';
				remark = tx.remark || '';
				images = tx.images || [];
				tagIds = (tx.tags || []).map((t: any) => t.id);
				includeInBudget = tx.include_in_budget !== false;
				// B11：回填币种与汇率
				currency = tx.currency || baseCurrency;
				exchangeRate = (tx as any).exchange_rate ? String((tx as any).exchange_rate) : '';
			})
			.catch(() => {
				hzToast.error('加载交易失败');
				goto('/transactions');
			});
	});

	// 复制模式：从 clone prop 预填数据，时间使用当前时间
	$effect(() => {
		if (!clone) return;
		type = clone.type as any;
		// clone.amount 来自列表接口，已是「元」（Money 序列化结果），勿再 /100
		amount = String(clone.amount ?? 0);
		description = clone.description || '';
		categoryId = clone.category_id;
		accountId = clone.account_id;
		toAccountId = clone.to_account_id || 0;
		txDate = formatDate(new Date(), 'YYYY-MM-DD');
		merchant = clone.merchant || '';
		location = clone.location || '';
		remark = clone.remark || '';
		images = clone.images || [];
		tagIds = (clone.tags || []).map((t: any) => t.id);
		includeInBudget = clone.include_in_budget !== false;
		currency = clone.currency || baseCurrency;
		exchangeRate = clone.exchange_rate ? String(clone.exchange_rate) : '';
	});

	function resetForNext() {
		amount = '';
		description = '';
		remark = '';
		merchant = '';
		location = '';
		images = [];
		tagIds = [];
		// 保留日期、账户、分类、类型，方便连续记账（B10）
	}

	async function handleSave() {
		const amt = parseFloat(amount);
		if (!amt || amt <= 0) {
			hzToast.warning('请输入有效金额');
			return;
		}
		if (!accountId) {
			hzToast.warning('请选择账户');
			return;
		}
		if (type !== 'transfer' && !categoryId) {
			hzToast.warning('请选择分类');
			return;
		}
		if (type === 'transfer' && (!toAccountId || accountId === toAccountId)) {
			hzToast.warning('请选择不同的转入账户');
			return;
		}

		loading = true;
		try {
			const data: any = {
				type,
				amount: amt,
				category_id: categoryId,
				account_id: accountId,
				tx_date: txDate,
				description: description || '',
				book_id: appStore.effectiveBookId(),
				// C3：此前前端从不传该字段 → 后端按 false 处理 → 预算进度恒为 0
				include_in_budget: includeInBudget,
				include_in_balance: true
			};
			if (type === 'transfer') data.to_account_id = toAccountId;
			// B11：币种与汇率
			if (currency && currency !== baseCurrency) {
				data.currency = currency;
				const rate = parseFloat(exchangeRate);
				if (rate > 0) data.exchange_rate = rate;
			} else {
				data.currency = baseCurrency;
			}
			if (merchant) data.merchant = merchant;
			if (location) data.location = location;
			if (remark) data.remark = remark;
			if (tagIds.length) data.tag_ids = tagIds;
			if (images.length) data.images = images;

			if (isEdit && id) {
				await txApi.update(Number(id), data);
				hzToast.success('更新成功');
				goto('/transactions');
			} else {
				await txApi.create(data);
				hzToast.success('记账成功');
				if (keepGoing) {
					resetForNext();
				} else {
					goto('/transactions');
				}
			}
		} catch (e: any) {
			hzToast.error(e.message || '保存失败');
		} finally {
			loading = false;
		}
	}

	async function handleAI() {
		if (!description.trim()) {
			hzToast.warning('请先输入描述');
			return;
		}
		aiLoading = true;
		try {
			const res = await aiApi.classify({ description, book_id: appStore.effectiveBookId() });
			if (res.category_id) categoryId = res.category_id;
			if (res.type) type = res.type as any;
			if (res.confidence > 0.7) hzToast.success(`AI 推荐: ${res.category}`);
			else hzToast.info(`AI 推荐: ${res.category}（置信度较低）`);
		} catch {
			hzToast.error('AI 识别失败');
		} finally {
			aiLoading = false;
		}
	}

	// A6：粘贴文本 → 一次性解析出金额/分类/日期/商户
	async function handleSmartRecord() {
		if (!smartText.trim()) {
			hzToast.warning('请粘贴一段消费描述');
			return;
		}
		smartLoading = true;
		try {
			const res = await aiApi.smartRecord({ text: smartText, book_id: appStore.effectiveBookId() });
			if (res.amount) amount = String(res.amount);
			if (res.description) description = res.description;
			if (res.category_id) categoryId = res.category_id;
			if (res.account_id) accountId = res.account_id;
			if (res.tx_date) txDate = String(res.tx_date).slice(0, 10);
			if (res.type) type = res.type as any;
			showSmartInput = false;
			hzToast.success('已识别，请确认后保存');
		} catch {
			hzToast.error('智能记账识别失败');
		} finally {
			smartLoading = false;
		}
	}

	async function handleUpload(e: Event) {
		const input = e.target as HTMLInputElement;
		const files = input.files;
		if (!files?.length) return;
		uploading = true;
		try {
			for (const f of Array.from(files)) {
				const { url } = await uploadApi.image(f);
				images = [...images, url];
			}
		} catch (err: any) {
			hzToast.error(err.message || '上传失败');
		} finally {
			uploading = false;
			input.value = '';
		}
	}

	function toggleTag(tid: number) {
		tagIds = tagIds.includes(tid) ? tagIds.filter((t) => t !== tid) : [...tagIds, tid];
	}
</script>

<div class="max-w-xl mx-auto">
	<Card>
		<CardHeader>
			<CardTitle>{isEdit ? '编辑交易' : '记一笔'}</CardTitle>
		</CardHeader>
		<CardContent class="space-y-5">
			<!-- A6：智能记账入口 -->
			{#if !isEdit}
				<div class="rounded-lg border border-dashed p-3">
					<button
						type="button"
						class="flex items-center gap-2 text-sm text-muted-foreground hover:text-foreground w-full"
						onclick={() => (showSmartInput = !showSmartInput)}
					>
						<Wand2 size={14} />
						粘贴文本自动识别（如「今天午饭 32 元」）
						{#if showSmartInput}<ChevronUp size={14} />{:else}<ChevronDown size={14} />{/if}
					</button>
					{#if showSmartInput}
						<div class="mt-3 flex gap-2">
							<Input bind:value={smartText} placeholder="粘贴一段消费记录…" />
							<Button variant="outline" onclick={handleSmartRecord} disabled={smartLoading}>
								{smartLoading ? '识别中…' : '识别'}
							</Button>
						</div>
					{/if}
				</div>
			{/if}

			<!-- 类型切换 -->
			<Tabs bind:value={type}>
				<TabsTrigger value="expense">支出</TabsTrigger>
				<TabsTrigger value="income">收入</TabsTrigger>
				<TabsTrigger value="transfer">转账</TabsTrigger>
			</Tabs>

			<!-- 金额 -->
			<div class="space-y-2">
				<Label>金额</Label>
				<div class="relative">
					<span class="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground">¥</span>
					<Input
						class="text-xl h-12 pl-8 font-semibold tabular-nums"
						type="number"
						step="0.01"
						placeholder="0.00"
						bind:value={amount}
					/>
				</div>
			</div>

			{#if type !== 'transfer'}
				<div class="space-y-2">
					<Label>分类</Label>
					<CategoryPicker bind:value={categoryId} kind={type === 'income' ? 'income' : 'expense'} />
				</div>
			{/if}

			<div class="space-y-2">
				<Label>{type === 'transfer' ? '从账户' : '账户'}</Label>
				<AccountSelect bind:value={accountId} placeholder="选择账户" />
			</div>

			{#if type === 'transfer'}
				<div class="space-y-2">
					<Label>到账户</Label>
					<AccountSelect bind:value={toAccountId} exclude={accountId} placeholder="选择对方账户" />
				</div>
			{/if}

			<div class="space-y-2">
				<Label>日期</Label>
				<Input type="date" bind:value={txDate} />
			</div>

			<div class="space-y-2">
				<Label>描述</Label>
				<div class="flex gap-2">
					<Input class="flex-1" placeholder="例如: 午餐、打车..." bind:value={description} />
					<Button type="button" variant="outline" onclick={handleAI} disabled={aiLoading}>
						{#if aiLoading}...{:else}<Sparkles size={16} />AI{/if}
					</Button>
				</div>
			</div>

			<!-- A5：更多字段 -->
			<button
				type="button"
				class="flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
				onclick={() => (showMore = !showMore)}
			>
				{#if showMore}<ChevronUp size={14} />{:else}<ChevronDown size={14} />{/if}
				更多字段（商户 / 标签 / 凭证 / 地点 / 备注）
			</button>

			{#if showMore}
				<div class="space-y-4 rounded-lg border p-3">
					<div class="grid grid-cols-2 gap-3">
						<div class="space-y-2">
							<Label>商户</Label>
							<Input bind:value={merchant} placeholder="如 星巴克" />
						</div>
						<div class="space-y-2">
							<Label>地点</Label>
							<Input bind:value={location} placeholder="如 上海·静安" />
						</div>
					</div>

					<!-- B11：多币种与原币金额折算 -->
					<div class="grid grid-cols-2 gap-3">
						<div class="space-y-2">
							<Label>币种</Label>
							<select
								class="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm focus:outline-none focus:ring-1 focus:ring-ring"
								bind:value={currency}
							>
								<option value="CNY">CNY 人民币</option>
								<option value="USD">USD 美元</option>
								<option value="EUR">EUR 欧元</option>
								<option value="HKD">HKD 港元</option>
								<option value="JPY">JPY 日元</option>
								<option value="GBP">GBP 英镑</option>
								<option value="SGD">SGD 新元</option>
							</select>
						</div>
						<div class="space-y-2">
							<Label>汇率（1 {currency} = ? {baseCurrency}）</Label>
							<Input
								type="number"
								step="0.0001"
								placeholder={isForeign ? '如 7.1800' : '仅外币需要填写'}
								bind:value={exchangeRate}
								disabled={!isForeign}
							/>
						</div>
					</div>
					{#if isForeign}
						<p class="text-[11px] text-muted-foreground">
							金额按所选币种录入，统计时按汇率折算为 {baseCurrency}
						</p>
					{/if}

					<div class="space-y-2">
						<Label>标签</Label>
						{#if appStore.tags.length === 0}
							<p class="text-xs text-muted-foreground">还没有标签，可在「标签中心」创建</p>
						{:else}
						<div class="flex flex-wrap gap-2">
							{#each appStore.tags as tag (tag.id)}
								<Badge
									variant={tagIds.includes(tag.id) ? 'default' : 'outline'}
									class="cursor-pointer select-none"
									role="button"
									tabindex={0}
									onclick={() => toggleTag(tag.id)}
									onkeydown={(e: KeyboardEvent) => {
										if (e.key === 'Enter' || e.key === ' ') {
											e.preventDefault();
											toggleTag(tag.id);
										}
									}}
								>
									{tag.name}
								</Badge>
							{/each}
						</div>
						{/if}
					</div>

					<div class="space-y-2">
						<Label>凭证图片</Label>
						<div class="flex flex-wrap items-center gap-2">
							{#each images as url, i (url)}
								<div class="relative">
									<img src={url} alt="凭证" class="w-16 h-16 object-cover rounded border" />
									<button
										type="button"
										class="absolute -top-2 -right-2 bg-background rounded-full p-0.5 border"
										onclick={() => (images = images.filter((_, idx) => idx !== i))}
									>
										<Trash2 size={12} class="text-destructive" />
									</button>
								</div>
							{/each}
							<label
								class="w-16 h-16 grid place-items-center rounded border border-dashed cursor-pointer hover:bg-accent"
							>
								<ImagePlus size={18} class="text-muted-foreground" />
								<input
									type="file"
									accept="image/*"
									multiple
									class="hidden"
									onchange={handleUpload}
								/>
							</label>
							{#if uploading}<span class="text-xs text-muted-foreground">上传中…</span>{/if}
						</div>
					</div>

					<div class="space-y-2">
						<Label>备注</Label>
						<Input bind:value={remark} placeholder="选填" />
					</div>

					<label class="flex items-center gap-2 text-sm">
						<input type="checkbox" bind:checked={includeInBudget} class="rounded border-input" />
						计入预算（取消勾选则该笔不计入预算已用金额）
					</label>
				</div>
			{/if}

			<!-- 保存 -->
			<div class="flex gap-2 pt-2">
				<Button class="flex-1" onclick={handleSave} disabled={loading}>
					<Save size={16} />
					{isEdit ? '保存修改' : '保存'}
				</Button>
				<Button variant="outline" onclick={() => goto('/transactions')}>
					<X size={16} />
					取消
				</Button>
			</div>

			{#if !isEdit}
				<label class="flex items-center gap-2 text-sm text-muted-foreground">
					<input type="checkbox" bind:checked={keepGoing} class="rounded border-input" />
					连续记账（保存后停留在本页，保留账户与分类）
				</label>
			{/if}
		</CardContent>
	</Card>
</div>
