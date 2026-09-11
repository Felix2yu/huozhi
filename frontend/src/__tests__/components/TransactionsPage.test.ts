/**
 * 账单流水页回归测试 —— 「全部账本」模式
 *
 * 历史 bug: loadData() 里先写
 *     const bid = appStore.currentBookId;
 *     if (!bid) return;      // ← 提前返回
 *     loading = true;
 * 而 loading 的初值是 true。当用户在账本切换器里选择「全部账本」
 * (currentBookId === 0) 时，函数在发出任何请求之前就返回了，
 * loading 永远停在 true，页面永久卡在骨架屏（截图里的现象）。
 *
 * 后端本身就把 book_id=0 当作「按用户聚合全部账本」，
 * 所以正确行为是：照常发请求，并把 loading 正常结束。
 */
import { describe, it, expect, vi, beforeAll, beforeEach } from 'vitest';
import { render, screen, waitFor, cleanup } from '@testing-library/svelte';

const list = vi.fn(async () => ({
	grouped: [],
	summary: { total_income: 0, total_expense: 0, net: 0 }
}));

vi.mock('$lib/api/modules/transactions', () => ({
	txApi: { list: (...args: any[]) => (list as any)(...args) }
}));

vi.mock('$lib/stores/app', () => ({
	appStore: {
		get currentBookId() {
			return 0;
		},
		get categories() {
			return { expense: [], income: [], system: [] };
		},
		get accounts() {
			return [];
		}
	}
}));

let Page: any;

// 首次加载需要现场编译页面及其依赖组件，耗时较长，这里提前预热
beforeAll(async () => {
	Page = (await import('../../routes/(auth)/transactions/+page.svelte')).default;
}, 120000);

beforeEach(() => {
	cleanup();
	list.mockClear();
});

describe('账单流水页 -「全部账本」模式（currentBookId = 0）', () => {
	it('必须照常发出请求，而不是提前 return 跳过', async () => {
		render(Page);
		await waitFor(() => expect(list).toHaveBeenCalled(), { timeout: 5000 });
	});

	it('请求需带 book_id=0，交给后端聚合全部账本', async () => {
		render(Page);
		await waitFor(() => expect(list).toHaveBeenCalled(), { timeout: 5000 });
		expect(list.mock.calls[0][0]).toMatchObject({ book_id: 0 });
	});

	it('加载态必须结束，不得永久停留在骨架屏', async () => {
		render(Page);
		// 请求成功返回后应渲染空状态
		await waitFor(() => expect(screen.getByTestId('tx-empty')).toBeTruthy(), { timeout: 5000 });
		expect(screen.queryByTestId('tx-skeleton')).toBeNull();
	});
});
