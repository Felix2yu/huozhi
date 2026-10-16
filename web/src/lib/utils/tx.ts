/**
 * 交易展示口径的单一事实来源。
 *
 * 修复背景（原 F-02 / F-04 / F-05）：
 *  1. 列表行、日小计、详情弹窗三处各写一份「金额符号 / 颜色」判断，
 *     口径互相矛盾 —— 同一屏出现「今日 +500」但明细加总是 -320；
 *  2. 日小计由前端把服务端小计直接相加，跨页重复条目会被重复计入；
 *  3. 头像字符用 `str[0]`（UTF-16 code unit），emoji 开头的描述渲染成乱码。
 *
 * 这里的所有规则与后端 models.TxStatsBucket / Transaction.AmountInBase() 一一对应：
 *  - income / refund  → 收入（+）
 *  - expense / reimburse → 支出（-）
 *  - transfer / adjust → 不计入收支（无符号、中性色）
 */

export interface Txn {
	id: number;
	type: string;
	amount: number;
	exchange_rate?: number;
	amount_base?: number;
	include_in_balance?: boolean;
}

/** 该类型归入哪个收支统计口径；空串 = 不计入收支 */
export function statsBucket(type: string): 'income' | 'expense' | '' {
	switch (type) {
		case 'income':
		case 'refund':
			return 'income';
		case 'expense':
		case 'reimburse':
			return 'expense';
		default:
			return '';
	}
}

/** 折算到基准币种的金额（优先用服务端算好的 amount_base） */
export function baseAmount(tx: Txn): number {
	const amount = Number(tx.amount) || 0;
	if (typeof tx.amount_base === 'number' && isFinite(tx.amount_base)) {
		return tx.amount_base;
	}
	const rate = Number(tx.exchange_rate);
	if (isFinite(rate) && rate > 0 && Math.abs(rate - 1) > 1e-9) {
		return amount * rate;
	}
	return amount;
}

/**
 * 金额展示：符号 + 绝对值 + 色调。
 * 列表行、详情弹窗、日小计全部共用此函数，从根上消除三处口径分叉。
 */
export function amountDisplay(tx: Txn): {
	sign: '' | '+' | '-';
	abs: number;
	tone: 'income' | 'expense' | 'muted';
} {
	const abs = Math.abs(baseAmount(tx));
	switch (statsBucket(tx.type)) {
		case 'income':
			return { sign: '+', abs, tone: 'income' };
		case 'expense':
			return { sign: '-', abs, tone: 'expense' };
		default:
			return { sign: '', abs, tone: 'muted' };
	}
}

export function toneClass(tone: 'income' | 'expense' | 'muted'): string {
	if (tone === 'income') return 'text-[var(--color-income)]';
	if (tone === 'expense') return 'text-[var(--color-expense)]';
	return 'text-muted-foreground';
}

/** 类型中文名 */
export function typeLabel(t: string): string {
	switch (t) {
		case 'income':
			return '收入';
		case 'expense':
			return '支出';
		case 'transfer':
			return '转账';
		case 'refund':
			return '退款';
		case 'reimburse':
			return '报销';
		case 'adjust':
			return '余额调整';
		default:
			return t;
	}
}

/** 按日重算小计 —— 由去重后的条目算出，绝不累加服务端小计 */
export function recomputeDaySubtotal(
	day: { day_income: number; day_expense: number; day_balance: number; transactions: Txn[] }
): void {
	let income = 0;
	let expense = 0;
	for (const t of day.transactions ?? []) {
		if (t.include_in_balance === false) continue;
		const amt = baseAmount(t);
		switch (statsBucket(t.type)) {
			case 'income':
				income += amt;
				break;
			case 'expense':
				expense += amt;
				break;
		}
	}
	// 分→元：后端 Money 序列化为「元」，但累加可能产生浮点尾差，收敛到 2 位
	day.day_income = round2(income);
	day.day_expense = round2(expense);
	day.day_balance = round2(day.day_income - day.day_expense);
}

export function round2(n: number): number {
	return Math.round((Number(n) || 0) * 100) / 100;
}

/**
 * 关键词高亮：返回「文本片段 + 是否命中」的序列，交给模板条件渲染 <mark>。
 *
 * 这是替代 `{@html}` 的零风险方案 —— 文本始终以文本节点渲染，
 * 不存在 HTML 注入面；也不需要正则，`$&` / `$'` 这类替换模式元字符
 * 不会被展开（原 F-01 的两个漏洞同时消除）。
 */
export function highlightSegments(
	text: string,
	keyword: string
): Array<{ text: string; hit: boolean }> {
	const src = text ?? '';
	const k = (keyword ?? '').trim();
	if (!src) return [];
	if (!k) return [{ text: src, hit: false }];
	const hay = src.toLowerCase();
	const needle = k.toLowerCase();
	const out: Array<{ text: string; hit: boolean }> = [];
	let i = 0;
	for (;;) {
		const idx = hay.indexOf(needle, i);
		if (idx === -1) {
			if (i < src.length) out.push({ text: src.slice(i), hit: false });
			break;
		}
		if (idx > i) out.push({ text: src.slice(i, idx), hit: false });
		out.push({ text: src.slice(idx, idx + needle.length), hit: true });
		i = idx + needle.length;
	}
	return out;
}

/**
 * 取首个字素（用户可感知的字符）。
 * `str[0]` 按 UTF-16 code unit 取值，emoji / 部分生僻字会被截成孤立代理项，
 * 渲染为乱码方块（原 F-05）。优先用 Intl.Segmenter 处理字素簇，
 * 不支持时退化为 Array.from（码点级）。
 */
export function firstGrapheme(s: string, fallback = '¥'): string {
	const t = (s ?? '').trim();
	if (!t) return fallback;
	try {
		const Segmenter = (Intl as any)?.Segmenter;
		if (Segmenter) {
			const it = new Segmenter('zh', { granularity: 'grapheme' }).segment(t);
			for (const seg of it) return seg.segment;
		}
	} catch {
		// 环境不支持时走兜底
	}
	return Array.from(t)[0] ?? fallback;
}

/** 把用户输入截断到两位小数（原 F-11：后端会静默四舍五入，无提示） */
export function clampMoneyInput(v: string): string {
	if (v === '') return v;
	const m = /^-?\d*(\.\d*)?/.exec(v.replace(/[^\d.-]/g, ''));
	if (!m) return '';
	let s = m[0];
	const dot = s.indexOf('.');
	if (dot >= 0) {
		s = s.slice(0, dot + 3);
	}
	return s;
}
