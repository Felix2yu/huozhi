import { create } from 'zustand';
import type { Book, User, Category, Tag, Account } from '@/types';
import { authApi, bookApi, categoryApi, tagApi, accountApi } from '@/api';

interface AppState {
  // 全局
  loading: boolean;
  sidebarOpen: boolean;
  // 用户
  user: User | null;
  token: string | null;
  isAuth: boolean;
  // 当前账本
  books: Book[];
  currentBookId: number;
  // 缓存数据
  categories: { expense: Category[]; income: Category[]; system: Category[] };
  tags: Tag[];
  accounts: Account[];
  // 列表刷新触发器
  listVersion: number;

  // actions
  setLoading: (b: boolean) => void;
  toggleSidebar: () => void;
  closeSidebar: () => void;
  bumpListVersion: () => void;

  // auth
  setAuth: (token: string, user: User) => void;
  logout: () => void;
  checkAuth: () => Promise<boolean>;

  // books
  loadBooks: () => Promise<void>;
  setCurrentBook: (id: number) => void;

  // 字典数据
  loadDictionaries: (bookId?: number) => Promise<void>;
}

export const useAppStore = create<AppState>((set, get) => ({
  loading: false,
  sidebarOpen: false,
  user: null,
  token: localStorage.getItem('hz_token'),
  isAuth: !!localStorage.getItem('hz_token'),
  books: [],
  currentBookId: Number(localStorage.getItem('hz_book_id') || 0),
  categories: { expense: [], income: [], system: [] },
  tags: [],
  accounts: [],
  listVersion: 0,

  setLoading: (b) => set({ loading: b }),
  toggleSidebar: () => set({ sidebarOpen: !get().sidebarOpen }),
  closeSidebar: () => set({ sidebarOpen: false }),
  bumpListVersion: () => set({ listVersion: get().listVersion + 1 }),

  setAuth(token, user) {
    localStorage.setItem('hz_token', token);
    set({ token, user, isAuth: true });
  },
  logout() {
    localStorage.removeItem('hz_token');
    localStorage.removeItem('hz_book_id');
    set({ token: null, user: null, isAuth: false, books: [], currentBookId: 0 });
  },
  async checkAuth() {
    const tok = get().token;
    if (!tok) return false;
    try {
      const user = await authApi.me();
      set({ user });
      return true;
    } catch {
      localStorage.removeItem('hz_token');
      set({ token: null, user: null, isAuth: false });
      return false;
    }
  },

  async loadBooks() {
    const list = await bookApi.list();
    // currentBookId = 0 表示「全部账本」聚合视图（显式选择后保留，不自动跳回默认账本）
    const stored = localStorage.getItem('hz_book_id');
    let cur = get().currentBookId;
    if (!cur && stored !== '0') {
      const def = list.find(b => b.is_default) || list[0];
      if (def) {
        cur = def.id;
        localStorage.setItem('hz_book_id', String(cur));
      }
    }
    set({ books: list, currentBookId: cur });
  },
  setCurrentBook(id) {
    localStorage.setItem('hz_book_id', String(id));
    set({ currentBookId: id });
    get().loadDictionaries(id);
  },

  async loadDictionaries(bookId?: number) {
    // 0 = 全部账本聚合视图，是合法取值，不能当缺省值跳过
    const bid = bookId !== undefined ? bookId : get().currentBookId;
    const [cats, tags, accs] = await Promise.all([
      categoryApi.list({ book_id: bid }),
      tagApi.list(),
      accountApi.list({ book_id: bid, include_archived: 0 }).catch(() => ({ accounts: [] as Account[] } as any)),
    ]);
    // 后端返回的是嵌套树（二级分类在 roots 的 children 里），
    // 而各页面（分类管理 / 记一笔分类抽屉等）按平铺列表 + parent_id 消费，
    // 这里统一拍平，二级分类直接进入同层数组。
    const flatten = (list?: any[]): any[] =>
      (list || []).flatMap((c: any) => (c.children?.length ? [c, ...c.children] : [c]));
    set({
      categories: cats
        ? {
            expense: flatten(cats.expense),
            income: flatten(cats.income),
            system: flatten(cats.system),
          }
        : { expense: [], income: [], system: [] },
      tags,
      accounts: (accs as any).accounts || accs || [],
    });
  },
}));
