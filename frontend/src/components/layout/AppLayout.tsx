import { NavLink, Outlet, useLocation, useNavigate } from 'react-router-dom';
import { useEffect, useState } from 'react';
import {
  LayoutDashboard, Receipt, Wallet as WalletIcon, PieChart, BarChart3,
  Tags, Target, Users as UsersGroupIcon, Settings, Plus, BookMarked,
  Menu, X, LogOut, TrendingUp, CloudOff, CreditCard, ChevronDown, Check,
  Repeat, FileText,
} from 'lucide-react';
import { useAppStore } from '@/stores/app';
import { cn, formatMoney } from '@/utils';
import { authApi } from '@/api';
import { toast } from 'sonner';
import { queueCount as getQueueCount, subscribe, replay, snapshot, clearAll } from '@/utils/offline';

const navItems = [
  { to: '/dashboard',    label: '首页总览', icon: LayoutDashboard },
  { to: '/transactions', label: '账单流水', icon: Receipt },
  { to: '/accounts',     label: '账户资产', icon: WalletIcon },
  { to: '/cards',        label: '我的银行卡', icon: CreditCard },
  { to: '/categories',   label: '分类管理', icon: PieChart },
  { to: '/budgets',      label: '预算管理', icon: Target },
  { to: '/statistics',   label: '统计分析', icon: BarChart3 },
  { to: '/tags',         label: '标签中心', icon: Tags },
  { to: '/savings',      label: '存钱计划', icon: TrendingUp },
  { to: '/recurring',    label: '周期记账', icon: Repeat },
  { to: '/installments', label: '分期管理', icon: CreditCard },
  { to: '/reimbursements', label: '报销管理', icon: FileText },
  { to: '/shared-books', label: '共享账本', icon: UsersGroupIcon },
  { to: '/settings',     label: '系统设置', icon: Settings },
];

export default function AppLayout() {
  const user = useAppStore(s => s.user);
  const sidebarOpen = useAppStore(s => s.sidebarOpen);
  const toggleSidebar = useAppStore(s => s.toggleSidebar);
  const closeSidebar = useAppStore(s => s.closeSidebar);
  const logout = useAppStore(s => s.logout);
  const nav = useNavigate();
  const location = useLocation();

  // 移动端：路由切换后自动收起侧边栏
  useEffect(() => {
    closeSidebar();
  }, [location.pathname, closeSidebar]);

  // 视口从移动端切换到桌面端时确保侧栏不被错误收起（桌面端始终静态展示）
  useEffect(() => {
    const mq = window.matchMedia('(min-width: 768px)');
    const onChange = () => { if (mq.matches) closeSidebar(); };
    mq.addEventListener('change', onChange);
    return () => mq.removeEventListener('change', onChange);
  }, [closeSidebar]);

  // 移动端顶栏展示的页面标题
  const pageTitle = location.pathname === '/transactions/add' ? '记一笔'
    : location.pathname.startsWith('/bill-export') ? '账单导出'
    : navItems.find(it => location.pathname.startsWith(it.to))?.label || '货殖';

  const onLogout = async () => {
    try { await authApi.logout(); } catch {}
    logout();
    toast.success('已退出登录');
    nav('/login');
  };

  return (
    <div className="flex min-h-screen bg-slate-50 dark:bg-slate-950">
      {/* ========== 侧边栏（移动端从右侧滑出） ========== */}
      <aside
        className={cn(
          'fixed md:sticky md:top-0 inset-y-0 right-0 z-40 w-64 h-screen bg-white dark:bg-slate-900 border-l border-slate-100 dark:border-slate-800 flex flex-col transition-transform shrink-0',
          sidebarOpen ? 'translate-x-0' : 'translate-x-full md:translate-x-0'
        )}
      >
        {/* Logo */}
        <div className="px-5 py-4 border-b border-slate-100 dark:border-slate-800 flex items-center gap-2">
          <div className="w-9 h-9 rounded-xl bg-brand-600 grid place-items-center text-white font-bold">
            账
          </div>
          <div className="flex-1 min-w-0">
            <div className="font-semibold text-slate-800 dark:text-slate-100 leading-tight">货殖</div>
            <div className="text-xs text-slate-500 dark:text-slate-400">简洁纯粹的记账本</div>
          </div>
          <button className="md:hidden btn-ghost btn-sm" onClick={toggleSidebar}>
            <X size={18} />
          </button>
        </div>

        {/* 账本切换 */}
        <div className="px-3 py-3 border-b border-slate-100 dark:border-slate-800">
          <BookSwitcher />
        </div>

        {/* 菜单 */}
        <nav className="flex-1 overflow-y-auto px-3 py-3 space-y-0.5">
          {navItems.map(it => (
            <NavLink
              key={it.to}
              to={it.to}
              end={it.to === '/dashboard'}
              className={({ isActive }) =>
                cn('sidebar-link', isActive && 'sidebar-link-active')
              }
            >
              <it.icon size={18} />
              <span className="text-sm">{it.label}</span>
            </NavLink>
          ))}
        </nav>

        {/* 快捷记账按钮 */}
        <div className="px-3 py-3 border-t border-slate-100 dark:border-slate-800 space-y-2">
          <NavLink
            to="/transactions/add"
            className="btn-primary w-full"
          >
            <Plus size={18} />
            记一笔
          </NavLink>
          <div className="flex items-center justify-between text-xs text-slate-500 dark:text-slate-400">
            <div className="flex items-center gap-2 min-w-0">
              <div className="w-7 h-7 rounded-full bg-brand-100 text-brand-700 grid place-items-center font-semibold truncate">
                {user?.nickname?.[0] || 'U'}
              </div>
              <div className="truncate">
                <div className="text-slate-700 dark:text-slate-200 font-medium truncate">{user?.nickname}</div>
                <div className="text-[11px] truncate">{user?.email || user?.username}</div>
              </div>
            </div>
            <button
              title="退出登录"
              className="p-2 rounded-lg text-slate-400 hover:text-red-500 hover:bg-red-50"
              onClick={onLogout}
            >
              <LogOut size={16} />
            </button>
          </div>
        </div>
      </aside>

      {/* 遮罩 */}
      {sidebarOpen && (
        <div
          className="md:hidden fixed inset-0 bg-black/30 z-30"
          onClick={toggleSidebar}
        />
      )}

      {/* ========== 主内容 ========== */}
      <div className="flex-1 flex flex-col min-w-0">
        {/* 顶栏：移动端 = Logo + 页面标题 + 右侧汉堡；桌面端 = 右侧统计 */}
        <header className="sticky top-0 z-20 bg-white/80 dark:bg-slate-900/80 backdrop-blur border-b border-slate-100 dark:border-slate-800">
          <div className="px-4 md:px-8 py-3 flex items-center gap-3">
            {/* 移动端 */}
            <div className="md:hidden w-8 h-8 rounded-lg bg-brand-600 grid place-items-center text-white text-sm font-bold shrink-0">
              账
            </div>
            <h1 className="md:hidden flex-1 min-w-0 text-base font-semibold text-slate-800 dark:text-slate-100 truncate">
              {pageTitle}
            </h1>
            <button className="md:hidden btn-ghost btn-sm shrink-0" onClick={toggleSidebar} title="打开菜单">
              <Menu size={20} />
            </button>

            {/* 桌面端 */}
            <div className="hidden md:block flex-1" />
            <OfflineBadge />
            <TopMiniStats />
          </div>
        </header>

        <main className="flex-1 px-4 md:px-8 pt-5 md:pt-8 pb-24 md:pb-8 max-w-[1400px] w-full mx-auto">
          <Outlet />
        </main>

        {/* 移动端底部Tab */}
        <MobileTabs />
      </div>
    </div>
  );
}

import { accountApi, txApi } from '@/api';
import { getMonthRange } from '@/utils';

/** 侧边栏账本切换器：展示当前账本，下拉可选全部账本/单账本 */
function BookSwitcher() {
  const books = useAppStore(s => s.books);
  const currentBookId = useAppStore(s => s.currentBookId);
  const setCurrentBook = useAppStore(s => s.setCurrentBook);
  const [open, setOpen] = useState(false);

  const isAll = currentBookId === 0;
  const current = books.find(b => b.id === currentBookId);
  const activeBooks = books.filter(b => !b.is_archived);

  const pick = (id: number) => {
    setCurrentBook(id);
    setOpen(false);
  };

  const rowCls = (active: boolean) =>
    cn(
      'w-full flex items-center gap-2.5 rounded-lg px-2 py-2 text-left transition',
      active
        ? 'bg-brand-50 dark:bg-brand-500/15'
        : 'hover:bg-slate-100 dark:hover:bg-slate-700',
    );

  return (
    <div className="relative">
      {/* 触发按钮：当前账本概览 */}
      <button
        onClick={() => setOpen(o => !o)}
        className={cn(
          'w-full flex items-center gap-2.5 rounded-xl border px-2.5 py-2 text-left transition',
          'border-slate-200 dark:border-slate-700 hover:border-brand-400 dark:hover:border-brand-500',
          'hover:bg-slate-50 dark:hover:bg-slate-800',
        )}
      >
        <span className="w-8 h-8 rounded-lg bg-slate-100 dark:bg-slate-800 grid place-items-center text-base shrink-0">
          {isAll ? '📚' : current?.icon || '📘'}
        </span>
        <span className="flex-1 min-w-0">
          <span className="block text-sm font-medium text-slate-800 dark:text-slate-100 truncate">
            {isAll ? '全部账本' : current?.name || '选择账本'}
          </span>
          <span className="block text-[11px] text-slate-400 dark:text-slate-500 truncate whitespace-nowrap">
            {isAll ? '聚合所有账本收支' : `共 ${books.length} 个账本`}
          </span>
        </span>
        <ChevronDown
          size={15}
          className={cn('shrink-0 text-slate-400 transition-transform', open && 'rotate-180')}
        />
      </button>

      {open && (
        <>
          <div className="fixed inset-0 z-40" onClick={() => setOpen(false)} />
          <div className="absolute left-0 top-full mt-2 z-50 w-64 max-w-[calc(100vw-2rem)] rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 shadow-xl p-1.5 max-h-80 overflow-y-auto">
            {/* 全部账本聚合视图 */}
            <button onClick={() => pick(0)} className={rowCls(isAll)}>
              <span className="w-7 h-7 rounded-md bg-slate-100 dark:bg-slate-700 grid place-items-center text-sm shrink-0">
                📚
              </span>
              <span className="flex-1 min-w-0">
                <span className={cn(
                  'block text-sm truncate',
                  isAll ? 'font-semibold text-brand-700 dark:text-brand-300' : 'text-slate-700 dark:text-slate-200',
                )}>全部账本</span>
              </span>
              {isAll && <Check size={15} className="shrink-0 text-brand-600 dark:text-brand-400" />}
            </button>

            {/* 单账本 */}
            {activeBooks.map(b => {
              const active = b.id === currentBookId;
              return (
                <button key={b.id} onClick={() => pick(b.id)} className={rowCls(active)}>
                  <span className="w-7 h-7 rounded-md bg-slate-100 dark:bg-slate-700 grid place-items-center text-sm shrink-0">
                    {b.icon || '📘'}
                  </span>
                  <span className="flex-1 min-w-0">
                    <span className={cn(
                      'block text-sm truncate',
                      active ? 'font-semibold text-brand-700 dark:text-brand-300' : 'text-slate-700 dark:text-slate-200',
                    )}>{b.name}</span>
                  </span>
                  {!active && b.is_default && (
                    <span className="shrink-0 text-[10px] px-1.5 py-0.5 rounded bg-slate-100 dark:bg-slate-700 text-slate-500 dark:text-slate-400">
                      默认
                    </span>
                  )}
                  {active && <Check size={15} className="shrink-0 text-brand-600 dark:text-brand-400" />}
                </button>
              );
            })}

            {/* 管理入口 */}
            <NavLink
              to="/shared-books"
              onClick={() => setOpen(false)}
              className="mt-1 pt-2 border-t border-slate-100 dark:border-slate-700 flex items-center gap-2 px-2 py-2 rounded-lg text-xs text-slate-500 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-700 hover:text-slate-700 dark:hover:text-slate-200 transition"
            >
              <BookMarked size={13} />
              管理账本
            </NavLink>
          </div>
        </>
      )}
    </div>
  );
}

function TopMiniStats() {
  const [st, setSt] = useState<{ in: number; out: number; net: number; asset: number } | null>(null);
  const bookId = useAppStore(s => s.currentBookId);
  useEffect(() => {
    if (bookId === undefined) return;
    (async () => {
      try {
        const { start, end } = getMonthRange();
        const { summary } = await txApi.list({
          book_id: bookId,
          start_date: start,
          end_date: end,
          page_size: 1,
        });
        const { accounts } = await accountApi.list({ book_id: bookId });
        const asset = accounts.reduce((s, a) => s + (a.include_in_total ? a.balance : 0), 0);
        setSt({ in: summary.total_income, out: summary.total_expense, net: summary.net, asset });
      } catch {}
    })();
  }, [bookId]);
  if (!st) return null;
  return (
    <div className="hide-sm hidden md:flex items-center gap-4 text-xs">
      <Stat label="本月收入" value={formatMoney(st.in)} cls="text-income" />
      <Stat label="本月支出" value={formatMoney(st.out)} cls="text-expense" />
      <Stat label="本月结余" value={formatMoney(st.net)} cls={st.net >= 0 ? 'text-brand-600 dark:text-brand-400' : 'text-expense'} />
      <Stat label="总资产" value={formatMoney(st.asset)} cls="text-indigo-600 dark:text-indigo-400" />
    </div>
  );
}

function Stat({ label, value, cls }: { label: string; value: string; cls: string }) {
  return (
    <div className="text-right leading-tight">
      <div className="text-xs text-slate-400">{label}</div>
      <div className={cn('font-semibold tabular-nums text-sm', cls)}>{value}</div>
    </div>
  );
}

/** 离线待同步角标 */
function OfflineBadge() {
  const [count, setCount] = useState(0);
  const [show, setShow] = useState(false);

  useEffect(() => {
    setCount(getQueueCount());
    const unsub = subscribe(() => setCount(getQueueCount()));
    const onOnline = () => setCount(getQueueCount());
    window.addEventListener('online', onOnline);
    window.addEventListener('offline', onOnline);
    return () => {
      unsub();
      window.removeEventListener('online', onOnline);
      window.removeEventListener('offline', onOnline);
    };
  }, []);

  if (count === 0) return null;

  const items = snapshot().slice(-5).reverse();

  return (
    <div className="relative">
      <button
        onClick={() => setShow(s => !s)}
        className="flex items-center gap-1 px-2.5 py-1 rounded-lg bg-amber-50 border border-amber-200 text-amber-700 text-xs font-medium hover:bg-amber-100 transition"
        title="有离线请求待同步"
      >
        <CloudOff size={14} />
        <span>{count} 条待同步</span>
      </button>

      {show && (
        <div
          className="absolute right-0 mt-2 w-80 max-h-80 overflow-y-auto bg-white border border-slate-200 rounded-xl shadow-xl p-3 z-50"
          onClick={(e) => e.stopPropagation()}
        >
          <div className="flex items-center justify-between mb-2">
            <div className="text-sm font-semibold text-slate-700">离线请求队列</div>
            <div className="flex items-center gap-1">
              <button
                onClick={async () => {
                  setShow(false);
                  const r = await replay();
                  if (r.ok > 0) toast.success(`已同步 ${r.ok} 条请求`);
                  if (r.remaining > 0) toast.warning(`${r.remaining} 条仍未同步，稍后重试`);
                  window.dispatchEvent(new CustomEvent('hz:data-changed'));
                }}
                className="px-2 py-1 text-xs rounded-md bg-brand-600 text-white hover:bg-brand-700"
              >立即同步</button>
              <button
                onClick={() => { clearAll(); toast.success('已清空离线队列'); setShow(false); }}
                className="px-2 py-1 text-xs rounded-md text-slate-500 hover:text-red-600 hover:bg-red-50"
                title="清空队列"
              >清空</button>
            </div>
          </div>
          <div className="space-y-1.5">
            {items.map(it => (
              <div key={it.id} className="px-2 py-1.5 rounded-md bg-slate-50 border border-slate-100 text-[11px]">
                <div className="flex items-center gap-1.5">
                  <span className={cn(
                    'px-1.5 py-0.5 rounded text-[10px] font-bold',
                    it.method === 'POST' && 'bg-blue-100 text-blue-700',
                    it.method === 'PUT' && 'bg-amber-100 text-amber-700',
                    it.method === 'DELETE' && 'bg-red-100 text-red-700',
                    it.method === 'PATCH' && 'bg-purple-100 text-purple-700',
                  )}>{it.method}</span>
                  <span className="text-slate-600 truncate flex-1">{it.url}</span>
                  <span className="text-slate-400">{it.retries > 0 ? `${it.retries}/${3}` : ''}</span>
                </div>
                {it.data && (
                  <div className="text-slate-400 truncate">
                    {JSON.stringify(it.data).slice(0, 80)}
                  </div>
                )}
              </div>
            ))}
          </div>
          <div className="mt-2 text-[11px] text-slate-400 text-center">共 {count} 条，联网后自动同步</div>
        </div>
      )}

      {show && (
        <div className="fixed inset-0 z-40" onClick={() => setShow(false)} />
      )}
    </div>
  );
}

import { Home, PlusCircle, Wallet as WalletMob, User as UserMob } from 'lucide-react';

function MobileTabs() {
  const nav = useNavigate();
  const location = useLocation();
  const path = location.pathname;
  const tabs = [
    { to: '/dashboard',    label: '首页', icon: Home },
    { to: '/transactions', label: '账单', icon: FileText },
    { to: '/transactions/add', label: '', icon: PlusCircle, center: true },
    { to: '/accounts',     label: '资产', icon: WalletMob },
    { to: '/settings',     label: '我的', icon: UserMob },
  ];
  return (
    <nav className="only-sm fixed bottom-0 inset-x-0 z-20 bg-white dark:bg-slate-900 border-t border-slate-100 dark:border-slate-800 pb-[env(safe-area-inset-bottom)]">
      <div className="grid grid-cols-5">
        {tabs.map(t => {
          const active = t.to === path || (t.to !== '/transactions/add' && path.startsWith(t.to) && t.to !== '/dashboard')
            || (t.to === '/dashboard' && path === '/dashboard');
          if (t.center) {
            if (path === t.to) return null;
            return (
              <button
                key={t.to}
                onClick={() => nav(t.to)}
                className="relative flex items-center justify-center"
              >
                <div className="absolute -top-5 w-14 h-14 rounded-full bg-brand-600 text-white grid place-items-center shadow-lg border-4 border-slate-50 dark:border-slate-900">
                  <t.icon size={24} />
                </div>
              </button>
            );
          }
          return (
            <button
              key={t.to}
              onClick={() => nav(t.to)}
              className={cn(
                'flex flex-col items-center justify-center gap-0.5 py-2.5 text-[11px]',
                active ? 'text-brand-600 dark:text-brand-400' : 'text-slate-400 dark:text-slate-500'
              )}
            >
              <t.icon size={20} />
              <span>{t.label}</span>
            </button>
          );
        })}
      </div>
    </nav>
  );
}
