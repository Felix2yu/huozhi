/** 全局应用状态 - Svelte 5 runes */
import { http } from '$lib/api/http';
import type { User, Book, Category, Tag, Account } from '$lib/types';

const BOOK_ID_KEY = 'hz_book_id';

// ===== 状态 =====
let user = $state<User | null>(null);
let token = $state<string | null>(null);
let isAuth = $state<boolean>(false);
let loading = $state<boolean>(false);

// 初始化 token
if (typeof localStorage !== 'undefined') {
	token = localStorage.getItem('hz_token');
	isAuth = !!token;
}

let books = $state<Book[]>([]);
let currentBookId = $state<number>(
	typeof localStorage !== 'undefined'
		? Number(localStorage.getItem(BOOK_ID_KEY) || 0)
		: 0
);

let categories = $state<{
	expense: Category[];
	income: Category[];
	system: Category[];
}>({ expense: [], income: [], system: [] });

let tags = $state<Tag[]>([]);
let accounts = $state<Account[]>([]);
let listVersion = $state<number>(0);

// ===== Actions =====
function bumpListVersion() {
	listVersion++;
}

function setTokenAndAuth(newToken: string, u: User) {
	http.setToken(newToken);
	token = newToken;
	user = u;
	isAuth = true;
}

function logout() {
	http.removeToken();
	token = null;
	user = null;
	isAuth = false;
	books = [];
	currentBookId = 0;
	if (typeof localStorage !== 'undefined') {
		localStorage.removeItem(BOOK_ID_KEY);
	}
}

async function checkAuth(): Promise<boolean> {
	if (!token) return false;
	try {
		const u = await http.get<User>('/auth/me');
		user = u;
		return true;
	} catch {
		logout();
		return false;
	}
}

async function loadBooks(): Promise<void> {
	try {
		const list = await http.get<Book[]>('/books');
		books = list;

		// 验证 currentBookId
		if (
			currentBookId !== 0 &&
			!list.some((b) => b.id === currentBookId)
		) {
			currentBookId = 0;
		}
		if (
			!currentBookId &&
			typeof localStorage !== 'undefined' &&
			localStorage.getItem(BOOK_ID_KEY) !== '0'
		) {
			const def = list.find((b) => b.is_default) || list[0];
			if (def) {
				currentBookId = def.id;
				localStorage.setItem(BOOK_ID_KEY, String(def.id));
			}
		}
	} catch (e) {
		console.warn('[app] loadBooks failed', e);
	}
}

function setCurrentBook(id: number) {
	currentBookId = id;
	if (typeof localStorage !== 'undefined') {
		localStorage.setItem(BOOK_ID_KEY, String(id));
	}
	loadDictionaries(id);
}

async function loadDictionaries(bookId?: number): Promise<void> {
	const bid = bookId !== undefined ? bookId : currentBookId;
	try {
		const [cats, tagsRes, accsRes] = await Promise.allSettled([
			http.get<{ expense: Category[]; income: Category[]; system: Category[] }>(
				'/categories',
				{ params: { book_id: bid } }
			),
			http.get<Tag[]>('/tags'),
			http.get<{ accounts: Account[] }>('/accounts', {
				params: { book_id: bid, include_archived: 0 }
			})
		]);

		if (cats.status === 'fulfilled') {
			const flatten = (list?: Category[]): Category[] =>
				(list || []).flatMap((c: any) =>
					c.children?.length ? [c, ...c.children] : [c]
				);
			const c = cats.value;
			categories = {
				expense: flatten(c.expense),
				income: flatten(c.income),
				system: flatten(c.system)
			};
		}
		if (tagsRes.status === 'fulfilled') tags = tagsRes.value;
		if (accsRes.status === 'fulfilled') accounts = accsRes.value.accounts || [];
	} catch (e) {
		console.warn('[app] loadDictionaries failed', e);
	}
}

export const appStore = {
	// state getters
	get user() {
		return user;
	},
	get token() {
		return token;
	},
	get isAuth() {
		return isAuth;
	},
	get loading() {
		return loading;
	},
	get books() {
		return books;
	},
	get currentBookId() {
		return currentBookId;
	},
	get categories() {
		return categories;
	},
	get tags() {
		return tags;
	},
	get accounts() {
		return accounts;
	},
	get listVersion() {
		return listVersion;
	},

	// actions
	bumpListVersion,
	setTokenAndAuth,
	logout,
	checkAuth,
	loadBooks,
	setCurrentBook,
	loadDictionaries
};
