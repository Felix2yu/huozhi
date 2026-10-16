/**
 * GET /books 的返回体是 BookView（Book + is_shared + role）
 *
 * 后端 handlers/book.go 在 models.Book 外面包了一层
 * `bookView{ models.Book; IsShared; Role }`，共享账本据此显示「可记账 / 只读」。
 * 前端原先把它当 Book[] 收，`book.role` 在类型上是缺失的（svelte-check 报
 * `Property 'role' does not exist on type 'Book'`），恢复类型覆盖后浮出。
 *
 * 这里同时锁两件事：
 *  1. bookApi.list() 的返回类型是 BookView[]（写成 `const views: BookView[]` 由 tsc 把关）；
 *  2. is_shared / role 这两个字段在真实 http 解包链路上不会被丢；
 *  3. appStore.loadBooks() 走同一个 BookView[] 形状，books 里仍读得到共享标注。
 */
import { describe, it, expect, vi, beforeEach } from 'vitest';

const BOOKS_PAYLOAD = [
	{
		id: 1,
		user_id: 7,
		name: '日常',
		currency: 'CNY',
		is_default: true,
		is_archived: false,
		created_at: '2026-01-01T00:00:00Z',
		is_shared: false,
		role: 'owner'
	},
	{
		id: 2,
		user_id: 9,
		name: '家庭共享账本',
		currency: 'CNY',
		is_default: false,
		is_archived: false,
		created_at: '2026-02-01T00:00:00Z',
		is_shared: true,
		role: 'editor'
	}
];

const store: Record<string, string> = { hz_token: 'fake-jwt' };
vi.stubGlobal('localStorage', {
	getItem: (k: string) => store[k] ?? null,
	setItem: (k: string, v: string) => {
		store[k] = v;
	},
	removeItem: (k: string) => {
		delete store[k];
	},
	clear: () => {
		Object.keys(store).forEach((k) => delete store[k]);
	}
});

const { bookApi } = await import('#lib/api/modules/books');
const { appStore } = await import('#lib/stores/app');

function stubFetchJson(data: unknown) {
	const fetchMock = vi.fn().mockResolvedValue({
		ok: true,
		status: 200,
		json: () => Promise.resolve({ code: 0, data })
	});
	vi.stubGlobal('fetch', fetchMock);
	return fetchMock;
}

beforeEach(() => {
	vi.unstubAllGlobals();
	vi.stubGlobal('localStorage', {
		getItem: (k: string) => store[k] ?? null,
		setItem: (k: string, v: string) => {
			store[k] = v;
		},
		removeItem: (k: string) => {
			delete store[k];
		},
		clear: () => {
			Object.keys(store).forEach((k) => delete store[k]);
		}
	});
});

describe('bookApi.list - 返回 BookView[]', () => {
	it('请求打到 /books，返回值可直接当 BookView[] 用（编译期 + 运行期）', async () => {
		const fetchMock = stubFetchJson(BOOKS_PAYLOAD);

		// 这一行是类型断言的主体：若 list 仍返回 Book[]，svelte-check 会在此报 TS2322
		const views: import('#lib/types').BookView[] = await bookApi.list();

		expect(fetchMock).toHaveBeenCalledTimes(1);
		expect(String(fetchMock.mock.calls[0][0])).toContain('/books');
		expect(views).toHaveLength(2);
	});

	it('is_shared / role 在解包后仍然存在（共享标注不被 http 层吞掉）', async () => {
		stubFetchJson(BOOKS_PAYLOAD);
		const views = await bookApi.list();

		expect(views.map((b) => b.is_shared)).toEqual([false, true]);
		expect(views.map((b) => b.role)).toEqual(['owner', 'editor']);
		// Book 自身字段仍完整（BookView extends Book）
		expect(views[1].name).toBe('家庭共享账本');
		expect(views[1].is_default).toBe(false);
	});

	it('role 取值限定在 owner / editor / viewer', async () => {
		stubFetchJson([{ ...BOOKS_PAYLOAD[1], role: 'viewer' }]);
		const [book] = await bookApi.list();
		expect(book.role).toBe('viewer');
		expect(book.is_shared).toBe(true);
	});
});

describe('appStore.loadBooks - 同样按 BookView[] 收列表', () => {
	it('拉取 /books 后 books 里保留 is_shared / role', async () => {
		const fetchMock = stubFetchJson(BOOKS_PAYLOAD);
		await appStore.loadBooks();

		expect(String(fetchMock.mock.calls[0][0])).toContain('/books');
		const books = appStore.books as import('#lib/types').BookView[];
		expect(books).toHaveLength(2);
		expect(books[1].is_shared).toBe(true);
		expect(books[1].role).toBe('editor');
	});

	it('默认账本落在 is_default 的那一本上', async () => {
		stubFetchJson(BOOKS_PAYLOAD);
		await appStore.loadBooks();
		expect(appStore.currentBookId).toBe(1);
	});
});
