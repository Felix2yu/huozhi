import { useEffect, useMemo, useState } from 'react';
import { toast } from 'sonner';
import { useAppStore } from '@/stores/app';
import { installmentApi, accountApi } from '@/api';
import type { Installment, Account } from '@/types';
import { cn, formatMoney, formatDate } from '@/utils';
import { Plus, Trash2, Calendar, CreditCard, CheckCircle } from 'lucide-react';
import { Modal, ConfirmDialog, Empty, Progress } from '@/components/common';
import { PageHeader, HeroCard, HeroStat } from '@/components/common/page';

export default function InstallmentsPage() {
  const bookId = useAppStore(s => s.currentBookId);
  const categories = useAppStore(s => s.categories);

  const [list, setList] = useState<Installment[]>([]);
  const [loading, setLoading] = useState(false);
  const [accList, setAccList] = useState<Account[]>([]);

  const [createOpen, setCreateOpen] = useState(false);
  const [form, setForm] = useState(blankForm());
  const [delTarget, setDelTarget] = useState<Installment | null>(null);

  function blankForm() {
    const today = formatDate(new Date());
    return {
      name: '',
      total_amount: '',
      total_months: 12,
      interest_amount: '0',
      category_id: 0,
      account_id: 0,
      first_repay_date: today,
      description: '',
    };
  }

  const load = async () => {
    if (!bookId) return;
    setLoading(true);
    try {
      const [res, accRes] = await Promise.all([
        installmentApi.list(),
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

  const totals = useMemo(() => ({
    total: list.reduce((s, i) => s + i.total_amount, 0),
    paid: list.reduce((s, i) => s + i.monthly_amount * i.paid_months, 0),
    active: list.filter(i => i.status === 'active').length,
  }), [list]);

  const openCreate = () => {
    setForm(blankForm());
    setCreateOpen(true);
  };

  const submitCreate = async () => {
    if (!form.name.trim()) { toast.error('请输入名称'); return; }
    const totalAmount = parseFloat(form.total_amount);
    if (!totalAmount || totalAmount <= 0) { toast.error('请输入有效总额'); return; }
    if (!form.total_months || form.total_months < 1) { toast.error('请输入有效期数'); return; }
    if (!form.category_id) { toast.error('请选择分类'); return; }
    if (!form.account_id) { toast.error('请选择账户'); return; }

    await installmentApi.create({
      book_id: bookId,
      name: form.name.trim(),
      total_amount: totalAmount,
      total_months: form.total_months,
      interest_amount: parseFloat(form.interest_amount || '0'),
      category_id: form.category_id,
      account_id: form.account_id,
      first_repay_date: form.first_repay_date,
      description: form.description || undefined,
    });
    toast.success('已创建分期');
    setCreateOpen(false);
    load();
  };

  const doDelete = async () => {
    if (!delTarget) return;
    await installmentApi.remove(delTarget.id);
    toast.success('已删除');
    setDelTarget(null);
    load();
  };

  const expenseCategories = categories.expense.filter(c => !c.parent_id);

  // 计算月供
  const monthlyAmount = form.total_amount && form.total_months
    ? (parseFloat(form.total_amount) / form.total_months).toFixed(2)
    : '0.00';

  return (
    <div className="space-y-5">
      <PageHeader
        title="分期管理"
        subtitle="管理分期付款计划和还款进度"
        actions={
          <button className="btn-primary btn-sm" onClick={openCreate}>
            <Plus size={14} /> 新增分期
          </button>
        }
      />
      <HeroCard>
        <div className="flex items-center gap-2 text-white/70 text-sm">
          <CreditCard size={16} /> 分期管理总览
        </div>
        <div className="mt-2 flex items-end justify-between flex-wrap gap-4">
          <HeroStat label="分期总额" value={formatMoney(totals.total)} className="text-3xl md:text-4xl" />
          <div className="flex gap-3">
            <HeroStat label="已还" value={formatMoney(totals.paid)} />
            <HeroStat label="进行中" value={`${totals.active} 笔`} />
          </div>
        </div>
      </HeroCard>

      {/* 分期列表 */}
      <section className="space-y-3">
        {loading ? (
          <div className="card card-body py-16 text-center text-slate-400 text-sm">加载中...</div>
        ) : filtered.length === 0 ? (
          <div className="card card-body">
            <Empty text="还没有分期记录" icon={<CreditCard size={32} />} />
          </div>
        ) : (
          filtered.map(item => {
            const acc = accList.find(a => a.id === item.account_id);
            const progress = item.total_months > 0 ? (item.paid_months / item.total_months) * 100 : 0;
            const isDone = item.status === 'done' || progress >= 100;
            return (
              <div
                key={item.id}
                className={cn('card card-body', isDone && 'opacity-60')}
              >
                <div className="flex items-start justify-between gap-3">
                  <div className="flex items-center gap-3">
                    <div className={cn(
                      'w-12 h-12 rounded-xl grid place-items-center',
                      isDone ? 'bg-emerald-50' : 'bg-indigo-50'
                    )}>
                      {isDone
                        ? <CheckCircle size={20} className="text-emerald-600" />
                        : <CreditCard size={20} className="text-indigo-600" />}
                    </div>
                    <div>
                      <div className="font-semibold text-slate-800">{item.name}</div>
                      <div className="text-xs text-slate-400 mt-0.5">
                        {item.total_months}期 · 每期 {formatMoney(item.monthly_amount)}
                        {item.interest_amount > 0 && (
                          <span className="ml-1 text-amber-500">利息 {formatMoney(item.interest_amount)}</span>
                        )}
                      </div>
                    </div>
                  </div>
                  <div className="text-right">
                    <div className="text-xl font-bold tabular-nums text-slate-800">
                      {formatMoney(item.total_amount)}
                    </div>
                    <div className="text-xs text-slate-400 mt-0.5">
                      {acc?.name || '-'}
                    </div>
                  </div>
                </div>

                <div className="mt-4">
                  <div className="flex items-center justify-between text-xs mb-1.5">
                    <span className="text-slate-500">
                      已还 {item.paid_months}/{item.total_months} 期 ({Math.round(progress)}%)
                    </span>
                    <span className="text-slate-400">
                      下次还款: {formatDate(item.next_repay_date)}
                    </span>
                  </div>
                  <Progress value={item.paid_months} total={item.total_months} />
                </div>

                <div className="mt-3 flex items-center justify-between text-xs text-slate-400">
                  <div className="flex items-center gap-1">
                    <Calendar size={12} />
                    首期: {formatDate(item.first_repay_date)}
                  </div>
                  <button
                    className="p-1.5 rounded-lg text-red-500 hover:bg-red-50"
                    onClick={() => setDelTarget(item)}
                    title="删除"
                  >
                    <Trash2 size={14} />
                  </button>
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
        title="新建分期"
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
              placeholder="如：iPhone 16 Pro 分期"
              value={form.name}
              onChange={e => setForm(f => ({ ...f, name: e.target.value }))}
            />
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="label">总额 *</label>
              <div className="relative">
                <span className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400 font-semibold">¥</span>
                <input
                  className="input pl-8"
                  type="number"
                  placeholder="0.00"
                  value={form.total_amount}
                  onChange={e => setForm(f => ({ ...f, total_amount: e.target.value }))}
                />
              </div>
            </div>
            <div>
              <label className="label">期数 *</label>
              <input
                type="number"
                className="input"
                min={1}
                placeholder="12"
                value={form.total_months}
                onChange={e => setForm(f => ({ ...f, total_months: Number(e.target.value) }))}
              />
            </div>
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="label">利息总额</label>
              <input
                type="number"
                className="input"
                placeholder="0"
                value={form.interest_amount}
                onChange={e => setForm(f => ({ ...f, interest_amount: e.target.value }))}
              />
            </div>
            <div>
              <label className="label">月供（自动计算）</label>
              <input
                className="input bg-slate-50"
                value={monthlyAmount}
                disabled
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
                {expenseCategories.map(c => (
                  <option key={c.id} value={c.id}>{c.icon} {c.name}</option>
                ))}
              </select>
            </div>
            <div>
              <label className="label">还款账户 *</label>
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

          <div>
            <label className="label">首期还款日 *</label>
            <input
              type="date"
              className="input"
              value={form.first_repay_date}
              onChange={e => setForm(f => ({ ...f, first_repay_date: e.target.value }))}
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
        title="删除分期"
        desc={`确定删除「${delTarget?.name}」吗？`}
        okText="删除"
        danger
      />
    </div>
  );
}
