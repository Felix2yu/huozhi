import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, within } from '@testing-library/svelte';
import AccountSelect from '$lib/components/AccountSelect.svelte';
import alipayLogo from '$lib/assets/bank-icons/alipay.svg?url';
import icbcLogo from '$lib/assets/bank-icons/icbc.svg?url';
import bocLogo from '$lib/assets/bank-icons/boc.svg?url';

const accounts = [
	{ id: 1, name: '日常支付', bank_name: '支付宝', icon: 'legacy-icon', type: 'virtual' },
	{ id: 2, name: '工资卡', bank_name: '招商银行', icon: 'icbc', type: 'debit_card' },
	{ id: 3, name: '旧账户', bank_name: '中国银行', is_archived: true },
	{ id: 4, name: '现金', type: 'cash' },
	{ id: 5, name: '备用', bank_name: '未知机构' }
];

let mockAccounts = accounts;

vi.mock('$lib/stores/app', () => ({
	appStore: {
		get accounts() {
			return mockAccounts;
		}
	}
}));

beforeEach(() => {
	mockAccounts = accounts;
});

describe('账户选择器 Logo', () => {
	it('旧图标无效时自动识别 Logo，并在选中后保留', async () => {
		const onChange = vi.fn();
		render(AccountSelect, { value: 0, onChange });
		await fireEvent.click(screen.getByRole('button', { name: '选择账户' }));
		const option = screen.getByRole('button', { name: /日常支付/ });
		expect(within(option).getByRole('img').getAttribute('src')).toBe(alipayLogo);
		await fireEvent.click(option);
		expect(onChange).toHaveBeenCalledWith(1);
		const selected = screen.getByRole('button', { name: /日常支付/ });
		expect(selected.getAttribute('aria-expanded')).toBe('false');
		expect(within(selected).getByRole('img').getAttribute('src')).toBe(alipayLogo);
	});

	it('有效手动 Logo 优先于银行自动识别', () => {
		render(AccountSelect, { value: 2 });
		expect(screen.getByRole('img').getAttribute('src')).toBe(icbcLogo);
	});

	it('现金和无法识别的账户保留默认头像', async () => {
		render(AccountSelect, { value: 0 });
		await fireEvent.click(screen.getByRole('button', { name: '选择账户' }));
		const cash = screen.getByRole('button', { name: /现金/ });
		expect(within(cash).queryByRole('img')).toBeNull();
		expect(within(cash).getByText('¥')).toBeTruthy();
		const unknown = screen.getByRole('button', { name: /备用/ });
		expect(within(unknown).queryByRole('img')).toBeNull();
		expect(within(unknown).getByTitle('未知机构')).toBeTruthy();
	});

	it('默认排除归档账户及转账对方账户', async () => {
		render(AccountSelect, { value: 0, exclude: 2 });
		await fireEvent.click(screen.getByRole('button', { name: '选择账户' }));
		expect(screen.queryByRole('button', { name: /旧账户/ })).toBeNull();
		expect(screen.queryByRole('button', { name: /工资卡/ })).toBeNull();
	});

	it('筛选模式允许选择归档账户并恢复全部账户', async () => {
		const onChange = vi.fn();
		render(AccountSelect, {
			value: 0,
			placeholder: '全部账户',
			includeArchived: true,
			clearable: true,
			onChange
		});
		await fireEvent.click(screen.getByRole('button', { name: '全部账户' }));
		const archived = screen.getByRole('button', { name: /旧账户/ });
		expect(within(archived).getByRole('img').getAttribute('src')).toBe(bocLogo);
		await fireEvent.click(archived);
		expect(onChange).toHaveBeenLastCalledWith(3);
		await fireEvent.click(screen.getByRole('button', { name: /旧账户/ }));
		await fireEvent.click(screen.getByRole('button', { name: '全部账户' }));
		expect(onChange).toHaveBeenLastCalledWith(0);
		expect(screen.getByRole('button', { name: '全部账户' }).getAttribute('aria-expanded')).toBe('false');
	});

	it('没有账户时显示空状态', async () => {
		mockAccounts = [];
		render(AccountSelect);
		await fireEvent.click(screen.getByRole('button', { name: '选择账户' }));
		expect(screen.getByText('暂无账户')).toBeTruthy();
	});
});
