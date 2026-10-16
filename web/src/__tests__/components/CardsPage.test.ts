/**
 * 银行卡页 - formatCardNumber 的入参与渲染
 *
 * 修复点：`formatCardNumber(card: Account)` → `formatCardNumber(card: Pick<Account, 'card_no4'>)`。
 * 还款提醒里渲染的是 CreditRepayItem（后端账单视图，不是 Account），
 * 旧签名要求调用方凑出一个假的 Account —— 恢复类型覆盖后这条浮出。
 *
 * 这里验证收窄后的实际行为：
 *  - 有 card_no4 → 「•••• 1234」；
 *  - 没有 card_no4 → 占位「•••• ••••」；
 *  - 还款条目只给 { card_no4 } 这个偏对象也能正常格式化（不需要完整 Account）。
 */
import { describe, it, expect, vi, beforeAll } from 'vitest';
import { render, screen, waitFor } from '@testing-library/svelte';

const ACCOUNTS = [
	{
		id: 1,
		name: '招行信用卡',
		bank_name: '招商银行',
		type: 'credit',
		card_no4: '1234',
		balance: 0,
		icon: ''
	},
	{
		id: 2,
		name: '支付宝',
		bank_name: '',
		type: 'bank',
		card_no4: '',
		balance: 100,
		icon: ''
	}
];

const creditSummaryFn = vi.fn(async () => [
	{
		account_id: 1,
		name: '招行信用卡',
		bank_name: '招商银行',
		card_no4: '5678',
		bill_amount: 123400,
		repay_amount: 123400,
		repay_date: '2026-10-10',
		overdue: false
	}
]);

vi.mock('#lib/stores/app', () => ({
	appStore: {
		get accounts() {
			return ACCOUNTS;
		},
		currentBookId: 1
	}
}));

vi.mock('#lib/api/modules/accounts', () => ({
	accountApi: {
		creditSummary: creditSummaryFn
	}
}));

let Page: any;

beforeAll(async () => {
	Page = (await import('../../routes/(auth)/cards/+page.svelte')).default;
}, 120000);

describe('卡片页 - 卡号格式化', () => {
	it('有末四位的信用卡渲染成「•••• 1234」', async () => {
		render(Page);
		await waitFor(() => expect(screen.getByText('•••• 1234')).toBeTruthy());
	});

	it('没有末四位的账户渲染成占位「•••• ••••」', async () => {
		render(Page);
		await waitFor(() => expect(screen.getByText('•••• 1234')).toBeTruthy());
		expect(screen.getByText('•••• ••••')).toBeTruthy();
	});

	it('还款提醒只用 card_no4 的偏对象即可完成格式化（条目不是 Account）', async () => {
		render(Page);
		await waitFor(() => expect(screen.getByText(/招商银行 · •••• 5678/)).toBeTruthy());
		expect(creditSummaryFn).toHaveBeenCalled();
	});
});
