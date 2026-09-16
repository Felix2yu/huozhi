import { describe, it, expect, vi, beforeAll, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/svelte';

const MOCK_TXS = [
	{
		id: 1,
		type: 'expense',
		amount: 5000,
		category_id: 1,
		account_id: 1,
		tx_date: '2026-09-16',
		description: '午餐',
		merchant: '食堂',
		remark: '周一午餐',
		tags: [{ id: 1, name: '工作日' }]
	},
	{
		id: 2,
		type: 'income',
		amount: 100000,
		category_id: 2,
		account_id: 1,
		tx_date: '2026-09-16',
		description: '工资',
		images: ['https://example.com/receipt.jpg']
	},
	{
		id: 3,
		type: 'transfer',
		amount: 20000,
		account_id: 1,
		to_account_id: 2,
		tx_date: '2026-09-16',
		description: '转入定期'
	}
];

const MOCK_GROUPED = [
	{
		date: '2026-09-16',
		day_income: 100000,
		day_expense: 5000,
		day_balance: 95000,
		transactions: MOCK_TXS
	}
];

const listFn = vi.fn(async () => ({
	grouped: MOCK_GROUPED,
	summary: { total_income: 100000, total_expense: 5000, net: 95000 },
	pagination: { total: 3 }
}));

const removeFn = vi.fn(async () => ({}));
const batchRemoveFn = vi.fn(async () => ({ deleted_count: 2 }));

vi.mock('$lib/stores/app', () => ({
	appStore: {
		get categories() {
			return {
				expense: [{ id: 1, name: '餐饮', icon: '🍔', color: '#10B981' }],
				income: [{ id: 2, name: '工资', icon: '💰', color: '#3B82F6' }],
				system: []
			};
		},
		get accounts() {
			return [
				{ id: 1, name: '支付宝', icon: '💳' },
				{ id: 2, name: '招行储蓄', icon: '🏦' }
			];
		},
		get tags() {
			return [{ id: 1, name: '工作日' }];
		},
		get currentBookId() {
			return 1;
		},
		get listVersion() {
			return 0;
		},
		effectiveBookId: () => 1,
		loadDictionaries: vi.fn(async () => {})
	}
}));

vi.mock('$lib/api/modules/transactions', () => ({
	txApi: {
		list: listFn,
		remove: removeFn,
		batchRemove: batchRemoveFn
	}
}));

vi.mock('$lib/components/ui/toast', () => ({
	hzToast: {
		success: vi.fn(),
		error: vi.fn(),
		warning: vi.fn(),
		info: vi.fn(),
		offline: vi.fn()
	}
}));

vi.mock('$lib/utils/tx', () => ({
	amountDisplay: (tx: any) => ({
		sign: tx.type === 'income' ? '+' : tx.type === 'expense' ? '-' : '',
		abs: tx.amount / 100,
		tone: tx.type === 'income' ? 'income' : tx.type === 'expense' ? 'expense' : 'muted'
	}),
	baseAmount: (tx: any) => tx.amount / 100,
	highlightSegments: (text: string) => [{ text, hit: false }],
	recomputeDaySubtotal: (day: any) => {
		day.day_income = day.transactions
			.filter((t: any) => t.type === 'income')
			.reduce((s: number, t: any) => s + t.amount / 100, 0);
		day.day_expense = day.transactions
			.filter((t: any) => t.type === 'expense')
			.reduce((s: number, t: any) => s + t.amount / 100, 0);
	},
	toneClass: (tone: string) => (tone === 'income' ? 'text-income' : tone === 'expense' ? 'text-expense' : 'text-muted'),
	typeLabel: (t: string) => ({ income: '收入', expense: '支出', transfer: '转账', refund: '退款', reimburse: '报销', adjust: '余额调整' }[t] || t)
}));

vi.mock('$lib/utils/format', () => ({
	formatMoney: (v: number) => `¥${v}`,
	formatRelativeDate: () => '今天'
}));

let Page: any;

beforeAll(async () => {
	Page = (await import('../../routes/(auth)/transactions/+page.svelte')).default;
}, 120000);

describe('交易列表页 - 有数据', () => {
	beforeEach(() => {
		listFn.mockClear();
		removeFn.mockClear();
		batchRemoveFn.mockClear();
	});

	it('渲染汇总卡片', async () => {
		render(Page);
		await waitFor(() => {
			expect(screen.getAllByText('收入').length).toBeGreaterThanOrEqual(1);
			expect(screen.getAllByText('支出').length).toBeGreaterThanOrEqual(1);
		});
	});

	it('渲染交易列表含expense/income/transfer', async () => {
		render(Page);
		await waitFor(() => {
			expect(screen.getByText('午餐')).toBeTruthy();
			expect(screen.getByText('转入定期')).toBeTruthy();
		});
	});

	it('点击交易项显示预览弹窗', async () => {
		render(Page);
		await waitFor(() => screen.getByText('午餐'));
		await fireEvent.click(screen.getByText('午餐'));
		await waitFor(() => {
			expect(screen.getByText('编辑')).toBeTruthy();
			expect(screen.getByText('关闭')).toBeTruthy();
		});
	});

	it('预览弹窗显示商户和备注信息', async () => {
		render(Page);
		await waitFor(() => screen.getByText('午餐'));
		await fireEvent.click(screen.getByText('午餐'));
		await waitFor(() => {
			expect(screen.getByText('商户')).toBeTruthy();
			expect(screen.getByText('食堂')).toBeTruthy();
		});
	});

	it('预览弹窗显示标签信息', async () => {
		render(Page);
		await waitFor(() => screen.getByText('午餐'));
		await fireEvent.click(screen.getByText('午餐'));
		await waitFor(() => {
			expect(screen.getByText('标签')).toBeTruthy();
			expect(screen.getByText('工作日')).toBeTruthy();
		});
	});

	it('预览弹窗显示图片', async () => {
		render(Page);
		await waitFor(() => screen.getByText('午餐'));
		const items = screen.getAllByText('工资');
		await fireEvent.click(items[0]);
		await waitFor(() => {
			const imgs = screen.getAllByAltText('凭证');
			expect(imgs.length).toBeGreaterThan(0);
		});
	});

	it('关闭预览弹窗', async () => {
		render(Page);
		await waitFor(() => screen.getByText('午餐'));
		await fireEvent.click(screen.getByText('午餐'));
		await waitFor(() => screen.getByText('关闭'));
		await fireEvent.click(screen.getByText('关闭'));
		await waitFor(() => {
			expect(screen.queryByText('编辑')).toBeNull();
		});
	});

	it('点击多选进入选择模式', async () => {
		render(Page);
		await waitFor(() => screen.getByText('多选'));
		await fireEvent.click(screen.getByText('多选'));
		await waitFor(() => {
			expect(screen.getByText('退出多选')).toBeTruthy();
			expect(screen.getByText('已选 0 笔')).toBeTruthy();
		});
	});

	it('点击筛选显示筛选面板', async () => {
		render(Page);
		await waitFor(() => screen.getByText('筛选'));
		await fireEvent.click(screen.getByText('筛选'));
		await waitFor(() => {
			expect(screen.getByText('开始日期')).toBeTruthy();
			expect(screen.getByText('结束日期')).toBeTruthy();
			expect(screen.getByText('分类')).toBeTruthy();
			expect(screen.getByText('账户')).toBeTruthy();
			expect(screen.getByText('标签')).toBeTruthy();
			expect(screen.getByText('报销状态')).toBeTruthy();
		});
	});

	it('筛选面板包含金额范围', async () => {
		render(Page);
		await waitFor(() => screen.getByText('筛选'));
		await fireEvent.click(screen.getByText('筛选'));
		await waitFor(() => {
			expect(screen.getByText('最小金额')).toBeTruthy();
			expect(screen.getByText('最大金额')).toBeTruthy();
		});
	});

	it('筛选面板有应用筛选按钮', async () => {
		render(Page);
		await waitFor(() => screen.getByText('筛选'));
		await fireEvent.click(screen.getByText('筛选'));
		await waitFor(() => {
			expect(screen.getByText('应用筛选')).toBeTruthy();
		});
	});

	it('搜索输入框存在', async () => {
		render(Page);
		await waitFor(() => {
			expect(screen.getByPlaceholderText('搜索...')).toBeTruthy();
		});
	});
});

describe('交易列表页 - 空状态', () => {
	beforeEach(() => {
		listFn.mockClear();
	});

	it('显示空状态提示', async () => {
		listFn.mockResolvedValueOnce({
			grouped: [],
			summary: { total_income: 0, total_expense: 0, net: 0 },
			pagination: { total: 0 }
		});
		render(Page);
		await waitFor(() => {
			expect(screen.getByText('还没有交易记录')).toBeTruthy();
		});
	});

	it('显示记一笔按钮', async () => {
		listFn.mockResolvedValueOnce({
			grouped: [],
			summary: { total_income: 0, total_expense: 0, net: 0 },
			pagination: { total: 0 }
		});
		render(Page);
		await waitFor(() => {
			expect(screen.getByText('记一笔')).toBeTruthy();
		});
	});
});
