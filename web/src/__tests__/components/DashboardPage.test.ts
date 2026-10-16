/**
 * 首页最近交易列表 - {@const cat} 提取后的渲染
 *
 * 修复点：模板里一行连写三次 `categoryOf(tx)`，其中两次直接 `.color` 访问
 * 被 TS 判「Object is possibly 'undefined'」。现在先用 `{@const cat = categoryOf(tx)}`
 * 查一次，再在 style / 图标 / 副标题里复用 cat。
 *
 * 断言的是这条 {#each} 真实渲染出来的结果：
 *  - 命中分类：图标底色 CSS 用分类色（background:<color>22;color:<color>），
 *    副标题为「分类 · 账户 · 日期」；
 *  - 未命中分类（category_id 不存在）：不写 style，副标题显示「未分类 · — · 日期」
 *    —— 这正是旧代码会抛 undefined.color 的那条分支。
 */
import { describe, it, expect, vi, beforeAll, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/svelte';

const CATEGORY = { id: 1, name: '餐饮', icon: '🍔', color: '#10B981', kind: 'expense' };
const TXS = [
	{ id: 1, type: 'expense', amount: 5000, category_id: 1, account_id: 9, tx_date: '2026-09-16', description: '午餐' },
	{ id: 2, type: 'expense', amount: 8800, category_id: 404, account_id: 404, tx_date: '2026-09-15', merchant: '便利店' }
];

const listFn = vi.fn(async () => ({
	grouped: [{ date: '2026-09-16', transactions: TXS }],
	summary: { total_income: 0, total_expense: 13800, net: -13800 },
	pagination: { total: 2 }
}));

vi.mock('#lib/api/modules', () => ({
	accountApi: { creditSummary: vi.fn(async () => []) },
	txApi: { list: listFn },
	statsApi: {
		assets: vi.fn(async () => ({ total: 0, accounts: [] })),
		monthly: vi.fn(async () => ({}))
	}
}));

vi.mock('#lib/stores/app', () => ({
	appStore: {
		get categories() {
			return { expense: [CATEGORY], income: [], system: [] };
		},
		get accounts() {
			return [{ id: 9, name: '支付宝', icon: '💳' }];
		},
		currentBookId: 1,
		user: { id: 1, username: 'tester' }
	}
}));

let Page: any;

beforeAll(async () => {
	Page = (await import('../../routes/(auth)/dashboard/+page.svelte')).default;
}, 120000);

beforeEach(() => {
	listFn.mockClear();
});

describe('首页 - 最近交易的分类渲染', () => {
	it('拉到最近交易并渲染描述', async () => {
		render(Page);
		await waitFor(() => expect(screen.getByText('午餐')).toBeTruthy());
		expect(listFn).toHaveBeenCalled();
	});

	it('命中分类时图标底色用分类色（{@const cat} 的 style 分支）', async () => {
		render(Page);
		await waitFor(() => expect(screen.getByText('午餐')).toBeTruthy());

		// jsdom 会把 #10B98122 归一化成 rgba()，这里按归一化后的值断言
		const icon = screen.getByText('🍔').closest('div')!;
		const style = icon.getAttribute('style') || '';
		expect(style).toMatch(/background: rgba\(16, 185, 129/);
		expect(style).toMatch(/color: rgb\(16, 185, 129\)/);

		// 副标题 = 分类 · 账户 · 日期（文本被插值拆成多个节点，按 textContent 断言）
		expect(icon.parentElement?.textContent).toContain('餐饮');
		expect(icon.parentElement?.textContent).toContain('支付宝');
	});

	it('查不到分类时不写 style，副标题退化为「未分类 · — 」（undefined 分支）', async () => {
		render(Page);
		await waitFor(() => expect(screen.getByText('便利店')).toBeTruthy());

		const row = screen.getByText('便利店').closest('button')!;
		expect(row.textContent).toContain('未分类 · —');
		// 旧代码在这里会读到 undefined.color：该行分类格不应带 color 声明
		const cell = row.querySelector('div[class*="bg-muted"]')!;
		expect(cell.getAttribute('style') || '').toBe('');
	});

	it('金额按收入/支出符号展示', async () => {
		render(Page);
		await waitFor(() => expect(screen.getByText('午餐')).toBeTruthy());

		const row = screen.getByText('午餐').closest('button')!;
		expect(row.textContent).toContain('5,000.00');
		expect(row.textContent).toContain('-');
	});
});
