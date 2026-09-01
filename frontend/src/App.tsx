import { useEffect, useState } from 'react';
import { Navigate, Route, Routes, useLocation } from 'react-router-dom';
import { useAppStore } from '@/stores/app';
import AppLayout from '@/components/layout/AppLayout';

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
      }
      setChecking(false);
    })();
    // eslint-disable-next-line
  }, []);

  if (checking) {
    return (
      <div className="min-h-screen grid place-items-center text-slate-500 text-sm">
        <div className="animate-pulse">正在加载 Huozhi...</div>
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
      </Route>

      <Route path="*" element={<Navigate to="/dashboard" replace />} />
    </Routes>
  );
}
