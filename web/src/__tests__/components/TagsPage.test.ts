/**
 * 标签页 - 编辑态改用共享 Tag 模型
 *
 * 修复点：`editingTag` / `openEdit()` 原先写的是行内字面量
 * `{ id: number; name: string; color: string; sort: number }`，
 * 与 `#lib/types` 的 `Tag` 只是长得像（缺 count 等字段），
 * 恢复类型覆盖后暴露出这个「假同名」结构；现在统一用 `Tag as TagModel`。
 *
 * 这里跑通能脱离浏览器到达的那条路径：
 *  打开某标签的编辑弹窗 → 弹窗预填 name/标识色 → 改名保存
 *  → tagApi.update 收到 (id, { name, color, sort })，且 sort 取自共享模型。
 */
import { describe, it, expect, vi, beforeAll, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';

const TAGS = [
	{ id: 1, name: '工作日', color: '#ef4444', sort: 3, count: 12 },
	{ id: 2, name: '报销', color: '', sort: 7, count: 0 }
];

const loadDictionaries = vi.fn(async (..._args: any[]): Promise<void> => {});
const updateFn = vi.fn(async (..._args: any[]): Promise<any> => ({}));
const createFn = vi.fn(async (..._args: any[]): Promise<any> => ({}));
const removeFn = vi.fn(async (..._args: any[]): Promise<any> => ({}));

vi.mock('#lib/stores/app', () => ({
	appStore: {
		get tags() {
			return TAGS;
		},
		currentBookId: 1,
		loadDictionaries
	}
}));

vi.mock('#lib/api/modules/tags', () => ({
	tagApi: { update: updateFn, create: createFn, remove: removeFn }
}));

const toastSuccess = vi.fn();
vi.mock('#lib/components/ui/toast', () => ({
	hzToast: { success: (...a: any[]) => toastSuccess(...a), error: vi.fn(), warning: vi.fn(), info: vi.fn() }
}));

let Page: any;

beforeAll(async () => {
	Page = (await import('../../routes/(auth)/tags/+page.svelte')).default;
}, 120000);

/** 打开第 index 个标签的编辑弹窗 */
async function openEditDialog(index: number) {
	render(Page);
	await waitFor(() => expect(screen.getByText(TAGS[index].name)).toBeTruthy());
	await fireEvent.click(screen.getAllByTitle('编辑')[index]);
	await waitFor(() => expect(screen.getByPlaceholderText('例如：餐饮、交通、拼多多')).toBeTruthy());
}

describe('标签页 - 编辑弹窗（共享 Tag 模型）', () => {
	beforeEach(() => {
		updateFn.mockClear();
		createFn.mockClear();
		toastSuccess.mockClear();
		loadDictionaries.mockClear();
	});

	it('列表渲染标签名与用量', async () => {
		render(Page);
		await waitFor(() => expect(screen.getByText('工作日')).toBeTruthy());
		expect(screen.getByText('报销')).toBeTruthy();
		expect(screen.getByText(/12 笔/)).toBeTruthy();
	});

	it('点编辑后弹窗预填标签名与标识色', async () => {
		await openEditDialog(0);
		const input = screen.getByDisplayValue('工作日');
		expect(input).toBeTruthy();
	});

	it('改名保存 → tagApi.update 带 id 与共享模型的 name/color/sort', async () => {
		await openEditDialog(0);
		const input = screen.getByDisplayValue('工作日');
		await fireEvent.input(input, { target: { value: '工作日午餐' } });
		await fireEvent.click(screen.getByText('保存'));

		await waitFor(() => expect(updateFn).toHaveBeenCalledTimes(1));
		expect(updateFn.mock.calls[0][0]).toBe(1);
		expect(updateFn.mock.calls[0][1]).toEqual({ name: '工作日午餐', color: '#ef4444', sort: 3 });
		await waitFor(() => expect(toastSuccess).toHaveBeenCalledWith('标签已更新'));
		expect(loadDictionaries).toHaveBeenCalled();
	});

	it('编辑没有配色的标签时，color 回退成空串而不是 undefined', async () => {
		await openEditDialog(1);
		await fireEvent.click(screen.getByText('保存'));
		await waitFor(() => expect(updateFn).toHaveBeenCalledTimes(1));
		expect(updateFn.mock.calls[0][1].color).toBe('');
		expect(updateFn.mock.calls[0][1].sort).toBe(7);
	});
});
