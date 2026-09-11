/**
 * 分类管理页结构回归测试
 *
 * 历史 bug: 二级分类被放在一级分类网格之后的独立 {#each} 里渲染，
 * 导致展开任意一级分类时，二级分类都出现在页面最底部，与其父分类脱节。
 *
 * 当前结构（修复后）：
 *  - 一级分类全部平铺成网格（`category-roots`），一屏可见；
 *  - 点击某个一级分类后，其二级面板以 `col-span-full` 的形式作为该一级
 *    在网格中的「下一行」内联渲染（`category-children-panel[data-parent-id]`），
 *    即紧贴在所属一级的同一行下方，绝不跳到整张网格的底部；
 *    未展开的一级分类不渲染任何二级 DOM。
 *
 * 这里验证：默认收起时不渲染二级；展开后二级内联出现在对应一级的面板内；展开全部一次性展开。
 * 另含移动端可读性回归（二级名称曾因隐藏的悬浮按钮占宽而被截断成 1 个字）。
 */
import { describe, it, expect, vi, beforeAll, beforeEach } from 'vitest';
import { render, screen, fireEvent, within, waitFor } from '@testing-library/svelte';

const CATEGORIES = [
	{ id: 1, parent_id: 0, name: '餐饮', kind: 'expense', icon: '🍔', sort: 1, is_system: false },
	{ id: 2, parent_id: 1, name: '早餐', kind: 'expense', icon: '🍜', sort: 1, is_system: false },
	{ id: 3, parent_id: 0, name: '交通', kind: 'expense', icon: '🚗', sort: 2, is_system: false },
	{ id: 4, parent_id: 3, name: '地铁', kind: 'expense', icon: '🚇', sort: 1, is_system: false }
];

const loadDictionaries = vi.fn(async () => {});

vi.mock('$lib/stores/app', () => ({
	appStore: {
		get categories() {
			return { expense: CATEGORIES, income: [], system: [] };
		},
		get currentBookId() {
			return 1;
		},
		// 页面按「写入账本」收窄分类列表（全部账本模式下后端会合并所有账本的分类）
		effectiveBookId: () => 1,
		loadDictionaries
	}
}));

vi.mock('$lib/api/modules/categories', () => ({
	categoryApi: {
		create: vi.fn(async () => ({})),
		update: vi.fn(async () => ({})),
		remove: vi.fn(async () => ({}))
	}
}));

vi.mock('$lib/components/ui/toast', () => ({
	hzToast: {
		success: vi.fn(),
		error: vi.fn(),
		warning: vi.fn(),
		info: vi.fn(),
		offline: vi.fn()
	}
}));

let Page: any;

// 首次加载需要现场编译页面与其依赖的图标组件，耗时较长，这里提前预热
beforeAll(async () => {
	Page = (await import('../../routes/(auth)/categories/+page.svelte')).default;
}, 120000);

function mountPage() {
	return render(Page);
}

function groupAt(index: number): HTMLElement {
	return screen.getAllByTestId('category-group')[index];
}

function toggleOf(group: HTMLElement): HTMLElement {
	const btn = group.querySelector<HTMLElement>('[data-testid="group-toggle"]');
	if (!btn) throw new Error('未找到分组展开按钮');
	return btn;
}

/** 找到某个一级分类对应的二级面板（按 data-parent-id 匹配） */
function panelOf(parentId: number): HTMLElement {
	const hit = panelById(parentId);
	if (!hit) throw new Error(`未找到一级分类 ${parentId} 的二级面板`);
	return hit;
}

/** 某个一级分类的二级面板是否存在（未展开则不应存在） */
function panelById(parentId: number): HTMLElement | undefined {
	return screen
		.getAllByTestId('category-children-panel')
		.find((p) => p.getAttribute('data-parent-id') === String(parentId));
}

beforeEach(() => {
	loadDictionaries.mockClear();
});

describe('分类管理页 - 层级展示', () => {
	it('每个一级分类渲染为平铺网格中的独立分组', async () => {
		mountPage();
		await waitFor(() => expect(screen.getAllByTestId('category-group')).toHaveLength(2));
		expect(groupAt(0)).toHaveAttribute('data-parent-id', '1');
		expect(groupAt(1)).toHaveAttribute('data-parent-id', '3');
	});

	it('默认收起时不渲染任何二级分类', async () => {
		mountPage();
		await waitFor(() => screen.getByText('餐饮'));
		expect(screen.queryAllByTestId('category-children-panel')).toHaveLength(0);
		expect(screen.queryByText('早餐')).toBeNull();
		expect(screen.queryByText('地铁')).toBeNull();
	});

	it('展开一级分类后，二级分类内联出现在该一级同屏的面板内（核心修复点）', async () => {
		mountPage();
		await waitFor(() => screen.getByText('餐饮'));
		const foodGroup = groupAt(0);
		const trafficGroup = groupAt(1);

		await fireEvent.click(toggleOf(foodGroup));

		await waitFor(() => {
			expect(within(panelOf(1)).getByText('早餐')).toBeTruthy();
		});

		// 早餐只属于餐饮，不应出现在交通的面板内
		expect(within(panelOf(1)).queryByText('早餐')).toBeTruthy();
		// 未展开的一级分类，其二级面板与二级分类均不应渲染
		expect(panelById(3)).toBeUndefined();
		expect(screen.queryByText('地铁')).toBeNull();
	});

	it('二级分类 DOM 位于其一级分类的面板中，data-child-id 正确', async () => {
		mountPage();
		await waitFor(() => screen.getByText('餐饮'));
		const foodGroup = groupAt(0);

		await fireEvent.click(toggleOf(foodGroup));
		await waitFor(() => screen.getByTestId('category-child'));

		const panel = panelOf(1);
		const child = panel.querySelector<HTMLElement>('[data-testid="category-child"]')!;
		// 早餐必须位于餐饮的二级面板内部，且带正确的 data-child-id
		expect(panel.contains(child)).toBe(true);
		expect(child).toHaveAttribute('data-child-id', '2');
	});

	it('「展开全部」一次性展开所有一级分类', async () => {
		mountPage();
		await waitFor(() => screen.getByText('餐饮'));
		await fireEvent.click(screen.getByRole('button', { name: /展开全部/ }));

		await waitFor(() => {
			expect(within(panelOf(1)).getByText('早餐')).toBeTruthy();
			expect(within(panelOf(3)).getByText('地铁')).toBeTruthy();
		});
	});
});

describe('分类管理页 - 图标选择器', () => {
	/** 打开某个二级分类的编辑弹窗 */
	async function openFirstChildDialog() {
		mountPage();
		await waitFor(() => screen.getByText('餐饮'));
		await fireEvent.click(toggleOf(groupAt(0)));
		await waitFor(() => screen.getByTestId('category-child'));
		await fireEvent.click(screen.getByTestId('category-child'));
		await waitFor(() => screen.getByTestId('icon-grid'));
	}

	it('图标库按主题分组，标签齐备（大量图标也可快速定位）', async () => {
		await openFirstChildDialog();
		const labels = screen
			.getAllByTestId('icon-group-tab')
			.map((t) => t.textContent?.trim());
		expect(labels.length).toBeGreaterThanOrEqual(8);
		expect(labels).toContain('常用');
		expect(labels).toContain('饮食');
		expect(labels).toContain('休闲运动');
	});

	it('切换分组后，图标网格切换为该分组的图标集合', async () => {
		await openFirstChildDialog();

		const countOf = () =>
			within(screen.getByTestId('icon-grid')).getAllByRole('button').length;

		// 默认「常用」组：数量较多
		const commonCount = countOf();
		expect(commonCount).toBeGreaterThan(30);

		// 切到「人情节日」组（图标较少），数量应下降且含 🎁
		const socialTab = screen
			.getAllByTestId('icon-group-tab')
			.find((t) => t.textContent?.trim() === '人情节日')!;
		await fireEvent.click(socialTab);

		const socialGrid = screen.getByTestId('icon-grid');
		expect(within(socialGrid).getByText('🎁')).toBeTruthy();
		expect(countOf()).toBeLessThan(commonCount);
	});
});

/**
 * 历史 bug: 二级分类瓦片里的「编辑 / 删除」悬浮按钮用 `opacity-0` 隐藏，
 * 但它仍以 `shrink-0` 占据约 58px 的布局宽度；同时名称 span 未参与弹性伸缩。
 * 二者叠加后，窄屏（360px 两列）下名称可用宽度只剩约 27px，
 * 「午餐」被截断成「午…」（用户实测反馈）。
 *
 * jsdom 无布局引擎，无法断言像素宽度，这里锁定导致该 bug 的关键类名与触屏等价入口。
 */
describe('分类管理页 - 二级瓦片移动端可读性', () => {
	async function expandFirstGroup() {
		mountPage();
		await waitFor(() => screen.getByText('餐饮'));
		await fireEvent.click(toggleOf(groupAt(0)));
		await waitFor(() => screen.getByTestId('category-child'));
	}

	it('二级名称参与弹性伸缩，不会被挤压成零宽（flex-1 + min-w-0 + truncate）', async () => {
		await expandFirstGroup();
		const nameSpan = screen.getByTestId('category-child').querySelector('span.truncate')!;
		expect(nameSpan.textContent?.trim()).toBe('早餐');
		expect(nameSpan.className).toContain('flex-1');
		expect(nameSpan.className).toContain('min-w-0');
	});

	it('悬浮操作按钮在触屏尺寸下隐藏，不再为空占宽度，且宽屏才启用 flex', async () => {
		await expandFirstGroup();
		const child = screen.getByTestId('category-child');
		const editBtn = within(child).getByTitle('编辑');
		const actions = editBtn.parentElement!;

		// 触屏（<sm）必须 display:none —— 既不为不可用的 hover 操作预留宽度，
		// 也避免出现「看不见但可点」的删除热区。
		expect(actions.className).toContain('hidden');
		expect(actions.className).toContain('sm:flex');
		// 若这里出现裸 `flex`，则 <sm 下按钮会重新占据宽度，回归 bug 复现
		expect(/\bflex\b/.test(actions.className.replace(/sm:flex/g, ''))).toBe(false);
	});

	it('编辑弹窗内提供「删除分类」入口，保证触屏也能删除（替代被隐藏的悬浮按钮）', async () => {
		await expandFirstGroup();
		await fireEvent.click(screen.getByTestId('category-child'));
		await waitFor(() => screen.getByTestId('icon-grid'));

		expect(screen.getByTestId('dialog-delete')).toBeTruthy();
	});
});
