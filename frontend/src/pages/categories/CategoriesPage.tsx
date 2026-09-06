import { useEffect, useMemo, useRef, useState } from 'react';
import { toast } from 'sonner';
import { useAppStore } from '@/stores/app';
import { categoryApi } from '@/api';
import type { Category, CategoryKind } from '@/types';
import { cn, formatMoney } from '@/utils';
import {
  Plus, Edit3, Trash2, ChevronDown, ChevronRight, FolderPlus,
  Check, X, GripVertical, Minus, ArrowUpRight,
} from 'lucide-react';
import { Modal, ConfirmDialog, Empty } from '@/components/common';
import { CATEGORY_ICONS as PRESET_ICONS, randomIcon } from '@/constants/icons';

const PRESET_COLORS = [
  '#6366F1', '#10B981', '#F59E0B', '#EF4444', '#8B5CF6',
  '#EC4899', '#06B6D4', '#F97316', '#84CC16', '#64748b',
  '#14B8A6', '#22C55E', '#A855F7', '#0EA5E9', '#F43F5E',
];

type TabKind = 'expense' | 'income';

export default function CategoriesPage() {
  const bookId = useAppStore(s => s.currentBookId);
  const { expense: expRaw, income: incRaw } = useAppStore(s => s.categories);
  const loadDicts = useAppStore(s => s.loadDictionaries);

  const [tab, setTab] = useState<TabKind>('expense');
  const [loading, setLoading] = useState(false);
  const [expanded, setExpanded] = useState<Set<number>>(new Set());

  const [editOpen, setEditOpen] = useState(false);
  const [editKind, setEditKind] = useState<CategoryKind>('expense');
  const [editParentId, setEditParentId] = useState(0);
  const [editItem, setEditItem] = useState<Category | null>(null);
  const [editForm, setEditForm] = useState({
    name: '', parent_id: 0, kind: 'expense' as CategoryKind,
    color: PRESET_COLORS[0], icon: '📦', sort: 0, need_tag: false,
  });

  const [delTarget, setDelTarget] = useState<Category | null>(null);

  const load = async () => {
    if (!bookId) return;
    await loadDicts(bookId);
  };

  useEffect(() => { load(); }, [bookId]); // eslint-disable-line

  const rawList = tab === 'expense' ? expRaw : incRaw;

  const tree = useMemo(() => {
    const roots = rawList.filter(c => !c.parent_id || c.parent_id === 0)
      .sort((a, b) => a.sort - b.sort);
    return roots.map(p => ({
      ...p,
      children: rawList.filter(c => c.parent_id === p.id).sort((a, b) => a.sort - b.sort),
    } as Category & { children: Category[] }));
  }, [rawList]);

  const openCreate = (kind: TabKind, parentId = 0) => {
    setEditKind(kind);
    setEditParentId(parentId);
    setEditItem(null);
    setEditForm({
      name: '', parent_id: parentId, kind,
      color: PRESET_COLORS[Math.floor(Math.random() * PRESET_COLORS.length)],
      icon: randomIcon(),
      sort: 0, need_tag: false,
    });
    setEditOpen(true);
  };

  const openEdit = (c: Category) => {
    setEditKind(c.kind as CategoryKind);
    setEditParentId(c.parent_id || 0);
    setEditItem(c);
    setEditForm({
      name: c.name, parent_id: c.parent_id || 0, kind: c.kind as CategoryKind,
      color: c.color || PRESET_COLORS[0], icon: c.icon || '📦',
      sort: c.sort, need_tag: c.need_tag,
    });
    setEditOpen(true);
  };

  const submitEdit = async () => {
    if (!editForm.name.trim()) { toast.error('请输入分类名称'); return; }
    const payload = {
      book_id: bookId,
      name: editForm.name.trim(),
      kind: editForm.kind,
      parent_id: editForm.parent_id || 0,
      color: editForm.color,
      icon: editForm.icon,
      sort: editForm.sort,
      need_tag: editForm.need_tag,
    };
    if (editItem) {
      await categoryApi.update(editItem.id, payload);
      toast.success('已更新');
    } else {
      await categoryApi.create(payload);
      toast.success('已创建');
    }
    setEditOpen(false);
    load();
  };

  const doDelete = async () => {
    if (!delTarget) return;
    await categoryApi.remove(delTarget.id);
    toast.success('已删除');
    setDelTarget(null);
    load();
  };

  const toggleExpand = (id: number) => {
    setExpanded(s => {
      const n = new Set(s);
      if (n.has(id)) n.delete(id); else n.add(id);
      return n;
    });
  };

  // ===== 拖拽排序 =====
  const dragRef = useRef<{ id: number; parentId: number } | null>(null);
  const [dropHint, setDropHint] = useState<{ id: number; pos: 'before' | 'after' } | null>(null);

  const onDragStart = (c: Category) => (e: React.DragEvent) => {
    dragRef.current = { id: c.id, parentId: c.parent_id || 0 };
    e.dataTransfer.effectAllowed = 'move';
    e.dataTransfer.setData('text/plain', String(c.id));
  };

  // 插入位置指示条：显示在目标行的上/下边缘
  const DropBar = ({ show, pos }: { show: boolean; pos: 'before' | 'after' }) =>
    show ? (
      <div
        className={cn(
          'absolute inset-x-2 h-1 bg-brand-500 rounded-full shadow-[0_0_6px_rgba(16,185,129,0.6)] pointer-events-none z-10',
          pos === 'before' ? 'top-0' : 'bottom-0'
        )}
      />
    ) : null;

  const onDragOver = (parentId: number, target: Category) => (e: React.DragEvent) => {
    const drag = dragRef.current;
    if (!drag || drag.parentId !== parentId || drag.id === target.id) return;
    e.preventDefault();
    const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
    const pos = e.clientY < rect.top + rect.height / 2 ? 'before' as const : 'after' as const;
    setDropHint(h => (h && h.id === target.id && h.pos === pos ? h : { id: target.id, pos }));
  };

  // 放下后：把被拖项移到目标行位置，并按新顺序重排同级 sort（0,10,20...）
  const onDrop = (parentId: number, target: Category) => async (e: React.DragEvent) => {
    e.preventDefault();
    const drag = dragRef.current;
    dragRef.current = null;
    setDropHint(null);
    if (!drag || drag.id === target.id || drag.parentId !== parentId) return;

    const siblings = (parentId === 0
      ? tree
      : tree.find(p => p.id === parentId)?.children || []
    ).slice();
    const from = siblings.findIndex(x => x.id === drag.id);
    if (from < 0) return;
    const [moved] = siblings.splice(from, 1);
    let at = siblings.findIndex(x => x.id === target.id);
    if (at < 0) return;
    const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
    if (e.clientY >= rect.top + rect.height / 2) at += 1; // 下半部分 → 放到目标之后
    siblings.splice(at, 0, moved);

    try {
      await categoryApi.reorder(siblings.map((x, i) => ({ id: x.id, sort: i * 10 })));
      toast.success('已调整顺序');
      load();
    } catch {
      toast.error('排序失败');
    }
  };

  return (
    <div className="space-y-5">
      {/* Tab */}
      <section className="card p-1.5">
        <div className="grid grid-cols-2 gap-1.5">
          <button
            onClick={() => setTab('expense')}
            className={cn(
              'flex items-center justify-center gap-1.5 py-2.5 rounded-lg font-medium transition',
              tab === 'expense' ? 'bg-expense-soft text-expense-fg' : 'text-slate-500 hover:bg-slate-50'
            )}
          >
            <Minus size={16} /> 支出分类
            <span className="chip chip-expense">{expRaw.length}</span>
          </button>
          <button
            onClick={() => setTab('income')}
            className={cn(
              'flex items-center justify-center gap-1.5 py-2.5 rounded-lg font-medium transition',
              tab === 'income' ? 'bg-income-soft text-income-fg' : 'text-slate-500 hover:bg-slate-50'
            )}
          >
            <ArrowUpRight size={16} /> 收入分类
            <span className="chip chip-income">{incRaw.length}</span>
          </button>
        </div>
      </section>

      {/* 顶部操作 */}
      <section className="card card-body">
        <div className="flex items-center justify-between flex-wrap gap-3">
          <h3 className="font-semibold text-slate-800">
            {tab === 'expense' ? '支出分类' : '收入分类'}树形结构
          </h3>
          <div className="flex items-center gap-2">
            <button
              className="btn-secondary btn-sm"
              onClick={() => openCreate(tab)}
            >
              <FolderPlus size={14} /> 新增一级分类
            </button>
            <button
              className="btn-primary btn-sm"
              onClick={() => openCreate(tab)}
            >
              <Plus size={14} /> 新增
            </button>
          </div>
        </div>
      </section>

      {/* 分类树 */}
      <section className="card card-body">
        {loading ? (
          <div className="py-16 text-center text-slate-400 text-sm">加载中...</div>
        ) : tree.length === 0 ? (
          <Empty text={`还没有${tab === 'expense' ? '支出' : '收入'}分类，点击右上角新增`} />
        ) : (
          <ul className="space-y-2">
            {tree.map(p => {
              const hasChildren = p.children.length > 0;
              const isOpen = expanded.has(p.id);
              return (
                <li key={p.id} className="border border-slate-100 rounded-xl overflow-hidden">
                  {/* 一级 */}
                  <div
                    className="relative flex items-center gap-2 p-3 hover:bg-slate-50 transition bg-slate-50/50"
                    onDragOver={onDragOver(0, p)}
                    onDrop={onDrop(0, p)}
                  >
                    <DropBar show={dropHint?.id === p.id && dropHint.pos === 'before'} pos="before" />
                    <DropBar show={dropHint?.id === p.id && dropHint.pos === 'after'} pos="after" />
                    <div
                      draggable
                      onDragStart={onDragStart(p)}
                      onDragEnd={() => setDropHint(null)}
                      title="拖动调整顺序"
                      className="w-8 h-8 rounded-lg grid place-items-center text-slate-300 hover:text-slate-500 hover:bg-slate-100 cursor-grab active:cursor-grabbing shrink-0"
                    >
                      <GripVertical size={16} />
                    </div>
                    <button
                      className="w-8 h-8 rounded-lg grid place-items-center text-slate-400 hover:bg-slate-200"
                      onClick={() => hasChildren && toggleExpand(p.id)}
                      disabled={!hasChildren}
                    >
                      {hasChildren
                        ? (isOpen ? <ChevronDown size={18} /> : <ChevronRight size={18} />)
                        : <span className="w-[18px]" />}
                    </button>
                    <div
                      className="w-10 h-10 rounded-lg grid place-items-center text-xl shrink-0"
                      style={{ background: (p.color || '#64748b') + '15' }}
                    >
                      {p.icon || '📦'}
                    </div>
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2">
                        <span className="font-medium text-slate-800">{p.name}</span>
                        {p.is_system && <span className="chip bg-blue-50 text-blue-600">系统</span>}
                        {p.need_tag && <span className="chip bg-purple-50 text-purple-600">需标签</span>}
                      </div>
                      <div className="text-xs text-slate-400 mt-0.5">
                        子分类 {p.children.length} 个
                      </div>
                    </div>
                    <button
                      className="btn-ghost btn-sm text-brand-600"
                      onClick={() => openCreate(tab, p.id)}
                      title="新增子分类"
                    >
                      <Plus size={14} />
                    </button>
                    <button
                      className="btn-ghost btn-sm"
                      onClick={() => openEdit(p)}
                      disabled={p.is_system}
                      title="编辑"
                    >
                      <Edit3 size={14} />
                    </button>
                    <button
                      className="btn-ghost btn-sm text-red-500 hover:bg-red-50"
                      onClick={() => setDelTarget(p)}
                      disabled={p.is_system}
                      title="删除"
                    >
                      <Trash2 size={14} />
                    </button>
                  </div>
                  {/* 二级 */}
                  {hasChildren && isOpen && (
                    <ul className="border-t border-slate-100">
                      {p.children.map(c => (
                        <li
                          key={c.id}
                          className="relative flex items-center gap-2 p-3 pl-14 hover:bg-slate-50 border-b border-slate-50 last:border-b-0 transition"
                          onDragOver={onDragOver(p.id, c)}
                          onDrop={onDrop(p.id, c)}
                        >
                          <DropBar show={dropHint?.id === c.id && dropHint.pos === 'before'} pos="before" />
                          <DropBar show={dropHint?.id === c.id && dropHint.pos === 'after'} pos="after" />
                          <div
                            draggable
                            onDragStart={onDragStart(c)}
                            onDragEnd={() => setDropHint(null)}
                            title="拖动调整顺序"
                            className="w-8 h-8 rounded-lg grid place-items-center text-slate-300 hover:text-slate-500 hover:bg-slate-100 cursor-grab active:cursor-grabbing shrink-0"
                          >
                            <GripVertical size={14} />
                          </div>
                          <div className="w-8 h-8 rounded-lg grid place-items-center text-base shrink-0"
                            style={{ background: (c.color || '#64748b') + '15' }}
                          >
                            {c.icon || '📦'}
                          </div>
                          <div className="flex-1 min-w-0">
                            <div className="flex items-center gap-2">
                              <span className="text-sm font-medium text-slate-700">{c.name}</span>
                              {c.is_system && <span className="chip bg-blue-50 text-blue-600 !text-[10px]">系统</span>}
                              {c.need_tag && <span className="chip bg-purple-50 text-purple-600 !text-[10px]">需标签</span>}
                            </div>
                          </div>
                          <button className="btn-ghost btn-sm" onClick={() => openEdit(c)} disabled={c.is_system}>
                            <Edit3 size={14} />
                          </button>
                          <button
                            className="btn-ghost btn-sm text-red-500 hover:bg-red-50"
                            onClick={() => setDelTarget(c)}
                            disabled={c.is_system}
                          >
                            <Trash2 size={14} />
                          </button>
                        </li>
                      ))}
                    </ul>
                  )}
                </li>
              );
            })}
          </ul>
        )}
      </section>

      {/* 新增/编辑弹窗 */}
      <Modal
        open={editOpen}
        onClose={() => setEditOpen(false)}
        title={
          editItem
            ? `编辑${editKind === 'expense' ? '支出' : '收入'}分类`
            : `新增${editKind === 'expense' ? '支出' : '收入'}分类${editParentId ? '（子分类）' : ''}`
        }
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
            <label className="label">分类名称 *</label>
            <input
              className="input"
              placeholder="如：餐饮、交通..."
              value={editForm.name}
              onChange={e => setEditForm(f => ({ ...f, name: e.target.value }))}
            />
          </div>
          <div>
            <label className="label">选择图标</label>
            <div className="grid grid-cols-8 gap-1.5 max-h-60 overflow-y-auto p-1 border border-slate-100 rounded-lg">
              {PRESET_ICONS.map(ic => (
                <button
                  key={ic}
                  onClick={() => setEditForm(f => ({ ...f, icon: ic }))}
                  className={cn(
                    'aspect-square rounded-lg grid place-items-center text-xl transition',
                    editForm.icon === ic ? 'bg-brand-100 ring-2 ring-brand-500' : 'hover:bg-slate-50'
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
                    'w-8 h-8 rounded-full transition relative',
                    editForm.color === c && 'ring-2 ring-offset-2 ring-slate-400'
                  )}
                  style={{ background: c }}
                >
                  {editForm.color === c && (
                    <Check size={14} className="absolute inset-0 m-auto text-white" />
                  )}
                </button>
              ))}
            </div>
          </div>
          {!editParentId && (
            <div>
              <label className="label">父级分类</label>
              <select
                className="input"
                value={editForm.parent_id}
                onChange={e => setEditForm(f => ({ ...f, parent_id: Number(e.target.value) }))}
              >
                <option value={0}>无（作为一级分类）</option>
                {(tab === 'expense' ? expRaw : incRaw)
                  .filter(c => !c.parent_id)
                  .filter(c => !editItem || c.id !== editItem.id)
                  .map(c => (
                    <option key={c.id} value={c.id}>{c.icon} {c.name}</option>
                  ))}
              </select>
            </div>
          )}
          <label className="flex items-center justify-between p-3 bg-slate-50 rounded-lg">
            <span className="text-sm text-slate-700">选择该分类时必须填写标签</span>
            <input
              type="checkbox" className="w-4 h-4 accent-brand-600"
              checked={editForm.need_tag}
              onChange={e => setEditForm(f => ({ ...f, need_tag: e.target.checked }))}
            />
          </label>
        </div>
      </Modal>

      <ConfirmDialog
        open={!!delTarget}
        onClose={() => setDelTarget(null)}
        onConfirm={doDelete}
        title="删除分类"
        desc={
          delTarget?.parent_id
            ? `确定删除子分类「${delTarget.name}」吗？如有已使用该分类的账单，将变为「未分类」。`
            : `确定删除分类「${delTarget?.name}」吗？其下子分类也会被删除，且已使用该分类的账单将变为「未分类」。`
        }
        okText="删除"
        danger
      />
    </div>
  );
}

// 避免 formatMoney 未使用警告（保留以便扩展总金额列）
void formatMoney;
