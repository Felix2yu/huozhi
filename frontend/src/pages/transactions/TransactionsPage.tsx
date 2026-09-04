import { useEffect, useMemo, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { toast } from 'sonner';
import { useAppStore } from '@/stores/app';
import { txApi } from '@/api';
import type { Transaction, TransactionListData, DayGroup, TransactionType } from '@/types';
import { formatMoney, formatDate, getMonthRange, cn } from '@/utils';
import {
  Search, Filter, Plus, Trash2, Edit3, X, ChevronDown, Check,
  ArrowUpRight, ArrowDownRight, ArrowLeftRight, Calendar, Receipt,
  Image as ImageIcon, MapPin, Store, StickyNote,
} from 'lucide-react';
import { AmountBadge, ConfirmDialog, Drawer, Empty, Modal, TagChip } from '@/components/common';

export default function TransactionsPage() {
  const navigate = useNavigate();
  const bookId = useAppStore(s => s.currentBookId);
  const accounts = useAppStore(s => s.accounts);
  const tags = useAppStore(s => s.tags);
  const listVersion = useAppStore(s => s.listVersion);
  const { expense: expCats, income: incCats } = useAppStore(s => s.categories);

  const [loading, setLoading] = useState(false);
  const [data, setData] = useState<TransactionListData | null>(null);
  const [selectedIds, setSelectedIds] = useState<Set<number>>(new Set());
  const [delTarget, setDelTarget] = useState<{ type: 'single' | 'batch'; id?: number } | null>(null);
  const [detailTx, setDetailTx] = useState<Transaction | null>(null);
  const [previewImg, setPreviewImg] = useState<string | null>(null);

  // 筛选条件
  const range = getMonthRange();
  const [filters, setFilters] = useState({
    type: 'all' as TransactionType | 'all',
    category_id: 0,
    account_id: 0,
    tag_id: 0,
    reimburse_status: '' as '' | 'none' | 'pending' | 'done',
    keyword: '',
    start_date: formatDate(range.start),
    end_date: formatDate(range.end),
  });
  const [filterOpen, setFilterOpen] = useState(false);

  const allCats = useMemo(() => [...expCats, ...incCats], [expCats, incCats]);

  const loadList = async () => {
    if (!bookId) return;
    setLoading(true);
    try {
      const params: Record<string, any> = { book_id: bookId, ...filters };
      if (!params.category_id) delete params.category_id;
      if (!params.account_id) delete params.account_id;
      if (!params.tag_id) delete params.tag_id;
      if (!params.keyword) delete params.keyword;
      if (!params.reimburse_status) delete params.reimburse_status;
      if (params.type === 'all') delete params.type;
      const res = await txApi.list(params);
      setData(res);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadList();
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [bookId, filters.type, filters.category_id, filters.account_id, filters.tag_id, filters.start_date, filters.end_date, listVersion]);

  const summary = data?.summary;
  const groups: DayGroup[] = data?.grouped || [];

  const toggleSelect = (id: number) => {
    setSelectedIds(prev => {
      const s = new Set(prev);
      if (s.has(id)) s.delete(id); else s.add(id);
      return s;
    });
  };
  const selectAll = () => {
    if (selectedIds.size > 0) {
      setSelectedIds(new Set());
    } else {
      const all = (data?.flat_list || []).map(t => t.id);
      setSelectedIds(new Set(all));
    }
  };

  const doDelete = async () => {
    if (delTarget?.type === 'single' && delTarget.id) {
      await txApi.remove(delTarget.id);
      toast.success('已删除');
    } else if (delTarget?.type === 'batch') {
      if (selectedIds.size === 0) return;
      await txApi.batchRemove([...selectedIds]);
      toast.success(`已删除 ${selectedIds.size} 条`);
      setSelectedIds(new Set());
    }
    loadList();
  };

  const catOf = (id: number) => allCats.find(c => c.id === id);
  // 解析分类全路径：有二级分类时返回「一级 / 二级」，否则返回一级名
  const catPath = (id: number) => {
    const c = catOf(id);
    if (!c) return '';
    if (c.parent_id) {
      const p = catOf(c.parent_id);
      if (p) return `${p.name} / ${c.name}`;
    }
    return c.name;
  };
  const accOf = (id: number) => accounts.find(a => a.id === a.id);

  const resetFilters = () => {
    const r = getMonthRange();
    setFilters({
      type: 'all', category_id: 0, account_id: 0, tag_id: 0, reimburse_status: '', keyword: '',
      start_date: formatDate(r.start), end_date: formatDate(r.end),
    });
  };

  return (
    <div className="space-y-5">
      {/* 顶部汇总卡片 */}
      <section className="card card-body">
        <div className="grid grid-cols-3 gap-3 md:gap-6">
          <div className="text-center md:text-left">
            <div className="text-xs text-slate-500 flex items-center gap-1 md:justify-start justify-center">
              <ArrowUpRight size={14} className="text-emerald-500" /> 收入
            </div>
            <div className="text-lg md:text-2xl font-bold text-emerald-600 tabular-nums mt-1">
              {formatMoney(summary?.total_income || 0)}
            </div>
          </div>
          <div className="text-center">
            <div className="text-xs text-slate-500 flex items-center gap-1 justify-center">
              <ArrowDownRight size={14} className="text-red-500" /> 支出
            </div>
            <div className="text-lg md:text-2xl font-bold text-red-500 tabular-nums mt-1">
              {formatMoney(summary?.total_expense || 0)}
            </div>
          </div>
          <div className="text-center md:text-right">
            <div className="text-xs text-slate-500 flex items-center gap-1 md:justify-end justify-center">
              <Receipt size={14} className="text-brand-600" /> 结余
            </div>
            <div className={cn(
              'text-lg md:text-2xl font-bold tabular-nums mt-1',
              (summary?.net || 0) >= 0 ? 'text-brand-600' : 'text-red-500'
            )}>
              {(summary?.net || 0) >= 0 ? '+' : ''}{formatMoney(summary?.net || 0)}
            </div>
          </div>
        </div>
      </section>

      {/* 操作栏 */}
      <section className="card">
        <div className="card-body space-y-3">
          <div className="flex items-center gap-2 flex-wrap">
            <div className="flex-1 min-w-[180px] relative">
              <Search size={16} className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400" />
              <input
                className="input pl-9"
                placeholder="搜索描述/商户/备注"
                value={filters.keyword}
                onChange={(e) => setFilters(f => ({ ...f, keyword: e.target.value }))}
                onKeyDown={(e) => e.key === 'Enter' && loadList()}
              />
            </div>
            <button className="btn-secondary" onClick={() => setFilterOpen(true)}>
              <Filter size={16} /> 筛选
              {(filters.type !== 'all' || filters.category_id || filters.account_id || filters.tag_id) && (
                <span className="ml-1 w-2 h-2 rounded-full bg-red-500 inline-block" />
              )}
            </button>
            <Link to="/transactions/add" className="btn-primary">
              <Plus size={16} /> 记一笔
            </Link>
          </div>

          {/* 类型快捷筛选 */}
          <div className="flex gap-2 flex-wrap text-sm">
            {[
              { k: 'all', label: '全部' },
              { k: 'expense', label: '支出' },
              { k: 'income', label: '收入' },
              { k: 'transfer', label: '转账' },
            ].map(t => (
              <button
                key={t.k}
                onClick={() => setFilters(f => ({ ...f, type: t.k as any }))}
                className={cn(
                  'px-3 py-1 rounded-full border transition',
                  filters.type === t.k
                    ? 'bg-brand-600 text-white border-brand-600'
                    : 'bg-white border-slate-200 text-slate-600 hover:border-brand-300'
                )}
              >{t.label}</button>
            ))}
            <span className="ml-auto text-xs text-slate-400 self-center">
              {filters.start_date} ~ {filters.end_date}
            </span>
          </div>
        </div>

        {/* 批量操作栏 */}
        {selectedIds.size > 0 && (
          <div className="px-5 py-3 border-t border-slate-100 bg-brand-50 flex items-center justify-between flex-wrap gap-2">
            <span className="text-sm text-brand-700">已选 {selectedIds.size} 条</span>
            <div className="flex gap-2">
              <button className="btn-secondary btn-sm" onClick={selectAll}>全选/取消</button>
              <button
                className="btn-danger btn-sm"
                onClick={() => setDelTarget({ type: 'batch' })}
              >
                <Trash2 size={14} /> 批量删除
              </button>
            </div>
          </div>
        )}
      </section>

      {/* 列表 */}
      <section className="card card-body">
        {loading ? (
          <div className="py-16 text-center text-slate-400 text-sm">加载中...</div>
        ) : groups.length === 0 ? (
          <Empty text="还没有账单记录，点击右上角开始记账吧" />
        ) : (
          <div className="space-y-6">
            {groups.map(g => (
              <div key={g.date}>
                <div className="flex items-center justify-between mb-2 px-1">
                  <div className="flex items-center gap-2">
                    <Calendar size={14} className="text-slate-400" />
                    <span className="font-medium text-slate-700">{g.date}</span>
                    <span className="text-xs text-slate-400">
                      ({['周日','周一','周二','周三','周四','周五','周六'][new Date(g.date).getDay()]})
                    </span>
                  </div>
                  <div className="text-xs tabular-nums space-x-3">
                    <span className="text-emerald-600">+{formatMoney(g.day_income)}</span>
                    <span className="text-red-500">-{formatMoney(g.day_expense)}</span>
                  </div>
                </div>
                <ul className="divide-y divide-slate-50 rounded-xl border border-slate-100 overflow-hidden">
                  {g.transactions.map(t => {
                    const cat = catOf(t.category_id);
                    const acc = accounts.find(a => a.id === t.account_id);
                    const toAcc = t.to_account_id ? accounts.find(a => a.id === t.to_account_id) : null;
                    const checked = selectedIds.has(t.id);
                    return (
                      <li
                        key={t.id}
                        className={cn(
                          'flex items-center gap-3 p-3 hover:bg-slate-50 transition cursor-pointer',
                          checked && 'bg-brand-50/50'
                        )}
                        onClick={() => setDetailTx(t)}
                      >
                        <input
                          type="checkbox"
                          className="w-4 h-4 rounded accent-brand-600"
                          checked={checked}
                          onClick={e => e.stopPropagation()}
                          onChange={() => toggleSelect(t.id)}
                        />
                        <div
                          className="w-10 h-10 rounded-lg grid place-items-center text-xl shrink-0"
                          style={{ background: (cat?.color || '#64748b') + '15' }}
                        >
                          {t.type === 'transfer' ? <ArrowLeftRight size={18} className="text-indigo-600" /> : (cat?.icon || '📦')}
                        </div>
                        <div className="flex-1 min-w-0">
                          <div className="flex items-center gap-2 flex-wrap">
                            <span className="text-sm font-medium text-slate-800 truncate">
                              {t.description || cat?.name || (t.type === 'transfer' ? '转账' : '未分类')}
                            </span>
                            {t.type !== 'transfer' && (
                              <span
                                className={cn('chip',
                                  t.type === 'income' ? 'chip-income' :
                                  t.type === 'expense' ? 'chip-expense' : 'chip-transfer'
                                )}
                              >{catPath(t.category_id) || '-'}</span>
                            )}
                            {t.merchant && (
                              <span className="text-xs text-slate-400">{t.merchant}</span>
                            )}
                            {!!t.images?.length && (
                              <span className="inline-flex items-center gap-0.5 text-[11px] text-slate-400">
                                <ImageIcon size={12} /> {t.images.length}
                              </span>
                            )}
                            {t.remark && (
                              <StickyNote size={12} className="text-amber-400 shrink-0" />
                            )}
                          </div>
                          <div className="text-xs text-slate-400 mt-0.5 flex items-center gap-2 flex-wrap">
                            <span>{formatDate(t.tx_date, 'HH:mm')}</span>
                            {t.type === 'transfer' ? (
                              <span>{acc?.name || '-'} → {toAcc?.name || '-'}</span>
                            ) : (
                              <span>账户: {acc?.name || '-'}</span>
                            )}
                            {t.tags?.map(tg => (
                              <TagChip key={tg.id} label={tg.name} color={tg.color} />
                            ))}
                            {t.reimburse_status === 'done' && (
                              <span className="rounded px-1.5 py-0.5 text-[11px] font-medium bg-emerald-50 text-emerald-600">已报销</span>
                            )}
                            {t.reimburse_status === 'pending' && (
                              <span className="rounded px-1.5 py-0.5 text-[11px] font-medium bg-amber-50 text-amber-600">报销中</span>
                            )}
                          </div>
                        </div>
                        <div className="flex items-center gap-2 shrink-0" onClick={e => e.stopPropagation()}>
                          <AmountBadge type={t.type} amount={t.amount} />
                          <button
                            className="btn-ghost btn-sm hide-sm"
                            onClick={() => navigate(`/transactions/add?id=${t.id}`)}
                            title="编辑"
                          >
                            <Edit3 size={14} />
                          </button>
                          <button
                            className="btn-ghost btn-sm text-red-500 hover:bg-red-50 hide-sm"
                            onClick={() => setDelTarget({ type: 'single', id: t.id })}
                            title="删除"
                          >
                            <Trash2 size={14} />
                          </button>
                        </div>
                      </li>
                    );
                  })}
                </ul>
              </div>
            ))}
          </div>
        )}
      </section>

      {/* 账单详情弹窗 */}
      <Modal open={!!detailTx} onClose={() => setDetailTx(null)} title="账单明细" className="max-w-xl">
        {detailTx && (() => {
          const t = detailTx;
          const cat = catOf(t.category_id);
          const acc = accounts.find(a => a.id === t.account_id);
          const toAcc = t.to_account_id ? accounts.find(a => a.id === t.to_account_id) : null;
          return (
            <div className="space-y-5">
              {/* 金额 + 分类 */}
              <div className="text-center py-2">
                <div className="w-14 h-14 rounded-2xl grid place-items-center text-3xl mx-auto mb-3"
                  style={{ background: (cat?.color || '#64748b') + '15' }}>
                  {t.type === 'transfer' ? '🔄' : (cat?.icon || '📦')}
                </div>
                <AmountBadge type={t.type} amount={t.amount} />
                <div className="text-sm text-slate-500 mt-1">
                  {t.type === 'transfer' ? '转账' : (catPath(t.category_id) || '未分类')}
                </div>
              </div>

              {/* 字段 */}
              <div className="rounded-xl border border-slate-100 divide-y divide-slate-50 text-sm">
                <div className="flex justify-between px-4 py-2.5">
                  <span className="text-slate-400">时间</span>
                  <span className="text-slate-700">{formatDate(t.tx_date, 'YYYY-MM-DD HH:mm')}</span>
                </div>
                <div className="flex justify-between px-4 py-2.5">
                  <span className="text-slate-400">{t.type === 'transfer' ? '转出 → 转入' : '账户'}</span>
                  <span className="text-slate-700">
                    {t.type === 'transfer' ? `${acc?.name || '-'} → ${toAcc?.name || '-'}` : (acc?.name || '-')}
                  </span>
                </div>
                {t.description && (
                  <div className="flex justify-between px-4 py-2.5">
                    <span className="text-slate-400">描述</span>
                    <span className="text-slate-700 text-right">{t.description}</span>
                  </div>
                )}
                {t.merchant && (
                  <div className="flex justify-between px-4 py-2.5">
                    <span className="text-slate-400 flex items-center gap-1"><Store size={13} /> 商户</span>
                    <span className="text-slate-700">{t.merchant}</span>
                  </div>
                )}
                {t.location && (
                  <div className="flex justify-between px-4 py-2.5">
                    <span className="text-slate-400 flex items-center gap-1"><MapPin size={13} /> 位置</span>
                    <span className="text-slate-700 text-right">{t.location}</span>
                  </div>
                )}
                {t.tags && t.tags.length > 0 && (
                  <div className="flex justify-between items-center px-4 py-2.5">
                    <span className="text-slate-400">标签</span>
                    <span className="flex gap-1 flex-wrap justify-end">
                      {t.tags.map(tg => <TagChip key={tg.id} label={tg.name} color={tg.color} />)}
                    </span>
                  </div>
                )}
                {t.reimburse_status !== 'none' && (
                  <div className="flex justify-between px-4 py-2.5">
                    <span className="text-slate-400">报销</span>
                    <span className={t.reimburse_status === 'done' ? 'text-emerald-600' : 'text-amber-600'}>
                      {t.reimburse_status === 'done' ? '已报销' : '报销中'}
                    </span>
                  </div>
                )}
                {t.remark && (
                  <div className="px-4 py-2.5">
                    <div className="text-slate-400 mb-1 flex items-center gap-1"><StickyNote size={13} /> 备注</div>
                    <div className="text-slate-700 whitespace-pre-wrap break-words">{t.remark}</div>
                  </div>
                )}
              </div>

              {/* 图片 */}
              <div>
                <div className="text-xs text-slate-400 mb-2 flex items-center gap-1">
                  <ImageIcon size={13} /> 凭证照片 {t.images?.length ? `(${t.images.length})` : ''}
                </div>
                {t.images?.length ? (
                  <div className="grid grid-cols-4 md:grid-cols-5 gap-2">
                    {t.images.map((img, i) => (
                      <img
                        key={i}
                        src={img}
                        alt={`凭证 ${i + 1}`}
                        className="aspect-square w-full object-cover rounded-lg border border-slate-100 cursor-zoom-in hover:opacity-90 transition"
                        onClick={() => setPreviewImg(img)}
                        loading="lazy"
                      />
                    ))}
                  </div>
                ) : (
                  <div className="text-sm text-slate-300 py-4 text-center rounded-xl border border-dashed border-slate-100">
                    无图片
                  </div>
                )}
              </div>

              {/* 操作 */}
              <div className="flex gap-2 pt-2 border-t border-slate-100">
                <button
                  className="btn-primary flex-1"
                  onClick={() => navigate(`/transactions/add?id=${t.id}`)}
                >
                  <Edit3 size={15} /> 编辑
                </button>
                <button
                  className="btn-danger flex-1"
                  onClick={() => { setDetailTx(null); setDelTarget({ type: 'single', id: t.id }); }}
                >
                  <Trash2 size={15} /> 删除
                </button>
              </div>
            </div>
          );
        })()}
      </Modal>

      {/* 图片放大预览 */}
      {previewImg && (
        <div className="fixed inset-0 z-[60] bg-black/80 grid place-items-center p-4" onClick={() => setPreviewImg(null)}>
          <img src={previewImg} alt="预览" className="max-w-full max-h-full object-contain rounded-lg" />
          <button className="absolute top-4 right-4 w-9 h-9 rounded-full bg-white/20 text-white grid place-items-center hover:bg-white/30 transition">
            <X size={18} />
          </button>
        </div>
      )}

      {/* 筛选抽屉 */}
      <Drawer open={filterOpen} onClose={() => setFilterOpen(false)} title="高级筛选">
        <div className="space-y-4">
          <div>
            <label className="label">交易类型</label>
            <div className="grid grid-cols-3 gap-2">
              {['all', 'expense', 'income', 'transfer', 'refund', 'reimburse'].map(t => (
                <button
                  key={t}
                  onClick={() => setFilters(f => ({ ...f, type: t as any }))}
                  className={cn(
                    'btn-sm',
                    filters.type === t ? 'btn-primary' : 'btn-secondary'
                  )}
                >
                  {{all:'全部',expense:'支出',income:'收入',transfer:'转账',refund:'退款',reimburse:'报销'}[t]}
                </button>
              ))}
            </div>
          </div>
          <div>
            <label className="label">分类</label>
            <select
              className="input"
              value={filters.category_id}
              onChange={e => setFilters(f => ({ ...f, category_id: Number(e.target.value) }))}
            >
              <option value={0}>全部分类</option>
              <optgroup label="支出">
                {expCats.map(c => <option key={c.id} value={c.id}>{c.icon} {c.name}</option>)}
              </optgroup>
              <optgroup label="收入">
                {incCats.map(c => <option key={c.id} value={c.id}>{c.icon} {c.name}</option>)}
              </optgroup>
            </select>
          </div>
          <div>
            <label className="label">账户</label>
            <select
              className="input"
              value={filters.account_id}
              onChange={e => setFilters(f => ({ ...f, account_id: Number(e.target.value) }))}
            >
              <option value={0}>全部账户</option>
              {accounts.map(a => <option key={a.id} value={a.id}>{a.name}</option>)}
            </select>
          </div>
          <div>
            <label className="label">报销状态</label>
            <select
              className="input"
              value={filters.reimburse_status}
              onChange={e => setFilters(f => ({ ...f, reimburse_status: e.target.value as any }))}
            >
              <option value="">全部</option>
              <option value="none">未报销</option>
              <option value="pending">报销中</option>
              <option value="done">已报销</option>
            </select>
          </div>
          <div>
            <label className="label">标签</label>
            <select
              className="input"
              value={filters.tag_id}
              onChange={e => setFilters(f => ({ ...f, tag_id: Number(e.target.value) }))}
            >
              <option value={0}>全部标签</option>
              {tags.map(t => <option key={t.id} value={t.id}>#{t.name}</option>)}
            </select>
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="label">开始日期</label>
              <input
                type="date" className="input"
                value={filters.start_date}
                onChange={e => setFilters(f => ({ ...f, start_date: e.target.value }))}
              />
            </div>
            <div>
              <label className="label">结束日期</label>
              <input
                type="date" className="input"
                value={filters.end_date}
                onChange={e => setFilters(f => ({ ...f, end_date: e.target.value }))}
              />
            </div>
          </div>
          <div className="flex gap-2 pt-3 border-t border-slate-100">
            <button className="btn-secondary flex-1" onClick={resetFilters}>重置</button>
            <button className="btn-primary flex-1" onClick={() => { setFilterOpen(false); loadList(); }}>
              <Check size={16} /> 应用筛选
            </button>
          </div>
        </div>
      </Drawer>

      <ConfirmDialog
        open={!!delTarget}
        onClose={() => setDelTarget(null)}
        onConfirm={doDelete}
        title={delTarget?.type === 'batch' ? '批量删除' : '删除账单'}
        desc={
          delTarget?.type === 'batch'
            ? `确定要删除选中的 ${selectedIds.size} 条账单记录吗？此操作不可撤销。`
            : '确定要删除该条账单记录吗？此操作不可撤销。'
        }
        okText="删除"
        danger
      />
    </div>
  );
}
