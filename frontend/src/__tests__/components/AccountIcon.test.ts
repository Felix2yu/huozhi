/**
 * AccountIcon 图标解析回归测试
 *
 * 历史 bug：图标解析只认「SVG 文件名 id」，且散落在 AccountIcon / IconPicker /
 * 银行卡页三处各自拼接 `/src/lib/assets/bank-icons/<id>.svg`。后果有两个：
 *   1. 存量账户的 icon 是 emoji（种子账户现金 💵 / 储蓄卡 💳），查不到 SVG 就
 *      **静默**回退到首字头像，用户改图标也看不出变化；
 *   2. 「自动」在 bank_name 为空时同样无结果，只能显示首字。
 *
 * 现在统一由 resolveAccountIcon() 解析：手动 icon → 自动识别 → 类型兜底，
 * 非 SVG 的 icon 值按字面量（emoji）渲染。
 */
import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import Host from './AccountIconHost.svelte';

const node = () => screen.getByTestId('icon');
const img = () => node().querySelector('img');
const glyph = () => node().querySelector('div')?.textContent?.trim() ?? '';

describe('AccountIcon 图标解析', () => {
	it('手动选择 SVG 图标时渲染图标图片', () => {
		render(Host, { account: { id: 1, name: '零钱', type: 'virtual', icon: 'alipay' } });
		expect(img()).not.toBeNull();
		expect(img()?.getAttribute('src')).toBeTruthy();
	});

	it('icon 是 emoji 等历史值时按字面量渲染，而不是退化成首字', () => {
		render(Host, { account: { id: 2, name: '储蓄卡', type: 'bank', icon: '💳' } });
		expect(img()).toBeNull();
		expect(glyph()).toBe('💳');
	});

	it('未设置 icon 时按 bank_name 自动识别', () => {
		render(Host, { account: { id: 3, name: '信用卡', bank_name: '招商银行', type: 'credit' } });
		expect(img()).not.toBeNull();
	});

	it('bank_name 为空的银行卡按类型兜底，不再显示首字', () => {
		render(Host, { account: { id: 4, name: '储蓄卡', type: 'bank' } });
		expect(img()).not.toBeNull();
		expect(glyph()).not.toBe('储');
	});

	it('id 形态但已下线的旧 icon 视为失效，回落到自动识别', () => {
		render(Host, { account: { id: 6, name: '日常支付', bank_name: '支付宝', type: 'virtual', icon: 'legacy-icon' } });
		expect(img()).not.toBeNull();
		expect(glyph()).not.toBe('legacy-icon');
	});

	it('现金账户保持 ¥ 兜底', () => {
		render(Host, { account: { id: 5, name: '现金', type: 'cash' } });
		expect(glyph()).toBe('¥');
	});
});
