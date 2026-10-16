/**
 * ToastProvider：Toaster 的全局 toastOptions.style 必须是 CSS 字符串
 *
 * 历史 bug 与 toast.ts 同源 —— 组件写成
 *   toastOptions={{ style: { background: 'var(--color-card)', ... } }}
 * svelte-sonner 会把它拼进 `style={`${restProps.style} ${toast.style}`}`，
 * 对象只会得到 "[object Object]"，全站 toast 的配色（含暗色主题适配）静默失效。
 *
 * 这里用真实的 svelte-sonner 挂载 ToastProvider，再发一条 toast，
 * 断言落到 DOM 上的 style 属性是真实 CSS 声明而不是 [object Object]。
 */
import { describe, it, expect } from 'vitest';
import { render, waitFor } from '@testing-library/svelte';
import ToastProvider from '#lib/components/ui/ToastProvider.svelte';
import { hzToast } from '#lib/components/ui/toast';

function toastEl(): HTMLElement {
	const el = document.querySelector<HTMLElement>('[data-sonner-toast]');
	if (!el) throw new Error('未找到 toast DOM 节点');
	return el;
}

describe('ToastProvider - 全局 style 以 CSS 字符串下发', () => {
	it('挂载后渲染 sonner 容器（Toaster 真的挂上了）', async () => {
		render(ToastProvider);
		await waitFor(() =>
			expect(document.querySelector('section[aria-live="polite"]')).toBeTruthy()
		);
	});

	it('toast 的 style 属性含 --color-card 引用，且不含 [object Object]', async () => {
		render(ToastProvider);
		hzToast.info('颜色检查');

		await waitFor(() => expect(document.querySelectorAll('[data-sonner-toast]').length).toBeGreaterThan(0));

		const style = toastEl().getAttribute('style') || '';
		expect(style).toContain('var(--color-card)');
		expect(style).toContain('1px solid var(--color-border)');
		expect(style).not.toContain('[object');
	});

	it('offline toast 走同一个 Toaster，style 同样是 CSS 声明串（不出现 [object Object]）', async () => {
		render(ToastProvider);
		hzToast.offline('离线');

		await waitFor(() => expect(document.querySelectorAll('[data-sonner-toast]').length).toBeGreaterThan(0));

		const el = toastEl();
		const style = el.getAttribute('style') || '';
		// 全局 toastOptions.style 生效（对象写法在这里会变成 [object Object]）
		expect(style).toContain('var(--color-card)');
		expect(style).not.toContain('[object Object]');
		// 内容仍在
		expect(el.textContent).toContain('离线');
	});
});
