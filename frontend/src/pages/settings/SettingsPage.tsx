import { useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { toast } from 'sonner';
import { useAppStore } from '@/stores/app';
import { authApi, bookApi, ioApi } from '@/api';
import type { User as UserT } from '@/types';
import { cn, formatDate } from '@/utils';
import {
  Settings as SettingsIcon, User as UserIcon, Lock, Book, Globe, Upload, Download,
  FileText, LogOut, Check, X, ChevronRight, AlertCircle, Info,
  Bell, Palette, Shield, HelpCircle, Heart, Smartphone,
} from 'lucide-react';
import { Modal, ConfirmDialog } from '@/components/common';

type TabKey = 'profile' | 'password' | 'prefs' | 'io' | 'about';

const TABS: Array<{ k: TabKey; label: string; Icon: any; badge?: string }> = [
  { k: 'profile', label: '个人信息', Icon: UserIcon },
  { k: 'password', label: '修改密码', Icon: Lock },
  { k: 'prefs', label: '偏好设置', Icon: Palette },
  { k: 'io', label: '导入 / 导出', Icon: FileText, badge: 'PRO' },
  { k: 'about', label: '关于版本', Icon: Info },
];

export default function SettingsPage() {
  const navigate = useNavigate();
  const user = useAppStore(s => s.user) as UserT;
  const setUser = (u: UserT) => useAppStore.setState({ user: u });
  const books = useAppStore(s => s.books);
  const currentBookId = useAppStore(s => s.currentBookId);
  const setCurrentBook = useAppStore(s => s.setCurrentBook);
  const logoutStore = useAppStore(s => s.logout);
  const loadBooks = useAppStore(s => s.loadBooks);

  const [tab, setTab] = useState<TabKey>('profile');

  // 个人信息
  const [profileForm, setProfileForm] = useState({
    nickname: user?.nickname || '',
    email: user?.email || '',
    phone: user?.phone || '',
    avatar: user?.avatar || '',
  });
  const [profileSaving, setProfileSaving] = useState(false);

  // 修改密码
  const [pwdForm, setPwdForm] = useState({
    old_password: '', new_password: '', confirm_password: '',
  });
  const [pwdSaving, setPwdSaving] = useState(false);

  // 偏好设置
  const [prefs, setPrefs] = useState({
    default_book_id: currentBookId,
    currency: user?.currency || 'CNY',
    month_start: user?.month_start || 1,
    locale: user?.locale || 'zh-CN',
    timezone: user?.timezone || 'Asia/Shanghai',
  });
  const [prefsSaving, setPrefsSaving] = useState(false);

  // 导入
  const [importSource, setImportSource] = useState<'wechat' | 'alipay' | 'template' | 'csv'>('template');
  const [importFile, setImportFile] = useState<File | null>(null);
  const [importBookId, setImportBookId] = useState<number>(currentBookId);
  const [importing, setImporting] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);

  // 登出
  const [logoutOpen, setLogoutOpen] = useState(false);

  const submitProfile = async () => {
    if (!profileForm.nickname.trim()) { toast.error('昵称不能为空'); return; }
    setProfileSaving(true);
    try {
      const u = await authApi.updateMe({
        nickname: profileForm.nickname.trim(),
        email: profileForm.email || undefined,
        phone: profileForm.phone || undefined,
      });
      setUser(u);
      toast.success('已更新个人信息');
    } finally {
      setProfileSaving(false);
    }
  };

  const submitPassword = async () => {
    if (!pwdForm.old_password) { toast.error('请输入原密码'); return; }
    if (pwdForm.new_password.length < 6) { toast.error('新密码至少 6 位'); return; }
    if (pwdForm.new_password !== pwdForm.confirm_password) { toast.error('两次密码不一致'); return; }
    setPwdSaving(true);
    try {
      await authApi.changePwd({
        old_password: pwdForm.old_password,
        new_password: pwdForm.new_password,
      });
      toast.success('密码已修改，请重新登录');
      setPwdForm({ old_password: '', new_password: '', confirm_password: '' });
      setTimeout(() => doLogout(), 600);
    } finally {
      setPwdSaving(false);
    }
  };

  const submitPrefs = async () => {
    setPrefsSaving(true);
    try {
      // 更新默认账本
      if (prefs.default_book_id) {
        const b = books.find(x => x.id === prefs.default_book_id);
        if (b && !b.is_default) {
          await bookApi.update(b.id, { is_default: true }).catch(() => {});
        }
        setCurrentBook(prefs.default_book_id);
      }
      const u = await authApi.updateMe({
        currency: prefs.currency,
        month_start: prefs.month_start,
        locale: prefs.locale,
        timezone: prefs.timezone,
      });
      setUser(u);
      loadBooks();
      toast.success('偏好已保存');
    } finally {
      setPrefsSaving(false);
    }
  };

  const triggerImport = () => fileInputRef.current?.click();

  const doImport = async () => {
    if (!importBookId) { toast.error('请先选择账本'); return; }
    if (!importFile) { toast.error('请先选择文件'); return; }
    setImporting(true);
    try {
      const r = await ioApi.import(importSource, importBookId, importFile);
      toast.success(
        `导入完成：成功 ${r.created} 条，跳过 ${r.skipped} 条，共 ${r.total} 条`,
        { duration: 4000 }
      );
      setImportFile(null);
      if (fileInputRef.current) fileInputRef.current.value = '';
    } finally {
      setImporting(false);
    }
  };

  const doExport = () => {
    if (!importBookId) { toast.error('请选择账本'); return; }
    ioApi.exportCSV({ book_id: importBookId });
    toast.success('已开始导出，文件即将下载');
  };

  const doLogout = async () => {
    try { await authApi.logout(); } catch (_) { /* ignore */ }
    logoutStore();
    navigate('/login', { replace: true });
  };

  const onFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const f = e.target.files?.[0];
    if (!f) return;
    setImportFile(f);
    toast.success(`已选择文件：${f.name}`);
  };

  return (
    <div className="grid lg:grid-cols-[220px_1fr] gap-5">
      {/* 侧边 Tab */}
      <aside className="card p-2 h-fit sticky top-5">
        <ul className="space-y-1">
          {TABS.map(({ k, label, Icon, badge }) => (
            <li key={k}>
              <button
                onClick={() => setTab(k)}
                className={cn(
                  'w-full flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm transition',
                  tab === k
                    ? 'bg-brand-50 text-brand-700 font-semibold'
                    : 'text-slate-600 hover:bg-slate-50'
                )}
              >
                <Icon size={16} />
                <span className="flex-1 text-left">{label}</span>
                {badge && (
                  <span className="chip bg-amber-50 text-amber-600 !text-[10px]">{badge}</span>
                )}
              </button>
            </li>
          ))}
          <li className="pt-2 mt-2 border-t border-slate-100">
            <button
              onClick={() => setLogoutOpen(true)}
              className="w-full flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm text-red-500 hover:bg-red-50 transition"
            >
              <LogOut size={16} />
              <span>退出登录</span>
            </button>
          </li>
        </ul>
      </aside>

      {/* 内容区 */}
      <div className="space-y-5 min-w-0">
        {/* 个人信息 */}
        {tab === 'profile' && (
          <div className="space-y-5">
            <SectionHeader title="个人信息" subtitle="您的账号资料与显示信息" Icon={UserIcon} />

            <section className="card card-body">
              <div className="flex items-center gap-5 flex-wrap mb-6 pb-5 border-b border-slate-100">
                <div className="w-20 h-20 rounded-2xl bg-gradient-to-br from-brand-100 to-brand-50 grid place-items-center text-3xl font-bold text-brand-700 shadow-sm">
                  {user?.avatar ? (
                    <img src={user.avatar} className="w-full h-full rounded-2xl" alt="" />
                  ) : (
                    (user?.nickname || user?.username || '?').slice(0, 1).toUpperCase()
                  )}
                </div>
                <div>
                  <div className="text-lg font-bold text-slate-800">{user?.nickname}</div>
                  <div className="text-sm text-slate-500 mt-0.5">@{user?.username}</div>
                  {user?.is_vip && (
                    <span className="chip bg-gradient-to-r from-amber-400 to-orange-500 text-white mt-2">
                      ✨ 高级会员
                    </span>
                  )}
                  <div className="text-xs text-slate-400 mt-2">
                    注册于 {user?.created_at ? formatDate(user.created_at, 'YYYY-MM-DD') : '-'}
                  </div>
                </div>
              </div>

              <div className="grid md:grid-cols-2 gap-4">
                <div>
                  <label className="label">昵称 *</label>
                  <input
                    className="input"
                    value={profileForm.nickname}
                    onChange={e => setProfileForm(f => ({ ...f, nickname: e.target.value }))}
                  />
                </div>
                <div>
                  <label className="label">邮箱</label>
                  <input
                    className="input" type="email"
                    placeholder="you@example.com"
                    value={profileForm.email}
                    onChange={e => setProfileForm(f => ({ ...f, email: e.target.value }))}
                  />
                </div>
                <div>
                  <label className="label">手机号</label>
                  <input
                    className="input" type="tel"
                    placeholder="13xxxxxxxxx"
                    value={profileForm.phone}
                    onChange={e => setProfileForm(f => ({ ...f, phone: e.target.value }))}
                  />
                </div>
                <div>
                  <label className="label">头像 URL（可选）</label>
                  <input
                    className="input"
                    placeholder="https://..."
                    value={profileForm.avatar}
                    onChange={e => setProfileForm(f => ({ ...f, avatar: e.target.value }))}
                  />
                </div>
              </div>

              <div className="mt-6 flex justify-end">
                <button
                  className="btn-primary"
                  disabled={profileSaving}
                  onClick={submitProfile}
                >
                  <Check size={16} /> {profileSaving ? '保存中...' : '保存修改'}
                </button>
              </div>
            </section>
          </div>
        )}

        {/* 修改密码 */}
        {tab === 'password' && (
          <div className="space-y-5">
            <SectionHeader title="修改密码" subtitle="建议定期更换密码以保障账号安全" Icon={Lock} />

            <section className="card card-body max-w-xl">
              <div className="space-y-4">
                <div>
                  <label className="label">当前密码 *</label>
                  <input
                    className="input" type="password"
                    placeholder="请输入当前登录密码"
                    value={pwdForm.old_password}
                    onChange={e => setPwdForm(f => ({ ...f, old_password: e.target.value }))}
                  />
                </div>
                <div>
                  <label className="label">新密码 *</label>
                  <input
                    className="input" type="password"
                    placeholder="至少 6 位，建议字母+数字组合"
                    value={pwdForm.new_password}
                    onChange={e => setPwdForm(f => ({ ...f, new_password: e.target.value }))}
                  />
                </div>
                <div>
                  <label className="label">确认新密码 *</label>
                  <input
                    className="input" type="password"
                    placeholder="再次输入新密码"
                    value={pwdForm.confirm_password}
                    onChange={e => setPwdForm(f => ({ ...f, confirm_password: e.target.value }))}
                  />
                  {pwdForm.confirm_password && pwdForm.new_password !== pwdForm.confirm_password && (
                    <p className="mt-1.5 text-xs text-red-500 flex items-center gap-1">
                      <AlertCircle size={12} /> 两次密码输入不一致
                    </p>
                  )}
                </div>

                <div className="p-3 rounded-lg bg-slate-50 text-xs text-slate-500 flex items-start gap-2">
                  <Shield size={14} className="shrink-0 mt-0.5 text-slate-400" />
                  <span>密码长度至少 6 位，建议包含大小写字母、数字与特殊字符。修改成功后将需要重新登录。</span>
                </div>
              </div>

              <div className="mt-6 flex justify-end">
                <button
                  className="btn-primary"
                  disabled={pwdSaving}
                  onClick={submitPassword}
                >
                  <Lock size={16} /> {pwdSaving ? '修改中...' : '确认修改密码'}
                </button>
              </div>
            </section>
          </div>
        )}

        {/* 偏好设置 */}
        {tab === 'prefs' && (
          <div className="space-y-5">
            <SectionHeader title="偏好设置" subtitle="默认账本、币种与语言等个性化设置" Icon={Palette} />

            <section className="card card-body max-w-2xl">
              <div className="grid md:grid-cols-2 gap-4">
                <div className="md:col-span-2">
                  <label className="label">默认账本</label>
                  <select
                    className="input"
                    value={prefs.default_book_id}
                    onChange={e => setPrefs(p => ({ ...p, default_book_id: Number(e.target.value) }))}
                  >
                    {books.map(b => (
                      <option key={b.id} value={b.id}>
                        {b.icon} {b.name}{b.is_default ? '（当前默认）' : ''}
                      </option>
                    ))}
                  </select>
                </div>
                <div>
                  <label className="label">默认币种</label>
                  <select
                    className="input"
                    value={prefs.currency}
                    onChange={e => setPrefs(p => ({ ...p, currency: e.target.value }))}
                  >
                    <option value="CNY">CNY 人民币 (¥)</option>
                    <option value="USD">USD 美元 ($)</option>
                    <option value="EUR">EUR 欧元 (€)</option>
                    <option value="JPY">JPY 日元 (¥)</option>
                    <option value="HKD">HKD 港币 (HK$)</option>
                    <option value="GBP">GBP 英镑 (£)</option>
                  </select>
                </div>
                <div>
                  <label className="label">每月起始日</label>
                  <select
                    className="input"
                    value={prefs.month_start}
                    onChange={e => setPrefs(p => ({ ...p, month_start: Number(e.target.value) }))}
                  >
                    {Array.from({ length: 28 }).map((_, i) => (
                      <option key={i + 1} value={i + 1}>{i + 1} 号</option>
                    ))}
                  </select>
                  <p className="mt-1 text-xs text-slate-400">决定预算与统计的周期起始点</p>
                </div>
                <div>
                  <label className="label">语言</label>
                  <select
                    className="input"
                    value={prefs.locale}
                    onChange={e => setPrefs(p => ({ ...p, locale: e.target.value }))}
                  >
                    <option value="zh-CN">简体中文</option>
                    <option value="zh-TW">繁體中文</option>
                    <option value="en-US">English</option>
                    <option value="ja-JP">日本語</option>
                  </select>
                </div>
                <div>
                  <label className="label">时区</label>
                  <select
                    className="input"
                    value={prefs.timezone}
                    onChange={e => setPrefs(p => ({ ...p, timezone: e.target.value }))}
                  >
                    <option value="Asia/Shanghai">UTC+08:00 北京/上海</option>
                    <option value="Asia/Tokyo">UTC+09:00 东京</option>
                    <option value="Asia/Singapore">UTC+08:00 新加坡</option>
                    <option value="Europe/London">UTC+00:00 伦敦</option>
                    <option value="America/New_York">UTC-05:00 纽约</option>
                    <option value="America/Los_Angeles">UTC-08:00 洛杉矶</option>
                  </select>
                </div>
              </div>

              <div className="mt-6 grid md:grid-cols-2 gap-4 pt-5 border-t border-slate-100">
                <SettingToggle label="通知提醒" hint="账单、预算与到期提醒" />
                <SettingToggle label="深色模式" hint="跟随系统或手动切换" />
                <SettingToggle label="显示动画" hint="页面切换与加载动画" />
                <SettingToggle label="指纹/面容解锁" hint="打开 APP 时验证身份" />
              </div>

              <div className="mt-6 flex justify-end">
                <button
                  className="btn-primary"
                  disabled={prefsSaving}
                  onClick={submitPrefs}
                >
                  <Check size={16} /> {prefsSaving ? '保存中...' : '保存偏好'}
                </button>
              </div>
            </section>
          </div>
        )}

        {/* 导入导出 */}
        {tab === 'io' && (
          <div className="space-y-5">
            <SectionHeader
              title="导入 / 导出数据"
              subtitle="支持微信、支付宝、通用模板格式，快速迁移历史账单"
              Icon={FileText}
            />

            {/* 导出 */}
            <section className="card card-body">
              <div className="flex items-center justify-between mb-3 flex-wrap gap-2">
                <h3 className="font-semibold text-slate-800 flex items-center gap-2">
                  <Download size={18} className="text-emerald-600" /> 导出账单为 CSV
                </h3>
                <button className="btn-secondary btn-sm" onClick={() => ioApi.template()}>
                  <FileText size={14} /> 下载 CSV 模板
                </button>
              </div>
              <div className="grid md:grid-cols-[1fr_auto] gap-4 items-end">
                <div>
                  <label className="label">选择账本</label>
                  <select
                    className="input"
                    value={importBookId}
                    onChange={e => setImportBookId(Number(e.target.value))}
                  >
                    {books.map(b => (
                      <option key={b.id} value={b.id}>{b.icon} {b.name}</option>
                    ))}
                  </select>
                </div>
                <button className="btn-primary" onClick={doExport}>
                  <Download size={16} /> 立即导出
                </button>
              </div>
              <p className="text-xs text-slate-400 mt-3 flex items-start gap-1.5">
                <Info size={12} className="shrink-0 mt-0.5" />
                导出内容包含所选账本的交易流水、分类结构（仅 CSV），可在 Excel/Numbers 打开用于存档。
              </p>
            </section>

            {/* 导入 */}
            <section className="card card-body">
              <h3 className="font-semibold text-slate-800 mb-3 flex items-center gap-2">
                <Upload size={18} className="text-brand-600" /> 导入账单
              </h3>

              <div className="mb-4">
                <label className="label mb-2">选择数据源</label>
                <div className="grid sm:grid-cols-2 lg:grid-cols-4 gap-3">
                  {[
                    { k: 'template', label: '通用 CSV 模板', desc: '适用于模板导入', emoji: '📋' },
                    { k: 'wechat', label: '微信账单', desc: '微信支付导出', emoji: '💬' },
                    { k: 'alipay', label: '支付宝账单', desc: '支付宝导出', emoji: '🅰️' },
                    { k: 'csv', label: '自定义 CSV', desc: '自定义列映射', emoji: '📄' },
                  ].map(s => (
                    <button
                      key={s.k}
                      onClick={() => setImportSource(s.k as any)}
                      className={cn(
                        'p-4 rounded-xl border text-left transition',
                        importSource === s.k
                          ? 'border-brand-500 bg-brand-50 shadow-sm'
                          : 'border-slate-200 hover:bg-slate-50'
                      )}
                    >
                      <div className="text-2xl mb-2">{s.emoji}</div>
                      <div className="font-medium text-slate-800">{s.label}</div>
                      <div className="text-xs text-slate-400 mt-0.5">{s.desc}</div>
                    </button>
                  ))}
                </div>
              </div>

              <div className="grid md:grid-cols-[1fr_auto] gap-4 items-end">
                <div>
                  <label className="label">目标账本</label>
                  <select
                    className="input"
                    value={importBookId}
                    onChange={e => setImportBookId(Number(e.target.value))}
                  >
                    {books.map(b => (
                      <option key={b.id} value={b.id}>{b.icon} {b.name}</option>
                    ))}
                  </select>
                </div>
                <div>
                  <label className="label">&nbsp;</label>
                  <button className="btn-secondary" onClick={triggerImport}>
                    <Upload size={14} /> 选择文件
                  </button>
                  <input
                    ref={fileInputRef}
                    type="file"
                    accept=".csv,.xlsx,.xls"
                    className="hidden"
                    onChange={onFileChange}
                  />
                </div>
              </div>

              {importFile && (
                <div className="mt-4 p-3 rounded-lg bg-emerald-50 border border-emerald-100 flex items-center justify-between gap-3">
                  <div className="text-sm text-emerald-800 truncate min-w-0">
                    <FileText size={14} className="inline mr-1.5 -mt-0.5" />
                    {importFile.name}
                    <span className="text-emerald-600/70 ml-2">
                      ({Math.round(importFile.size / 1024)} KB)
                    </span>
                  </div>
                  <div className="flex gap-2 shrink-0">
                    <button
                      className="btn-ghost btn-sm text-slate-500"
                      onClick={() => {
                        setImportFile(null);
                        if (fileInputRef.current) fileInputRef.current.value = '';
                      }}
                    >
                      <X size={14} /> 取消
                    </button>
                    <button
                      className="btn-primary btn-sm"
                      disabled={importing}
                      onClick={doImport}
                    >
                      {importing ? '导入中...' : <>
                        <Upload size={14} /> 开始导入
                      </>}
                    </button>
                  </div>
                </div>
              )}

              <div className="mt-4 p-4 rounded-xl bg-slate-50 text-xs text-slate-500 space-y-2">
                <div className="font-medium text-slate-700 flex items-center gap-1">
                  <HelpCircle size={14} /> 导入指南
                </div>
                <ul className="list-disc pl-5 space-y-1">
                  <li><b>通用 CSV 模板</b>：推荐方式，先点击"下载 CSV 模板"填写后上传。</li>
                  <li><b>微信账单</b>：在微信【我 → 服务 → 钱包 → 账单 → 常见问题 → 下载账单 → 用于个人对账】获取。</li>
                  <li><b>支付宝账单</b>：在支付宝【我的 → 账单 → 右上角 ... → 开具交易流水证明 → 用于个人对账】获取。</li>
                  <li>系统会自动进行分类匹配，部分无法识别的记录会标记为「未分类」。</li>
                </ul>
              </div>
            </section>
          </div>
        )}

        {/* 关于 */}
        {tab === 'about' && (
          <div className="space-y-5">
            <SectionHeader title="关于" subtitle="版本信息、许可证与反馈渠道" Icon={Info} />

            <section className="card card-body text-center">
              <div className="w-20 h-20 mx-auto rounded-3xl bg-gradient-to-br from-brand-500 via-brand-600 to-purple-600 grid place-items-center text-4xl shadow-lg shadow-brand-200/50">
                🔥
              </div>
              <h2 className="text-2xl font-bold text-slate-800 mt-4">火记 HuoZhi</h2>
              <p className="text-slate-500 mt-1">一个清爽、好用的个人/家庭记账系统</p>
              <div className="mt-5 inline-flex items-center gap-2 chip bg-slate-100 !py-1.5 !px-3">
                <Smartphone size={14} /> 版本号 <b>v1.0.0</b> (build 20250101)
              </div>
            </section>

            <section className="card card-body divide-y divide-slate-100">
              <AboutRow label="更新日志" desc="查看当前版本的新功能与修复" Icon={Bell} />
              <AboutRow label="隐私政策" desc="我们如何收集与使用您的数据" Icon={Shield} />
              <AboutRow label="用户协议" desc="使用本软件前请认真阅读" Icon={Book} />
              <AboutRow label="许可证" desc="基于 MIT 协议开源" Icon={Globe} />
              <AboutRow label="评分支持" desc="给我们一颗 ⭐ 是莫大的鼓励" Icon={Heart} accent />
              <AboutRow label="反馈建议" desc="遇到问题或有新想法？请告诉我们" Icon={HelpCircle} />
            </section>

            <section className="card card-body">
              <h3 className="font-semibold text-slate-800 mb-3">感谢您的使用 ❤️</h3>
              <p className="text-sm text-slate-500 leading-relaxed">
                火记的初衷是做一款足够轻便、数据归自己掌控的记账工具。无论您是在记录日常开支、家庭账本共享，还是为了实现某个存钱小目标，
                希望我们都能陪伴您一路。如果产品有任何不足之处，欢迎通过反馈告诉我们，我们会认真对待每一条建议。
              </p>
              <div className="mt-5 flex items-center justify-between text-xs text-slate-400">
                <span>© 2025 HuoZhi Team</span>
                <span>Made with ❤️ in Shanghai</span>
              </div>
            </section>
          </div>
        )}
      </div>

      <ConfirmDialog
        open={logoutOpen}
        onClose={() => setLogoutOpen(false)}
        onConfirm={doLogout}
        title="退出登录"
        desc="确定要退出当前账号吗？您可以随时重新登录查看账本数据。"
        okText="退出"
        danger
      />
    </div>
  );
}

function SectionHeader({ title, subtitle, Icon }: { title: string; subtitle?: string; Icon: any }) {
  return (
    <div className="flex items-center gap-3 pb-1">
      <div className="w-10 h-10 rounded-xl bg-brand-50 text-brand-600 grid place-items-center">
        <Icon size={20} />
      </div>
      <div className="flex-1">
        <h2 className="font-bold text-slate-800 text-lg leading-tight">{title}</h2>
        {subtitle && <p className="text-sm text-slate-400 mt-0.5">{subtitle}</p>}
      </div>
    </div>
  );
}

function SettingToggle({ label, hint }: { label: string; hint?: string }) {
  const [on, setOn] = useState(false);
  return (
    <label className="flex items-start justify-between p-3 rounded-xl border border-slate-100 hover:bg-slate-50 transition cursor-pointer">
      <div>
        <div className="text-sm font-medium text-slate-700">{label}</div>
        {hint && <div className="text-xs text-slate-400 mt-0.5">{hint}</div>}
      </div>
      <button
        type="button"
        onClick={() => setOn(v => !v)}
        className={cn(
          'relative w-11 h-6 rounded-full transition shrink-0 mt-0.5',
          on ? 'bg-brand-600' : 'bg-slate-200'
        )}
      >
        <span className={cn(
          'absolute top-0.5 w-5 h-5 rounded-full bg-white shadow transition-all',
          on ? 'left-[22px]' : 'left-0.5'
        )} />
      </button>
    </label>
  );
}

function AboutRow({
  label, desc, Icon, accent,
}: { label: string; desc?: string; Icon: any; accent?: boolean }) {
  return (
    <div className="flex items-center gap-3 py-3 cursor-pointer hover:bg-slate-50 -mx-5 px-5 transition">
      <div className={cn(
        'w-9 h-9 rounded-lg grid place-items-center shrink-0',
        accent ? 'bg-red-50 text-red-500' : 'bg-slate-100 text-slate-500'
      )}>
        <Icon size={16} />
      </div>
      <div className="flex-1 min-w-0">
        <div className="text-sm font-medium text-slate-800">{label}</div>
        {desc && <div className="text-xs text-slate-400 mt-0.5">{desc}</div>}
      </div>
      <ChevronRight size={16} className="text-slate-300 shrink-0" />
    </div>
  );
}

// 复用 Modal 引用（保留 import 用途）
void Modal;
