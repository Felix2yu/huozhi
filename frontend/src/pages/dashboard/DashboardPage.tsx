import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { useAppStore } from '@/stores/app';
import { statsApi, txApi, budgetApi, creditApi } from '@/api';
import type { AssetOverview, StatisticsData, BudgetView, TrendPoint, CreditRepayItem } from '@/types';
import { formatMoney, formatDate, getMonthRange, pct } from '@/utils';
import {
  ArrowUpRight, ArrowDownRight, Wallet, PiggyBank, PieChart as PieIcon,
  Plus, ChevronRight, Receipt, Target, CalendarDays, CreditCard, AlertCircle, Clock,
} from 'lucide-react';
import { AmountBadge, Empty, Progress } from '@/components/common';
import {
  ResponsiveContainer, PieChart, Pie, Cell, Tooltip,
  BarChart, Bar, XAxis, YAxis, CartesianGrid, LineChart, Line,
} from 'recharts';

export default function DashboardPage() {
  const bookId = useAppStore(s => s.currentBookId);
  const [asset, setAsset] = useState<AssetOverview | null>(null);
  const [stats, setStats] = useState<StatisticsData | null>(null);
  const [budgets, setBudgets] = useState<BudgetView[]>([]);
  const [recent, setRecent] = useState<any[]>([]);
  const [creditRepays, setCreditRepays] = useState<CreditRepayItem[]>([]);
  const [loading, setLoading] = useState(true);

  const range = getMonthRange();

  useEffect(() => {
    if (!bookId) return;
    setLoading(true);
    Promise.all([
      statsApi.assets(),
      statsApi.overview({
        book_id: bookId, dimension: 'all',
        start_date: formatDate(range.start), end_date: formatDate(range.end),
      }),
      budgetApi.list({ book_id: bookId }),
      txApi.list({
        book_id: bookId, page_size: 6, page: 1,
      }),
      creditApi.summary().catch(() => []),
    ]).then(([a, s, b, t, c]) => {
      setAsset(a);
      setStats(s);
      setBudgets(b);
      setRecent((t.list ?? t).flat_list || []);
      setCreditRepays(c || []);
    }).finally(() => setLoading(false));
  }, [bookId]);

  if (loading && !asset) {
    return (
      <div className="grid place-items-center py-32 text-slate-400 text-sm">加载中...</div>
    );
  }

  const trend: TrendPoint[] = stats?.trend || [];
  const pieData = (stats?.by_category_expense || []).slice(0, 8).map(c => ({
    name: `${c.icon || ''} ${c.name}`, value: c.amount, color: c.color,
  }));
  const PALETTE = ['#10B981', '#3B82F6', '#F59E0B', '#EF4444', '#8B5CF6',
    '#EC4899', '#06B6D4', '#F97316', '#84CC16'];

  return (
    <div className="space-y-6">
      {/* 顶部资产卡片 */}
      <section className="rounded-2xl bg-gradient-to-br from-brand-600 via-brand-500 to-emerald-500 p-6 md:p-8 text-white shadow-soft overflow-hidden relative">
        <div className="absolute -right-20 -top-20 w-72 h-72 bg-white/10 rounded-full blur-3xl" />
        <div className="absolute -right-10 bottom-0 w-52 h-52 bg-white/5 rounded-full" />
        <div className="relative">
          <div className="flex flex-wrap items-end justify-between gap-4">
            <div>
              <div className="text-white/70 text-sm flex items-center gap-2">
                <PiggyBank size={16} /> 资产净值
              </div>
              <div className="text-3xl md:text-5xl font-bold tracking-tight mt-1 tabular-nums">
                {formatMoney(asset?.net_asset || 0)}
              </div>
              <div className="mt-2 text-white/80 text-sm space-x-4">
                <span>总资产 <b className="text-white">{formatMoney(asset?.total_asset || 0)}</b></span>
                <span>总负债 <b className="text-white">{formatMoney(asset?.total_debt || 0)}</b></span>
              </div>
            </div>
            <Link to="/transactions/add" className="btn bg-white text-brand-700 hover:bg-white/90 shadow-lg">
              <Plus size={18} /> 记一笔
            </Link>
          </div>

          <div className="mt-8 grid grid-cols-2 md:grid-cols-4 gap-3 md:gap-6">
            <StatBox
              icon={<ArrowUpRight className="text-emerald-200" size={18} />}
              label="本月收入"
              value={formatMoney(stats?.summary.total_income || 0)}
              sub={`${stats?.summary.income_count || 0} 笔`}
            />
            <StatBox
              icon={<ArrowDownRight className="text-red-200" size={18} />}
              label="本月支出"
              value={formatMoney(stats?.summary.total_expense || 0)}
              sub={`${stats?.summary.expense_count || 0} 笔 · 日均 ${formatMoney(stats?.summary.avg_daily_expense || 0)}`}
            />
            <StatBox
              icon={<Wallet className="text-amber-200" size={18} />}
              label="本月结余"
              value={formatMoney(stats?.summary.net || 0)}
              sub={(stats?.summary.net || 0) >= 0 ? '收支平衡 ✨' : '超支了 ⚠️'}
            />
            <StatBox
              icon={<CalendarDays className="text-cyan-200" size={18} />}
              label="手头上现金"
              value={formatMoney(asset?.cash_on_hand || 0)}
              sub={`共 ${asset?.account_count || 0} 个账户`}
            />
          </div>
        </div>
      </section>

      <div className="grid lg:grid-cols-3 gap-5">
        {/* 支出分类占比 */}
        <section className="card card-body">
          <div className="flex items-center justify-between mb-4">
            <h3 className="font-semibold text-slate-800 flex items-center gap-2">
              <PieIcon size={18} className="text-brand-600" />
              本月支出分类
            </h3>
            <Link to="/statistics" className="text-xs text-slate-500 hover:text-brand-600 flex items-center gap-0.5">
              查看详情 <ChevronRight size={14} />
            </Link>
          </div>
          {pieData.length === 0 ? (
            <Empty text="本月暂无支出" />
          ) : (
            <>
              <div className="h-56">
                <ResponsiveContainer>
                  <PieChart>
                    <Pie data={pieData} dataKey="value" nameKey="name"
                      innerRadius={50} outerRadius={85} paddingAngle={1}>
                      {pieData.map((_, i) => (
                        <Cell key={i} fill={(pieData[i] as any).color || PALETTE[i % PALETTE.length]} />
                      ))}
                    </Pie>
                    <Tooltip
                      formatter={(v) => formatMoney(Number(v) || 0)}
                      contentStyle={{ borderRadius: 10, border: '1px solid #e2e8f0' }}
                    />
                  </PieChart>
                </ResponsiveContainer>
              </div>
              <ul className="mt-2 space-y-1.5 max-h-40 overflow-y-auto">
                {(stats?.by_category_expense || []).slice(0, 6).map((c, i) => (
                  <li key={c.id} className="flex items-center gap-2 text-sm">
                    <span
                      className="w-2.5 h-2.5 rounded-full shrink-0"
                      style={{ background: c.color || PALETTE[i % PALETTE.length] }}
                    />
                    <span className="flex-1 truncate text-slate-600">{c.icon} {c.name}</span>
                    <span className="tabular-nums text-slate-700">{formatMoney(c.amount)}</span>
                    <span className="text-xs text-slate-400 w-12 text-right">{c.percent}%</span>
                  </li>
                ))}
              </ul>
            </>
          )}
        </section>

        {/* 收支趋势 */}
        <section className="card card-body lg:col-span-2">
          <div className="flex items-center justify-between mb-4">
            <h3 className="font-semibold text-slate-800">每日收支趋势</h3>
            <span className="text-xs text-slate-400">{range.start.getMonth() + 1}月</span>
          </div>
          {trend.length === 0 ? (
            <Empty text="暂无数据" />
          ) : (
            <div className="h-64">
              <ResponsiveContainer>
                <LineChart data={trend}>
                  <CartesianGrid strokeDasharray="3 3" stroke="#f1f5f9" />
                  <XAxis dataKey="date" tick={{ fontSize: 11, fill: '#94a3b8' }} tickFormatter={(s) => s.slice(8)} />
                  <YAxis tick={{ fontSize: 11, fill: '#94a3b8' }} />
                  <Tooltip
                    formatter={(v, n) => [formatMoney(Number(v) || 0), n === 'income' ? '收入' : n === 'expense' ? '支出' : '结余']}
                    contentStyle={{ borderRadius: 10, border: '1px solid #e2e8f0' }}
                  />
                  <Line type="monotone" dataKey="income" name="income"
                    stroke="#10B981" strokeWidth={2.2} dot={false} />
                  <Line type="monotone" dataKey="expense" name="expense"
                    stroke="#EF4444" strokeWidth={2.2} dot={false} />
                </LineChart>
              </ResponsiveContainer>
            </div>
          )}

          {/* 近期账单 */}
          <div className="mt-6 border-t border-slate-100 pt-4">
            <div className="flex items-center justify-between mb-3">
              <h4 className="font-medium text-slate-800 flex items-center gap-2">
                <Receipt size={16} className="text-brand-600" /> 近期账单
              </h4>
              <Link to="/transactions" className="text-xs text-brand-600 hover:underline">全部流水 →</Link>
            </div>
            {recent.length === 0 ? (
              <Empty text="暂无账单，点击右上角开始记账吧" />
            ) : (
              <ul className="divide-y divide-slate-50">
                {recent.slice(0, 5).map((t: any) => {
                  const cat = useAppStore.getState().categories.expense.concat(
                    useAppStore.getState().categories.income
                  ).find(c => c.id === t.category_id);
                  return (
                    <li key={t.id} className="flex items-center gap-3 py-2.5">
                      <div className="w-9 h-9 rounded-lg bg-slate-50 grid place-items-center text-lg">
                        {cat?.icon || '📦'}
                      </div>
                      <div className="flex-1 min-w-0">
                        <div className="text-sm text-slate-800 truncate">
                          {t.description || cat?.name || '未分类'}
                        </div>
                        <div className="text-xs text-slate-400">
                          {t.tx_date.slice(0, 10)} · {t.merchant || cat?.name || ''}
                        </div>
                      </div>
                      <AmountBadge type={t.type} amount={t.amount} />
                    </li>
                  );
                })}
              </ul>
            )}
          </div>
        </section>
      </div>

      {/* 预算 + 账户类型分布 */}
      <div className="grid lg:grid-cols-2 gap-5">
        <section className="card card-body">
          <div className="flex items-center justify-between mb-4">
            <h3 className="font-semibold text-slate-800 flex items-center gap-2">
              <Target size={18} className="text-amber-500" />
              本月预算
            </h3>
            <Link to="/budgets" className="text-xs text-brand-600 hover:underline">管理预算 →</Link>
          </div>
          {budgets.length === 0 ? (
            <Empty text="还没有创建预算，合理规划支出更有利于攒钱 💡" />
          ) : (
            <ul className="space-y-4">
              {budgets.slice(0, 5).map(b => {
                const cat = useAppStore.getState().categories.expense.find(c => c.id === b.category_id);
                return (
                  <li key={b.id}>
                    <div className="flex items-center justify-between mb-1.5">
                      <div className="text-sm text-slate-700">
                        <span className="mr-2">{cat?.icon || '📊'}</span>
                        {b.category_id === 0 ? '总预算' : cat?.name || '分类预算'}
                      </div>
                      <div className="text-xs tabular-nums">
                        <span className={b.is_over_budget ? 'text-red-500 font-medium' : 'text-slate-700'}>
                          {formatMoney(b.used_amount)}
                        </span>
                        <span className="text-slate-400 mx-1">/</span>
                        <span className="text-slate-500">{formatMoney(b.amount)}</span>
                        <span className="ml-2 text-slate-400">{pct(b.used_amount, b.amount)}%</span>
                      </div>
                    </div>
                    <Progress value={b.used_amount} total={b.amount} />
                    <div className="mt-1 text-xs text-slate-400">
                      剩余 <b className={b.daily_budget < 0 ? 'text-red-500' : 'text-slate-600'}>
                        {formatMoney(Math.max(0, b.remaining))}
                      </b>
                      {b.daily_budget > 0 && <> · 日均可用 <b className="text-slate-600">{formatMoney(b.daily_budget)}</b></>}
                    </div>
                  </li>
                );
              })}
            </ul>
          )}
        </section>

        <section className="card card-body">
          <div className="flex items-center justify-between mb-4">
            <h3 className="font-semibold text-slate-800 flex items-center gap-2">
              <Wallet size={18} className="text-indigo-500" />
              账户类型分布
            </h3>
            <Link to="/accounts" className="text-xs text-brand-600 hover:underline">账户详情 →</Link>
          </div>
          {asset ? (
            <div className="h-60">
              <ResponsiveContainer>
                <BarChart
                  layout="vertical"
                  data={Object.entries(asset.by_type).map(([k, v]) => ({
                    name: accountTypeLabel(k), value: v,
                  }))}
                >
                  <CartesianGrid strokeDasharray="3 3" stroke="#f1f5f9" horizontal={false} />
                  <XAxis type="number" tick={{ fontSize: 11, fill: '#94a3b8' }} />
                  <YAxis type="category" dataKey="name" tick={{ fontSize: 12 }} width={80} />
                  <Tooltip
                    formatter={(v) => formatMoney(Number(v) || 0)}
                    contentStyle={{ borderRadius: 10, border: '1px solid #e2e8f0' }}
                  />
                  <Bar dataKey="value" radius={[0, 6, 6, 0]}>
                    {Object.entries(asset.by_type).map((_, i) => (
                      <Cell key={i} fill={PALETTE[i % PALETTE.length]} />
                    ))}
                  </Bar>
                </BarChart>
              </ResponsiveContainer>
            </div>
          ) : null}
        </section>
      </div>

      {/* 信用卡还款提醒 */}
      {creditRepays.length > 0 && (
        <section className="card card-body">
          <div className="flex items-center justify-between mb-4">
            <h3 className="font-semibold text-slate-800 flex items-center gap-2">
              <CreditCard size={18} className="text-rose-500" />
              信用卡还款提醒
            </h3>
            <Link to="/cards" className="text-xs text-brand-600 hover:underline">卡面管理 →</Link>
          </div>
          <div className="grid sm:grid-cols-2 lg:grid-cols-3 gap-3">
            {creditRepays.map((c) => {
              const urgent = !c.overdue && c.days_left <= 3;
              const warn = !c.overdue && c.days_left <= 7 && c.days_left > 3;
              return (
                <div
                  key={c.id}
                  className={[
                    'rounded-xl border p-4 transition',
                    c.overdue ? 'bg-red-50 border-red-200' :
                    urgent ? 'bg-amber-50 border-amber-200' :
                    'bg-slate-50 border-slate-200',
                  ].join(' ')}
                >
                  <div className="flex items-center justify-between mb-2">
                    <div className="flex items-center gap-2 min-w-0">
                      <div className="text-lg">💳</div>
                      <div className="min-w-0">
                        <div className="text-sm font-semibold text-slate-800 truncate">{c.name}</div>
                        {c.bank_name && <div className="text-[11px] text-slate-500 truncate">{c.bank_name} · 尾号{c.card_no4}</div>}
                      </div>
                    </div>
                    {c.overdue ? (
                      <span className="shrink-0 flex items-center gap-1 text-[11px] font-semibold text-red-600 bg-red-100 px-2 py-0.5 rounded-full">
                        <AlertCircle size={12} /> 逾期{c.days_left}天
                      </span>
                    ) : urgent ? (
                      <span className="shrink-0 flex items-center gap-1 text-[11px] font-semibold text-amber-700 bg-amber-100 px-2 py-0.5 rounded-full">
                        <Clock size={12} /> 仅剩{c.days_left}天
                      </span>
                    ) : warn ? (
                      <span className="shrink-0 text-[11px] font-medium text-slate-600 bg-white px-2 py-0.5 rounded-full">
                        {c.days_left}天后
                      </span>
                    ) : (
                      <span className="shrink-0 text-[11px] text-slate-500">{c.days_left}天后</span>
                    )}
                  </div>
                  <div className="flex items-end justify-between text-sm">
                    <div>
                      <div className="text-[11px] text-slate-400">已出账</div>
                      <div className={[
                        'font-bold tabular-nums',
                        c.overdue ? 'text-red-600' : 'text-slate-800',
                      ].join(' ')}>
                        {formatMoney(c.bill_amount || Math.max(0, c.balance))}
                      </div>
                    </div>
                    <div className="text-right">
                      <div className="text-[11px] text-slate-400">还款日</div>
                      <div className="text-xs font-medium text-slate-600 tabular-nums">{c.repay_date}</div>
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
        </section>
      )}
    </div>
  );
}

function StatBox({ icon, label, value, sub }: any) {
  return (
    <div className="rounded-xl bg-white/10 backdrop-blur px-4 py-3 border border-white/10">
      <div className="flex items-center gap-2 text-white/80 text-xs mb-1">
        {icon}{label}
      </div>
      <div className="text-xl md:text-2xl font-bold tabular-nums">{value}</div>
      {sub && <div className="text-xs text-white/70 mt-0.5">{sub}</div>}
    </div>
  );
}

function accountTypeLabel(k: string) {
  return {
    cash: '💵 现金', bank: '🏦 储蓄卡', credit: '💳 信用卡',
    prepaid: '🎟️ 储值卡', investment: '📈 投资',
    liability: '💸 负债', virtual: '📱 虚拟账户',
  }[k] || k;
}
