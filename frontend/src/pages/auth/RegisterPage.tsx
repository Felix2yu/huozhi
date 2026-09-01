import { useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { toast } from 'sonner';
import { authApi } from '@/api';
import { useAppStore } from '@/stores/app';
import { UserPlus } from 'lucide-react';

export default function RegisterPage() {
  const [form, setForm] = useState({
    username: '', nickname: '', email: '', password: '', confirm: '',
  });
  const [loading, setLoading] = useState(false);
  const setAuth = useAppStore(s => s.setAuth);
  const loadBooks = useAppStore(s => s.loadBooks);
  const nav = useNavigate();

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (form.password !== form.confirm) {
      toast.error('两次密码输入不一致');
      return;
    }
    setLoading(true);
    try {
      const res = await authApi.register({
        username: form.username, nickname: form.nickname || form.username,
        email: form.email, password: form.password,
      }) as any;
      setAuth(res.token, res.user);
      toast.success('注册成功，已自动登录');
      try { await loadBooks(); } catch {}
      nav('/dashboard', { replace: true });
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen grid md:grid-cols-2 bg-gradient-to-br from-indigo-50 via-white to-brand-50">
      <div className="hidden md:flex flex-col justify-center p-16">
        <div className="max-w-md mx-auto">
          <h1 className="text-4xl font-bold text-slate-800 mb-4 leading-tight">
            开始你的记账之旅
          </h1>
          <p className="text-slate-500 leading-relaxed">
            注册一个 Huozhi 账户，你的数据将安全地保存于云端，并支持多设备同步。
          </p>
          <div className="mt-12 card card-body">
            <ul className="space-y-3 text-sm text-slate-600">
              <li>✅ 完全免费的云端同步</li>
              <li>✅ 多账本 · 共享账本 · 多账户</li>
              <li>✅ 支持微信 / 支付宝账单导入</li>
              <li>✅ 响应式界面，手机/桌面皆可使用（PWA）</li>
              <li>✅ 开源 · 可自部署 · 支持 Docker</li>
            </ul>
          </div>
        </div>
      </div>
      <div className="flex flex-col justify-center p-6 md:p-16">
        <div className="max-w-sm mx-auto w-full">
          <h2 className="text-2xl font-bold text-slate-800 mb-1">创建账户</h2>
          <p className="text-slate-500 text-sm mb-8">只需要几秒 · 信息安全加密存储</p>

          <form onSubmit={submit} className="space-y-3">
            <div>
              <label className="label">用户名 *</label>
              <input
                className="input" value={form.username}
                onChange={(e) => setForm({ ...form, username: e.target.value })}
                required minLength={3} placeholder="至少3位字母/数字"
              />
            </div>
            <div>
              <label className="label">昵称</label>
              <input
                className="input" value={form.nickname}
                onChange={(e) => setForm({ ...form, nickname: e.target.value })}
                placeholder="留空则同用户名"
              />
            </div>
            <div>
              <label className="label">邮箱</label>
              <input
                type="email" className="input" value={form.email}
                onChange={(e) => setForm({ ...form, email: e.target.value })}
              />
            </div>
            <div>
              <label className="label">密码 *</label>
              <input
                type="password" className="input" value={form.password}
                onChange={(e) => setForm({ ...form, password: e.target.value })}
                required minLength={6}
              />
            </div>
            <div>
              <label className="label">确认密码 *</label>
              <input
                type="password" className="input" value={form.confirm}
                onChange={(e) => setForm({ ...form, confirm: e.target.value })}
                required minLength={6}
              />
            </div>
            <button type="submit" disabled={loading} className="btn-primary btn-lg w-full mt-2">
              <UserPlus size={18} />
              {loading ? '注册中...' : '创建账户'}
            </button>
          </form>

          <div className="mt-6 text-center text-sm text-slate-500">
            已有账户？
            <Link to="/login" className="text-brand-600 font-medium hover:underline ml-1">
              返回登录
            </Link>
          </div>
        </div>
      </div>
    </div>
  );
}
