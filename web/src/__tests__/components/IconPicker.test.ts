/**
 * IconPicker 图标选择器回归测试
 *
 * 历史 bug：账户编辑/新建页使用 `<IconPicker bind:value={icon} />`，
 * 但 IconPicker 早期把 `value` 声明为普通 prop 并只通过 `onchange` 回调通知，
 * 没有用 Svelte 5 的 `$bindable()`。于是 `bind:value` 为单向绑定，
 * 用户在图标选择器里选了图标也不会写回父组件的 `icon`，
 * 保存后账户依旧回退到「首字头像」。
 *
 * 这里验证：选择图标能通过 `bind:value` 回传；选择「自动」能回传空串。
 */
import { describe, it, expect } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/svelte';
import Host from './IconPickerHost.svelte';

const boundIcon = () => screen.getByTestId('bound-icon').textContent ?? '';

describe('IconPicker 图标选择双向绑定', () => {
	it('手动选择图标后通过 bind:value 回传图标 id', async () => {
		render(Host, { name: '招商银行', bankName: '招商银行', type: 'bank' });
		expect(boundIcon()).toBe('');

		await fireEvent.click(screen.getByTestId('icon-trigger'));
		await fireEvent.click(screen.getByTitle('支付宝'));

		// 支付宝 ≠ 自动识别出的招商银行，因此应写回 'alipay'
		expect(boundIcon()).toBe('alipay');
	});

	it('选择「自动」后回传空串以恢复自动匹配', async () => {
		render(Host, { name: '招商银行', bankName: '招商银行', type: 'bank' });

		await fireEvent.click(screen.getByTestId('icon-trigger'));
		await fireEvent.click(screen.getByTitle('支付宝'));
		expect(boundIcon()).toBe('alipay');

		await fireEvent.click(screen.getByTestId('icon-trigger'));
		const autoBtn = screen.getByText('自动').closest('button')!;
		await fireEvent.click(autoBtn);

		expect(boundIcon()).toBe('');
	});
});
