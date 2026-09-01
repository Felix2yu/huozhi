import { useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { toast } from 'sonner';
import { useAppStore } from '@/stores/app';
import { tagApi } from '@/api';
import type { Tag } from '@/types';
import { cn } from '@/utils';
import {
  Plus, Edit3, Trash2, Hash, Search, Check, X,
  ArrowUpRight, Flame, Star,
} from 'lucide-react';
import { Modal, ConfirmDialog, Empty } from '@/components/common';

const PRESET_COLORS = [
  '#6366F1', '#10B981', '#F59E0B', '#EF4444', '#8B5CF6',
  '#EC4899', '#06B6D4', '#F97316', '#84CC16', '#14B8A6',
  '#22C55E', '#A855F7', '#0EA5E9', '#F43F5E', '#64748b',
  '#0891B2', '#DC2626', '#EA580C', '#65A30D', '#4F46E5',
];

export default function TagsPage() {
  const navigate = useNavigate();
  const bookId = useAppStore(s => s.currentBookId);
  const storeTags = useAppStore(s => s.tags);
  const loadDicts = useAppStore(s => s.loadDictionaries);

  const [list, setList] = useState<Tag[]>([]);
  const [loading, setLoading] = useState(false);
  const [keyword, setKeyword] = useState('');

  const [editOpen, setEditOpen] = useState(false);
  const [editItem, setEditItem] = useState<Tag | null>(null);
  const [editForm, setEditForm] = useState({
    name: '', color: PRESET_COLORS[0], sort: 0,
  });
  const [delTarget, setDelTarget] = useState<Tag | null>(null);
  const [sortMode, setSortMode] = useState<'count' | 'name' | 'sort'>('count');

  const load = async () => {
    if (!bookId) return;
    setLoading(true);
    try {
      const res = await tagApi.list();
      setList(res || []);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, [bookId]); // eslint-disable-line

  const filtered = useMemo(() => {
    let arr = list;
    if (keyword.trim()) {
      const k = keyword.trim().toLowerCase();
      arr = arr.filter(t => t.name.toLowerCase().includes(k));
    }
    arr = [...arr].sort((a, b) => {
      if (sortMode === 'count') return b.count - a.count;
      if (sortMode === 'name') return a.name.localeCompare(b.name);
      return a.sort - b.sort;
    });
    return arr;
  }, [list, keyword, sortMode]);

  const totalUsed = useMemo(() => list.reduce((s, t) => s + t.count, 0), [list]);
  const maxCount = useMemo(() => Math.max(1, ...list.map(t => t.count)), [list]);

  const openCreate = () => {
    setEditItem(null);
    setEditForm({
      name: '',
      color: PRESET_COLORS[Math.floor(Math.random() * PRESET_COLORS.length)],
      sort: 0,
    });
    setEditOpen(true);
  };
  const openEdit = (t: Tag) => {
    setEditItem(t);
    setEditForm({ name: t.name, color: t.color || PRESET_COLORS[0], sort: t.sort });
    setEditOpen(true);
  };

  const submitEdit = async () => {
    if (!editForm.name.trim()) { toast.error('请输入标签名称'); return; }
    const payload = {
      book_id: bookId,
      name: editForm.name.trim().replace(/^#/, ''),
      color: editForm.color,
      sort: editForm.sort,
    };
    if (editItem) {
      await tagApi.update(editItem.id, payload);
      toast.success('已更新标签');
    } else {
      await tagApi.create(payload);
      toast.success('已创建标签');
    }
    setEditOpen(false);
    loadDicts(bookId);
    load();
  };

  const doDelete = async () => {
    if (!delTarget) return;
    await tagApi.remove(delTarget.id);
    toast.success('已删除标签');
    setDelTarget(null);
    loadDicts(bookId);
    load();
  };

  const goFilteredTransactions = (t: Tag) => {
    navigate('/transactions?tag_id=' + t.id);
  };

  return (
    <div className="space-y-5">
      {/* 概览 */}
      <section className="rounded-2xl bg-gradient-to-br from-violet-600 via-purple-600 to-fuchsia-600 p-6 text-white shadow-soft relative overflow-hidden">
        <div className="absolute -right-16 -top-16 w-64 h-64 bg-white/10 rounded-full blur-2xl" />
        <div className="relative flex items-center justify-between flex-wrap gap-4">
          <div>
            <div className="flex items-center gap-2 text-white/70 text-sm">
              <Hash size={16} /> 标签管理
            </div>
            <div className="text-4xl font-bold tabular-nums mt-2 tracking-tight">
              {list.length}
              <span className="text-xl font-medium text-white/60 ml-2">个标签</span>
            </div>
            <div className="text-sm text-white/70 mt-2 flex items-center gap-4">
              <span className="flex items-center gap-1">
                <Flame size={14} /> 总计被使用 <b className="text-white">{totalUsed}</b> 次
              </span>
              <span className="flex items-center gap-1">
                <Star size={14} /> Top：
                <b className="text-white">
                  #
                  {list.length > 0 ? [...list].sort((a, b) => b.count - a.count)[0].name : '-'}
                </b>
              </span>
            </div>
          </div>
          <button className="btn bg-white text-purple-700 hover:bg-white/90 shadow-lg" onClick={openCreate}>
            <Plus size={16} /> 新建标签
          </button>
        </div>
      </section>

      {/* 搜索 + 排序 */}
      <section className="card card-body">
        <div className="flex items-center gap-2 flex-wrap">
          <div className="flex-1 min-w-[180px] relative">
            <Search size={16} className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400" />
            <input
              className="input pl-9"
              placeholder="搜索标签名称..."
              value={keyword}
              onChange={e => setKeyword(e.target.value)}
            />
          </div>
          <div className="flex gap-1 p-0.5 bg-slate-100 rounded-lg">
            <button
              onClick={() => setSortMode('count')}
              className={cn(
                'px-3 py-1.5 rounded-md text-xs font-medium transition flex items-center gap-1',
                sortMode === 'count' ? 'bg-white text-slate-700 shadow-sm' : 'text-slate-500'
              )}
            ><Flame size={13} /> 使用次数</button>
            <button
              onClick={() => setSortMode('name')}
              className={cn(
                'px-3 py-1.5 rounded-md text-xs font-medium transition',
                sortMode === 'name' ? 'bg-white text-slate-700 shadow-sm' : 'text-slate-500'
              )}
            >名称</button>
            <button
              onClick={() => setSortMode('sort')}
              className={cn(
                'px-3 py-1.5 rounded-md text-xs font-medium transition',
                sortMode === 'sort' ? 'bg-white text-slate-700 shadow-sm' : 'text-slate-500'
              )}
            >自定义</button>
          </div>
        </div>
      </section>

      {/* 标签云 */}
      <section className="card card-body">
        <h3 className="font-semibold text-slate-800 mb-4 flex items-center gap-2">
          <Hash size={18} className="text-violet-600" /> 标签云
        </h3>
        {loading ? (
          <div className="py-16 text-center text-slate-400 text-sm">加载中...</div>
        ) : filtered.length === 0 ? (
          <Empty text="还没有标签，点击右上角创建吧" icon={<Hash size={32} />} />
        ) : (
          <div className="flex flex-wrap gap-3">
            {filtered.map(t => {
              const scale = 0.85 + (t.count / maxCount) * 0.8;
              return (
                <div
                  key={t.id}
                  className="group relative"
                  style={{ transform: `scale(${scale})`, transformOrigin: 'left center' }}
                >
                  <button
                    onClick={() => goFilteredTransactions(t)}
                    className="inline-flex items-center gap-1.5 px-4 py-2 rounded-full font-medium transition hover:-translate-y-0.5 hover:shadow-md"
                    style={{
                      background: (t.color || '#6366F1') + '15',
                      color: t.color || '#6366F1',
                      border: `1px solid ${(t.color || '#6366F1')}30`,
                    }}
                    title="点击查看使用该标签的账单"
                  >
                    <Hash size={14} />
                    <span>{t.name}</span>
                    <span
                      className="text-xs px-1.5 py-0.5 rounded-full"
                      style={{ background: (t.color || '#6366F1') + '25' }}
                    >
                      {t.count}
                    </span>
                  </button>
                  <div className="absolute -top-1 -right-1 flex gap-0.5 opacity-0 group-hover:opacity-100 transition">
                    <button
                      onClick={(e) => { e.stopPropagation(); openEdit(t); }}
                      className="w-6 h-6 rounded-full bg-white shadow grid place-items-center text-slate-500 hover:text-brand-600 border border-slate-200"
                      title="编辑"
                    >
                      <Edit3 size={11} />
                    </button>
                    <button
                      onClick={(e) => { e.stopPropagation(); setDelTarget(t); }}
                      className="w-6 h-6 rounded-full bg-white shadow grid place-items-center text-slate-500 hover:text-red-500 border border-slate-200"
                      title="删除"
                    >
                      <Trash2 size={11} />
                    </button>
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </section>

      {/* 标签卡片列表（详细信息） */}
      <section className="card card-body">
        <div className="flex items-center justify-between mb-4 flex-wrap gap-2">
          <h3 className="font-semibold text-slate-800">标签详情</h3>
          <span className="text-xs text-slate-400">共 {filtered.length} 个标签</span>
        </div>
        {filtered.length === 0 ? (
          <Empty text="暂无数据" />
        ) : (
          <div className="grid sm:grid-cols-2 lg:grid-cols-3 gap-3">
            {filtered.map(t => (
              <div
                key={t.id}
                className="p-4 rounded-xl border border-slate-100 hover:shadow-md hover:border-slate-200 transition group"
              >
                <div className="flex items-start justify-between">
                  <div className="flex items-center gap-2">
                    <span
                      className="inline-flex items-center gap-1 px-3 py-1 rounded-full font-medium"
                      style={{
                        background: (t.color || '#6366F1') + '15',
                        color: t.color || '#6366F1',
                      }}
                    >
                      <Hash size={14} /> {t.name}
                    </span>
                  </div>
                  <div className="flex gap-1 opacity-0 group-hover:opacity-100 transition">
                    <button className="btn-ghost btn-sm" onClick={() => openEdit(t)}>
                      <Edit3 size={14} />
                    </button>
                    <button
                      className="btn-ghost btn-sm text-red-500 hover:bg-red-50"
                      onClick={() => setDelTarget(t)}
                    >
                      <Trash2 size={14} />
                    </button>
                  </div>
                </div>
                <div className="mt-3 flex items-end justify-between">
                  <div>
                    <div className="text-xs text-slate-400">使用次数</div>
                    <div className="text-2xl font-bold tabular-nums text-slate-800">
                      {t.count}
                      <span className="text-sm font-normal text-slate-400 ml-1">次</span>
                    </div>
                  </div>
                  <button
                    onClick={() => goFilteredTransactions(t)}
                    className="text-xs text-brand-600 hover:underline flex items-center gap-0.5"
                  >
                    相关账单 <ArrowUpRight size={12} />
                  </button>
                </div>
                <div className="mt-3 w-full h-1.5 bg-slate-100 rounded-full overflow-hidden">
                  <div
                    className="h-full rounded-full transition-all"
                    style={{
                      width: `${(t.count / maxCount) * 100}%`,
                      background: t.color || '#6366F1',
                    }}
                  />
                </div>
              </div>
            ))}
          </div>
        )}
      </section>

      {/* 新增/编辑标签弹窗 */}
      <Modal
        open={editOpen}
        onClose={() => setEditOpen(false)}
        title={editItem ? '编辑标签' : '新建标签'}
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
            <label className="label">标签名称 *</label>
            <div className="relative">
              <Hash size={16} className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400" />
              <input
                className="input pl-9"
                placeholder="如：出差、生日、购物..."
                value={editForm.name}
                onChange={e => setEditForm(f => ({ ...f, name: e.target.value.replace(/^#/, '') }))}
              />
            </div>
          </div>
          <div>
            <label className="label">选择颜色</label>
            <div className="grid grid-cols-5 gap-2">
              {PRESET_COLORS.map(c => (
                <button
                  key={c}
                  onClick={() => setEditForm(f => ({ ...f, color: c }))}
                  className={cn(
                    'aspect-square rounded-lg grid place-items-center transition',
                    editForm.color === c && 'ring-2 ring-offset-2 ring-slate-400'
                  )}
                  style={{ background: c }}
                >
                  {editForm.color === c && <Check size={18} className="text-white" />}
                </button>
              ))}
            </div>
            <div className="mt-3 p-3 rounded-lg flex items-center gap-2"
              style={{ background: (editForm.color) + '15' }}
            >
              <span
                className="inline-flex items-center gap-1 px-3 py-1 rounded-full font-medium"
                style={{ background: (editForm.color) + '25', color: editForm.color }}
              >
                <Hash size={14} />
                {editForm.name || '预览标签名'}
              </span>
              <span className="text-xs text-slate-400 ml-2">预览效果</span>
            </div>
          </div>
          <div>
            <label className="label">排序号（越小越靠前）</label>
            <input
              type="number" className="input"
              value={editForm.sort}
              onChange={e => setEditForm(f => ({ ...f, sort: Number(e.target.value) }))}
            />
          </div>
          {editItem && (
            <div className="text-xs text-slate-400 p-3 bg-slate-50 rounded-lg">
              该标签已被使用 <b className="text-slate-600">{editItem.count}</b> 次
            </div>
          )}
        </div>
      </Modal>

      <ConfirmDialog
        open={!!delTarget}
        onClose={() => setDelTarget(null)}
        onConfirm={doDelete}
        title="删除标签"
        desc={`确定删除标签「#${delTarget?.name}」吗？已使用该标签的账单记录不会被删除，仅解绑此标签关系。`}
        okText="删除"
        danger
      />
    </div>
  );
}
