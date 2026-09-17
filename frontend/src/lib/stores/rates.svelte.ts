/**
 * 全局汇率状态（Svelte 5 runes）。
 *
 * 之所以单独成 store 而不是各页面各拉一次：
 *  1. 记账表单、账单列表、设置页都需要「当前汇率」，各自请求会打三遍上游；
 *  2. 基准货币切换后需要一处统一失效，否则列表还在用旧基准币的汇率折算；
 *  3. 刷新节流必须集中，否则连续记账会触发上游限流。
 */
import { fxApi } from '$lib/api/modules/exrate';
import type { FxSnapshot } from '$lib/types';

let base = $state('CNY');
let rates = $state<Record<string, number>>({});
let currencies = $state<string[]>([]);
let source = $state('');
let fetchedAt = $state<string | null>(null);
let stale = $state(false);
let error = $state('');
let enabled = $state(true);
let autoRefresh = $state(true);
let refreshHours = $state(12);
let loading = $state(false);
let refreshing = $state(false);

/** 已加载过的基准币：切换基准币时才重新拉，同一基准币重复进入页面不再请求 */
let loadedBase = $state('');
/** 前端侧节流：与后端 10 秒节流叠加，防止列表/表单同时触发刷新 */
let lastRefreshAt = 0;
const MIN_REFRESH_GAP_MS = 10_000;

function apply(snap: FxSnapshot) {
	base = snap.base || base;
	rates = snap.rates ?? {};
	currencies = snap.currencies ?? Object.keys(rates);
	source = snap.source ?? '';
	fetchedAt = snap.fetched_at ?? null;
	stale = !!snap.stale;
	error = snap.error ?? '';
	enabled = snap.enabled !== false;
	autoRefresh = snap.auto_refresh !== false;
	refreshHours = snap.refresh_hours ?? 12;
	loadedBase = snap.base || loadedBase;
}

export const ratesStore = {
	get base() {
		return base;
	},
	get rates() {
		return rates;
	},
	get currencies() {
		return currencies;
	},
	get source() {
		return source;
	},
	get fetchedAt() {
		return fetchedAt;
	},
	get stale() {
		return stale;
	},
	get error() {
		return error;
	},
	get enabled() {
		return enabled;
	},
	get autoRefresh() {
		return autoRefresh;
	},
	get refreshHours() {
		return refreshHours;
	},
	get loading() {
		return loading;
	},
	get refreshing() {
		return refreshing;
	},

	/**
	 * 取「1 单位 currency = ? 单位基准币」的汇率。
	 * 拿不到时返回 null —— 调用方据此区分「汇率为 1」与「没有汇率数据」，
	 * 与后端 exrate.Get 的 (rate, ok) 语义保持一致。
	 */
	rate(currency?: string): number | null {
		const cur = (currency || '').toUpperCase();
		if (!cur || cur === base) return 1;
		const r = rates[cur];
		return typeof r === 'number' && isFinite(r) && r > 0 ? r : null;
	},

	/** 折算到基准币；没有汇率时返回 null（表示无法折算，而不是返回原值冒充折算结果） */
	convert(amount: number, currency?: string): number | null {
		const r = this.rate(currency);
		if (r == null || !isFinite(amount)) return null;
		return amount * r;
	},

	/**
	 * 确保已加载指定基准币的汇率。
	 * 幂等：同一基准币且未过期时不会重复请求；数据过期且开启了自动刷新则顺带刷新一次。
	 */
	async ensure(wantBase?: string, opts: { force?: boolean } = {}): Promise<void> {
		const b = (wantBase || base || 'CNY').toUpperCase();
		if (!opts.force && loadedBase === b && Object.keys(rates).length > 0 && !stale) return;
		await this.load(b, opts.force);
	},

	async load(wantBase?: string, force = false): Promise<void> {
		const b = (wantBase || base || 'CNY').toUpperCase();
		if (loading) return;
		loading = true;
		try {
			const snap = await fxApi.list(b);
			apply(snap);
			// 数据过期且用户允许自动刷新 → 后台补一次真实的拉取。
			// 失败不影响本次结果：库里那份历史汇率仍然可用（后端已标记 stale）。
			if (force || (snap.stale && snap.enabled && snap.auto_refresh)) {
				void this.refresh(b, true);
			}
		} catch (e: any) {
			error = e?.message || '汇率加载失败';
		} finally {
			loading = false;
		}
	},

	async refresh(wantBase?: string, silent = false): Promise<boolean> {
		const b = (wantBase || base || 'CNY').toUpperCase();
		const now = Date.now();
		if (now - lastRefreshAt < MIN_REFRESH_GAP_MS) return false;
		lastRefreshAt = now;
		if (!silent) refreshing = true;
		try {
			const snap = await fxApi.refresh(b);
			apply(snap);
			return !snap.error;
		} catch (e: any) {
			error = e?.message || '汇率刷新失败';
			return false;
		} finally {
			if (!silent) refreshing = false;
		}
	},

	/** 基准货币变更后立刻让缓存失效（下次 ensure 会重新拉取） */
	invalidate() {
		loadedBase = '';
		lastRefreshAt = 0;
	}
};
