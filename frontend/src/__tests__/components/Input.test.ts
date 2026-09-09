/**
 * Bug #3 回归测试: Input 组件必须暴露 $bindable 的 value prop
 *
 * 历史 bug: Input.svelte 没有声明 value = $bindable('')，
 * 导致 login/+page.svelte 里 bind:value={username} 完全无效。
 *
 * 这里采用源码静态分析 + cn() 工具函数验证的方式，
 * 避免 Svelte 5 在 Vitest/jsdom 下 runtime mount 的兼容性问题。
 */
import { describe, it, expect } from 'vitest';
import { readFileSync } from 'fs';
import { fileURLToPath } from 'url';
import { dirname, resolve } from 'path';
import { cn } from '$lib/utils/cn';

const __dirname = dirname(fileURLToPath(import.meta.url));
const inputSource = readFileSync(
	resolve(__dirname, '../../lib/components/ui/Input.svelte'),
	'utf-8'
);

describe('Input component - Bug #3 $bindable value', () => {
	it('源码必须声明 value = $bindable("")', () => {
		expect(inputSource).toContain('value = $bindable(');
	});

	it('必须把 value 绑定到原生 <input>', () => {
		expect(inputSource).toMatch(/bind:value/);
	});

	it('cn() 工具函数正确合并 class', () => {
		expect(cn('base', 'extra')).toBe('base extra');
		expect(cn('base', false && 'hidden')).toBe('base');
		expect(cn('base', undefined, null, 'extra')).toBe('base extra');
		expect(cn({ active: true, disabled: false })).toBe('active');
		expect(cn('px-2 py-1', 'px-4')).toBe('py-1 px-4'); // tailwind-merge 去重
	});

	it('Input 组件应用 cn() 合并 className', () => {
		expect(inputSource).toContain('cn(');
		expect(inputSource).toContain('className');
	});

	it('Input 支持所有标准 input 属性透传 (type/class/placeholder)', () => {
		// 验证 props 解构里包含 class 和 type
		expect(inputSource).toContain("class: className");
		expect(inputSource).toContain("type = 'text'");
		// 验证用了 ...rest 透传
		expect(inputSource).toContain('...rest');
	});

	it('Input 的 base class 包含 Svelte 项目标准的样式 token', () => {
		expect(inputSource).toContain('flex h-9 w-full');
		expect(inputSource).toContain('border-input');
		expect(inputSource).toContain('bg-transparent');
		expect(inputSource).toContain('focus-visible:ring-ring');
	});
});
