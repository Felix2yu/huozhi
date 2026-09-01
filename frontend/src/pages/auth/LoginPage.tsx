import { Link } from 'react-router-dom';
import { useState } from 'react';
import { toast } from 'sonner';
import { authApi } from '@/api';
import { useAppStore } from '@/stores/app';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { BookCheck, Loader2 } from 'lucide-react';

export default function LoginPage() {
  const [form, setForm] = useState({ username: '', password: '' });
  const [loading, setLoading] = useState(false);
  const setAuth = useAppStore(s => s.setAuth);
  const loadBooks = useAppStore(s => s.loadBooks);
  const loadDicts = useAppStore(s => s.loadDictionaries);
  const nav = useNavigate();
  const [qs] = useSearchParams();

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    try {
      const res = await authApi.login(form);
      setAuth(res.token, res.user);
      toast.success(`欢迎回来，${res.user.nickname}`);
      try {
        await loadBooks();
        await loadDicts();
      } catch {}
      const rd = qs.get('redirect') || '/dashboard';
      nav(rd, { replace: true });
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen grid md:grid-cols-2 bg-gradient-to-br from-brand-50 via-white to-emerald-50">
      {/* 左侧宣传 */}
      <div className="hidden md:flex flex-col justify-center items-center p-16 relative overflow-hidden">
        <div className="absolute top-10 left-10 w-40 h-40 bg-brand-200 rounded-full blur-3xl opacity-40" />
        <div className="absolute bottom-10 right-10 w-52 h-52 bg-emerald-300 rounded-full blur-3xl opacity-40" />
        <div className="relative z-10 max-w-md">
          <div className="w-16 h-16 rounded-2xl bg-brand-600 text-white grid place-items-center text-3xl shadow-xl mb-6">
            账
          </div>
          <h1 className="text-4xl font-bold text-slate-800 mb-4 leading-tight">
            简洁纯粹的<br/>个人记账系统
          </h1>
          <p className="text-slate-500 leading-relaxed mb-8">
            多账本、资产管理、预算规划、统计分析、周期记账、存钱计划，
            一站式掌握你所有的收支。
          </p>
          <ul className="space-y-3 text-slate-600">
            <Feature icon="📘" title="多账本 & 共享" desc="区分不同场景，支持多人协同记账" />
            <Feature icon="💳" title="全账户资产管理" desc="现金、银行卡、信用卡、储值卡、负债" />
            <Feature icon="📊" title="多维度统计" desc="分类/账户/标签/趋势 饼图、折线图" />
            <Feature icon="💰" title="存钱计划 & 分期" desc="目标管理、周期记账、分期管理" />
          </ul>
        </div>
      </div>

      {/* 右侧登录 */}
      <div className="flex flex-col justify-center p-6 md:p-16">
        <div className="max-w-sm mx-auto w-full">
          <div className="md:hidden flex items-center gap-2 mb-10">
            <div className="w-10 h-10 rounded-xl bg-brand-600 text-white grid place-items-center font-bold">账</div>
            <div>
              <div className="font-semibold text-slate-800">Huozhi 记账</div>
              <div className="text-xs text-slate-500">简洁纯粹的记账本</div>
            </div>
          </div>
          <h2 className="text-2xl font-bold text-slate-800 mb-1">登录账户</h2>
          <p className="text-slate-500 text-sm mb-8">输入用户名与密码登录你的 Huozhi 账户</p>

          <form onSubmit={submit} className="space-y-4">
            <div>
              <label className="label">用户名 / 邮箱 / 手机号</label>
              <input
                className="input"
                autoFocus
                value={form.username}
                onChange={(e) => setForm({ ...form, username: e.target.value })}
                placeholder="请输入用户名"
                required
              />
            </div>
            <div>
              <label className="label">密码</label>
              <input
                type="password"
                className="input"
                value={form.password}
                onChange={(e) => setForm({ ...form, password: e.target.value })}
                placeholder="请输入密码"
                required
                minLength={6}
              />
            </div>
            <button type="submit" disabled={loading} className="btn-primary btn-lg w-full">
              {loading ? <Loader2 className="animate-spin" size={18} /> : <BookCheck size={18} />}
              {loading ? '登录中...' : '登 录'}
            </button>
          </form>

          <div className="mt-8 text-center text-sm text-slate-500">
            还没有账户？
            <Link to="/register" className="text-brand-600 font-medium hover:underline ml-1">
              立即注册
            </Link>
          </div>

          <div className="mt-10 text-xs text-center text-slate-400">
            <p>登录即代表你同意 Huozhi 的用户协议与隐私政策</p>
          </div>
        </div>
      </div>
    </div>
  );
}

function Feature({ icon, title, desc }: { icon: string; title: string; desc: string }) {
  return (
    <li className="flex items-start gap-3">
      <div className="w-10 h-10 rounded-xl bg-white shadow-card grid place-items-center text-xl shrink-0">
        {icon}
      </div>
      <div>
        <div className="font-medium text-slate-700">{title}</div>
        <div className="text-sm text-slate-500">{desc}</div>
      </div>
    </li>
  );
}
