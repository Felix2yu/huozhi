import { useEffect, useMemo, useState } from 'react';
import { toast } from 'sonner';
import { useAppStore } from '@/stores/app';
import { savingApi, accountApi } from '@/api';
import type { SavingPlan, Account } from '@/types';
import { cn, formatMoney, formatDate, pct } from '@/utils';
import {
  Plus, Edit3, Trash2, Calendar, Wallet, Target, PiggyBank,
  Check, X, Sparkles, Rocket, Coins, Award,
} from 'lucide-react';
import { Modal, ConfirmDialog, Empty, Progress } from '@/components/common';

const PRESET_COLORS = ['#F59E0B', '#10B981', '#6366F1', '#EC4899', '#06B6D4', '#EF4444'];
const PRESET_ICONS = ['🏠', '🚗', '💍', '🎓', '🌍', '💻', '📱', '🎮', '🎬', '🛍️', '⚽', '🎨'];
const PRESET_NAMES = [
  { name: '旅行基金', icon: '🌍' },
  { name: '买房首付', icon: '🏠' },
  { name: '买车储蓄', icon: '🚗' },
  { name: '应急基金', icon: '🛡️' },
  { name: '换新手机', icon: '📱' },
  { name: '进修学习', icon: '🎓' },
];

type StatusFilter = 'all' | 'active' | 'done' | 'paused';

export default function SavingsPage() {
  const bookId = useAppStore(s => s.currentBookId);
  const accounts = useAppStore(s => s.accounts);

  const [list, setList] = useState<SavingPlan[]>([]);
  const [loading, setLoading] = useState(false);
  const [statusFilter, setStatusFilter] = useState<StatusFilter>('all');
  const [accList, setAccList] = useState<Account[]>([]);

  // 编辑
  const [editOpen, setEditOpen] = useState(false);
  const [editItem, setEditItem] = useState<SavingPlan | null>(null);
  const [editForm, setEditForm] = useState(blankForm());

  // 记录存钱
  const [recordOpen, setRecordOpen] = useState(false);
  const [recordItem, setRecordItem] = useState<SavingPlan | null>(null);
  const [recordForm, setRecordForm] = useState({
    amount: '', description: '', tx_date: formatDate(new Date()),
  });

  const [delTarget, setDelTarget] = useState<SavingPlan | null>(null);

  function blankForm() {
    const now = new Date();
    const nextYear = new Date(now.getFullYear() + 1, now.getMonth(), now.getDate());
    return {
      name: '', account_id: 0,
      target_amount: '', current_amount: '0',
      start_date: formatDate(now),
      target_date: formatDate(nextYear),
      icon: '🌍', color: PRESET_COLORS[0],
      status: 'active' as SavingPlan['status'],
    };
  }

  const load = async () => {
    if (!bookId) return;
    setLoading(true);
    try {
      const [res, accRes] = await Promise.all([
        savingApi.list(),
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
    let arr = list;
    if (statusFilter !== 'all') arr = arr.filter(s => s.status === statusFilter);
    return arr.sort((a, b) => a.created_at < b.created_at ? 1 : -1);
  }, [list, statusFilter]);

  const totals = useMemo(() => ({
    target: list.reduce((s, p) => s + p.target_amount, 0),
    current: list.reduce((s, p) => s + p.current_amount, 0),
    done: list.filter(p => p.status === 'done').length,
  }), [list]);

  const openCreate = () => {
    setEditItem(null);
    setEditForm(blankForm());
    setEditOpen(true);
  };
  const openEdit = (p: SavingPlan) => {
    setEditItem(p);
    setEditForm({
      name: p.name,
      account_id: p.account_id,
      target_amount: String(p.target_amount),
      current_amount: String(p.current_amount),
      start_date: p.start_date,
      target_date: p.target_date,
      icon: p.icon || '🌍',
      color: p.color || PRESET_COLORS[0],
      status: p.status,
    });
    setEditOpen(true);
  };

  const submitEdit = async () => {
    if (!editForm.name.trim()) { toast.error('请输入计划名称'); return; }
    const ta = parseFloat(editForm.target_amount);
    if (!ta || ta <= 0) { toast.error('请输入有效目标金额'); return; }
    const payload = {
      book_id: bookId,
      name: editForm.name.trim(),
      account_id: editForm.account_id || undefined,
      target_amount: ta,
      current_amount: parseFloat(editForm.current_amount || '0'),
      start_date: editForm.start_date,
      target_date: editForm.target_date,
      icon: editForm.icon,
      color: editForm.color,
      status: editForm.status,
    };
    if (editItem) {
      await savingApi.update(editItem.id, payload);
      toast.success('已更新计划');
    } else {
      await savingApi.create(payload);
      toast.success('已创建计划');
    }
    setEditOpen(false);
    load();
  };

  const openRecord = (p: SavingPlan) => {
    setRecordItem(p);
    setRecordForm({
      amount: '',
      description: '',
      tx_date: formatDate(new Date()),
    });
    setRecordOpen(true);
  };

  const submitRecord = async () => {
    if (!recordItem) return;
    const amount = parseFloat(recordForm.amount);
    if (!amount || amount <= 0) { toast.error('请输入有效金额'); return; }
    await savingApi.addRecord(recordItem.id, {
      amount,
      description: recordForm.description || undefined,
      tx_date: recordForm.tx_date,
    });
    toast.success(`已存入 ${formatMoney(amount)}`);
    setRecordOpen(false);
    load();
  };

  const doDelete = async () => {
    if (!delTarget) return;
    await savingApi.remove(delTarget.id);
    toast.success('已删除计划');
    setDelTarget(null);
    load();
  };

  // 计算剩余天数
  function daysLeft(targetDateStr: string) {
    const t = new Date(targetDateStr).getTime();
    const now = Date.now();
    return Math.ceil((t - now) / 86_400_000);
  }

  return (
    <div className="space-y-5">
      {/* 总览卡片 */}
      <section className="rounded-2xl bg-gradient-to-br from-amber-500 via-orange-500 to-rose-500 p-6 text-white shadow-soft relative overflow-hidden">
        <div className="absolute -right-10 -top-10 w-48 h-48 bg-white/10 rounded-full blur-2xl" />
        <div className="absolute right-20 bottom-0 w-32 h-32 bg-white/10 rounded-full blur-xl" />
        <div className="relative">
          <div className="flex items-center gap-2 text-white/70 text-sm">
            <PiggyBank size={16} /> 存钱计划总览
          </div>
          <div className="mt-2 flex items-end justify-between flex-wrap gap-4">
            <div>
              <div className="text-white/60 text-xs">已存金额</div>
              <div className="text-4xl font-bold tabular-nums tracking-tight">
                {formatMoney(totals.current)}
              </div>
              <div className="text-sm text-white/70 mt-1">
                目标金额 {formatMoney(totals.target)}
                <span className="mx-2 text-white/40">·</span>
                进度 {pct(totals.current, totals.target)}%
              </div>
            </div>
            <div className="space-y-1 text-right">
              <div className="text-white/60 text-xs">共 {list.length} 个计划</div>
              <div className="flex gap-2">
                <div className="px-3 py-1.5 rounded-lg bg-white/10 backdrop-blur text-xs">
                  <Award size={12} className="inline mr-1" />
                  已达成 <b>{totals.done}</b>
                </div>
                <div className="px-3 py-1.5 rounded-lg bg-white/10 backdrop-blur text-xs">
                  <Rocket size={12} className="inline mr-1" />
                  进行中 <b>{list.filter(p => p.status === 'active').length}</b>
                </div>
              </div>
            </div>
          </div>
          <div className="mt-5">
            <div className="w-full h-2 bg-white/20 rounded-full overflow-hidden backdrop-blur">
              <div
                className="h-full rounded-full bg-white/80 transition-all"
                style={{ width: `${Math.min(100, pct(totals.current, totals.target))}%` }}
              />
            </div>
          </div>
        </div>
      </section>

      {/* 操作栏 */}
      <section className="card card-body flex items-center justify-between flex-wrap gap-3">
        <div className="flex gap-1 p-0.5 bg-slate-100 rounded-lg">
          {[
            { k: 'all', label: '全部' },
            { k: 'active', label: '进行中' },
            { k: 'done', label: '已达成' },
            { k: 'paused', label: '已暂停' },
          ].map(s => (
            <button
              key={s.k}
              onClick={() => setStatusFilter(s.k as StatusFilter)}
              className={cn(
                'px-3 py-1.5 rounded-md text-xs font-medium transition',
                statusFilter === s.k ? 'bg-white text-slate-700 shadow-sm' : 'text-slate-500'
              )}
            >{s.label}</button>
          ))}
        </div>
        <button className="btn-primary btn-sm" onClick={openCreate}>
          <Plus size={14} /> 新建存钱计划
        </button>
      </section>

      {/* 计划卡片列表 */}
      <section className="grid sm:grid-cols-2 lg:grid-cols-3 gap-4">
        {loading ? (
          <div className="col-span-full py-16 text-center text-slate-400 text-sm">加载中...</div>
        ) : filtered.length === 0 ? (
          <div className="col-span-full">
            <Empty text="还没有存钱计划，从一个小目标开始吧 🎯" icon={<PiggyBank size={32} />} />
          </div>
        ) : (
          filtered.map(p => {
            const progress = pct(p.current_amount, p.target_amount);
            const done = p.status === 'done' || progress >= 100;
            const days = daysLeft(p.target_date);
            const remaining = p.target_amount - p.current_amount;
            const acc = accList.find(a => a.id === p.account_id);
            return (
              <div
                key={p.id}
                className={cn(
                  'card card-body relative overflow-hidden transition hover:-translate-y-0.5 hover:shadow-lg',
                  done && 'ring-1 ring-emerald-200'
                )}
                style={{
                  background: done
                    ? 'linear-gradient(135deg, #ecfdf5 0%, #f0fdf4 100%)'
                    : undefined,
                }}
              >
                {/* 角标 */}
                <div className="absolute top-0 right-0 w-20 h-20 rounded-bl-full opacity-10 pointer-events-none"
                  style={{ background: p.color }}
                />
                {done && (
                  <span className="absolute top-3 right-3 chip bg-emerald-500 text-white flex items-center gap-1">
                    <Sparkles size={12} /> 已达成
                  </span>
                )}
                {p.status === 'paused' && !done && (
                  <span className="absolute top-3 right-3 chip bg-slate-200 text-slate-600">已暂停</span>
                )}

                <div className="flex items-start justify-between relative">
                  <div className="flex items-center gap-3">
                    <div
                      className="w-14 h-14 rounded-2xl grid place-items-center text-3xl shadow-sm"
                      style={{ background: (p.color || PRESET_COLORS[0]) + '20' }}
                    >
                      {p.icon || '💰'}
                    </div>
                    <div>
                      <div className="font-bold text-slate-800 text-lg">{p.name}</div>
                      {acc && (
                        <div className="text-xs text-slate-400 mt-0.5 flex items-center gap-1">
                          <Wallet size={11} /> {acc.name}
                        </div>
                      )}
                    </div>
                  </div>
                </div>

                <div className="mt-5 grid grid-cols-2 gap-3 text-sm">
                  <div>
                    <div className="text-xs text-slate-400 flex items-center gap-1">
                      <Coins size={12} /> 当前
                    </div>
                    <div className="text-xl font-bold tabular-nums text-slate-800 mt-0.5">
                      {formatMoney(p.current_amount)}
                    </div>
                  </div>
                  <div>
                    <div className="text-xs text-slate-400 flex items-center gap-1">
                      <Target size={12} /> 目标
                    </div>
                    <div className="text-xl font-bold tabular-nums mt-0.5"
                      style={{ color: p.color }}
                    >
                      {formatMoney(p.target_amount)}
                    </div>
                  </div>
                </div>

                <div className="mt-4">
                  <div className="flex items-center justify-between text-xs mb-1.5">
                    <span className="text-slate-500">进度 {progress}%</span>
                    <span className={cn(
                      'font-medium tabular-nums',
                      done ? 'text-emerald-600' : days < 0 ? 'text-red-500' : 'text-slate-500'
                    )}>
                      {done ? '🎉 恭喜完成'
                        : remaining > 0 && days > 0
                          ? `还需 ${formatMoney(remaining)} · ${days} 天`
                          : days < 0 ? `已超期 ${-days} 天` : '加油！'}
                    </span>
                  </div>
                  <Progress
                    value={p.current_amount}
                    total={p.target_amount}
                    alert={0.8}
                  />
                </div>

                <div className="mt-3 flex items-center justify-between text-xs text-slate-400">
                  <div className="flex items-center gap-1">
                    <Calendar size={12} />
                    {p.start_date.slice(0, 7)} → {p.target_date.slice(0, 7)}
                  </div>
                  {remaining > 0 && days > 0 && (
                    <div>
                      日均需存 <b className="text-slate-600 tabular-nums">
                        {formatMoney(remaining / days)}
                      </b>
                    </div>
                  )}
                </div>

                <div className="mt-4 pt-3 border-t border-slate-100 flex items-center gap-1">
                  <button
                    className="btn-primary btn-sm flex-1"
                    onClick={() => openRecord(p)}
                    disabled={done}
                  >
                    <Coins size={14} /> 存入
                  </button>
                  <button className="btn-secondary btn-sm" onClick={() => openEdit(p)}>
                    <Edit3 size={14} />
                  </button>
                  <button
                    className="btn-ghost btn-sm text-red-500 hover:bg-red-50"
                    onClick={() => setDelTarget(p)}
                  >
                    <Trash2 size={14} />
                  </button>
                </div>
              </div>
            );
          })
        )}
      </section>

      {/* 新增/编辑 弹窗 */}
      <Modal
        open={editOpen}
        onClose={() => setEditOpen(false)}
        title={editItem ? '编辑存钱计划' : '新建存钱计划'}
        footer={
          <>
            <button className="btn-secondary" onClick={() => setEditOpen(false)}>
              <X size={14} /> 取消
            </button>
            <button className="btn-primary" onClick={submitEdit}>
              <Check size={16} /> 保存
            </button>
          </>
        }
      >
        <div className="space-y-4">
          <div>
            <label className="label">快捷模板</label>
            <div className="flex gap-2 flex-wrap">
              {PRESET_NAMES.map(p => (
                <button
                  key={p.name}
                  onClick={() => setEditForm(f => ({ ...f, name: p.name, icon: p.icon }))}
                  className="chip !py-1.5 !px-3 bg-slate-50 hover:bg-brand-50 hover:text-brand-600 border border-slate-100"
                >
                  {p.icon} {p.name}
                </button>
              ))}
            </div>
          </div>
          <div>
            <label className="label">计划名称 *</label>
            <input
              className="input"
              placeholder="如：6个月存够旅行基金"
              value={editForm.name}
              onChange={e => setEditForm(f => ({ ...f, name: e.target.value }))}
            />
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="label">当前已存</label>
              <input
                type="number" className="input"
                value={editForm.current_amount}
                onChange={e => setEditForm(f => ({ ...f, current_amount: e.target.value }))}
              />
            </div>
            <div>
              <label className="label">目标金额 *</label>
              <input
                type="number" className="input"
                placeholder="如 30000"
                value={editForm.target_amount}
                onChange={e => setEditForm(f => ({ ...f, target_amount: e.target.value }))}
              />
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
              <label className="label">目标日期</label>
              <input
                type="date" className="input"
                value={editForm.target_date}
                onChange={e => setEditForm(f => ({ ...f, end_date: e.target.value, target_date: e.target.value }))}
              />
            </div>
          </div>
          <div>
            <label className="label">关联账户（可选）</label>
            <select
              className="input"
              value={editForm.account_id}
              onChange={e => setEditForm(f => ({ ...f, account_id: Number(e.target.value) }))}
            >
              <option value={0}>不关联</option>
              {accList.filter(a => !a.is_archived).map(a => (
                <option key={a.id} value={a.id}>{a.name}（余额 {formatMoney(a.balance)}）</option>
              ))}
            </select>
          </div>
          <div>
            <label className="label">选择图标</label>
            <div className="grid grid-cols-6 gap-2">
              {PRESET_ICONS.map(ic => (
                <button
                  key={ic}
                  onClick={() => setEditForm(f => ({ ...f, icon: ic }))}
                  className={cn(
                    'aspect-square rounded-lg grid place-items-center text-2xl border transition',
                    editForm.icon === ic
                      ? 'border-brand-500 bg-brand-50'
                      : 'border-slate-100 hover:bg-slate-50'
                  )}
                >{ic}</button>
              ))}
            </div>
          </div>
          <div>
            <label className="label">选择颜色</label>
            <div className="flex gap-2 flex-wrap">
              {PRESET_COLORS.map(c => (
                <button
                  key={c}
                  onClick={() => setEditForm(f => ({ ...f, color: c }))}
                  className={cn(
                    'w-8 h-8 rounded-full transition',
                    editForm.color === c && 'ring-2 ring-offset-2 ring-slate-400'
                  )}
                  style={{ background: c }}
                />
              ))}
            </div>
          </div>
          {editItem && (
            <div>
              <label className="label">状态</label>
              <select
                className="input"
                value={editForm.status}
                onChange={e => setEditForm(f => ({ ...f, status: e.target.value as any }))}
              >
                <option value="active">进行中</option>
                <option value="paused">暂停</option>
                <option value="done">已达成</option>
              </select>
            </div>
          )}
        </div>
      </Modal>

      {/* 记录存钱 */}
      <Modal
        open={recordOpen}
        onClose={() => setRecordOpen(false)}
        title={`存入 · ${recordItem?.name || ''}`}
        footer={
          <>
            <button className="btn-secondary" onClick={() => setRecordOpen(false)}>取消</button>
            <button className="btn-primary" onClick={submitRecord}>
              <Check size={16} /> 确认存入
            </button>
          </>
        }
      >
        <div className="space-y-4">
          <div>
            <label className="label">存入金额 *</label>
            <div className="relative">
              <span className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400 font-semibold">¥</span>
              <input
                className="input pl-8 !text-xl !font-semibold"
                type="number"
                placeholder="如 1000"
                value={recordForm.amount}
                onChange={e => setRecordForm(r => ({ ...r, amount: e.target.value }))}
              />
            </div>
            {recordItem && !isNaN(parseFloat(recordForm.amount)) && (
              <div className="mt-2 p-3 rounded-lg bg-emerald-50 text-sm text-emerald-700">
                存入后进度：
                <b className="tabular-nums ml-1">
                  {pct(recordItem.current_amount + parseFloat(recordForm.amount), recordItem.target_amount)}%
                </b>
              </div>
            )}
          </div>
          <div>
            <label className="label">存入日期</label>
            <input
              type="date" className="input"
              value={recordForm.tx_date}
              onChange={e => setRecordForm(r => ({ ...r, tx_date: e.target.value }))}
            />
          </div>
          <div>
            <label className="label">备注（可选）</label>
            <input
              className="input"
              placeholder="比如：发工资存入、年终奖..."
              value={recordForm.description}
              onChange={e => setRecordForm(r => ({ ...r, description: e.target.value }))}
            />
          </div>
        </div>
      </Modal>

      <ConfirmDialog
        open={!!delTarget}
        onClose={() => setDelTarget(null)}
        onConfirm={doDelete}
        title="删除存钱计划"
        desc={`确定删除计划「${delTarget?.name}」吗？已存入的金额不会从账户中扣除，仅删除此计划的记录。`}
        okText="删除"
        danger
      />
    </div>
  );
}
