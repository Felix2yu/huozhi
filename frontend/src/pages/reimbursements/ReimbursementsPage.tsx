import { useEffect, useMemo, useState } from 'react';
import { toast } from 'sonner';
import { useAppStore } from '@/stores/app';
import { reimbApi } from '@/api';
import type { Reimbursement } from '@/types';
import { cn, formatMoney, formatDate } from '@/utils';
import { Plus, Trash2, Edit3, CheckCircle, Clock, AlertCircle, FileText } from 'lucide-react';
import { Modal, ConfirmDialog, Empty } from '@/components/common';
import { PageHeader, HeroCard, HeroStat } from '@/components/common/page';

const STATUS_MAP: Record<string, { label: string; color: string; icon: React.ReactNode }> = {
  pending: { label: '待报销', color: 'text-amber-600 bg-amber-50', icon: <Clock size={14} /> },
  partial: { label: '部分到账', color: 'text-blue-600 bg-blue-50', icon: <AlertCircle size={14} /> },
  received: { label: '已到账', color: 'text-emerald-600 bg-emerald-50', icon: <CheckCircle size={14} /> },
};

export default function ReimbursementsPage() {
  const bookId = useAppStore(s => s.currentBookId);

  const [list, setList] = useState<Reimbursement[]>([]);
  const [loading, setLoading] = useState(false);

  const [createOpen, setCreateOpen] = useState(false);
  const [editOpen, setEditOpen] = useState(false);
  const [editItem, setEditItem] = useState<Reimbursement | null>(null);
  const [createForm, setCreateForm] = useState(blankCreateForm());
  const [editForm, setEditForm] = useState(blankEditForm());
  const [delTarget, setDelTarget] = useState<Reimbursement | null>(null);

  function blankCreateForm() {
    return {
      name: '',
      total_amount: '',
      remark: '',
    };
  }

  function blankEditForm() {
    return {
      status: 'pending',
      received_amount: '',
      remark: '',
    };
  }

  const load = async () => {
    if (!bookId) return;
    setLoading(true);
    try {
      const res = await reimbApi.list();
      setList(res || []);
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
    received: list.reduce((s, i) => s + i.received_amount, 0),
    pending: list.filter(i => i.status === 'pending').length,
  }), [list]);

  const openCreate = () => {
    setCreateForm(blankCreateForm());
    setCreateOpen(true);
  };

  const openEdit = (item: Reimbursement) => {
    setEditItem(item);
    setEditForm({
      status: item.status,
      received_amount: String(item.received_amount),
      remark: item.remark || '',
    });
    setEditOpen(true);
  };

  const submitCreate = async () => {
    if (!createForm.name.trim()) { toast.error('请输入名称'); return; }
    const amount = parseFloat(createForm.total_amount);
    if (!amount || amount <= 0) { toast.error('请输入有效金额'); return; }

    await reimbApi.create({
      book_id: bookId,
      name: createForm.name.trim(),
      total_amount: amount,
      remark: createForm.remark || undefined,
    });
    toast.success('已创建报销单');
    setCreateOpen(false);
    load();
  };

  const submitEdit = async () => {
    if (!editItem) return;

    await reimbApi.update(editItem.id, {
      status: editForm.status,
      received_amount: parseFloat(editForm.received_amount || '0'),
      remark: editForm.remark || undefined,
    });
    toast.success('已更新');
    setEditOpen(false);
    load();
  };

  const doDelete = async () => {
    if (!delTarget) return;
    await reimbApi.remove(delTarget.id);
    toast.success('已删除');
    setDelTarget(null);
    load();
  };

  return (
    <div className="space-y-5">
      <PageHeader
        title="报销管理"
        subtitle="跟踪报销申请和到账状态"
        actions={
          <button className="btn-primary btn-sm" onClick={openCreate}>
            <Plus size={14} /> 新增报销
          </button>
        }
      />
      <HeroCard>
        <div className="flex items-center gap-2 text-white/70 text-sm">
          <FileText size={16} /> 报销管理总览
        </div>
        <div className="mt-2 flex items-end justify-between flex-wrap gap-4">
          <HeroStat label="报销总额" value={formatMoney(totals.total)} className="text-3xl md:text-4xl" />
          <div className="flex gap-3">
            <HeroStat label="已到账" value={formatMoney(totals.received)} />
            <HeroStat label="待报销" value={`${totals.pending} 笔`} />
          </div>
        </div>
      </HeroCard>

      {/* 报销列表 */}
      <section className="space-y-3">
        {loading ? (
          <div className="card card-body py-16 text-center text-slate-400 text-sm">加载中...</div>
        ) : filtered.length === 0 ? (
          <div className="card card-body">
            <Empty text="还没有报销记录" icon={<FileText size={32} />} />
          </div>
        ) : (
          filtered.map(item => {
            const statusInfo = STATUS_MAP[item.status] || STATUS_MAP.pending;
            return (
              <div key={item.id} className="card card-body">
                <div className="flex items-start justify-between gap-3">
                  <div className="flex items-center gap-3">
                    <div className={cn('w-12 h-12 rounded-xl grid place-items-center', statusInfo.color)}>
                      {statusInfo.icon}
                    </div>
                    <div>
                      <div className="font-semibold text-slate-800">{item.name}</div>
                      <div className="text-xs text-slate-400 mt-0.5">
                        <span className={cn('px-1.5 py-0.5 rounded', statusInfo.color)}>
                          {statusInfo.label}
                        </span>
                        <span className="ml-2">{formatDate(item.submitted_at)}</span>
                      </div>
                    </div>
                  </div>
                  <div className="text-right">
                    <div className="text-xl font-bold tabular-nums text-slate-800">
                      {formatMoney(item.total_amount)}
                    </div>
                    {item.received_amount > 0 && (
                      <div className="text-xs text-emerald-600 mt-0.5">
                        已到账 {formatMoney(item.received_amount)}
                      </div>
                    )}
                  </div>
                </div>

                {item.remark && (
                  <div className="mt-3 text-sm text-slate-500 bg-slate-50 rounded-lg p-3">
                    {item.remark}
                  </div>
                )}

                <div className="mt-3 flex items-center justify-end gap-1">
                  <button
                    className="btn-secondary btn-sm"
                    onClick={() => openEdit(item)}
                  >
                    <Edit3 size={14} /> 更新状态
                  </button>
                  <button
                    className="btn-ghost btn-sm text-red-500 hover:bg-red-50"
                    onClick={() => setDelTarget(item)}
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
        title="新建报销单"
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
              placeholder="如：差旅报销、办公用品"
              value={createForm.name}
              onChange={e => setCreateForm(f => ({ ...f, name: e.target.value }))}
            />
          </div>
          <div>
            <label className="label">报销金额 *</label>
            <div className="relative">
              <span className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400 font-semibold">¥</span>
              <input
                className="input pl-8 !text-xl !font-semibold"
                type="number"
                placeholder="0.00"
                value={createForm.total_amount}
                onChange={e => setCreateForm(f => ({ ...f, total_amount: e.target.value }))}
              />
            </div>
          </div>
          <div>
            <label className="label">备注</label>
            <textarea
              className="input min-h-[80px]"
              placeholder="可选备注"
              value={createForm.remark}
              onChange={e => setCreateForm(f => ({ ...f, remark: e.target.value }))}
            />
          </div>
        </div>
      </Modal>

      {/* 更新状态弹窗 */}
      <Modal
        open={editOpen}
        onClose={() => setEditOpen(false)}
        title="更新报销状态"
        footer={
          <>
            <button className="btn-secondary" onClick={() => setEditOpen(false)}>取消</button>
            <button className="btn-primary" onClick={submitEdit}>保存</button>
          </>
        }
      >
        <div className="space-y-4">
          <div>
            <label className="label">报销状态</label>
            <div className="flex gap-2">
              {(['pending', 'partial', 'received'] as const).map(s => (
                <button
                  key={s}
                  onClick={() => setEditForm(f => ({ ...f, status: s }))}
                  className={cn(
                    'flex-1 py-2 rounded-lg border text-sm font-medium transition',
                    editForm.status === s
                      ? 'border-brand-500 bg-brand-50 text-brand-600'
                      : 'border-slate-200 text-slate-500 hover:bg-slate-50'
                  )}
                >
                  {STATUS_MAP[s].label}
                </button>
              ))}
            </div>
          </div>
          {editForm.status !== 'pending' && (
            <div>
              <label className="label">已到账金额</label>
              <div className="relative">
                <span className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400 font-semibold">¥</span>
                <input
                  className="input pl-8"
                  type="number"
                  placeholder="0.00"
                  value={editForm.received_amount}
                  onChange={e => setEditForm(f => ({ ...f, received_amount: e.target.value }))}
                />
              </div>
            </div>
          )}
          <div>
            <label className="label">备注</label>
            <textarea
              className="input min-h-[80px]"
              placeholder="可选备注"
              value={editForm.remark}
              onChange={e => setEditForm(f => ({ ...f, remark: e.target.value }))}
            />
          </div>
        </div>
      </Modal>

      <ConfirmDialog
        open={!!delTarget}
        onClose={() => setDelTarget(null)}
        onConfirm={doDelete}
        title="删除报销单"
        desc={`确定删除「${delTarget?.name}」吗？`}
        okText="删除"
        danger
      />
    </div>
  );
}
