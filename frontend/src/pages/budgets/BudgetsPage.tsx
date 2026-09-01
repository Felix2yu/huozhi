import { useEffect, useMemo, useState } from 'react';
import { toast } from 'sonner';
import { useAppStore } from '@/stores/app';
import { budgetApi } from '@/api';
import type { BudgetView } from '@/types';
import { cn, formatMoney, formatDate, getMonthRange, pct } from '@/utils';
import {
  Target, Plus, Edit3, Trash2, AlertTriangle, Calendar, Check,
  TrendingDown, TrendingUp, PieChart as PieIcon,
} from 'lucide-react';
import { Modal, ConfirmDialog, Empty, Progress } from '@/components/common';

type Period = 'monthly' | 'yearly' | 'custom';

const PERIOD_LABELS: Record<Period, string> = {
  monthly: '月度', yearly: '年度', custom: '自定义',
};

const blankForm = () => {
  const r = getMonthRange();
  return {
    period_type: 'monthly' as Period,
    category_id: 0,
    amount: '',
    start_date: formatDate(r.start),
    end_date: formatDate(r.end),
    alert_rate: 0.8,
    roll_over: false,
  };
};

export default function BudgetsPage() {
  const bookId = useAppStore(s => s.currentBookId);
  const { expense: expCats } = useAppStore(s => s.categories);

  const [list, setList] = useState<BudgetView[]>([]);
  const [loading, setLoading] = useState(false);

  const [editOpen, setEditOpen] = useState(false);
  const [editItem, setEditItem] = useState<BudgetView | null>(null);
  const [editForm, setEditForm] = useState(blankForm());

  const [delTarget, setDelTarget] = useState<BudgetView | null>(null);

  const load = async () => {
    if (!bookId) return;
    setLoading(true);
    try {
      const res = await budgetApi.list({ book_id: bookId });
      setList(res || []);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, [bookId]); // eslint-disable-line

  const totalBudget = useMemo(() => {
    const overall = list.find(b => b.category_id === 0);
    return overall || null;
  }, [list]);

  const catBudgets = useMemo(() => list.filter(b => b.category_id !== 0), [list]);

  const totalCatUsed = useMemo(
    () => catBudgets.reduce((s, b) => s + b.used_amount, 0),
    [catBudgets]
  );

  const openCreate = () => {
    setEditItem(null);
    setEditForm(blankForm());
    setEditOpen(true);
  };

  const openEdit = (b: BudgetView) => {
    setEditItem(b);
    setEditForm({
      period_type: b.period_type as Period,
      category_id: b.category_id,
      amount: String(b.amount),
      start_date: b.start_date,
      end_date: b.end_date,
      alert_rate: b.alert_rate,
      roll_over: b.roll_over,
    });
    setEditOpen(true);
  };

  const submitEdit = async () => {
    const amount = parseFloat(editForm.amount);
    if (!amount || amount <= 0) { toast.error('请输入有效预算金额'); return; }
    const payload = {
      book_id: bookId,
      category_id: editForm.category_id,
      period_type: editForm.period_type,
      amount,
      start_date: editForm.start_date,
      end_date: editForm.end_date,
      alert_rate: editForm.alert_rate,
      roll_over: editForm.roll_over,
    };
    if (editItem) {
      await budgetApi.update(editItem.id, payload);
      toast.success('已更新预算');
    } else {
      await budgetApi.create(payload);
      toast.success('已创建预算');
    }
    setEditOpen(false);
    load();
  };

  const doDelete = async () => {
    if (!delTarget) return;
    await budgetApi.remove(delTarget.id);
    toast.success('已删除预算');
    setDelTarget(null);
    load();
  };

  return (
    <div className="space-y-5">
      {/* 总预算卡片 */}
      <section className={cn(
        'rounded-2xl p-6 text-white shadow-soft relative overflow-hidden',
        totalBudget?.is_over_budget
          ? 'bg-gradient-to-br from-red-500 via-red-600 to-rose-600'
          : 'bg-gradient-to-br from-brand-600 via-indigo-600 to-purple-600'
      )}>
        <div className="absolute -right-20 -top-20 w-72 h-72 bg-white/10 rounded-full blur-3xl" />
        <div className="relative">
          <div className="flex items-center gap-2 text-white/70 text-sm">
            <Target size={16} /> 总预算
          </div>
          <div className="mt-2 flex items-end justify-between flex-wrap gap-3">
            <div>
              <div className="text-4xl font-bold tabular-nums tracking-tight">
                {formatMoney(totalBudget?.amount || 0)}
              </div>
              <div className="text-sm text-white/70 mt-1 flex items-center gap-2 flex-wrap">
                <span>周期：{PERIOD_LABELS[(totalBudget?.period_type || 'monthly') as Period]}</span>
                {totalBudget && (
                  <span>
                    <Calendar size={12} className="inline" />{' '}
                    {totalBudget.start_date} ~ {totalBudget.end_date}
                  </span>
                )}
              </div>
            </div>
            <div className="text-right">
              <div className="text-white/70 text-xs">已使用</div>
              <div className={cn(
                'text-2xl font-bold tabular-nums',
                totalBudget?.is_over_budget ? 'text-red-100' : ''
              )}>
                {formatMoney(totalBudget?.used_amount || 0)}
                <span className="text-white/60 text-lg ml-1">/ {formatMoney(totalBudget?.amount || 0)}</span>
              </div>
              <div className="text-xs mt-1">
                占比 {pct(totalBudget?.used_amount || 0, totalBudget?.amount || 1)}%
              </div>
            </div>
          </div>
          <div className="mt-5">
            <Progress
              value={totalBudget?.used_amount || 0}
              total={totalBudget?.amount || 1}
              alert={totalBudget?.alert_rate || 0.8}
            />
          </div>
          <div className="mt-3 grid grid-cols-2 md:grid-cols-3 gap-3 text-sm">
            <div className="rounded-xl bg-white/10 backdrop-blur px-3 py-2">
              <div className="text-white/60 text-xs flex items-center gap-1">
                <TrendingDown size={12} /> 剩余
              </div>
              <div className="font-semibold tabular-nums mt-0.5">
                {formatMoney(Math.max(0, totalBudget?.remaining || 0))}
              </div>
            </div>
            <div className="rounded-xl bg-white/10 backdrop-blur px-3 py-2">
              <div className="text-white/60 text-xs flex items-center gap-1">
                <PieIcon size={12} /> 日均预算
              </div>
              <div className="font-semibold tabular-nums mt-0.5">
                {(totalBudget && totalBudget.daily_budget > 0)
                  ? formatMoney(totalBudget.daily_budget)
                  : <span className="text-red-200">已超支</span>}
              </div>
            </div>
            <div className="rounded-xl bg-white/10 backdrop-blur px-3 py-2 col-span-2 md:col-span-1">
              <div className="text-white/60 text-xs flex items-center gap-1">
                {catBudgets.length > 0
                  ? <><TrendingUp size={12} /> 分类预算已分配</>
                  : <>分类预算未设置</>}
              </div>
              <div className="font-semibold tabular-nums mt-0.5">
                {formatMoney(totalCatUsed)} / {formatMoney(catBudgets.reduce((s, b) => s + b.amount, 0))}
                <span className="text-white/60 ml-1 text-xs">({catBudgets.length} 项)</span>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* 操作栏 */}
      <section className="card card-body flex items-center justify-between flex-wrap gap-3">
        <h3 className="font-semibold text-slate-800 flex items-center gap-2">
          <Target size={18} className="text-amber-500" /> 分类预算列表
        </h3>
        <div className="flex items-center gap-2">
          {!totalBudget && (
            <button
              className="btn-secondary btn-sm"
              onClick={() => {
                openCreate();
                setEditForm(f => ({ ...f, category_id: 0 }));
              }}
            >设置总预算</button>
          )}
          <button className="btn-primary btn-sm" onClick={openCreate}>
            <Plus size={14} /> 新增预算
          </button>
        </div>
      </section>

      {/* 预算列表 */}
      <section className="grid sm:grid-cols-2 lg:grid-cols-3 gap-4">
        {loading ? (
          <div className="col-span-full py-16 text-center text-slate-400 text-sm">加载中...</div>
        ) : catBudgets.length === 0 ? (
          <div className="col-span-full">
            <Empty text="还没有分类预算，合理设置预算更能控制消费 💡" />
          </div>
        ) : (
          catBudgets.map(b => {
            const cat = expCats.find(c => c.id === b.category_id);
            const over = b.is_over_budget;
            const warn = b.usage_rate >= b.alert_rate * 100 && !over;
            return (
              <div
                key={b.id}
                className={cn(
                  'card card-body relative overflow-hidden transition hover:-translate-y-0.5 hover:shadow-lg',
                  over && 'ring-1 ring-red-200'
                )}
              >
                {over && (
                  <span className="absolute top-3 right-3 chip bg-red-100 text-red-600 flex items-center gap-1">
                    <AlertTriangle size={12} /> 已超支
                  </span>
                )}
                {warn && !over && (
                  <span className="absolute top-3 right-3 chip bg-amber-100 text-amber-600 flex items-center gap-1">
                    <AlertTriangle size={12} /> 预警
                  </span>
                )}
                <div className="flex items-center gap-3">
                  <div
                    className="w-11 h-11 rounded-xl grid place-items-center text-2xl"
                    style={{ background: (cat?.color || '#64748b') + '15' }}
                  >
                    {cat?.icon || '📊'}
                  </div>
                  <div>
                    <div className="font-semibold text-slate-800">{cat?.name || '总预算'}</div>
                    <div className="text-xs text-slate-400 mt-0.5">
                      {PERIOD_LABELS[b.period_type as Period]} · {b.start_date.slice(5)} ~ {b.end_date.slice(5)}
                    </div>
                  </div>
                </div>
                <div className="mt-5">
                  <div className="flex justify-between items-baseline mb-1">
                    <div>
                      <span className={cn(
                        'text-2xl font-bold tabular-nums',
                        over ? 'text-red-500' : 'text-slate-800'
                      )}>
                        {formatMoney(b.used_amount)}
                      </span>
                      <span className="text-slate-400 text-sm tabular-nums ml-1">
                        / {formatMoney(b.amount)}
                      </span>
                    </div>
                    <span className={cn(
                      'text-sm font-semibold tabular-nums',
                      over ? 'text-red-500' : warn ? 'text-amber-500' : 'text-brand-600'
                    )}>
                      {b.usage_rate}%
                    </span>
                  </div>
                  <Progress value={b.used_amount} total={b.amount} alert={b.alert_rate} />
                </div>
                <div className="mt-4 pt-3 border-t border-slate-100 grid grid-cols-2 gap-2 text-sm">
                  <div>
                    <div className="text-xs text-slate-400">剩余</div>
                    <div className={cn(
                      'font-semibold tabular-nums',
                      b.remaining < 0 ? 'text-red-500' : 'text-slate-700'
                    )}>
                      {formatMoney(Math.max(0, b.remaining))}
                    </div>
                  </div>
                  <div>
                    <div className="text-xs text-slate-400">日均可用</div>
                    <div className={cn(
                      'font-semibold tabular-nums',
                      b.daily_budget < 0 ? 'text-red-500' : 'text-slate-700'
                    )}>
                      {b.daily_budget > 0 ? formatMoney(b.daily_budget) : <span>—</span>}
                    </div>
                  </div>
                </div>
                <div className="mt-3 flex items-center gap-1">
                  <button className="btn-ghost btn-sm flex-1" onClick={() => openEdit(b)}>
                    <Edit3 size={14} /> 编辑
                  </button>
                  <button
                    className="btn-ghost btn-sm flex-1 text-red-500 hover:bg-red-50"
                    onClick={() => setDelTarget(b)}
                  >
                    <Trash2 size={14} /> 删除
                  </button>
                </div>
              </div>
            );
          })
        )}
      </section>

      {/* 新增/编辑 预算弹窗 */}
      <Modal
        open={editOpen}
        onClose={() => setEditOpen(false)}
        title={editItem ? '编辑预算' : '新增预算'}
        footer={
          <>
            <button className="btn-secondary" onClick={() => setEditOpen(false)}>取消</button>
            <button className="btn-primary" onClick={submitEdit}>
              <Check size={16} /> 保存
            </button>
          </>
        }
      >
        <div className="space-y-4">
          <div>
            <label className="label">适用分类</label>
            <select
              className="input"
              value={editForm.category_id}
              onChange={e => setEditForm(f => ({ ...f, category_id: Number(e.target.value) }))}
            >
              <option value={0}>📊 总预算（全部支出）</option>
              <optgroup label="支出分类">
                {expCats.filter(c => !c.parent_id).map(c => (
                  <option key={c.id} value={c.id}>{c.icon} {c.name}</option>
                ))}
              </optgroup>
            </select>
          </div>
          <div>
            <label className="label">预算周期</label>
            <div className="grid grid-cols-3 gap-2">
              {(['monthly', 'yearly', 'custom'] as Period[]).map(p => (
                <button
                  key={p}
                  onClick={() => {
                    const nf = { ...editForm, period_type: p };
                    if (p === 'monthly') {
                      const r = getMonthRange();
                      nf.start_date = formatDate(r.start);
                      nf.end_date = formatDate(r.end);
                    } else if (p === 'yearly') {
                      const y = new Date().getFullYear();
                      nf.start_date = `${y}-01-01`;
                      nf.end_date = `${y}-12-31`;
                    }
                    setEditForm(nf);
                  }}
                  className={cn(
                    'py-2 rounded-lg border text-sm transition',
                    editForm.period_type === p
                      ? 'border-brand-500 bg-brand-50 text-brand-700'
                      : 'border-slate-200 text-slate-600 hover:bg-slate-50'
                  )}
                >
                  {PERIOD_LABELS[p]}
                </button>
              ))}
            </div>
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="label">开始日期</label>
              <input
                type="date" className="input"
                value={editForm.start_date}
                onChange={e => setEditForm(f => ({ ...f, start_date: e.target.value }))}
              />
            </div>
            <div>
              <label className="label">结束日期</label>
              <input
                type="date" className="input"
                value={editForm.end_date}
                onChange={e => setEditForm(f => ({ ...f, end_date: e.target.value }))}
              />
            </div>
          </div>
          <div>
            <label className="label">预算金额 *</label>
            <div className="relative">
              <span className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400 font-semibold">¥</span>
              <input
                className="input pl-8 !text-lg !font-semibold"
                type="number"
                placeholder="例如 3000"
                value={editForm.amount}
                onChange={e => setEditForm(f => ({ ...f, amount: e.target.value }))}
              />
            </div>
          </div>
          <div>
            <label className="label">
              预警比例：{Math.round(editForm.alert_rate * 100)}%
            </label>
            <input
              type="range"
              min={0.5} max={1} step={0.05}
              value={editForm.alert_rate}
              onChange={e => setEditForm(f => ({ ...f, alert_rate: parseFloat(e.target.value) }))}
              className="w-full accent-brand-600"
            />
            <div className="flex justify-between text-[11px] text-slate-400 mt-1">
              <span>50%</span><span>80%</span><span>100%</span>
            </div>
          </div>
          <label className="flex items-center justify-between p-3 bg-slate-50 rounded-lg">
            <div>
              <div className="text-sm font-medium text-slate-700">结余滚动</div>
              <div className="text-xs text-slate-400 mt-0.5">本期未用完的预算自动结转至下期</div>
            </div>
            <input
              type="checkbox" className="w-4 h-4 accent-brand-600"
              checked={editForm.roll_over}
              onChange={e => setEditForm(f => ({ ...f, roll_over: e.target.checked }))}
            />
          </label>
        </div>
      </Modal>

      <ConfirmDialog
        open={!!delTarget}
        onClose={() => setDelTarget(null)}
        onConfirm={doDelete}
        title="删除预算"
        desc={`确定删除预算「${delTarget && (expCats.find(c => c.id === delTarget.category_id)?.name || '总预算')}」吗？`}
        okText="删除"
        danger
      />
    </div>
  );
}
