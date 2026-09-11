/**
 * CategoryPicker 分类选择器回归测试
 *
 * 历史 bug 1: 记一笔界面的分类选择器把一级、二级分类全部平铺在同一个 4 列网格里，
 *            层级关系不直观、难以定位。
 * 历史 bug 2: 改成左栏一级 + 右栏二级的弹窗后，一级分类列表自身需要纵向滚动，
 *            仍然要「滚动下拉慢慢找」。现改为一级分类全部平铺一屏可见，
 *            点击一级在同一面板内展开二级。
 *
 * 这里验证：
 *  1) 打开后一级分类全部平铺可见，且二级面板同屏共存；
 *  2) 二级面板只展示当前一级分类的子级（非全部平铺）；
 *  3) 点击含子级的一级分类只展开、不直接选中，二级面板切换到对应子级；
 *  4) 点击二级分类选中并关闭，触发器显示「一级 / 二级」面包屑；
 *  5) 点击叶子一级分类（无子级）直接选中并关闭；
 *  6) 搜索框跨层级平铺过滤，类目多时无需展开翻找。
 */
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, within } from '@testing-library/svelte';
import Picker from '../../lib/components/CategoryPicker.svelte';

const CATEGORIES = [
	{ id: 1, parent_id: 0, name: '餐饮', kind: 'expense', icon: '🍔', sort: 1, is_system: false },
	{ id: 2, parent_id: 1, name: '早餐', kind: 'expense', icon: '🍜', sort: 1, is_system: false },
	{ id: 3, parent_id: 1, name: '午餐', kind: 'expense', icon: '🍲', sort: 2, is_system: false },
	{ id: 4, parent_id: 0, name: '交通', kind: 'expense', icon: '🚗', sort: 2, is_system: false },
	{ id: 5, parent_id: 4, name: '地铁', kind: 'expense', icon: '🚇', sort: 1, is_system: false },
	// 叶子一级分类（无子级）
	{ id: 6, parent_id: 0, name: '其他', kind: 'expense', icon: '📦', sort: 3, is_system: false }
];

/** 可变：便于验证「按写入账本收窄」行为（跨账本同名分类不应混入） */
let mockCategories: { expense: any[]; income: any[]; system: any[] } = {
	expense: CATEGORIES,
	income: [],
	system: []
};

vi.mock('$lib/stores/app', () => ({
	appStore: {
		get categories() {
			return mockCategories;
		},
		get currentBookId() {
			return 1;
		},
		// 写入目标账本（currentBookId=0 时回落到默认账本）
		effectiveBookId: () => 1,
		loadDictionaries: vi.fn(async () => {})
	}
}));

beforeEach(() => {
	mockCategories = { expense: CATEGORIES, income: [], system: [] };
});

describe('CategoryPicker 一级平铺 + 二级展开', () => {
	it('默认触发器显示占位文案', () => {
		render(Picker, { value: 0, kind: 'expense' });
		const trigger = screen.getByTestId('category-trigger');
		expect(trigger.textContent).toContain('选择分类');
	});

	it('打开后一级分类全部平铺，二级面板与之一屏共存', async () => {
		render(Picker, { value: 0, kind: 'expense' });
		await fireEvent.click(screen.getByTestId('category-trigger'));

		// 一级分类全部可见（无需滚动查找）
		const rootsBox = screen.getByTestId('category-roots');
		expect(within(rootsBox).getByText('餐饮')).toBeTruthy();
		expect(within(rootsBox).getByText('交通')).toBeTruthy();
		expect(within(rootsBox).getByText('其他')).toBeTruthy();

		// 二级面板同屏存在（不是跳到页面底部）
		expect(screen.getByTestId('category-children')).toBeTruthy();
	});

	it('二级面板只展示当前一级分类的子级，而非全部平铺', async () => {
		render(Picker, { value: 0, kind: 'expense' });
		await fireEvent.click(screen.getByTestId('category-trigger'));

		// 默认高亮第一个一级「餐饮」，二级面板应出现其子级 早餐/午餐
		const children = screen.getByTestId('category-children');
		expect(within(children).getByText('早餐')).toBeTruthy();
		expect(within(children).getByText('午餐')).toBeTruthy();

		// 关键：交通的子级「地铁」此时不应出现（证明不是全部平铺）
		expect(within(children).queryByText('地铁')).toBeNull();
	});

	it('点击含子级的一级分类只展开、不直接选中，二级面板切换到对应子级', async () => {
		render(Picker, { value: 0, kind: 'expense' });
		await fireEvent.click(screen.getByTestId('category-trigger'));

		await fireEvent.click(screen.getByRole('button', { name: /交通/ }));

		const children = screen.getByTestId('category-children');
		expect(within(children).getByText('地铁')).toBeTruthy();
		expect(within(children).queryByText('早餐')).toBeNull();

		// 只是展开浏览，未产生选中值，面板仍保持打开
		expect(screen.getByTestId('category-trigger').textContent).toContain('选择分类');
		expect(screen.getByTestId('category-children')).toBeTruthy();
	});

	it('点击二级分类后选中并关闭，触发器显示「一级 / 二级」面包屑', async () => {
		render(Picker, { value: 0, kind: 'expense' });
		await fireEvent.click(screen.getByTestId('category-trigger'));

		// 默认在「餐饮」，直接点二级「早餐」
		await fireEvent.click(within(screen.getByTestId('category-children')).getByRole('button', { name: /早餐/ }));

		const trigger = screen.getByTestId('category-trigger');
		expect(trigger.textContent).toContain('餐饮');
		expect(trigger.textContent).toContain('早餐');
		// 二级分类已选中，面板收起，二级容器不再存在
		expect(screen.queryByTestId('category-children')).toBeNull();
	});

	it('点击无子级的叶子一级分类直接选中并关闭', async () => {
		render(Picker, { value: 0, kind: 'expense' });
		await fireEvent.click(screen.getByTestId('category-trigger'));

		await fireEvent.click(screen.getByRole('button', { name: /其他/ }));

		const trigger = screen.getByTestId('category-trigger');
		expect(trigger.textContent).toContain('其他');
		expect(screen.queryByTestId('category-children')).toBeNull();
	});

	it('搜索框跨层级过滤，无需展开翻找', async () => {
		render(Picker, { value: 0, kind: 'expense' });
		await fireEvent.click(screen.getByTestId('category-trigger'));

		const input = screen.getByPlaceholderText('搜索分类…');
		await fireEvent.input(input, { target: { value: '地铁' } });

		const results = screen.getByTestId('category-search-results');
		expect(within(results).getByText('地铁')).toBeTruthy();
		// 命中项标注所属一级分类，便于确认层级
		expect(within(results).getByText('交通')).toBeTruthy();
		// 未命中的分类不再出现
		expect(within(results).queryByText('早餐')).toBeNull();

		await fireEvent.click(within(results).getByRole('button', { name: /地铁/ }));
		expect(screen.getByTestId('category-trigger').textContent).toContain('地铁');
	});
});

/**
 * 历史 bug 3: currentBookId === 0（全部账本）时后端 /categories 不做账本过滤，
 * 会把所有账本的分类合并返回 —— 本项目实测 16 个账本合并后一级分类有 153 行
 * （去重后仅 29 个名称，如「交通」有 11 份）。选择器因此被重复项撑长，
 * 二级面板被挤到滚动区底部。而记账写入的是具体账本（effectiveBookId()），
 * 所以选择器只应呈现该账本的分类。
 */
describe('CategoryPicker 按写入账本收窄分类', () => {
	it('只呈现写入账本的分类，其他账本的同名项不混入', async () => {
		mockCategories = {
			expense: [
				{ id: 1, parent_id: 0, name: '餐饮', kind: 'expense', book_id: 1, sort: 1, icon: '🍔' },
				{ id: 2, parent_id: 1, name: '早餐', kind: 'expense', book_id: 1, sort: 1, icon: '🍜' },
				// 另一个账本（book 4）的同名一级 + 独有分类，都不应出现在选择器里
				{ id: 99, parent_id: 0, name: '餐饮', kind: 'expense', book_id: 4, sort: 1, icon: '🍔' },
				{ id: 100, parent_id: 0, name: '日用品', kind: 'expense', book_id: 4, sort: 2, icon: '🧺' }
			],
			income: [],
			system: []
		};

		render(Picker, { value: 0, kind: 'expense' });
		await fireEvent.click(screen.getByTestId('category-trigger'));

		const rootsBox = screen.getByTestId('category-roots');
		// 本账本的分类可见，且「餐饮」只出现一次（不再是多账本重复项）
		expect(within(rootsBox).getAllByText('餐饮')).toHaveLength(1);
		// 其他账本的分类不出现
		expect(within(rootsBox).queryByText('日用品')).toBeNull();
	});

	it('二级区不参与滚动（固定在面板底部），一级区独立滚动', async () => {
		render(Picker, { value: 0, kind: 'expense' });
		await fireEvent.click(screen.getByTestId('category-trigger'));

		// 一级网格的容器必须是独立滚动区，否则一级一多二级区又会被推到滚动底部
		const rootsBox = screen.getByTestId('category-roots');
		const scroller = rootsBox.parentElement!;
		expect(scroller.className).toContain('overflow-y-auto');
		expect(scroller.className).toContain('min-h-0');

		// 二级区自身不是滚动容器的兄弟节点被一起滚动，而是 shrink-0 固定块
		const childrenBox = screen.getByTestId('category-children');
		expect(childrenBox.closest('.shrink-0')).toBeTruthy();
	});
});
