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
	import { formatDate, formatMoney, currencySymbol } from '$lib/utils/format';
	import { clampMoneyInput } from '$lib/utils/tx';
	import { ratesStore } from '$lib/stores/rates.svelte';
	import { CURRENCIES } from '$lib/types';
	import { onMount } from 'svelte';
	import {
		Sparkles, Save, X, ChevronDown, ChevronUp, ImagePlus, Trash2, Wand2, RefreshCw
	} from '@lucide/svelte';

	interface Props {
		id?: string | number | null;
		clone?: any | null;
	}
	let { id = null, clone = null }: Props = $props();

	const isEdit = $derived(!!id);

	let type = $state<'expense' | 'income' | 'transfer'>('expense');
	/**
	 * 编辑/复制模式下，原交易数据是否已回填完成。
	 * 回填完成前不跑「分类一致性校正」，否则会把原分类当成「与类型不匹配」
	 * 而替换成默认分类。
	 */
	let prefilled = $state(false);
	/** 后端还支持 refund / reimburse / adjust，它们没有对应的 Tab，编辑时保持原类型 */
	const HIDDEN_TYPE_LABEL: Record<string, string> = {
		refund: '退款',
		reimburse: '报销',
		adjust: '余额调整'
	};
	const hiddenTypeLabel = $derived(HIDDEN_TYPE_LABEL[type] || '');

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
	// 用户手动改过币种后就不再被「账户默认币种」覆盖（原 F-06）
	let currencyTouched = $state(false);
	// 用户手动改过汇率后，不再被自动汇率覆盖（编辑历史账单时必须保留当时的汇率）
	let rateManual = $state(false);
	let includeInBalance = $state(true);
	// 分类按收支种类分别记忆上次选择：切 tab 再切回来时保留用户原选，
	// 而不是被默认值静默重置（原 F-06）
	let lastCatByKind = $state<Record<string, number>>({ expense: 0, income: 0 });

	const baseCurrency = $derived((appStore.user as any)?.currency || 'CNY');
	// 账户币种优先，其次用户基准币种
	const effectiveCurrency = $derived(
		appStore.accounts.find((a) => a.id === accountId)?.currency || baseCurrency
	);
	const isForeign = $derived(currency !== baseCurrency);

	// ===== 汇率折算 =====
	// 录入外币账单时自动带出当前汇率，并实时给出折算后的基准币金额，
	// 让用户不必自己心算、也不必先去别处查汇率。
	const amountNum = $derived(parseFloat(amount) || 0);
	const rateNum = $derived(parseFloat(exchangeRate) || 0);
	/** 折算后的基准币金额；无法折算时为 null（不拿原值冒充） */
	const converted = $derived(
		isForeign && amountNum > 0 && rateNum > 0 ? Math.round(amountNum * rateNum * 100) / 100 : null
	);
	/** 汇率来源与新鲜度，供录入界面提示 */
	const fxMeta = $derived.by(() => {
		if (!ratesStore.fetchedAt) return '';
		const d = new Date(ratesStore.fetchedAt);
		const stamp = `${d.getMonth() + 1}-${String(d.getDate()).padStart(2, '0')} ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`;
		return `${ratesStore.source || '上游'} · ${stamp}`;
	});

	// 外币且用户没手填汇率 → 用当前汇率自动填充；切币种时重新带出
	$effect(() => {
		if (!isForeign) {
			if (!rateManual) exchangeRate = '';
			return;
		}
		if (rateManual) return;
		const r = ratesStore.rate(currency);
		exchangeRate = r && r !== 1 ? String(r) : '';
	});

	function onCurrencyChange(next: string) {
		currency = next;
		currencyTouched = true;
		// 主动切换币种 = 要求按新币种折算，账单上锁定的旧汇率已无意义，
		// 因此解除 rateManual 并重新带出当前汇率（否则编辑一笔本币账单后
		// 改成外币会留着 1 不动，折算金额直接错掉）。
		rateManual = false;
		const r = ratesStore.rate(next);
		exchangeRate = r && r !== 1 ? String(r) : '';
		if (next !== baseCurrency) ratesStore.ensure(baseCurrency);
	}

	onMount(() => {
		// 进入表单就确保有汇率可用（后端有缓存，只有首次 / 过期才真打上游）
		ratesStore.ensure(baseCurrency);
	});

	let categories = $derived.by(() => {
		if (type === 'income') return appStore.categories.income;
		return appStore.categories.expense;
	});

	// 默认值初始化（新增模式）
	$effect(() => {
		if (isEdit && !prefilled) return;
		// 账户默认值只在新增模式生效：编辑/复制时原账户优先
		if (!isEdit) {
			if (!accountId && appStore.accounts.length > 0) {
				accountId = appStore.accounts[0].id;
				if (appStore.accounts.length > 1 && !toAccountId) {
					toAccountId = appStore.accounts[1].id;
				}
			}
		}
		// 仅当当前分类对「当前收支种类」无效时才重新选择：
		//  - 优先恢复该种类上次的选择（误触 tab 再切回不会丢）
		//  - 没有记忆才回落到第一个叶子分类
		// 这样既不会静默覆盖用户已选，也不会留下与类型不匹配的分类 id。
		const kind = type === 'income' ? 'income' : 'expense';
		// 转账没有分类，不参与校正（保持原值，切回转出再切回来也不会丢）
		if (type !== 'transfer' && !categories.some((c) => c.id === categoryId)) {
			const remembered = lastCatByKind[kind];
			if (remembered && categories.some((c) => c.id === remembered)) {
				categoryId = remembered;
			} else {
				const firstLeaf = categories.find((c) => c.parent_id) || categories[0];
				if (firstLeaf) {
					categoryId = firstLeaf.id;
					lastCatByKind[kind] = firstLeaf.id;
				}
			}
		}
		// B11：币种跟随所选账户，用户手动改过之后不再覆盖。
		// 编辑/复制时原交易自带的币种优先，不被账户默认值冲掉。
		if (!currencyTouched && !prefilled) currency = effectiveCurrency;
	});

	function onCategoryChange(id: number) {
		categoryId = id;
		lastCatByKind[type === 'income' ? 'income' : 'expense'] = id;
	}

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
				includeInBalance = tx.include_in_balance !== false;
				// B11：回填币种与汇率。历史账单带的是「当时」的汇率，
				// 标记 rateManual，避免被当前汇率静默覆盖导致账面金额变化。
				currency = tx.currency || baseCurrency;
				exchangeRate = (tx as any).exchange_rate ? String((tx as any).exchange_rate) : '';
				rateManual = !!(tx as any).exchange_rate;
				prefilled = true;
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
		rateManual = !!clone.exchange_rate;
		prefilled = true;
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
				// 编辑时尊重原值，不再无条件置 true
				include_in_balance: includeInBalance
			};
			if (type === 'transfer') data.to_account_id = toAccountId;
			// 币种与汇率：未拿到汇率时后端会按当前汇率兜底，这里带上已解析的值即可
			data.currency = currency || baseCurrency;
			const rate = parseFloat(exchangeRate);
			if (rate > 0) data.exchange_rate = rate;
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

	/**
	 * F-11：金额输入即时截断到两位小数。
	 * step="0.01" 只约束步进器，不阻止手输 12.349；后端 FromYuan 会静默
	 * 四舍五入成 12.35 且无任何提示。这里在输入时就截断，所见即所存。
	 */
	function onAmountInput(e: Event) {
		const el = e.currentTarget as HTMLInputElement;
		const clamped = clampMoneyInput(el.value);
		if (clamped !== el.value) el.value = clamped;
		amount = clamped;
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
			{#if hiddenTypeLabel}
				<p class="-mt-3 text-xs text-muted-foreground">
					该交易原类型为「{hiddenTypeLabel}」，此处无可切换的标签，直接保存会保持原类型。
				</p>
			{/if}

			<!-- 金额 + 币种/汇率折算 -->
			<div class="space-y-2">
				<Label>金额</Label>
				<div class="relative">
					<span class="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground"
						>{currencySymbol(currency)}</span
					>
					<Input
						class="text-xl h-12 pl-8 font-semibold tabular-nums"
						type="number"
						step="0.01"
						placeholder="0.00"
						bind:value={amount}
						oninput={onAmountInput}
					/>
				</div>
			</div>

			<div class="grid grid-cols-2 gap-3">
				<div class="space-y-2">
					<Label>币种</Label>
					<select
						class="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm focus:outline-none focus:ring-1 focus:ring-ring"
						value={currency}
						onchange={(e) => onCurrencyChange((e.currentTarget as HTMLSelectElement).value)}
					>
						{#each CURRENCIES as c}
							<option value={c.code}>{c.code} {c.label}</option>
						{/each}
					</select>
				</div>
				<div class="space-y-2">
					<Label>汇率（1 {currency} = ? {baseCurrency}）</Label>
					<div class="flex gap-1">
						<Input
							type="number"
							step="0.0001"
							placeholder={isForeign ? '自动获取' : '基准货币'}
							bind:value={exchangeRate}
							disabled={!isForeign}
							oninput={() => (rateManual = true)}
						/>
						{#if isForeign}
							<Button
								size="icon"
								variant="outline"
								title="刷新汇率"
								disabled={ratesStore.refreshing}
								onclick={() => ratesStore.refresh(baseCurrency)}
							>
								<RefreshCw size={14} class={ratesStore.refreshing ? 'animate-spin' : ''} />
							</Button>
						{/if}
					</div>
				</div>
			</div>

			{#if isForeign}
				<div class="rounded-lg border bg-muted/40 px-3 py-2 text-sm space-y-1">
					{#if rateNum > 0}
						<div class="tabular-nums">
							<span class="text-muted-foreground">当前汇率</span>
							1 {currency} = {rateNum} {baseCurrency}
						</div>
						<div class="font-medium tabular-nums">
							{currencySymbol(currency)}{amountNum.toFixed(2)} ≈
							{currencySymbol(baseCurrency)}{converted?.toFixed(2) ?? '0.00'}
						</div>
					{:else}
						<div class="text-muted-foreground">
							未获取到 {currency} → {baseCurrency} 的汇率，可手动填写；留空则按 1:1 记录。
						</div>
					{/if}
					{#if fxMeta}
						<div class="text-[11px] text-muted-foreground">
							{fxMeta}
							{#if ratesStore.stale}<span class="text-amber-600 dark:text-amber-400">· 已过期</span>{/if}
						</div>
					{/if}
				</div>
			{/if}

			{#if type !== 'transfer'}
				<div class="space-y-2">
					<Label>分类</Label>
					<CategoryPicker
					bind:value={categoryId}
					kind={type === 'income' ? 'income' : 'expense'}
					onchange={onCategoryChange}
				/>
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
