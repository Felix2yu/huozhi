import { useEffect, useState } from 'react';
import { Navigate, Route, Routes, useLocation } from 'react-router-dom';
import { toast } from 'sonner';
import { useAppStore } from '@/stores/app';
import AppLayout from '@/components/layout/AppLayout';
import { connectWs, disconnectWs, onSync } from '@/utils/ws';

import LoginPage from '@/pages/auth/LoginPage';
import RegisterPage from '@/pages/auth/RegisterPage';

import DashboardPage from '@/pages/dashboard/DashboardPage';
import TransactionsPage from '@/pages/transactions/TransactionsPage';
import TransactionAddPage from '@/pages/transactions/TransactionAddPage';
import AccountsPage from '@/pages/accounts/AccountsPage';
import CategoriesPage from '@/pages/categories/CategoriesPage';
import BudgetsPage from '@/pages/budgets/BudgetsPage';
import StatisticsPage from '@/pages/statistics/StatisticsPage';
import TagsPage from '@/pages/tags/TagsPage';
import SavingsPage from '@/pages/savings/SavingsPage';
import SharedBooksPage from '@/pages/shared/SharedBooksPage';
import SettingsPage from '@/pages/settings/SettingsPage';
import BillExportPage from '@/pages/bill/BillExportPage';

const RequireAuth = ({ children }: { children: React.ReactNode }) => {
  const isAuth = useAppStore(s => s.isAuth);
  const loading = useAppStore(s => s.loading);
  const [checking, setChecking] = useState(true);
  const [ok, setOk] = useState(isAuth);
  const checkAuth = useAppStore(s => s.checkAuth);
  const loadBooks = useAppStore(s => s.loadBooks);
  const loadDictionaries = useAppStore(s => s.loadDictionaries);
  const currentBookId = useAppStore(s => s.currentBookId);

  useEffect(() => {
    (async () => {
      const res = await checkAuth();
      setOk(res);
      if (res) {
        try {
          await loadBooks();
          if (useAppStore.getState().currentBookId) {
            await loadDictionaries();
          }
        } catch (e) {
          console.warn('加载基础数据失败', e);
        }
        connectWs();
      }
      setChecking(false);
    })();
    // eslint-disable-next-line
  }, []);

  // 监听 sync 事件：自动刷新字典数据 & 触发页面级 refresh
  useEffect(() => {
    const off = onSync((msg) => {
      // alert 类型：预算超限等提醒
      if (msg.type === 'alert') {
        if (msg.table === 'budgets' && msg.action === 'over_budget') {
          const count = msg.data?.count || 1;
          toast.warning(
            count > 1
              ? `⚠️ ${count} 个预算已超额，注意控制支出！`
              : '⚠️ 预算已超额，注意控制支出！',
            { description: '点击查看预算管理', action: { label: '查看', onClick: () => { window.location.href = '/budgets'; } } }
          );
        }
        return;
      }
      if (!msg.table) return;
      // 字典相关自动 reload
      if (['accounts', 'categories', 'tags'].includes(msg.table)) {
        useAppStore.getState().loadDictionaries();
      }
      // 派发事件让具体页面刷新自己的数据
      window.dispatchEvent(new CustomEvent('hz:data-changed', { detail: msg }));
    });
    return () => off();
  }, []);

  // 退出登录时断开 WS
  useEffect(() => {
    if (!isAuth) disconnectWs();
  }, [isAuth]);

  if (checking) {
    return (
      <div className="min-h-screen grid place-items-center text-slate-500 text-sm">
        <div className="animate-pulse">正在加载货殖...</div>
      </div>
    );
  }
  if (!ok) {
    const url = useLocation().pathname;
    return <Navigate to={'/login?redirect=' + encodeURIComponent(url)} replace />;
  }
  return <>{children}</>;
};

export default function App() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route path="/register" element={<RegisterPage />} />

      <Route
        path="/"
        element={
          <RequireAuth>
            <AppLayout />
          </RequireAuth>
        }
      >
        <Route index element={<Navigate to="/dashboard" replace />} />
        <Route path="dashboard" element={<DashboardPage />} />
        <Route path="transactions" element={<TransactionsPage />} />
        <Route path="transactions/add" element={<TransactionAddPage />} />
        <Route path="transactions/edit/:id" element={<TransactionAddPage />} />
        <Route path="accounts" element={<AccountsPage />} />
        <Route path="categories" element={<CategoriesPage />} />
        <Route path="budgets" element={<BudgetsPage />} />
        <Route path="statistics" element={<StatisticsPage />} />
        <Route path="tags" element={<TagsPage />} />
        <Route path="savings" element={<SavingsPage />} />
        <Route path="shared-books" element={<SharedBooksPage />} />
        <Route path="settings" element={<SettingsPage />} />
        <Route path="bill-export" element={<BillExportPage />} />
      </Route>

      <Route path="*" element={<Navigate to="/dashboard" replace />} />
    </Routes>
  );
}
