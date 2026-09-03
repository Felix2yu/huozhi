import { describe, it, expect, beforeEach, vi } from 'vitest';

const mocks = vi.hoisted(() => ({
  authApi: {
    me: vi.fn(),
    login: vi.fn(),
    register: vi.fn(),
    updateMe: vi.fn(),
    changePwd: vi.fn(),
    logout: vi.fn(),
  },
  bookApi: { list: vi.fn(), get: vi.fn(), create: vi.fn(), update: vi.fn(), remove: vi.fn() },
  categoryApi: { list: vi.fn(), create: vi.fn(), update: vi.fn(), remove: vi.fn() },
  tagApi: { list: vi.fn(), create: vi.fn(), update: vi.fn(), remove: vi.fn() },
  accountApi: { list: vi.fn(), get: vi.fn(), create: vi.fn(), update: vi.fn(), remove: vi.fn() },
}));

vi.mock('@/api', () => ({
  authApi: mocks.authApi,
  bookApi: mocks.bookApi,
  categoryApi: mocks.categoryApi,
  tagApi: mocks.tagApi,
  accountApi: mocks.accountApi,
}));

import { useAppStore } from './app';

const { authApi, bookApi, categoryApi, tagApi, accountApi } = mocks;

const defaults = {
  user: null,
  token: null,
  isAuth: false,
  books: [] as any[],
  currentBookId: 0,
  categories: { expense: [], income: [], system: [] } as any,
  tags: [] as any[],
  accounts: [] as any[],
};

const user = { id: 1, username: 'u', nickname: 'U' } as any;
const book = { id: 7, name: '默认账本', is_default: true } as any;

describe('app store', () => {
  beforeEach(() => {
    localStorage.clear();
    vi.clearAllMocks();
    useAppStore.setState({ ...defaults });
    authApi.me.mockResolvedValue(user);
    bookApi.list.mockResolvedValue([book]);
    categoryApi.list.mockResolvedValue({ expense: [{ id: 1, name: '餐饮' }], income: [], system: [] });
    tagApi.list.mockResolvedValue([{ id: 1, name: 't' }]);
    accountApi.list.mockResolvedValue({ accounts: [{ id: 1, name: '现金' }] });
  });

  it('初始未登录状态', () => {
    const s = useAppStore.getState();
    expect(s.isAuth).toBe(false);
    expect(s.token).toBeNull();
  });

  it('setAuth 写入 token 并标记已登录', () => {
    useAppStore.getState().setAuth('tok123', user);
    const s = useAppStore.getState();
    expect(s.token).toBe('tok123');
    expect(s.isAuth).toBe(true);
    expect(localStorage.getItem('hz_token')).toBe('tok123');
  });

  it('logout 清空登录态与本地存储', () => {
    useAppStore.getState().setAuth('tok123', user);
    useAppStore.getState().logout();
    const s = useAppStore.getState();
    expect(s.isAuth).toBe(false);
    expect(s.token).toBeNull();
    expect(localStorage.getItem('hz_token')).toBeNull();
  });

  it('checkAuth 无 token 直接返回 false', async () => {
    const ok = await useAppStore.getState().checkAuth();
    expect(ok).toBe(false);
    expect(authApi.me).not.toHaveBeenCalled();
  });

  it('checkAuth 有 token 时拉取用户信息', async () => {
    useAppStore.getState().setAuth('tok123', user);
    const ok = await useAppStore.getState().checkAuth();
    expect(ok).toBe(true);
    expect(useAppStore.getState().user).toBe(user);
  });

  it('checkAuth 失败清除登录态', async () => {
    useAppStore.getState().setAuth('tok123', user);
    authApi.me.mockRejectedValue(new Error('401'));
    const ok = await useAppStore.getState().checkAuth();
    expect(ok).toBe(false);
    expect(useAppStore.getState().isAuth).toBe(false);
  });

  it('loadBooks 写入账本并默认选中默认账本', async () => {
    await useAppStore.getState().loadBooks();
    const s = useAppStore.getState();
    expect(s.books).toHaveLength(1);
    expect(s.currentBookId).toBe(7);
    expect(localStorage.getItem('hz_book_id')).toBe('7');
  });

  it('setCurrentBook 持久化并加载字典', async () => {
    await useAppStore.getState().setCurrentBook(7);
    const s = useAppStore.getState();
    expect(s.currentBookId).toBe(7);
    // loadDictionaries 是 fire-and-forget，等待异步落库
    await vi.waitFor(() => expect(useAppStore.getState().categories.expense).toHaveLength(1));
    expect(useAppStore.getState().tags).toHaveLength(1);
    expect(useAppStore.getState().accounts).toHaveLength(1);
    expect(categoryApi.list).toHaveBeenCalledWith({ book_id: 7 });
  });

  it('loadDictionaries 容错 accountApi 失败', async () => {
    accountApi.list.mockRejectedValue(new Error('net'));
    await useAppStore.getState().loadDictionaries(7);
    const s = useAppStore.getState();
    expect(s.categories.expense).toHaveLength(1);
    expect(s.accounts).toEqual([]);
  });

  it('setLoading / toggleSidebar 基础状态变更', () => {
    useAppStore.getState().setLoading(true);
    expect(useAppStore.getState().loading).toBe(true);
    const before = useAppStore.getState().sidebarOpen;
    useAppStore.getState().toggleSidebar();
    expect(useAppStore.getState().sidebarOpen).toBe(!before);
  });
});
