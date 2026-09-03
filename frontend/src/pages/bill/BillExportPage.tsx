import { useEffect, useState } from 'react';
import { useSearchParams, useNavigate } from 'react-router-dom';
import { useAppStore } from '@/stores/app';
import { billApi } from '@/api';
import type { BillData } from '@/api';
import { formatMoney } from '@/utils';
import { Download, ArrowLeft, Printer } from 'lucide-react';

export default function BillExportPage() {
  const [params] = useSearchParams();
  const navigate = useNavigate();
  const bookId = useAppStore(s => s.currentBookId);
  const [data, setData] = useState<BillData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const month = params.get('month') || new Date().toISOString().slice(0, 7);

  useEffect(() => {
    billApi.get({ month, book_id: bookId }).then(setData).catch((e) => setError(e.message)).finally(() => setLoading(false));
  }, [month, bookId]);

  const handlePrint = () => {
    window.print();
  };

  if (loading) return <div className="grid place-items-center py-32 text-slate-400">加载账单中...</div>;
  if (error || !data) return <div className="p-10 text-center text-red-500">{error || '加载失败'}</div>;

  const { meta, summary, category_expense, category_income, daily_trend, budgets, assets } = data;
  const PALETTE = ['#6366F1', '#EF4444', '#10B981', '#F59E0B', '#8B5CF6', '#EC4899', '#06B6D4', '#F97316'];

  return (
    <div className="max-w-4xl mx-auto p-6 md:p-10">
      {/* 工具栏（打印时隐藏） */}
      <div className="no-print flex items-center justify-between mb-6">
        <button className="btn-ghost" onClick={() => navigate(-1)}><ArrowLeft size={16} /> 返回</button>
        <div className="flex items-center gap-2">
          <button className="btn-secondary" onClick={handlePrint}>
            <Printer size={16} /> 打印 / 保存 PDF
          </button>
          <button className="btn-primary" onClick={handlePrint}>
            <Download size={16} /> 导出 PDF
          </button>
        </div>
      </div>

      {/* 账单主体 */}
      <div className="bill-paper bg-white rounded-2xl shadow-xl overflow-hidden print:shadow-none print:rounded-none">
        {/* 封面 */}
        <div className="bg-gradient-to-br from-indigo-600 via-purple-600 to-pink-500 text-white p-10 md:p-14 relative overflow-hidden">
          <div className="absolute -right-20 -top-20 w-72 h-72 bg-white/10 rounded-full blur-3xl" />
          <div className="absolute -left-10 bottom-0 w-52 h-52 bg-white/5 rounded-full" />
          <div className="relative">
            <div className="text-xs tracking-widest text-white/70 uppercase mb-2">Huozhi 货殖 · 月度账单</div>
            <h1 className="text-4xl md:text-5xl font-bold tracking-tight">
              {meta.month.replace('-', ' 年 ')} 月
            </h1>
            <div className="mt-4 flex items-center gap-4 text-sm text-white/80">
              <span>{meta.book_name || '全部账本'}</span>
              {meta.user_name && <span>· {meta.user_name}</span>}
            </div>
            <div className="mt-8 flex flex-wrap gap-6">
              <div>
                <div className="text-xs text-white/60">收入</div>
                <div className="text-2xl font-bold">{formatMoney(summary.total_income)}</div>
              </div>
              <div>
                <div className="text-xs text-white/60">支出</div>
                <div className="text-2xl font-bold">{formatMoney(summary.total_expense)}</div>
              </div>
              <div>
                <div className="text-xs text-white/60">结余</div>
                <div className={['text-2xl font-bold', summary.net >= 0 ? 'text-emerald-200' : 'text-red-200'].join(' ')}>
                  {formatMoney(summary.net)}
                </div>
              </div>
              <div>
                <div className="text-xs text-white/60">账单日</div>
                <div className="text-lg font-semibold">{meta.range_start} ~ {meta.range_end}</div>
              </div>
            </div>
          </div>
        </div>

        {/* 核心指标 */}
        <div className="grid grid-cols-4 border-b border-slate-100">
          <div className="p-5 text-center border-r border-slate-100">
            <div className="text-xs text-slate-400">交易笔数</div>
            <div className="text-2xl font-bold text-slate-800 mt-1 tabular-nums">{summary.transaction_count}</div>
          </div>
          <div className="p-5 text-center border-r border-slate-100">
            <div className="text-xs text-slate-400">日均支出</div>
            <div className="text-2xl font-bold text-slate-800 mt-1 tabular-nums">{formatMoney(summary.avg_daily_expense)}</div>
          </div>
          <div className="p-5 text-center border-r border-slate-100">
            <div className="text-xs text-slate-400">总收入</div>
            <div className="text-2xl font-bold text-emerald-600 mt-1 tabular-nums">{formatMoney(summary.total_income)}</div>
          </div>
          <div className="p-5 text-center">
            <div className="text-xs text-slate-400">总支出</div>
            <div className="text-2xl font-bold text-red-500 mt-1 tabular-nums">{formatMoney(summary.total_expense)}</div>
          </div>
        </div>

        {/* 分类汇总 */}
        <div className="grid md:grid-cols-2 divide-y md:divide-y-0 md:divide-x divide-slate-100">
          <div className="p-6">
            <h3 className="font-semibold text-slate-800 mb-4 flex items-center gap-2">
              <span className="w-2 h-2 rounded-full bg-red-500" /> 支出分类
            </h3>
            {category_expense.length === 0 ? (
              <div className="text-sm text-slate-400 py-8 text-center">本月暂无支出</div>
            ) : (
              <ul className="space-y-3">
                {category_expense.slice(0, 8).map((c, i) => (
                  <li key={c.id}>
                    <div className="flex items-center justify-between mb-1">
                      <span className="text-sm text-slate-700 flex items-center gap-1.5">
                        <span>{c.icon}</span>{c.name}
                      </span>
                      <span className="text-sm font-semibold text-slate-700 tabular-nums">{formatMoney(c.amount)}</span>
                    </div>
                    <div className="h-1.5 bg-slate-100 rounded-full overflow-hidden">
                      <div className="h-full rounded-full" style={{ width: `${c.percent}%`, background: c.color || PALETTE[i % PALETTE.length] }} />
                    </div>
                  </li>
                ))}
              </ul>
            )}
          </div>
          <div className="p-6">
            <h3 className="font-semibold text-slate-800 mb-4 flex items-center gap-2">
              <span className="w-2 h-2 rounded-full bg-emerald-500" /> 收入分类
            </h3>
            {category_income.length === 0 ? (
              <div className="text-sm text-slate-400 py-8 text-center">本月暂无收入</div>
            ) : (
              <ul className="space-y-3">
                {category_income.slice(0, 8).map((c, i) => (
                  <li key={c.id}>
                    <div className="flex items-center justify-between mb-1">
                      <span className="text-sm text-slate-700 flex items-center gap-1.5">
                        <span>{c.icon}</span>{c.name}
                      </span>
                      <span className="text-sm font-semibold text-slate-700 tabular-nums">{formatMoney(c.amount)}</span>
                    </div>
                    <div className="h-1.5 bg-slate-100 rounded-full overflow-hidden">
                      <div className="h-full rounded-full" style={{ width: `${c.percent}%`, background: c.color || PALETTE[i % PALETTE.length] }} />
                    </div>
                  </li>
                ))}
              </ul>
            )}
          </div>
        </div>

        {/* 每日趋势 */}
        <div className="p-6 border-t border-slate-100">
          <h3 className="font-semibold text-slate-800 mb-4">每日收支趋势</h3>
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-slate-200 text-xs text-slate-400">
                  <th className="py-2 text-left font-medium">日期</th>
                  <th className="py-2 text-right font-medium">收入</th>
                  <th className="py-2 text-right font-medium">支出</th>
                  <th className="py-2 text-right font-medium">结余</th>
                </tr>
              </thead>
              <tbody>
                {daily_trend.filter((d) => d.income > 0 || d.expense > 0).map((d, i) => (
                  <tr key={i} className="border-b border-slate-50">
                    <td className="py-2 text-slate-600 tabular-nums">{d.date}</td>
                    <td className="py-2 text-right text-emerald-600 tabular-nums">{d.income > 0 ? formatMoney(d.income) : '—'}</td>
                    <td className="py-2 text-right text-red-500 tabular-nums">{d.expense > 0 ? formatMoney(d.expense) : '—'}</td>
                    <td className={['py-2 text-right tabular-nums', d.net >= 0 ? 'text-slate-700' : 'text-red-500'].join(' ')}>
                      {formatMoney(d.net)}
                    </td>
                  </tr>
                ))}
                {daily_trend.filter((d) => d.income > 0 || d.expense > 0).length === 0 && (
                  <tr><td colSpan={4} className="py-6 text-center text-slate-400">本月暂无交易记录</td></tr>
                )}
              </tbody>
            </table>
          </div>
        </div>

        {/* 预算执行 */}
        {budgets.length > 0 && (
          <div className="p-6 border-t border-slate-100">
            <h3 className="font-semibold text-slate-800 mb-4">预算执行情况</h3>
            <div className="grid sm:grid-cols-2 gap-3">
              {budgets.map((b) => (
                <div key={b.id} className={['rounded-xl border p-4', b.is_over_budget ? 'border-red-200 bg-red-50' : 'border-slate-200 bg-slate-50'].join(' ')}>
                  <div className="flex items-center justify-between mb-2">
                    <span className="text-sm font-medium text-slate-700">{b.name}</span>
                    <span className={['text-xs font-semibold tabular-nums', b.is_over_budget ? 'text-red-600' : 'text-slate-500'].join(' ')}>
                      {b.usage_rate}%
                    </span>
                  </div>
                  <div className="flex justify-between text-sm mb-2">
                    <span className={['font-bold tabular-nums', b.is_over_budget ? 'text-red-600' : 'text-slate-800'].join(' ')}>
                      {formatMoney(b.used_amount)}
                    </span>
                    <span className="text-slate-400 tabular-nums">/ {formatMoney(b.amount)}</span>
                  </div>
                  <div className="h-1.5 bg-white rounded-full overflow-hidden">
                    <div className={['h-full rounded-full', b.is_over_budget ? 'bg-red-500' : b.usage_rate >= 80 ? 'bg-amber-500' : 'bg-emerald-500'].join(' ')}
                      style={{ width: `${Math.min(b.usage_rate, 100)}%` }} />
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}

        {/* 资产快照 */}
        <div className="p-6 border-t border-slate-100 bg-slate-50/50">
          <h3 className="font-semibold text-slate-800 mb-4">当前资产快照</h3>
          <div className="grid grid-cols-3 gap-4">
            <div className="rounded-xl bg-white p-4 border border-slate-200">
              <div className="text-xs text-slate-400 mb-1">总资产</div>
              <div className="text-xl font-bold text-slate-800 tabular-nums">{formatMoney(assets.total_asset)}</div>
            </div>
            <div className="rounded-xl bg-white p-4 border border-slate-200">
              <div className="text-xs text-slate-400 mb-1">总负债</div>
              <div className="text-xl font-bold text-red-500 tabular-nums">{formatMoney(assets.total_debt)}</div>
            </div>
            <div className="rounded-xl bg-white p-4 border border-slate-200">
              <div className="text-xs text-slate-400 mb-1">净资产</div>
              <div className="text-xl font-bold text-indigo-600 tabular-nums">{formatMoney(assets.net_asset)}</div>
            </div>
          </div>
        </div>

        {/* 页脚 */}
        <div className="px-6 py-4 bg-slate-50 text-xs text-slate-400 flex items-center justify-between border-t border-slate-100">
          <span>货殖 Huozhi · 智能记账助手</span>
          <span>生成于 {meta.generated_at}</span>
        </div>
      </div>
    </div>
  );
}
