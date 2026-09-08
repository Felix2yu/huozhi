import { useEffect, useMemo, useState } from 'react';
import { toast } from 'sonner';
import { useAppStore } from '@/stores/app';
import { recurringApi, accountApi } from '@/api';
import type { Recurring, Account } from '@/types';
import { cn, formatMoney, formatDate } from '@/utils';
import { Plus, Trash2, Pause, Play, Calendar, Repeat, ArrowRightLeft } from 'lucide-react';
import { Modal, ConfirmDialog, Empty } from '@/components/common';
import { PageHeader, HeroCard, HeroStat } from '@/components/common/page';

const TYPE_MAP: Record<string, { label: string; color: string }> = {
  expense: { label: '支出', color: 'text-expense' },
  income: { label: '收入', color: 'text-income' },
  transfer: { label: '转账', color: 'text-blue-600' },
};

const RECURRING_MAP: Record<string, string> = {
  daily: '每天',
  weekly: '每周',
  biweekly: '每两周',
  monthly: '每月',
  yearly: '每年',
  custom: '自定义间隔',
};

export default function RecurringPage() {
  const bookId = useAppStore(s => s.currentBookId);
  const categories = useAppStore(s => s.categories);

  const [list, setList] = useState<Recurring[]>([]);
  const [loading, setLoading] = useState(false);
  const [accList, setAccList] = useState<Account[]>([]);

  const [createOpen, setCreateOpen] = useState(false);
  const [form, setForm] = useState(blankForm());
  const [delTarget, setDelTarget] = useState<Recurring | null>(null);

  function blankForm() {
    return {
      name: '',
      type: 'expense',
      amount: '',
      category_id: 0,
      account_id: 0,
      to_account_id: 0,
      description: '',
      recurring_type: 'monthly',
      interval: 1,
      weekday: 1,
      month_day: 1,
      start_date: formatDate(new Date()),
      end_date: '',
      max_times: 0,
    };
  }

  const load = async () => {
    if (!bookId) return;
    setLoading(true);
    try {
      const [res, accRes] = await Promise.all([
        recurringApi.list(),
        accountApi.list({ book_id: bookId, include_archived: 0 }).catch(() => ({ accounts: [] as Account[] } as any)),
      ]);
      setList(res || []);
      setAccList((accRes as any).accounts || accRes || []);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, [bookId]); // eslint-disable-line

  const filtered = useMemo(() => {
    return [...list].sort((a, b) => (a.created_at < b.created_at ? 1 : -1));
  }, [list]);

  const openCreate = () => {
    setForm(blankForm());
    setCreateOpen(true);
  };

  const submitCreate = async () => {
    if (!form.name.trim()) { toast.error('请输入名称'); return; }
    const amount = parseFloat(form.amount);
    if (!amount || amount <= 0) { toast.error('请输入有效金额'); return; }
    if (!form.category_id) { toast.error('请选择分类'); return; }
    if (!form.account_id) { toast.error('请选择账户'); return; }

    await recurringApi.create({
      book_id: bookId,
      name: form.name.trim(),
      type: form.type,
      amount,
      category_id: form.category_id,
      account_id: form.account_id,
      to_account_id: form.type === 'transfer' ? form.to_account_id : 0,
      description: form.description || undefined,
      recurring_type: form.recurring_type,
      interval: form.interval,
      weekday: form.weekday,
      month_day: form.month_day,
      start_date: form.start_date,
      end_date: form.end_date || undefined,
      max_times: form.max_times,
    });
    toast.success('已创建周期记账');
    setCreateOpen(false);
    load();
  };

  const toggleStatus = async (item: Recurring) => {
    await recurringApi.toggle(item.id);
    toast.success(item.status === 'active' ? '已暂停' : '已启用');
    load();
  };

  const doDelete = async () => {
    if (!delTarget) return;
    await recurringApi.remove(delTarget.id);
    toast.success('已删除');
    setDelTarget(null);
    load();
  };

  const expenseCategories = categories.expense.filter(c => !c.parent_id);
  const incomeCategories = categories.income.filter(c => !c.parent_id);

  return (
    <div className="space-y-5">
      <PageHeader
        title="周期记账"
        subtitle="管理自动发生的周期性收支规则"
        actions={
          <button className="btn-primary btn-sm" onClick={openCreate}>
            <Plus size={14} /> 新增规则
          </button>
        }
      />
      <HeroCard>
        <div className="flex items-center gap-2 text-white/70 text-sm">
          <Repeat size={16} /> 周期记账
        </div>
        <div className="mt-2 flex items-end justify-between flex-wrap gap-4">
          <HeroStat label="规则总数" value={list.length} className="text-3xl md:text-4xl" />
          <div className="flex gap-3">
            <HeroStat label="进行中" value={list.filter(i => i.status === 'active').length} />
            <HeroStat label="已暂停" value={list.filter(i => i.status === 'paused').length} />
          </div>
        </div>
      </HeroCard>

      {/* 规则列表 */}
      <section className="space-y-3">
        {loading ? (
          <div className="card card-body py-16 text-center text-slate-400 text-sm">加载中...</div>
        ) : filtered.length === 0 ? (
          <div className="card card-body">
            <Empty text="还没有周期记账规则，创建一个试试" icon={<Repeat size={32} />} />
          </div>
        ) : (
          filtered.map(item => {
            const typeInfo = TYPE_MAP[item.type] || TYPE_MAP.expense;
            const acc = accList.find(a => a.id === item.account_id);
            const toAcc = accList.find(a => a.id === item.to_account_id);
            const paused = item.status === 'paused';
            return (
              <div
                key={item.id}
                className={cn('card card-body', paused && 'opacity-60')}
              >
                <div className="flex items-start justify-between gap-3">
                  <div className="flex items-center gap-3">
                    <div className={cn(
                      'w-12 h-12 rounded-xl grid place-items-center text-2xl',
                      item.type === 'expense' ? 'bg-red-50' : item.type === 'income' ? 'bg-emerald-50' : 'bg-blue-50'
                    )}>
                      {item.type === 'transfer' ? <ArrowRightLeft size={20} className="text-blue-600" />
                        : item.type === 'income' ? '💰' : '💸'}
                    </div>
                    <div>
                      <div className="font-semibold text-slate-800">{item.name}</div>
                      <div className="text-xs text-slate-400 flex items-center gap-2 mt-0.5">
                        <span className={typeInfo.color}>{typeInfo.label}</span>
                        <span>·</span>
                        <span>{RECURRING_MAP[item.recurring_type]}</span>
                        {item.recurring_type === 'custom' && <span>每 {item.interval} 天</span>}
                        <span>·</span>
                        <span>下次: {formatDate(item.next_run_at)}</span>
                      </div>
                    </div>
                  </div>
                  <div className="text-right">
                    <div className={cn('text-xl font-bold tabular-nums', typeInfo.color)}>
                      {item.type === 'income' ? '+' : '-'}{formatMoney(item.amount)}
                    </div>
                    <div className="text-xs text-slate-400 mt-0.5">
                      {acc?.name || '-'}
                      {item.type === 'transfer' && toAcc && (
                        <span> → {toAcc.name}</span>
                      )}
                    </div>
                  </div>
                </div>

                <div className="mt-3 flex items-center justify-between text-xs text-slate-400">
                  <div className="flex items-center gap-2">
                    <Calendar size={12} />
                    <span>已执行 {item.run_count} 次</span>
                    {item.max_times > 0 && <span>· 上限 {item.max_times} 次</span>}
                  </div>
                  <div className="flex items-center gap-1">
                    <button
                      className={cn(
                        'p-1.5 rounded-lg transition',
                        paused ? 'text-emerald-600 hover:bg-emerald-50' : 'text-amber-600 hover:bg-amber-50'
                      )}
                      onClick={() => toggleStatus(item)}
                      title={paused ? '启用' : '暂停'}
                    >
                      {paused ? <Play size={14} /> : <Pause size={14} />}
                    </button>
                    <button
                      className="p-1.5 rounded-lg text-red-500 hover:bg-red-50"
                      onClick={() => setDelTarget(item)}
                      title="删除"
                    >
                      <Trash2 size={14} />
                    </button>
                  </div>
                </div>
              </div>
            );
          })
        )}
      </section>

      {/* 新建弹窗 */}
      <Modal
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        title="新建周期记账"
        footer={
          <>
            <button className="btn-secondary" onClick={() => setCreateOpen(false)}>取消</button>
            <button className="btn-primary" onClick={submitCreate}>保存</button>
          </>
        }
      >
        <div className="space-y-4">
          <div>
            <label className="label">名称 *</label>
            <input
              className="input"
              placeholder="如：房租、工资、话费"
              value={form.name}
              onChange={e => setForm(f => ({ ...f, name: e.target.value }))}
            />
          </div>

          <div>
            <label className="label">类型 *</label>
            <div className="flex gap-2">
              {(['expense', 'income', 'transfer'] as const).map(t => (
                <button
                  key={t}
                  onClick={() => setForm(f => ({ ...f, type: t }))}
                  className={cn(
                    'flex-1 py-2 rounded-lg border text-sm font-medium transition',
                    form.type === t
                      ? 'border-brand-500 bg-brand-50 text-brand-600'
                      : 'border-slate-200 text-slate-500 hover:bg-slate-50'
                  )}
                >
                  {TYPE_MAP[t].label}
                </button>
              ))}
            </div>
          </div>

          <div>
            <label className="label">金额 *</label>
            <div className="relative">
              <span className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400 font-semibold">¥</span>
              <input
                className="input pl-8 !text-xl !font-semibold"
                type="number"
                placeholder="0.00"
                value={form.amount}
                onChange={e => setForm(f => ({ ...f, amount: e.target.value }))}
              />
            </div>
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="label">分类 *</label>
              <select
                className="input"
                value={form.category_id}
                onChange={e => setForm(f => ({ ...f, category_id: Number(e.target.value) }))}
              >
                <option value={0}>请选择</option>
                {(form.type === 'income' ? incomeCategories : expenseCategories).map(c => (
                  <option key={c.id} value={c.id}>{c.icon} {c.name}</option>
                ))}
              </select>
            </div>
            <div>
              <label className="label">账户 *</label>
              <select
                className="input"
                value={form.account_id}
                onChange={e => setForm(f => ({ ...f, account_id: Number(e.target.value) }))}
              >
                <option value={0}>请选择</option>
                {accList.filter(a => !a.is_archived).map(a => (
                  <option key={a.id} value={a.id}>{a.name}</option>
                ))}
              </select>
            </div>
          </div>

          {form.type === 'transfer' && (
            <div>
              <label className="label">转入账户</label>
              <select
                className="input"
                value={form.to_account_id}
                onChange={e => setForm(f => ({ ...f, to_account_id: Number(e.target.value) }))}
              >
                <option value={0}>请选择</option>
                {accList.filter(a => !a.is_archived).map(a => (
                  <option key={a.id} value={a.id}>{a.name}</option>
                ))}
              </select>
            </div>
          )}

          <div>
            <label className="label">周期类型 *</label>
            <select
              className="input"
              value={form.recurring_type}
              onChange={e => setForm(f => ({ ...f, recurring_type: e.target.value }))}
            >
              <option value="daily">每天</option>
              <option value="weekly">每周</option>
              <option value="biweekly">每两周</option>
              <option value="monthly">每月</option>
              <option value="yearly">每年</option>
              <option value="custom">自定义间隔</option>
            </select>
          </div>

          {form.recurring_type === 'custom' && (
            <div>
              <label className="label">间隔天数</label>
              <input
                type="number"
                className="input"
                min={1}
                value={form.interval}
                onChange={e => setForm(f => ({ ...f, interval: Number(e.target.value) }))}
              />
            </div>
          )}

          {form.recurring_type === 'monthly' && (
            <div>
              <label className="label">每月第几天</label>
              <input
                type="number"
                className="input"
                min={1}
                max={31}
                value={form.month_day}
                onChange={e => setForm(f => ({ ...f, month_day: Number(e.target.value) }))}
              />
            </div>
          )}

          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="label">开始日期 *</label>
              <input
                type="date"
                className="input"
                value={form.start_date}
                onChange={e => setForm(f => ({ ...f, start_date: e.target.value }))}
              />
            </div>
            <div>
              <label className="label">结束日期（可选）</label>
              <input
                type="date"
                className="input"
                value={form.end_date}
                onChange={e => setForm(f => ({ ...f, end_date: e.target.value }))}
              />
            </div>
          </div>

          <div>
            <label className="label">最大执行次数（0=无限）</label>
            <input
              type="number"
              className="input"
              min={0}
              value={form.max_times}
              onChange={e => setForm(f => ({ ...f, max_times: Number(e.target.value) }))}
            />
          </div>

          <div>
            <label className="label">备注</label>
            <input
              className="input"
              placeholder="可选备注"
              value={form.description}
              onChange={e => setForm(f => ({ ...f, description: e.target.value }))}
            />
          </div>
        </div>
      </Modal>

      <ConfirmDialog
        open={!!delTarget}
        onClose={() => setDelTarget(null)}
        onConfirm={doDelete}
        title="删除周期记账"
        desc={`确定删除「${delTarget?.name}」吗？已生成的交易不受影响。`}
        okText="删除"
        danger
      />
    </div>
  );
}
