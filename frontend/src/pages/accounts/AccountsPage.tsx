import { useEffect, useMemo, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { toast } from 'sonner';
import { useAppStore } from '@/stores/app';
import { accountApi, statsApi } from '@/api';
import type { Account, AccountType, AccountSummary, AssetPoint } from '@/types';
import { formatMoney, cn, pct } from '@/utils';
import {
  Plus, Edit3, Archive, TrendingUp, Wallet, CreditCard, Landmark, PiggyBank,
  Settings as SettingsIcon, ChevronDown, Check, X, TrendingDown, Eye, EyeOff,
} from 'lucide-react';
import { Modal, ConfirmDialog, Empty } from '@/components/common';
import { ResponsiveContainer, LineChart, Line, XAxis, YAxis, Tooltip, CartesianGrid, Area, AreaChart } from 'recharts';

const TYPE_FILTERS: Array<{ k: AccountType | 'all'; label: string; icon: string }> = [
  { k: 'all', label: '全部', icon: '👛' },
  { k: 'cash', label: '现金', icon: '💵' },
  { k: 'bank', label: '储蓄', icon: '🏦' },
  { k: 'credit', label: '信用卡', icon: '💳' },
  { k: 'prepaid', label: '储值卡', icon: '🎟️' },
  { k: 'investment', label: '投资', icon: '📈' },
  { k: 'liability', label: '负债', icon: '💸' },
  { k: 'virtual', label: '虚拟', icon: '📱' },
];

const TYPE_LABELS: Record<AccountType, string> = {
  cash: '现金', bank: '储蓄卡', credit: '信用卡', prepaid: '储值卡',
  investment: '投资账户', liability: '负债', virtual: '虚拟账户',
};

const TYPE_ICONS: Record<AccountType, string> = {
  cash: '💵', bank: '🏦', credit: '💳', prepaid: '🎟️',
  investment: '📈', liability: '💸', virtual: '📱',
};

const PRESET_COLORS = ['#6366F1', '#10B981', '#F59E0B', '#EF4444', '#8B5CF6', '#EC4899', '#06B6D4', '#F97316'];

// 国内主流银行（按使用频率排序）
const POPULAR_BANKS = [
  '招商银行', '中国工商银行', '中国建设银行', '中国农业银行', '中国银行',
  '交通银行', '浦发银行', '中信银行', '兴业银行', '民生银行',
  '光大银行', '平安银行', '华夏银行', '广发银行', '浙商银行',
  '渤海银行', '恒丰银行', '中国邮政储蓄银行', '北京银行', '上海银行',
  '宁波银行', '江苏银行', '南京银行', '杭州银行', '微众银行',
  '网商银行', '新网银行', '其他',
];

export default function AccountsPage() {
  const bookId = useAppStore(s => s.currentBookId);
  const storeAccounts = useAppStore(s => s.accounts);
  const loadDicts = useAppStore(s => s.loadDictionaries);
  const [searchParams, setSearchParams] = useSearchParams();

  const [summary, setSummary] = useState<AccountSummary | null>(null);
  const [list, setList] = useState<Account[]>([]);
  const [timeline, setTimeline] = useState<AssetPoint[]>([]);
  const [loading, setLoading] = useState(false);
  const [showArchived, setShowArchived] = useState(false);
  const [filterType, setFilterType] = useState<AccountType | 'all'>('all');

  // 新增/编辑弹窗
  const [editOpen, setEditOpen] = useState(false);
  const [editItem, setEditItem] = useState<Account | null>(null);
  const [editForm, setEditForm] = useState(blankForm());

  // 余额调整弹窗
  const [adjustOpen, setAdjustOpen] = useState(false);
  const [adjustItem, setAdjustItem] = useState<Account | null>(null);
  const [adjustAmount, setAdjustAmount] = useState('');
  const [adjustRemark, setAdjustRemark] = useState('');

  const [archiveTarget, setArchiveTarget] = useState<Account | null>(null);

  // 按需解密的完整卡信息缓存（accountId -> { full_card_no, cvv, expire_month, expire_year }）
  const [revealedCards, setRevealedCards] = useState<Record<number, {
    full_card_no: string; cvv: string; expire_month: number; expire_year: number;
  }>>({});

  const toggleRevealCardNo = async (acc: Account) => {
    if (revealedCards[acc.id]) {
      setRevealedCards(prev => { const n = { ...prev }; delete n[acc.id]; return n; });
      return;
    }
    try {
      const data = await accountApi.getFullCardNo(acc.id);
      if (data) setRevealedCards(prev => ({ ...prev, [acc.id]: data as any }));
      else toast.error('该账户未保存完整卡信息');
    } catch {
      toast.error('该账户未保存完整卡信息');
    }
  };

  function blankForm() {
    return {
      name: '', type: 'bank' as AccountType, currency: 'CNY',
      balance: '0', initial_amount: '0',
      color: PRESET_COLORS[0], icon: '',
      bank_name: '', card_no4: '', full_card_no: '', cvv: '',
      expire_month: '', expire_year: '',
      credit_limit: '', bill_day: '', repay_day: '',
      include_in_total: true, include_in_budget: true,
      remark: '',
    };
  }

  const load = async () => {
    if (!bookId) return;
    setLoading(true);
    try {
      const [accData, tl] = await Promise.all([
        accountApi.list({ book_id: bookId, include_archived: showArchived ? 1 : 0 }),
        statsApi.timeline().catch(() => [] as AssetPoint[]),
      ]);
      setList(accData.accounts || []);
      setSummary(accData.summary || null);
      setTimeline(tl);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    load();
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [bookId, showArchived]);

  const filteredList = useMemo(() => {
    let arr = list;
    if (filterType !== 'all') arr = arr.filter(a => a.type === filterType);
    return arr.sort((a, b) => a.sort - b.sort);
  }, [list, filterType]);

  const openCreate = () => {
    setEditItem(null);
    setEditForm(blankForm());
    setEditOpen(true);
  };
  const openEdit = (a: Account) => {
    setEditItem(a);
    setEditForm({
      name: a.name, type: a.type, currency: a.currency,
      balance: String(a.balance),
      initial_amount: String(a.initial_amount),
      color: a.color || PRESET_COLORS[0], icon: a.icon || '',
      bank_name: a.bank_name || '', card_no4: a.card_no4 || '', full_card_no: '', cvv: '',
      expire_month: a.expire_month ? String(a.expire_month) : '',
      expire_year: a.expire_year ? String(a.expire_year) : '',
      credit_limit: a.credit_limit != null ? String(a.credit_limit) : '',
      bill_day: a.bill_day != null ? String(a.bill_day) : '',
      repay_day: a.repay_day != null ? String(a.repay_day) : '',
      include_in_total: a.include_in_total,
      include_in_budget: a.include_in_budget,
      remark: a.remark || '',
    });
    setEditOpen(true);
  };

  // 从「我的银行卡」点击某张卡跳转过来时，自动打开对应账户的编辑弹窗
  useEffect(() => {
    const accId = searchParams.get('account');
    if (!accId) return;
    const id = Number(accId);
    if (!Number.isFinite(id)) return;
    const target = list.find(a => a.id === id);
    if (target) {
      openEdit(target);
      searchParams.delete('account');
      setSearchParams(searchParams, { replace: true });
    }
  }, [list, searchParams, setSearchParams, openEdit]);

  const submitEdit = async () => {
    if (!editForm.name.trim()) { toast.error('请输入账户名称'); return; }
    const payload = {
      book_id: bookId,
      name: editForm.name.trim(),
      type: editForm.type,
      currency: editForm.currency,
      balance: parseFloat(editForm.balance || '0'),
      initial_amount: parseFloat(editForm.initial_amount || '0'),
      color: editForm.color,
      icon: editForm.icon,
      bank_name: editForm.bank_name || undefined,
      card_no4: editForm.card_no4 || undefined,
      full_card_no: editForm.full_card_no?.replace(/\s/g, '') || undefined,
      cvv: editForm.cvv?.replace(/\D/g, '') || undefined,
      expire_month: editForm.expire_month ? Number(editForm.expire_month) : undefined,
      expire_year: editForm.expire_year ? Number(editForm.expire_year) : undefined,
      credit_limit: editForm.credit_limit ? parseFloat(editForm.credit_limit) : undefined,
      bill_day: editForm.bill_day ? Number(editForm.bill_day) : undefined,
      repay_day: editForm.repay_day ? Number(editForm.repay_day) : undefined,
      include_in_total: editForm.include_in_total,
      include_in_budget: editForm.include_in_budget,
      remark: editForm.remark || undefined,
    };
    if (editItem) {
      await accountApi.update(editItem.id, payload);
      toast.success('已更新账户');
    } else {
      await accountApi.create(payload);
      toast.success('已创建账户');
    }
    setEditOpen(false);
    loadDicts(bookId);
    load();
  };

  const submitAdjust = async () => {
    if (!adjustItem) return;
    const amount = parseFloat(adjustAmount);
    if (isNaN(amount)) { toast.error('请输入正确的金额'); return; }
    await accountApi.adjust(adjustItem.id, {
      new_balance: amount, remark: adjustRemark || undefined,
    });
    toast.success('余额已调整');
    setAdjustOpen(false);
    setAdjustAmount('');
    setAdjustRemark('');
    loadDicts(bookId);
    load();
  };

  const doArchive = async () => {
    if (!archiveTarget) return;
    await accountApi.update(archiveTarget.id, { is_archived: !archiveTarget.is_archived });
    toast.success(archiveTarget.is_archived ? '已取消归档' : '已归档');
    setArchiveTarget(null);
    loadDicts(bookId);
    load();
  };

  return (
    <div className="space-y-5">
      {/* 顶部资产总览卡片 */}
      <section className="rounded-2xl bg-gradient-to-br from-slate-800 via-slate-700 to-slate-900 p-6 text-white shadow-soft relative overflow-hidden">
        <div className="absolute -right-16 -top-16 w-64 h-64 bg-white/5 rounded-full blur-2xl" />
        <div className="relative">
          <div className="flex items-center gap-2 text-white/60 text-sm">
            <PiggyBank size={16} /> 资产净值
          </div>
          <div className="text-4xl font-bold tabular-nums mt-2 tracking-tight">
            {formatMoney(summary?.net_asset || 0)}
          </div>
          <div className="mt-4 grid grid-cols-3 gap-4 text-sm">
            <div>
              <div className="text-white/60 flex items-center gap-1">
                <Landmark size={14} /> 总资产
              </div>
              <div className="text-lg font-semibold tabular-nums mt-0.5">
                {formatMoney(summary?.total_asset || 0)}
              </div>
            </div>
            <div>
              <div className="text-white/60 flex items-center gap-1">
                <TrendingDown size={14} /> 总负债
              </div>
              <div className="text-lg font-semibold tabular-nums mt-0.5 text-red-300">
                {formatMoney(summary?.total_debt || 0)}
              </div>
            </div>
            <div>
              <div className="text-white/60 flex items-center gap-1">
                <TrendingUp size={14} /> 现金流
              </div>
              <div className={cn(
                'text-lg font-semibold tabular-nums mt-0.5',
                (summary?.cash_flow || 0) >= 0 ? 'text-emerald-300' : 'text-red-300'
              )}>
                {(summary?.cash_flow || 0) >= 0 ? '+' : ''}{formatMoney(summary?.cash_flow || 0)}
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* 资产净值曲线 */}
      <section className="card card-body">
        <div className="flex items-center justify-between mb-4">
          <h3 className="font-semibold text-slate-800 flex items-center gap-2">
            <TrendingUp size={16} className="text-brand-600" /> 资产净值趋势
          </h3>
          <span className="text-xs text-slate-400">近 12 个月</span>
        </div>
        {timeline.length === 0 ? (
          <Empty text="暂无历史数据" />
        ) : (
          <div className="h-56">
            <ResponsiveContainer>
              <AreaChart data={timeline}>
                <defs>
                  <linearGradient id="assetGrad" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="5%" stopColor="#6366F1" stopOpacity={0.25} />
                    <stop offset="95%" stopColor="#6366F1" stopOpacity={0} />
                  </linearGradient>
                </defs>
                <CartesianGrid strokeDasharray="3 3" stroke="#f1f5f9" />
                <XAxis dataKey="month" tick={{ fontSize: 11, fill: '#94a3b8' }} />
                <YAxis tick={{ fontSize: 11, fill: '#94a3b8' }} />
                <Tooltip
                  formatter={(v) => formatMoney(Number(v) || 0)}
                  contentStyle={{ borderRadius: 10, border: '1px solid #e2e8f0' }}
                />
                <Area type="monotone" dataKey="net_asset"
                  stroke="#6366F1" strokeWidth={2.5} fill="url(#assetGrad)" />
              </AreaChart>
            </ResponsiveContainer>
          </div>
        )}
      </section>

      {/* 筛选器 + 新增 */}
      <section className="card card-body">
        <div className="flex items-center justify-between mb-3 flex-wrap gap-2">
          <div className="flex items-center gap-2 overflow-x-auto no-scrollbar">
            {TYPE_FILTERS.map(t => (
              <button
                key={t.k}
                onClick={() => setFilterType(t.k)}
                className={cn(
                  'shrink-0 px-3 py-1.5 rounded-full text-sm font-medium transition whitespace-nowrap',
                  filterType === t.k
                    ? 'bg-brand-600 text-white'
                    : 'bg-slate-100 text-slate-600 hover:bg-slate-200'
                )}
              >
                <span className="mr-1">{t.icon}</span>{t.label}
              </button>
            ))}
          </div>
          <div className="flex items-center gap-2">
            <button
              className="btn-secondary btn-sm"
              onClick={() => setShowArchived(v => !v)}
            >
              <Archive size={14} /> {showArchived ? '隐藏归档' : '显示归档'}
            </button>
            <button className="btn-primary btn-sm" onClick={openCreate}>
              <Plus size={14} /> 新增账户
            </button>
          </div>
        </div>
      </section>

      {/* 账户列表 */}
      <section className="grid sm:grid-cols-2 lg:grid-cols-3 gap-4">
        {loading ? (
          <div className="col-span-full py-16 text-center text-slate-400 text-sm">加载中...</div>
        ) : filteredList.length === 0 ? (
          <div className="col-span-full">
            <Empty text="还没有账户，点击右上角添加" icon={<Wallet size={32} />} />
          </div>
        ) : (
          filteredList.map(a => (
            <div
              key={a.id}
              className={cn(
                'card card-body relative group transition hover:-translate-y-0.5 hover:shadow-lg',
                a.is_archived && 'opacity-60'
              )}
            >
              {a.is_archived && (
                <span className="absolute top-3 right-3 chip bg-slate-100 text-slate-500">已归档</span>
              )}
              <div className="flex items-start justify-between">
                <div className="flex items-center gap-3">
                  <div
                    className="w-12 h-12 rounded-xl grid place-items-center text-2xl"
                    style={{ background: (a.color || '#6366F1') + '15' }}
                  >
                    {TYPE_ICONS[a.type]}
                  </div>
                  <div>
                    <div className="font-semibold text-slate-800">{a.name}</div>
                    <div className="text-xs text-slate-400 mt-0.5 flex items-center gap-1 flex-wrap">
                      <CreditCard size={12} /> {TYPE_LABELS[a.type]}
                      {a.bank_name && ` · ${a.bank_name}`}
                      {a.card_no4 && (
                        <>
                          <span className="ml-1 flex items-center gap-1">
                            {revealedCards[a.id]?.full_card_no ? (
                              <span className="font-mono tracking-wider text-slate-600">
                                {revealedCards[a.id].full_card_no.replace(/(.{4})/g, '$1 ').trim()}
                              </span>
                            ) : (
                              <span className="font-mono tracking-wider">** ** **** {a.card_no4}</span>
                            )}
                            <button
                              type="button"
                              className="p-0.5 hover:text-brand-600 transition rounded"
                              onClick={() => toggleRevealCardNo(a)}
                              title={revealedCards[a.id] ? '隐藏完整卡信息' : '查看完整卡信息'}
                            >
                              {revealedCards[a.id] ? <EyeOff size={11} /> : <Eye size={11} />}
                            </button>
                          </span>
                        </>
                      )}
                      {(a.expire_month ?? 0) > 0 && (a.expire_year ?? 0) > 0 && (
                        <span className="font-mono text-slate-500 ml-1">
                          · 有效 {String(a.expire_month).padStart(2, '0')}/{String(a.expire_year).padStart(2, '0')}
                        </span>
                      )}
                      {revealedCards[a.id]?.cvv && (
                        <span className="font-mono text-slate-500 ml-1">· CVV {revealedCards[a.id].cvv}</span>
                      )}
                    </div>
                  </div>
                </div>
              </div>
              <div className="mt-5">
                <div className="text-xs text-slate-500">
                  {a.type === 'credit' || a.type === 'liability'
                    ? (a.balance > 0 ? '应还' : a.balance < 0 ? '溢缴' : '应还')
                    : '余额'}
                </div>
                <div className={cn(
                  'text-2xl font-bold tabular-nums mt-1',
                  a.type === 'credit' || a.type === 'liability'
                    ? (a.balance > 0 ? 'text-red-500' : a.balance < 0 ? 'text-emerald-500' : 'text-slate-800')
                    : (a.balance < 0 ? 'text-red-500' : 'text-slate-800')
                )}>
                  {formatMoney(a.balance)}
                </div>
                {a.type === 'credit' && a.credit_limit ? (
                  <div className="text-xs text-slate-400 mt-0.5">
                    可用额度 {formatMoney(Math.max(0, (a.credit_limit || 0) - a.balance))}
                  </div>
                ) : null}
              </div>
              {a.type === 'credit' && a.credit_limit ? (
                <div className="mt-4">
                  <div className="flex justify-between text-xs text-slate-500 mb-1">
                    <span>额度使用 {pct(Math.max(0, a.balance), a.credit_limit)}%</span>
                    <span>{formatMoney(Math.max(0, a.balance))} / {formatMoney(a.credit_limit)}</span>
                  </div>
                  <div className="w-full h-1.5 bg-slate-100 rounded-full overflow-hidden">
                    <div
                      className="h-full rounded-full bg-brand-500"
                      style={{ width: `${pct(Math.max(0, a.balance), a.credit_limit)}%` }}
                    />
                  </div>
                  {(a.bill_day || a.repay_day) && (
                    <div className="mt-2 flex gap-3 text-[11px] text-slate-400">
                      {a.bill_day && <span>账单日 {a.bill_day}号</span>}
                      {a.repay_day && <span>还款日 {a.repay_day}号</span>}
                    </div>
                  )}
                </div>
              ) : null}
              <div className="mt-4 pt-3 border-t border-slate-100 flex items-center gap-1">
                <button className="btn-ghost btn-sm flex-1" onClick={() => openEdit(a)}>
                  <Edit3 size={14} /> 编辑
                </button>
                <button
                  className="btn-ghost btn-sm flex-1 text-brand-600"
                  onClick={() => { setAdjustItem(a); setAdjustAmount(String(a.balance)); setAdjustOpen(true); }}
                >
                  <Wallet size={14} /> 调余额
                </button>
                <button
                  className="btn-ghost btn-sm flex-1 text-slate-500"
                  onClick={() => setArchiveTarget(a)}
                >
                  <Archive size={14} /> {a.is_archived ? '取消' : '归档'}
                </button>
              </div>
            </div>
          ))
        )}
      </section>

      {/* 新增/编辑 账户弹窗 */}
      <Modal
        open={editOpen}
        onClose={() => setEditOpen(false)}
        title={editItem ? '编辑账户' : '新增账户'}
        footer={
          <>
            <button className="btn-secondary" onClick={() => setEditOpen(false)}>取消</button>
            <button className="btn-primary" onClick={submitEdit}>
              <Check size={16} /> 保存
            </button>
          </>
        }
      >
        <div className="space-y-3">
          <div>
            <label className="label">账户名称 *</label>
            <input
              className="input"
              placeholder="如：招商银行储蓄卡"
              value={editForm.name}
              onChange={e => setEditForm(f => ({ ...f, name: e.target.value }))}
            />
          </div>
          <div>
            <label className="label">类型 *</label>
            <div className="grid grid-cols-4 gap-2">
              {(Object.keys(TYPE_LABELS) as AccountType[]).map(t => (
                <button
                  key={t}
                  onClick={() => setEditForm(f => ({ ...f, type: t }))}
                  className={cn(
                    'flex flex-col items-center gap-1 py-2 rounded-lg border transition',
                    editForm.type === t
                      ? 'border-brand-500 bg-brand-50 text-brand-700'
                      : 'border-slate-200 text-slate-500 hover:bg-slate-50'
                  )}
                >
                  <span className="text-xl">{TYPE_ICONS[t]}</span>
                  <span className="text-xs">{TYPE_LABELS[t]}</span>
                </button>
              ))}
            </div>
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="label">初始余额</label>
              <input
                className="input"
                type="number"
                value={editForm.initial_amount}
                onChange={e => setEditForm(f => ({ ...f, initial_amount: e.target.value }))}
              />
            </div>
            <div>
              <label className="label">当前余额</label>
              <input
                className="input"
                type="number"
                value={editForm.balance}
                onChange={e => setEditForm(f => ({ ...f, balance: e.target.value }))}
              />
            </div>
          </div>
          <div>
            <label className="label">颜色</label>
            <div className="flex gap-2 flex-wrap">
              {PRESET_COLORS.map(c => (
                <button
                  key={c}
                  onClick={() => setEditForm(f => ({ ...f, color: c }))}
                  className={cn(
                    'w-8 h-8 rounded-full transition',
                    editForm.color === c && 'ring-2 ring-offset-2 ring-slate-300'
                  )}
                  style={{ background: c }}
                />
              ))}
            </div>
          </div>

          {(editForm.type === 'bank' || editForm.type === 'credit') && (
            <div className="pt-2 border-t border-slate-100 space-y-3">
              <div>
                <label className="label">银行名称</label>
                <BankCombobox
                  value={editForm.bank_name}
                  onChange={v => setEditForm(f => ({ ...f, bank_name: v }))}
                />
              </div>
              <div>
                <label className="label flex items-center gap-1">
                  完整卡号
                  <span className="text-[11px] font-normal text-slate-400">· 加密存储 · 可留空</span>
                </label>
                <input
                  className="input font-mono tracking-wider"
                  placeholder="如 6222 **** **** 1234"
                  maxLength={26}
                  value={editForm.full_card_no}
                  onChange={e => {
                    const raw = e.target.value.replace(/\D/g, '').slice(0, 19);
                    // 每 4 位加一个空格
                    const formatted = raw.replace(/(\d{4})(?=\d)/g, '$1 ');
                    setEditForm(f => ({ ...f, full_card_no: formatted }));
                    // 自动同步尾号
                    if (raw.length >= 4) {
                      setEditForm(f => ({ ...f, card_no4: raw.slice(-4) }));
                    }
                  }}
                />
                {editForm.full_card_no && !isValidLuhn(editForm.full_card_no.replace(/\s/g, '')) && (
                  <div className="text-[11px] text-amber-600 mt-1">卡号校验位未通过，可能输入有误</div>
                )}
              </div>
              <div>
                <label className="label">尾号 4 位 {editForm.full_card_no && <span className="text-[11px] text-slate-400 font-normal">（已从完整卡号自动填充）</span>}</label>
                <input
                  className="input font-mono"
                  placeholder="如 1234"
                  maxLength={4}
                  value={editForm.card_no4}
                  onChange={e => setEditForm(f => ({ ...f, card_no4: e.target.value.replace(/\D/g, '') }))}
                />
              </div>
            </div>
          )}
          {editForm.type === 'credit' && (
            <div className="pt-2 border-t border-slate-100 space-y-3">
              <div className="grid grid-cols-3 gap-3">
                <div>
                  <label className="label">信用额度</label>
                  <input
                    className="input"
                    type="number"
                    value={editForm.credit_limit}
                    onChange={e => setEditForm(f => ({ ...f, credit_limit: e.target.value }))}
                  />
                </div>
                <div>
                  <label className="label">账单日</label>
                  <input
                    className="input"
                    type="number" min={1} max={31}
                    placeholder="1-31"
                    value={editForm.bill_day}
                    onChange={e => setEditForm(f => ({ ...f, bill_day: e.target.value }))}
                  />
                </div>
                <div>
                  <label className="label">还款日</label>
                  <input
                    className="input"
                    type="number" min={1} max={31}
                    placeholder="1-31"
                    value={editForm.repay_day}
                    onChange={e => setEditForm(f => ({ ...f, repay_day: e.target.value }))}
                  />
                </div>
              </div>
              <div className="grid grid-cols-3 gap-3">
                <div>
                  <label className="label flex items-center gap-1">
                    卡有效期
                    <span className="text-[11px] text-slate-400 font-normal">MM / YY</span>
                  </label>
                  <div className="flex items-center gap-2">
                    <input
                      className="input text-center font-mono"
                      type="number" min={1} max={12}
                      placeholder="MM"
                      maxLength={2}
                      value={editForm.expire_month}
                      onChange={e => setEditForm(f => ({ ...f, expire_month: e.target.value.replace(/\D/g, '').slice(0, 2) }))}
                    />
                    <span className="text-slate-400">/</span>
                    <input
                      className="input text-center font-mono"
                      type="number" min={0} max={99}
                      placeholder="YY"
                      maxLength={2}
                      value={editForm.expire_year}
                      onChange={e => setEditForm(f => ({ ...f, expire_year: e.target.value.replace(/\D/g, '').slice(0, 2) }))}
                    />
                  </div>
                </div>
                <div>
                  <label className="label flex items-center gap-1">
                    CVV2/CVC2
                    <span className="text-[11px] text-slate-400 font-normal">· 加密存储</span>
                  </label>
                  <input
                    className="input font-mono tracking-widest"
                    type="password"
                    placeholder="卡背面3位"
                    maxLength={4}
                    value={editForm.cvv}
                    onChange={e => setEditForm(f => ({ ...f, cvv: e.target.value.replace(/\D/g, '').slice(0, 4) }))}
                  />
                </div>
                <div>
                  <label className="label">&nbsp;</label>
                  <div className="text-[11px] text-slate-400 leading-relaxed">
                    有效期明文存便于筛选；CVV 后端 AES-GCM 加密存，默认不返回前端。
                  </div>
                </div>
              </div>
            </div>
          )}

          <div className="pt-2 border-t border-slate-100 space-y-2">
            <label className="flex items-center justify-between">
              <span className="text-sm text-slate-700">计入资产总计</span>
              <input
                type="checkbox" className="w-4 h-4 accent-brand-600"
                checked={editForm.include_in_total}
                onChange={e => setEditForm(f => ({ ...f, include_in_total: e.target.checked }))}
              />
            </label>
            <label className="flex items-center justify-between">
              <span className="text-sm text-slate-700">计入预算统计</span>
              <input
                type="checkbox" className="w-4 h-4 accent-brand-600"
                checked={editForm.include_in_budget}
                onChange={e => setEditForm(f => ({ ...f, include_in_budget: e.target.checked }))}
              />
            </label>
          </div>
          <div>
            <label className="label">备注</label>
            <input
              className="input"
              placeholder="可选"
              value={editForm.remark}
              onChange={e => setEditForm(f => ({ ...f, remark: e.target.value }))}
            />
          </div>
        </div>
      </Modal>

      {/* 余额调整弹窗 */}
      <Modal
        open={adjustOpen}
        onClose={() => setAdjustOpen(false)}
        title={`调整余额 - ${adjustItem?.name}`}
        footer={
          <>
            <button className="btn-secondary" onClick={() => setAdjustOpen(false)}>取消</button>
            <button className="btn-primary" onClick={submitAdjust}>
              <Check size={16} /> 确认调整
            </button>
          </>
        }
      >
        <div className="space-y-3">
          <div>
            <label className="label">
              {adjustItem && (adjustItem.type === 'credit' || adjustItem.type === 'liability') ? '当前应还' : '当前余额'}
            </label>
            <div className="text-lg font-semibold tabular-nums text-slate-700">
              {formatMoney(adjustItem?.balance || 0)}
            </div>
          </div>
          <div>
            <label className="label">
              {adjustItem && (adjustItem.type === 'credit' || adjustItem.type === 'liability') ? '新的应还' : '新的余额'}
            </label>
            <input
              className="input !text-xl !font-semibold"
              type="number"
              placeholder="0.00"
              value={adjustAmount}
              onChange={e => setAdjustAmount(e.target.value)}
            />
          </div>
          {adjustItem && !isNaN(parseFloat(adjustAmount)) && (
            <div className={cn(
              'p-3 rounded-lg text-sm',
              parseFloat(adjustAmount) > adjustItem.balance ? 'bg-emerald-50 text-emerald-700' :
              parseFloat(adjustAmount) < adjustItem.balance ? 'bg-red-50 text-red-700' :
              'bg-slate-50 text-slate-500'
            )}>
              差额：
              <b className="tabular-nums ml-1">
                {parseFloat(adjustAmount) > adjustItem.balance ? '+' : ''}
                {formatMoney(parseFloat(adjustAmount) - adjustItem.balance)}
              </b>
              （系统将生成一条调整流水）
            </div>
          )}
          <div>
            <label className="label">调整备注（可选）</label>
            <input
              className="input"
              placeholder="为什么要调整？"
              value={adjustRemark}
              onChange={e => setAdjustRemark(e.target.value)}
            />
          </div>
        </div>
      </Modal>

      <ConfirmDialog
        open={!!archiveTarget}
        onClose={() => setArchiveTarget(null)}
        onConfirm={doArchive}
        title={archiveTarget?.is_archived ? '取消归档账户' : '归档账户'}
        desc={
          archiveTarget?.is_archived
            ? `确定要取消归档账户「${archiveTarget.name}」吗？`
            : `归档账户「${archiveTarget?.name}」后，它将不再出现在记账默认列表中，但历史数据保留。`
        }
      />
    </div>
  );
}

// ========= 辅助：Luhn 校验 =========
function isValidLuhn(num: string): boolean {
  if (!num || !/^\d+$/.test(num)) return false;
  if (num.length < 13 || num.length > 19) return false;
  let sum = 0;
  let shouldDouble = false;
  for (let i = num.length - 1; i >= 0; i--) {
    let d = parseInt(num[i], 10);
    if (shouldDouble) { d *= 2; if (d > 9) d -= 9; }
    sum += d;
    shouldDouble = !shouldDouble;
  }
  return sum % 10 === 0;
}

// ========= 辅助：银行名下拉选择器 =========
function BankCombobox({ value, onChange }: { value: string; onChange: (v: string) => void }) {
  const [open, setOpen] = useState(false);
  const ref = (e: HTMLDivElement | null) => {
    // click outside
  };
  const filtered = POPULAR_BANKS.filter(b => b.toLowerCase().includes(value.toLowerCase())).slice(0, 12);
  return (
    <div ref={ref} className="relative">
      <div className="relative">
        <input
          className="input pr-8"
          placeholder="搜索或选择银行..."
          value={value}
          onFocus={() => setOpen(true)}
          onChange={e => { onChange(e.target.value); setOpen(true); }}
        />
        <ChevronDown size={16} className="absolute right-2 top-1/2 -translate-y-1/2 text-slate-400 pointer-events-none" />
      </div>
      {open && (
        <>
          <div className="fixed inset-0 z-10" onClick={() => setOpen(false)} />
          <div className="absolute left-0 right-0 mt-1 z-20 bg-white border border-slate-200 rounded-lg shadow-lg max-h-64 overflow-y-auto">
            {filtered.length === 0 && (
              <div className="px-3 py-2 text-xs text-slate-400">无匹配，可直接输入</div>
            )}
            {filtered.map(b => (
              <button
                key={b}
                type="button"
                className={cn(
                  'w-full text-left px-3 py-2 text-sm hover:bg-slate-50 transition',
                  b === value && 'bg-brand-50 text-brand-600',
                )}
                onClick={() => { onChange(b); setOpen(false); }}
              >
                {b}
              </button>
            ))}
          </div>
        </>
      )}
    </div>
  );
}
