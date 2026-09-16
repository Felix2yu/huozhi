import { describe, it, expect, vi, beforeAll } from 'vitest';
import { render, screen, waitFor } from '@testing-library/svelte';

vi.mock('$lib/stores/app', () => ({
	appStore: {
		get categories() {
			return {
				expense: [{ id: 1, name: '餐饮' }],
				income: [{ id: 2, name: '工资' }],
				system: []
			};
		},
		get accounts() {
			return [{ id: 1, name: '支付宝' }];
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
		list: vi.fn(async () => ({
			grouped: [],
			summary: { total_income: 0, total_expense: 0, net: 0 },
			pagination: { total: 0 }
		})),
		remove: vi.fn(async () => ({})),
		batchRemove: vi.fn(async () => ({ deleted_count: 0 }))
	}
}));

vi.mock('$lib/components/ui/toast', () => ({
	hzToast: { success: vi.fn(), error: vi.fn(), warning: vi.fn(), info: vi.fn(), offline: vi.fn() }
}));

vi.mock('$lib/utils/tx', () => ({
	amountDisplay: () => ({ sign: '+', abs: 0, tone: 'muted' }),
	baseAmount: () => 0,
	highlightSegments: (text: string) => [{ text, hit: false }],
	recomputeDaySubtotal: () => {},
	toneClass: () => '',
	typeLabel: (t: string) => t
}));

let Page: any;

beforeAll(async () => {
	Page = (await import('../../routes/(auth)/transactions/+page.svelte')).default;
}, 120000);

describe('交易列表页', () => {
	it('渲染空状态', async () => {
		render(Page);
		await waitFor(() => {
			expect(screen.getByText('还没有交易记录')).toBeTruthy();
		});
	});

	it('渲染筛选标签', async () => {
		render(Page);
		await waitFor(() => {
			expect(screen.getByText('全部')).toBeTruthy();
		});
	});

	it('显示新增按钮', async () => {
		render(Page);
		await waitFor(() => {
			expect(screen.getByText('记一笔')).toBeTruthy();
		});
	});
});
