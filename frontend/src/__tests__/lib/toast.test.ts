/**
 * hzToast 包装层：toast 样式参数形状回归测试
 *
 * 历史 bug: `hzToast.offline` 把 style 写成对象
 * `{ background: 'var(--color-muted)', border: '...' }`。
 * svelte-sonner v1 的 style 是「CSS 字符串」，内部会拼进
 * `style={`${restProps.style} ${toast.style}`}`，对象只会得到 "[object Object]"，
 * 浏览器直接忽略整条声明 —— 离线条幅因此静默失去配色（不报错，肉眼才看得见）。
 *
 * 这里断言的是「传给 svelte-sonner 的到底是什么」：
 *  - style 必须是 string（不是 object，否则就是历史 bug 复发）；
 *  - 内容必须是真正的 CSS 声明串，含 var(--color-*) 引用；
 *  - 拼进模板串后不得出现 "[object Object]"；
 *  - duration 仍为 4000；success/error/warning/info 的 description 原样透传。
 */
import { describe, it, expect, vi, beforeEach } from 'vitest';

const spies = vi.hoisted(() => ({
	toast: vi.fn(),
	success: vi.fn(),
	error: vi.fn(),
	warning: vi.fn(),
	info: vi.fn()
}));

// svelte-sonner 的 toast 既是可调用的函数，又带 .success/.error 等方法。
vi.mock('svelte-sonner', () => {
	const fn: any = (...args: any[]) => spies.toast(...args);
	fn.success = (...args: any[]) => spies.success(...args);
	fn.error = (...args: any[]) => spies.error(...args);
	fn.warning = (...args: any[]) => spies.warning(...args);
	fn.info = (...args: any[]) => spies.info(...args);
	return { toast: fn, Toaster: {} };
});

const { hzToast } = await import('#lib/components/ui/toast');

beforeEach(() => {
	Object.values(spies).forEach((s) => s.mockClear());
});

/** 取第 2 个实参（svelte-sonner 的选项对象） */
function optsOf(spy: ReturnType<typeof vi.fn>): any {
	expect(spy).toHaveBeenCalledTimes(1);
	return spy.mock.calls[0][1];
}

describe('hzToast.offline - style 必须是 CSS 字符串', () => {
	it('调用 toast(message, options) 且 style 是 string 而不是 object', () => {
		hzToast.offline('当前离线，已排队');
		const opts = optsOf(spies.toast);

		expect(spies.toast.mock.calls[0][0]).toBe('当前离线，已排队');
		expect(typeof opts.style).toBe('string');
		expect(Array.isArray(opts.style)).toBe(false);
	});

	it('style 是「background: …; border: …」的 CSS 声明串，且带 var(--color-*) 引用', () => {
		hzToast.offline('x');
		const style = optsOf(spies.toast).style as string;

		expect(style).toContain('background: var(--color-muted)');
		expect(style).toContain('border: 1px solid var(--color-border)');
		// 关键回归点：对象被字符串化会得到 [object Object]，浏览器会整条忽略
		expect(style).not.toContain('[object Object]');
	});

	it('模拟 svelte-sonner 的模板拼接，产物仍是合法 CSS 串（历史 bug 在此暴露）', () => {
		hzToast.offline('x');
		const opts = optsOf(spies.toast);
		// svelte-sonner 内部：style={`${toastOptions.style} ${toast.style}`}
		const composed = `${''} ${opts.style}`;

		expect(composed.trim().startsWith('background:')).toBe(true);
		expect(String(composed)).not.toContain('[object');
		// 若 style 是对象，JSON 化后是 {"background":...}；字符串则带引号 —— 以此锁定类型
		expect(JSON.stringify(opts.style)).toMatch(/^"/);
	});

	it('duration 固定 4000ms', () => {
		hzToast.offline('x');
		expect(optsOf(spies.toast).duration).toBe(4000);
	});
});

describe('hzToast 其余方法 - 透传 message / description', () => {
	it('success 走 toast.success 并带上 description', () => {
		hzToast.success('已保存', '账本「日常」');
		expect(spies.success).toHaveBeenCalledWith('已保存', { description: '账本「日常」' });
	});

	it('error 走 toast.error 并带上 description', () => {
		hzToast.error('失败', '网络超时');
		expect(spies.error).toHaveBeenCalledWith('失败', { description: '网络超时' });
	});

	it('warning 走 toast.warning', () => {
		hzToast.warning('请输入标签名称');
		expect(spies.warning).toHaveBeenCalledWith('请输入标签名称', { description: undefined });
	});

	it('info 走基础 toast（不带自定义样式）', () => {
		hzToast.info('同步完成');
		expect(spies.toast).toHaveBeenCalledWith('同步完成', { description: undefined });
	});
});
