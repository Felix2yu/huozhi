import { NavLink, Outlet, useLocation, useNavigate } from 'react-router-dom';
import { useEffect, useState } from 'react';
import {
  LayoutDashboard, Receipt, Wallet as WalletIcon, PieChart, BarChart3,
  Tags, Target, Users as UsersGroupIcon, Settings, Plus, BookMarked,
  Menu, X, LogOut, TrendingUp, CloudOff, CreditCard,
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
  { to: '/shared-books', label: '共享账本', icon: UsersGroupIcon },
  { to: '/settings',     label: '系统设置', icon: Settings },
];

export default function AppLayout() {
  const user = useAppStore(s => s.user);
  const books = useAppStore(s => s.books);
  const currentBookId = useAppStore(s => s.currentBookId);
  const setCurrentBook = useAppStore(s => s.setCurrentBook);
  const sidebarOpen = useAppStore(s => s.sidebarOpen);
  const toggleSidebar = useAppStore(s => s.toggleSidebar);
  const logout = useAppStore(s => s.logout);
  const nav = useNavigate();

  useEffect(() => {
    // 移动端默认关闭侧边栏
  }, []);

  const currentBook = books.find(b => b.id === currentBookId) || books[0];

  const onLogout = async () => {
    try { await authApi.logout(); } catch {}
    logout();
    toast.success('已退出登录');
    nav('/login');
  };

  return (
    <div className="flex min-h-screen bg-slate-50">
      {/* ========== 侧边栏 ========== */}
      <aside
        className={cn(
          'fixed md:static inset-y-0 left-0 z-40 w-64 bg-white border-r border-slate-100 flex flex-col transition-transform',
          sidebarOpen ? 'translate-x-0' : '-translate-x-full md:translate-x-0'
        )}
      >
        {/* Logo */}
        <div className="px-5 py-4 border-b border-slate-100 flex items-center gap-2">
          <div className="w-9 h-9 rounded-xl bg-brand-600 grid place-items-center text-white font-bold">
            {currentBook?.icon?.slice(0, 1) || '账'}
          </div>
          <div className="flex-1 min-w-0">
            <div className="font-semibold text-slate-800 leading-tight">货殖</div>
            <div className="text-xs text-slate-500">简洁纯粹的记账本</div>
          </div>
          <button className="md:hidden btn-ghost btn-sm" onClick={toggleSidebar}>
            <X size={18} />
          </button>
        </div>

        {/* 账本切换 */}
        <div className="px-3 py-3 border-b border-slate-100">
          <label className="text-xs text-slate-500 px-2 flex items-center gap-1">
            <BookMarked size={12} /> 当前账本
          </label>
          <select
            className="mt-1 input input-sm"
            value={currentBookId}
            onChange={(e) => setCurrentBook(Number(e.target.value))}
          >
            {books.filter(b => !b.is_archived).map(b => (
              <option key={b.id} value={b.id}>
                {b.icon || '📘'} {b.name} {b.is_default ? '(默认)' : ''}
              </option>
            ))}
          </select>
          <div className="mt-2 text-xs text-slate-500 px-2">
            共 <b className="text-slate-700">{books.length}</b> 个账本
          </div>
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
        <div className="px-3 py-3 border-t border-slate-100 space-y-2">
          <NavLink
            to="/transactions/add"
            className="btn-primary w-full"
          >
            <Plus size={18} />
            记一笔
          </NavLink>
          <div className="flex items-center justify-between text-xs text-slate-500">
            <div className="flex items-center gap-2 min-w-0">
              <div className="w-7 h-7 rounded-full bg-brand-100 text-brand-700 grid place-items-center font-semibold truncate">
                {user?.nickname?.[0] || 'U'}
              </div>
              <div className="truncate">
                <div className="text-slate-700 font-medium truncate">{user?.nickname}</div>
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
        {/* 顶栏 */}
        <header className="sticky top-0 z-20 bg-white/80 backdrop-blur border-b border-slate-100">
          <div className="px-4 md:px-8 py-3 flex items-center gap-3">
            <button className="md:hidden btn-ghost btn-sm" onClick={toggleSidebar}>
              <Menu size={20} />
            </button>
            <div className="flex-1 min-w-0">
              <h1 className="text-base md:text-lg font-semibold text-slate-800 truncate">
                {currentBook?.icon || '📘'} {currentBook?.name || '账本'}
              </h1>
            </div>
            <OfflineBadge />
            <TopMiniStats />
          </div>
        </header>

        <main className="flex-1 px-4 md:px-8 py-5 md:py-8 max-w-[1400px] w-full mx-auto">
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

function TopMiniStats() {
  const [st, setSt] = useState<{ in: number; out: number; net: number; asset: number } | null>(null);
  const bookId = useAppStore(s => s.currentBookId);
  useEffect(() => {
    if (!bookId) return;
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
      <Stat label="本月收入" value={formatMoney(st.in)} cls="text-emerald-600" />
      <Stat label="本月支出" value={formatMoney(st.out)} cls="text-red-500" />
      <Stat label="本月结余" value={formatMoney(st.net)} cls={st.net >= 0 ? 'text-brand-600' : 'text-red-500'} />
      <Stat label="总资产" value={formatMoney(st.asset)} cls="text-indigo-600" />
    </div>
  );
}

function Stat({ label, value, cls }: { label: string; value: string; cls: string }) {
  return (
    <div className="text-right leading-tight">
      <div className="text-slate-400">{label}</div>
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

import { Home, PlusCircle, FileText, Wallet as WalletMob, User as UserMob } from 'lucide-react';

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
    <nav className="only-sm fixed bottom-0 inset-x-0 z-20 bg-white border-t border-slate-100 pb-[env(safe-area-inset-bottom)]">
      <div className="grid grid-cols-5">
        {tabs.map(t => {
          const active = t.to === path || (t.to !== '/transactions/add' && path.startsWith(t.to) && t.to !== '/dashboard')
            || (t.to === '/dashboard' && path === '/dashboard');
          if (t.center) {
            return (
              <button
                key={t.to}
                onClick={() => nav(t.to)}
                className="relative flex items-center justify-center"
              >
                <div className="absolute -top-5 w-14 h-14 rounded-full bg-brand-600 text-white grid place-items-center shadow-lg border-4 border-slate-50">
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
                active ? 'text-brand-600' : 'text-slate-400'
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
