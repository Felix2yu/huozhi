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
		merchant: '食堂'
	},
	{
		id: 2,
		type: 'income',
		amount: 100000,
		category_id: 2,
		account_id: 1,
		tx_date: '2026-09-16',
		description: '工资'
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
	pagination: { total: 2 }
}));

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
			return [{ id: 1, name: '支付宝', icon: '💳' }];
		},
		get tags() {
			return [];
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
		remove: vi.fn(async () => ({})),
		batchRemove: vi.fn(async () => ({ deleted_count: 2 }))
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
		sign: tx.type === 'income' ? '+' : '-',
		abs: tx.amount / 100,
		tone: tx.type === 'income' ? 'income' : 'expense'
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
	toneClass: (tone: string) => (tone === 'income' ? 'text-income' : 'text-expense'),
	typeLabel: (t: string) => t
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
	});

	it('渲染汇总卡片', async () => {
		render(Page);
		await waitFor(() => {
			expect(screen.getAllByText('收入').length).toBeGreaterThanOrEqual(1);
			expect(screen.getAllByText('支出').length).toBeGreaterThanOrEqual(1);
		});
	});

	it('渲染交易列表', async () => {
		render(Page);
		await waitFor(() => {
			expect(screen.getByText('午餐')).toBeTruthy();
			expect(screen.getAllByText('工资').length).toBeGreaterThanOrEqual(1);
		});
	});

	it('点击多选进入选择模式', async () => {
		render(Page);
		await waitFor(() => screen.getByText('多选'));
		await fireEvent.click(screen.getByText('多选'));
		await waitFor(() => {
			expect(screen.getByText('退出多选')).toBeTruthy();
		});
	});

	it('点击筛选显示筛选面板', async () => {
		render(Page);
		await waitFor(() => screen.getByText('筛选'));
		await fireEvent.click(screen.getByText('筛选'));
		await waitFor(() => {
			expect(screen.getByText('开始日期')).toBeTruthy();
		});
	});

	it('点击交易项显示预览弹窗', async () => {
		render(Page);
		await waitFor(() => screen.getByText('午餐'));
		await fireEvent.click(screen.getByText('午餐'));
		await waitFor(() => {
			expect(screen.getByText('编辑')).toBeTruthy();
		});
	});

	it('点击加载更多按钮', async () => {
		listFn.mockResolvedValueOnce({
			grouped: MOCK_GROUPED,
			summary: { total_income: 100000, total_expense: 5000, net: 95000 },
			pagination: { total: 100 }
		});
		render(Page);
		await waitFor(() => {
			expect(screen.getByText(/加载更多/)).toBeTruthy();
		});
	});

	it('搜索输入框存在', async () => {
		render(Page);
		await waitFor(() => {
			expect(screen.getByPlaceholderText('搜索...')).toBeTruthy();
		});
	});

	it('多选模式下显示checkbox', async () => {
		render(Page);
		await waitFor(() => screen.getByText('多选'));
		await fireEvent.click(screen.getByText('多选'));
		await waitFor(() => {
			expect(screen.getByText('退出多选')).toBeTruthy();
			expect(screen.getByText('已选 0 笔')).toBeTruthy();
		});
	});

	it('筛选面板包含日期选择', async () => {
		render(Page);
		await waitFor(() => screen.getByText('筛选'));
		await fireEvent.click(screen.getByText('筛选'));
		await waitFor(() => {
			expect(screen.getByText('开始日期')).toBeTruthy();
			expect(screen.getByText('结束日期')).toBeTruthy();
			expect(screen.getByText('分类')).toBeTruthy();
			expect(screen.getByText('账户')).toBeTruthy();
		});
	});

	it('预览弹窗显示交易详情', async () => {
		render(Page);
		await waitFor(() => screen.getByText('午餐'));
		await fireEvent.click(screen.getByText('午餐'));
		await waitFor(() => {
			expect(screen.getByText('编辑')).toBeTruthy();
			expect(screen.getByText('关闭')).toBeTruthy();
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
});
