import { useEffect, useMemo, useState } from 'react';
import { useAppStore } from '@/stores/app';
import { statsApi } from '@/api';
import type { StatisticsData, StatsItem, AssetPoint, TrendPoint } from '@/types';
import { formatMoney, formatDate, cn, pct } from '@/utils';
import {
  Calendar, BarChart3, PieChart as PieIcon, TrendingUp,
  ArrowUpRight, ArrowDownRight, Wallet, ChevronDown,
  Trophy, AlertCircle,
} from 'lucide-react';
import { AmountBadge, Empty } from '@/components/common';
import {
  ResponsiveContainer, PieChart, Pie, Cell, Tooltip,
  BarChart, Bar, XAxis, YAxis, CartesianGrid, Legend,
  LineChart, Line, AreaChart, Area,
} from 'recharts';

type RangePreset = 'this_month' | 'last_month' | 'this_quarter' | 'this_year' | 'custom';

const PRESET_LABELS: Record<RangePreset, string> = {
  this_month: '本月',
  last_month: '上月',
  this_quarter: '本季',
  this_year: '本年',
  custom: '自定义',
};

const PALETTE = [
  '#6366F1', '#10B981', '#F59E0B', '#EF4444', '#8B5CF6',
  '#EC4899', '#06B6D4', '#F97316', '#84CC16', '#14B8A6',
  '#22C55E', '#A855F7', '#0EA5E9', '#F43F5E', '#64748b',
];

function getRange(preset: RangePreset, customStart = '', customEnd = '') {
  const now = new Date();
  const y = now.getFullYear();
  const m = now.getMonth();
  let start: Date, end: Date;
  switch (preset) {
    case 'this_month':
      start = new Date(y, m, 1);
      end = new Date(y, m + 1, 0);
      break;
    case 'last_month':
      start = new Date(y, m - 1, 1);
      end = new Date(y, m, 0);
      break;
    case 'this_quarter': {
      const qm = Math.floor(m / 3) * 3;
      start = new Date(y, qm, 1);
      end = new Date(y, qm + 3, 0);
      break;
    }
    case 'this_year':
      start = new Date(y, 0, 1);
      end = new Date(y, 11, 31);
      break;
    case 'custom':
    default:
      start = customStart ? new Date(customStart) : new Date(y, m, 1);
      end = customEnd ? new Date(customEnd) : new Date(y, m + 1, 0);
      break;
  }
  return {
    start_str: formatDate(start),
    end_str: formatDate(end),
    start, end,
  };
}

export default function StatisticsPage() {
  const bookId = useAppStore(s => s.currentBookId);
  const accounts = useAppStore(s => s.accounts);
  const { expense: expCats, income: incCats } = useAppStore(s => s.categories);
  const allCats = useMemo(() => [...expCats, ...incCats], [expCats, incCats]);

  const [preset, setPreset] = useState<RangePreset>('this_month');
  const [customStart, setCustomStart] = useState('');
  const [customEnd, setCustomEnd] = useState('');

  const [data, setData] = useState<StatisticsData | null>(null);
  const [timeline, setTimeline] = useState<AssetPoint[]>([]);
  const [loading, setLoading] = useState(false);
  const [viewMode, setViewMode] = useState<'expense' | 'income'>('expense');

  const range = getRange(preset, customStart, customEnd);

  const load = async () => {
    if (!bookId) return;
    setLoading(true);
    try {
      const [s, tl] = await Promise.all([
        statsApi.overview({
          book_id: bookId,
          start_date: range.start_str,
          end_date: range.end_str,
          dimension: 'all',
        }),
        statsApi.timeline().catch(() => [] as AssetPoint[]),
      ]);
      setData(s);
      setTimeline(tl);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, [bookId, preset, customStart, customEnd]); // eslint-disable-line

  const summary = data?.summary;
  const statsList = viewMode === 'expense'
    ? (data?.by_category_expense || [])
    : (data?.by_category_income || []);
  const trend: TrendPoint[] = data?.trend || [];
  const topExp = data?.top_expense || [];
  const byAccount = data?.by_account || {};

  // 饼图数据
  const pieData = statsList.slice(0, 10).map((s, i) => ({
    name: s.name,
    icon: s.icon,
    value: s.amount,
    color: s.color || PALETTE[i % PALETTE.length],
    percent: s.percent,
  }));

  // 柱状图数据（分类排行榜）
  const barData = statsList.slice(0, 15).map((s, i) => ({
    name: `${s.icon || ''} ${s.name}`,
    value: s.amount,
    count: s.count,
    color: s.color || PALETTE[i % PALETTE.length],
  }));

  // 账户维度数据
  const accountData = Object.values(byAccount).map(av => {
    const acc = accounts.find(a => a.id === av.account_id);
    return {
      name: acc?.name || `账户#${av.account_id}`,
      income: av.income,
      expense: av.expense,
      net: av.income - av.expense,
    };
  });

  return (
    <div className="space-y-5">
      {/* 时间范围选择 */}
      <section className="card card-body">
        <div className="flex items-center gap-2 flex-wrap">
          <div className="flex items-center gap-1 overflow-x-auto no-scrollbar">
            {(Object.keys(PRESET_LABELS) as RangePreset[]).map(p => (
              <button
                key={p}
                onClick={() => setPreset(p)}
                className={cn(
                  'shrink-0 px-3 py-1.5 rounded-full text-sm font-medium transition whitespace-nowrap',
                  preset === p
                    ? 'bg-brand-600 text-white'
                    : 'bg-slate-100 text-slate-600 hover:bg-slate-200'
                )}
              >
                <Calendar size={13} className="inline mr-1" />
                {PRESET_LABELS[p]}
              </button>
            ))}
          </div>
          {preset === 'custom' && (
            <div className="flex items-center gap-2">
              <input
                type="date" className="input input-sm w-auto"
                value={customStart}
                onChange={e => setCustomStart(e.target.value)}
              />
              <span className="text-slate-400">至</span>
              <input
                type="date" className="input input-sm w-auto"
                value={customEnd}
                onChange={e => setCustomEnd(e.target.value)}
              />
            </div>
          )}
          <span className="ml-auto text-xs text-slate-400 whitespace-nowrap">
            {range.start_str} ~ {range.end_str}
            {data?.range?.days ? ` (${data.range.days} 天)` : ''}
          </span>
        </div>
      </section>

      {/* 汇总卡片 */}
      <section className="grid sm:grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard
          icon={<ArrowUpRight size={18} className="text-emerald-600" />}
          label="总收入"
          value={formatMoney(summary?.total_income || 0)}
          sub={`${summary?.income_count || 0} 笔 · 日均 ${formatMoney(summary?.avg_daily_income || 0)}`}
          color="emerald"
        />
        <StatCard
          icon={<ArrowDownRight size={18} className="text-red-500" />}
          label="总支出"
          value={formatMoney(summary?.total_expense || 0)}
          sub={`${summary?.expense_count || 0} 笔 · 日均 ${formatMoney(summary?.avg_daily_expense || 0)}`}
          color="red"
        />
        <StatCard
          icon={<TrendingUp size={18} className={cn((summary?.net || 0) >= 0 ? 'text-brand-600' : 'text-red-500')} />}
          label="净结余"
          value={(summary?.net || 0) >= 0 ? `+${formatMoney(summary?.net || 0)}` : formatMoney(summary?.net || 0)}
          sub={`共 ${summary?.transaction_count || 0} 笔交易`}
          color={(summary?.net || 0) >= 0 ? 'brand' : 'red'}
        />
        <StatCard
          icon={<BarChart3 size={18} className="text-indigo-600" />}
          label="储蓄率"
          value={(summary?.total_income || 0) > 0
            ? `${pct(Math.max(0, summary?.net || 0), summary!.total_income)}%`
            : '—'}
          sub={summary?.total_income ? '结余/收入' : '暂无收入数据'}
          color="indigo"
        />
      </section>

      {/* 收支趋势折线图 */}
      <section className="card card-body">
        <div className="flex items-center justify-between mb-4 flex-wrap gap-2">
          <h3 className="font-semibold text-slate-800 flex items-center gap-2">
            <TrendingUp size={18} className="text-brand-600" /> 收支趋势
          </h3>
        </div>
        {trend.length === 0 ? (
          <Empty text="暂无数据" />
        ) : (
          <div className="h-72">
            <ResponsiveContainer>
              <LineChart data={trend}>
                <CartesianGrid strokeDasharray="3 3" stroke="#f1f5f9" />
                <XAxis dataKey="date" tick={{ fontSize: 11, fill: '#94a3b8' }}
                  tickFormatter={(s) => {
                    if (preset === 'this_year') return s.slice(5, 7) + '月';
                    if (preset === 'this_quarter') return s.slice(5);
                    return s.length >= 10 ? s.slice(8) : s;
                  }}
                />
                <YAxis tick={{ fontSize: 11, fill: '#94a3b8' }} />
                <Tooltip
                  formatter={(v, n) => [
                    formatMoney(Number(v) || 0),
                    n === 'income' ? '收入' : n === 'expense' ? '支出' : '结余',
                  ]}
                  contentStyle={{ borderRadius: 10, border: '1px solid #e2e8f0' }}
                />
                <Legend
                  formatter={(n) => n === 'income' ? '收入' : n === 'expense' ? '支出' : '结余'}
                />
                <Line type="monotone" dataKey="income" name="income"
                  stroke="#10B981" strokeWidth={2.5} dot={false} />
                <Line type="monotone" dataKey="expense" name="expense"
                  stroke="#EF4444" strokeWidth={2.5} dot={false} />
              </LineChart>
            </ResponsiveContainer>
          </div>
        )}
      </section>

      {/* 分类排行 + 饼图 */}
      <div className="grid lg:grid-cols-5 gap-5">
        {/* 饼图 */}
        <section className="card card-body lg:col-span-2">
          <div className="flex items-center justify-between mb-3 flex-wrap gap-2">
            <h3 className="font-semibold text-slate-800 flex items-center gap-2">
              <PieIcon size={18} className="text-brand-600" /> 分类占比
            </h3>
            <div className="flex gap-1 p-0.5 bg-slate-100 rounded-lg">
              <button
                onClick={() => setViewMode('expense')}
                className={cn(
                  'px-3 py-1 rounded-md text-xs font-medium transition',
                  viewMode === 'expense' ? 'bg-white text-red-600 shadow-sm' : 'text-slate-500'
                )}
              >支出</button>
              <button
                onClick={() => setViewMode('income')}
                className={cn(
                  'px-3 py-1 rounded-md text-xs font-medium transition',
                  viewMode === 'income' ? 'bg-white text-emerald-600 shadow-sm' : 'text-slate-500'
                )}
              >收入</button>
            </div>
          </div>
          {pieData.length === 0 ? (
            <Empty text={`暂无${viewMode === 'expense' ? '支出' : '收入'}数据`} />
          ) : (
            <>
              <div className="h-56">
                <ResponsiveContainer>
                  <PieChart>
                    <Pie
                      data={pieData} dataKey="value" nameKey="name"
                      innerRadius={55} outerRadius={85} paddingAngle={1.5}
                    >
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
              <ul className="mt-2 space-y-1.5 max-h-52 overflow-y-auto">
                {pieData.map((c, i) => (
                  <li key={i} className="flex items-center gap-2 text-sm">
                    <span
                      className="w-2.5 h-2.5 rounded-full shrink-0"
                      style={{ background: (c as any).color }}
                    />
                    <span className="flex-1 truncate text-slate-600">{c.icon} {c.name}</span>
                    <span className="tabular-nums text-slate-700">{formatMoney(c.value)}</span>
                    <span className="text-xs text-slate-400 w-12 text-right">{c.percent}%</span>
                  </li>
                ))}
              </ul>
            </>
          )}
        </section>

        {/* 分类排行柱状图 */}
        <section className="card card-body lg:col-span-3">
          <h3 className="font-semibold text-slate-800 mb-4 flex items-center gap-2">
            <Trophy size={18} className="text-amber-500" /> 分类排行榜
          </h3>
          {barData.length === 0 ? (
            <Empty text="暂无数据" />
          ) : (
            <div className="h-[420px]">
              <ResponsiveContainer>
                <BarChart layout="vertical" data={barData}>
                  <CartesianGrid strokeDasharray="3 3" stroke="#f1f5f9" horizontal={false} />
                  <XAxis type="number" tick={{ fontSize: 11, fill: '#94a3b8' }} />
                  <YAxis type="category" dataKey="name"
                    tick={{ fontSize: 12, fill: '#475569' }} width={110} />
                  <Tooltip
                    formatter={(v, n) => [
                      n === 'value' ? formatMoney(Number(v) || 0) : `${v} 笔`,
                      n === 'value' ? '金额' : '笔数',
                    ]}
                    contentStyle={{ borderRadius: 10, border: '1px solid #e2e8f0' }}
                  />
                  <Bar dataKey="value" radius={[0, 6, 6, 0]} name="value">
                    {barData.map((_, i) => (
                      <Cell key={i} fill={(barData[i] as any).color} />
                    ))}
                  </Bar>
                </BarChart>
              </ResponsiveContainer>
            </div>
          )}
        </section>
      </div>

      {/* Top 支出 */}
      <section className="card card-body">
        <h3 className="font-semibold text-slate-800 mb-4 flex items-center gap-2">
          <AlertCircle size={18} className="text-red-500" /> Top 大额支出
        </h3>
        {topExp.length === 0 ? (
          <Empty text="暂无大额支出" />
        ) : (
          <ul className="divide-y divide-slate-50">
            {topExp.map((t, i) => {
              const cat = allCats.find(c => c.id === t.category_id);
              return (
                <li key={`${t.id}-${i}`} className="flex items-center gap-3 py-3">
                  <div className={cn(
                    'w-8 h-8 rounded-full grid place-items-center text-sm font-bold shrink-0',
                    i === 0 ? 'bg-amber-100 text-amber-600' :
                    i === 1 ? 'bg-slate-200 text-slate-600' :
                    i === 2 ? 'bg-orange-100 text-orange-600' :
                    'bg-slate-100 text-slate-500'
                  )}>
                    {i + 1}
                  </div>
                  <div
                    className="w-10 h-10 rounded-lg grid place-items-center text-xl shrink-0"
                    style={{ background: (cat?.color || '#64748b') + '15' }}
                  >
                    {cat?.icon || '📦'}
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="text-sm font-medium text-slate-800 truncate">
                      {t.description || cat?.name || '未分类'}
                    </div>
                    <div className="text-xs text-slate-400 mt-0.5 flex items-center gap-2">
                      <span>{t.tx_date.slice(0, 10)}</span>
                      {t.merchant && <span>· {t.merchant}</span>}
                      <span>· {cat?.name || '未分类'}</span>
                    </div>
                  </div>
                  <div className="shrink-0">
                    <AmountBadge type="expense" amount={t.amount} />
                  </div>
                </li>
              );
            })}
          </ul>
        )}
      </section>

      {/* 账户维度 */}
      <section className="card card-body">
        <h3 className="font-semibold text-slate-800 mb-4 flex items-center gap-2">
          <Wallet size={18} className="text-indigo-500" /> 账户维度分布
        </h3>
        {accountData.length === 0 ? (
          <Empty text="暂无账户数据" />
        ) : (
          <div className="h-72">
            <ResponsiveContainer>
              <BarChart data={accountData}>
                <CartesianGrid strokeDasharray="3 3" stroke="#f1f5f9" />
                <XAxis dataKey="name" tick={{ fontSize: 11, fill: '#94a3b8' }} />
                <YAxis tick={{ fontSize: 11, fill: '#94a3b8' }} />
                <Tooltip
                  formatter={(v) => formatMoney(Number(v) || 0)}
                  contentStyle={{ borderRadius: 10, border: '1px solid #e2e8f0' }}
                />
                <Legend
                  formatter={(n) => n === 'income' ? '收入' : n === 'expense' ? '支出' : '净收支'}
                />
                <Bar dataKey="income" name="income" stackId="a"
                  fill="#10B981" radius={[4, 4, 0, 0]} />
                <Bar dataKey="expense" name="expense" stackId="a"
                  fill="#EF4444" radius={[4, 4, 0, 0]} />
              </BarChart>
            </ResponsiveContainer>
          </div>
        )}
      </section>

      {/* 资产净值曲线 */}
      <section className="card card-body">
        <div className="flex items-center justify-between mb-4 flex-wrap gap-2">
          <h3 className="font-semibold text-slate-800 flex items-center gap-2">
            <TrendingUp size={18} className="text-purple-600" /> 资产净值曲线
          </h3>
          <span className="text-xs text-slate-400">长期跟踪资产变化趋势</span>
        </div>
        {timeline.length === 0 ? (
          <Empty text="暂无历史资产数据" />
        ) : (
          <div className="h-72">
            <ResponsiveContainer>
              <AreaChart data={timeline}>
                <defs>
                  <linearGradient id="netAssetGrad" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="5%" stopColor="#8B5CF6" stopOpacity={0.25} />
                    <stop offset="95%" stopColor="#8B5CF6" stopOpacity={0} />
                  </linearGradient>
                  <linearGradient id="totalAssetGrad" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="5%" stopColor="#06B6D4" stopOpacity={0.18} />
                    <stop offset="95%" stopColor="#06B6D4" stopOpacity={0} />
                  </linearGradient>
                </defs>
                <CartesianGrid strokeDasharray="3 3" stroke="#f1f5f9" />
                <XAxis dataKey="month" tick={{ fontSize: 11, fill: '#94a3b8' }} />
                <YAxis tick={{ fontSize: 11, fill: '#94a3b8' }} />
                <Tooltip
                  formatter={(v, n) => [
                    formatMoney(Number(v) || 0),
                    n === 'net_asset' ? '净值' : n === 'total_asset' ? '总资产' : '总负债',
                  ]}
                  contentStyle={{ borderRadius: 10, border: '1px solid #e2e8f0' }}
                />
                <Legend
                  formatter={(n) => n === 'net_asset' ? '净值' : n === 'total_asset' ? '总资产' : '总负债'}
                />
                <Area type="monotone" dataKey="total_asset" name="total_asset"
                  stroke="#06B6D4" strokeWidth={1.8} fill="url(#totalAssetGrad)" />
                <Area type="monotone" dataKey="net_asset" name="net_asset"
                  stroke="#8B5CF6" strokeWidth={2.5} fill="url(#netAssetGrad)" />
                <Line type="monotone" dataKey="total_debt" name="total_debt"
                  stroke="#EF4444" strokeWidth={1.8} strokeDasharray="5 5" dot={false} />
              </AreaChart>
            </ResponsiveContainer>
          </div>
        )}
      </section>

      {loading && (
        <div className="fixed top-3 right-3 z-50 px-3 py-1.5 bg-white rounded-lg shadow border border-slate-200 text-xs text-slate-500">
          加载中...
        </div>
      )}
    </div>
  );
}

function StatCard({
  icon, label, value, sub, color,
}: {
  icon: React.ReactNode;
  label: string;
  value: string;
  sub: string;
  color: 'emerald' | 'red' | 'brand' | 'indigo';
}) {
  const ring = {
    emerald: 'border-emerald-100 bg-emerald-50/50',
    red: 'border-red-100 bg-red-50/50',
    brand: 'border-brand-100 bg-brand-50/50',
    indigo: 'border-indigo-100 bg-indigo-50/50',
  }[color];
  return (
    <div className={`card card-body border ${ring} transition hover:-translate-y-0.5 hover:shadow-lg`}>
      <div className="flex items-center gap-2">
        <div className="w-8 h-8 rounded-lg grid place-items-center bg-white shadow-sm">
          {icon}
        </div>
        <span className="text-sm text-slate-500">{label}</span>
      </div>
      <div className="mt-3 text-2xl md:text-3xl font-bold tabular-nums tracking-tight text-slate-800">
        {value}
      </div>
      <div className="mt-1 text-xs text-slate-400">{sub}</div>
    </div>
  );
}
