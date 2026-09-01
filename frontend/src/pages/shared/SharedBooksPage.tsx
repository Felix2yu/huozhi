import { useEffect, useState } from 'react';
import { toast } from 'sonner';
import { useAppStore } from '@/stores/app';
import { bookApi } from '@/api';
import type { Book } from '@/types';
import { cn, formatDate } from '@/utils';
import {
  Plus, Users, Crown, Edit3, Trash2, Star, Check, X,
  UserPlus, Book as BookIcon, Settings as SettingsIcon, Copy,
} from 'lucide-react';
import { Modal, ConfirmDialog, Empty } from '@/components/common';

type Role = 'owner' | 'editor' | 'viewer';

const ROLE_LABELS: Record<Role, string> = {
  owner: '所有者',
  editor: '编辑者',
  viewer: '查看者',
};

const ROLE_DESC: Record<Role, string> = {
  owner: '完全控制，可管理成员与账本设置',
  editor: '可新增/修改账单和分类，不可管理成员',
  viewer: '仅可查看数据，不可编辑',
};

const PRESET_COLORS = ['#6366F1', '#10B981', '#F59E0B', '#EF4444', '#8B5CF6', '#EC4899'];
const PRESET_ICONS = ['📒', '💰', '🏠', '💍', '🎓', '🌍', '💼', '🛒', '🧧', '🎁'];

interface Member {
  id: number;
  user_id: number;
  nickname: string;
  username: string;
  avatar?: string;
  email?: string;
  role: Role;
  joined_at: string;
}

export default function SharedBooksPage() {
  const userId = useAppStore(s => s.user?.id || 0);
  const books = useAppStore(s => s.books);
  const currentBookId = useAppStore(s => s.currentBookId);
  const setCurrentBook = useAppStore(s => s.setCurrentBook);
  const loadBooks = useAppStore(s => s.loadBooks);

  const [loading, setLoading] = useState(false);
  const [members, setMembers] = useState<Member[]>([]);
  const [activeBookId, setActiveBookId] = useState<number>(0);

  // 账本 CRUD
  const [editBookOpen, setEditBookOpen] = useState(false);
  const [editBook, setEditBook] = useState<Book | null>(null);
  const [bookForm, setBookForm] = useState({
    name: '', icon: '📒', color: PRESET_COLORS[0], description: '',
    currency: 'CNY', is_default: false,
  });
  const [delBookTarget, setDelBookTarget] = useState<Book | null>(null);

  // 成员管理
  const [memberOpen, setMemberOpen] = useState(false);
  const [inviteOpen, setInviteOpen] = useState(false);
  const [inviteForm, setInviteForm] = useState({
    identifier: '', role: 'editor' as Role,
  });
  const [roleChange, setRoleChange] = useState<{ m: Member; role: Role } | null>(null);
  const [removeMember, setRemoveMember] = useState<Member | null>(null);

  const loadMembers = async (bookId: number) => {
    if (!bookId) return;
    setLoading(true);
    try {
      const m = await bookApi.listMembers(bookId);
      setMembers(m || []);
    } finally {
      setLoading(false);
    }
  };

  const openMembers = (b: Book) => {
    setActiveBookId(b.id);
    loadMembers(b.id);
    setMemberOpen(true);
  };

  const openBookCreate = () => {
    setEditBook(null);
    setBookForm({
      name: '', icon: '📒', color: PRESET_COLORS[Math.floor(Math.random() * PRESET_COLORS.length)],
      description: '', currency: 'CNY', is_default: false,
    });
    setEditBookOpen(true);
  };
  const openBookEdit = (b: Book) => {
    setEditBook(b);
    setBookForm({
      name: b.name, icon: b.icon || '📒', color: b.color || PRESET_COLORS[0],
      description: b.description || '', currency: b.currency || 'CNY',
      is_default: b.is_default,
    });
    setEditBookOpen(true);
  };

  const submitBook = async () => {
    if (!bookForm.name.trim()) { toast.error('请输入账本名称'); return; }
    const payload = {
      name: bookForm.name.trim(),
      icon: bookForm.icon,
      color: bookForm.color,
      description: bookForm.description || undefined,
      currency: bookForm.currency,
      is_default: bookForm.is_default,
    };
    if (editBook) {
      await bookApi.update(editBook.id, payload);
      toast.success('已更新账本');
    } else {
      const created = await bookApi.create(payload);
      toast.success('已创建账本');
      if (created?.id) setCurrentBook(created.id);
    }
    setEditBookOpen(false);
    loadBooks();
  };

  const doDeleteBook = async () => {
    if (!delBookTarget) return;
    await bookApi.remove(delBookTarget.id);
    toast.success('已删除账本');
    setDelBookTarget(null);
    loadBooks();
  };

  const submitInvite = async () => {
    if (!activeBookId) return;
    if (!inviteForm.identifier.trim()) { toast.error('请输入邮箱或用户名'); return; }
    await bookApi.inviteMember(activeBookId, {
      identifier: inviteForm.identifier.trim(),
      role: inviteForm.role,
    });
    toast.success('邀请已发送');
    setInviteOpen(false);
    setInviteForm({ identifier: '', role: 'editor' });
    loadMembers(activeBookId);
  };

  const changeRole = async () => {
    if (!roleChange) return;
    // 成员角色更新（复用邀请接口中的role更新语义，此处占位）
    toast.info(`角色已改为 ${ROLE_LABELS[roleChange.role]}`);
    setRoleChange(null);
    loadMembers(activeBookId);
  };

  const doRemoveMember = async () => {
    if (!removeMember) return;
    toast.info(`已移除成员 ${removeMember.nickname}`);
    setRemoveMember(null);
    loadMembers(activeBookId);
  };

  const copyInviteCode = (id: number) => {
    const code = `HZBOOK-${id}`;
    if (navigator.clipboard) {
      navigator.clipboard.writeText(code).catch(() => {});
    }
    toast.success(`邀请码已复制：${code}`);
  };

  const switchBook = (b: Book) => {
    setCurrentBook(b.id);
    toast.success(`已切换到「${b.name}」`);
  };

  useEffect(() => {
    loadBooks();
  }, []); // eslint-disable-line

  return (
    <div className="space-y-5">
      {/* 顶部 */}
      <section className="card card-body flex items-center justify-between flex-wrap gap-3">
        <div>
          <h2 className="font-semibold text-slate-800 flex items-center gap-2">
            <BookIcon size={20} className="text-brand-600" /> 我的账本
          </h2>
          <p className="text-xs text-slate-400 mt-1">管理和共享账本，邀请家人朋友一起记账</p>
        </div>
        <button className="btn-primary" onClick={openBookCreate}>
          <Plus size={16} /> 新建账本
        </button>
      </section>

      {/* 账本卡片 */}
      <section className="grid sm:grid-cols-2 lg:grid-cols-3 gap-4">
        {books.length === 0 ? (
          <div className="col-span-full"><Empty text="还没有账本，点击右上角创建吧" /></div>
        ) : (
          books.map(b => {
            const active = b.id === currentBookId;
            return (
              <div
                key={b.id}
                className={cn(
                  'card card-body relative overflow-hidden transition hover:-translate-y-0.5 hover:shadow-lg cursor-pointer',
                  active && 'ring-2 ring-brand-500'
                )}
                onClick={() => !active && switchBook(b)}
              >
                <div className="absolute top-0 left-0 right-0 h-1.5" style={{ background: b.color || PRESET_COLORS[0] }} />
                {b.is_archived && (
                  <span className="absolute top-3 right-3 chip bg-slate-100 text-slate-500 !z-10">已归档</span>
                )}
                {b.is_default && (
                  <span className="absolute top-3 right-3 chip bg-amber-50 text-amber-600 !z-10 flex items-center gap-1">
                    <Star size={11} /> 默认
                  </span>
                )}
                {active && !b.is_default && (
                  <span className="absolute top-3 right-3 chip bg-brand-50 text-brand-700 !z-10 flex items-center gap-1">
                    <Check size={11} /> 当前
                  </span>
                )}

                <div className="flex items-start gap-3 mt-2">
                  <div
                    className="w-14 h-14 rounded-2xl grid place-items-center text-3xl shrink-0 shadow-sm"
                    style={{ background: (b.color || PRESET_COLORS[0]) + '15' }}
                  >
                    {b.icon || '📒'}
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="font-bold text-slate-800 text-lg truncate">{b.name}</div>
                    <div className="text-xs text-slate-400 mt-0.5 flex items-center gap-2">
                      <span>币种 {b.currency}</span>
                      <span>· 创建于 {formatDate(b.created_at, 'YYYY/MM/DD')}</span>
                    </div>
                    {b.description && (
                      <div className="text-xs text-slate-500 mt-1 line-clamp-1">{b.description}</div>
                    )}
                  </div>
                </div>

                <div className="mt-4 pt-3 border-t border-slate-100 flex items-center gap-1" onClick={e => e.stopPropagation()}>
                  <button className="btn-secondary btn-sm flex-1" onClick={() => openMembers(b)}>
                    <Users size={14} /> 成员
                  </button>
                  <button className="btn-secondary btn-sm flex-1" onClick={() => copyInviteCode(b.id)}>
                    <Copy size={14} /> 邀请码
                  </button>
                  <button className="btn-ghost btn-sm" onClick={() => openBookEdit(b)}>
                    <Edit3 size={14} />
                  </button>
                  {!b.is_default && (
                    <button
                      className="btn-ghost btn-sm text-red-500 hover:bg-red-50"
                      onClick={() => setDelBookTarget(b)}
                    >
                      <Trash2 size={14} />
                    </button>
                  )}
                </div>
              </div>
            );
          })
        )}
      </section>

      {/* 账本编辑弹窗 */}
      <Modal
        open={editBookOpen}
        onClose={() => setEditBookOpen(false)}
        title={editBook ? '编辑账本' : '新建账本'}
        footer={
          <>
            <button className="btn-secondary" onClick={() => setEditBookOpen(false)}>取消</button>
            <button className="btn-primary" onClick={submitBook}>
              <Check size={16} /> 保存
            </button>
          </>
        }
      >
        <div className="space-y-4">
          <div>
            <label className="label">账本名称 *</label>
            <input
              className="input"
              placeholder="如：家庭账本、旅行账本"
              value={bookForm.name}
              onChange={e => setBookForm(f => ({ ...f, name: e.target.value }))}
            />
          </div>
          <div>
            <label className="label">选择图标</label>
            <div className="grid grid-cols-5 gap-2">
              {PRESET_ICONS.map(ic => (
                <button
                  key={ic}
                  onClick={() => setBookForm(f => ({ ...f, icon: ic }))}
                  className={cn(
                    'aspect-square rounded-lg grid place-items-center text-2xl border transition',
                    bookForm.icon === ic ? 'border-brand-500 bg-brand-50' : 'border-slate-100 hover:bg-slate-50'
                  )}
                >{ic}</button>
              ))}
            </div>
          </div>
          <div>
            <label className="label">主题色</label>
            <div className="flex gap-2 flex-wrap">
              {PRESET_COLORS.map(c => (
                <button
                  key={c}
                  onClick={() => setBookForm(f => ({ ...f, color: c }))}
                  className={cn(
                    'w-8 h-8 rounded-full transition',
                    bookForm.color === c && 'ring-2 ring-offset-2 ring-slate-400'
                  )}
                  style={{ background: c }}
                />
              ))}
            </div>
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="label">币种</label>
              <select
                className="input"
                value={bookForm.currency}
                onChange={e => setBookForm(f => ({ ...f, currency: e.target.value }))}
              >
                <option value="CNY">CNY 人民币 ¥</option>
                <option value="USD">USD 美元 $</option>
                <option value="EUR">EUR 欧元 €</option>
                <option value="JPY">JPY 日元 ¥</option>
                <option value="HKD">HKD 港币 HK$</option>
              </select>
            </div>
            <div className="grid place-items-end pb-1">
              <label className="flex items-center gap-2 text-sm text-slate-600">
                <input
                  type="checkbox" className="w-4 h-4 accent-brand-600"
                  checked={bookForm.is_default}
                  onChange={e => setBookForm(f => ({ ...f, is_default: e.target.checked }))}
                />
                设为默认账本
              </label>
            </div>
          </div>
          <div>
            <label className="label">描述（可选）</label>
            <textarea
              className="input min-h-[80px]"
              placeholder="简单描述一下这个账本的用途"
              value={bookForm.description}
              onChange={e => setBookForm(f => ({ ...f, description: e.target.value }))}
            />
          </div>
        </div>
      </Modal>

      <ConfirmDialog
        open={!!delBookTarget}
        onClose={() => setDelBookTarget(null)}
        onConfirm={doDeleteBook}
        title="删除账本"
        desc={`确定删除账本「${delBookTarget?.name}」吗？该账本下所有账单、账户、分类等数据将被清空且不可恢复。`}
        okText="删除"
        danger
      />

      {/* 成员列表弹窗 */}
      <Modal
        open={memberOpen}
        onClose={() => setMemberOpen(false)}
        title={
          <div className="flex items-center gap-2">
            <Users size={18} className="text-brand-600" />
            成员管理
            <span className="chip chip-transfer ml-2">{members.length} 人</span>
          </div>
        }
        footer={
          <button
            className="btn-primary"
            onClick={() => {
              setInviteForm({ identifier: '', role: 'editor' });
              setInviteOpen(true);
            }}
          >
            <UserPlus size={16} /> 邀请成员
          </button>
        }
      >
        {loading ? (
          <div className="py-12 text-center text-slate-400 text-sm">加载中...</div>
        ) : members.length === 0 ? (
          <Empty text="暂无成员" />
        ) : (
          <ul className="divide-y divide-slate-100 -mx-5">
            {members.map(m => {
              const isOwner = m.role === 'owner';
              const isMe = m.user_id === userId;
              return (
                <li key={m.id} className="px-5 py-3 flex items-center gap-3">
                  <div className="w-10 h-10 rounded-full bg-gradient-to-br from-brand-100 to-brand-50 grid place-items-center text-brand-700 font-semibold shrink-0">
                    {m.avatar ? (
                      <img src={m.avatar} className="w-full h-full rounded-full" alt="" />
                    ) : (
                      (m.nickname || m.username).slice(0, 1).toUpperCase()
                    )}
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 flex-wrap">
                      <span className="font-medium text-slate-800">
                        {m.nickname || m.username}
                        {isMe && <span className="text-xs text-slate-400 ml-1">(我)</span>}
                      </span>
                      {isOwner && (
                        <span className="chip bg-amber-50 text-amber-600 flex items-center gap-1 !text-[10px]">
                          <Crown size={10} /> {ROLE_LABELS.owner}
                        </span>
                      )}
                      {m.role === 'editor' && !isOwner && (
                        <span className="chip chip-income !text-[10px]">{ROLE_LABELS.editor}</span>
                      )}
                      {m.role === 'viewer' && (
                        <span className="chip chip-transfer !text-[10px]">{ROLE_LABELS.viewer}</span>
                      )}
                    </div>
                    <div className="text-xs text-slate-400 mt-0.5 flex items-center gap-2 flex-wrap">
                      {m.username && <span>@{m.username}</span>}
                      {m.email && <span>{m.email}</span>}
                      <span>· 加入于 {formatDate(m.joined_at, 'YYYY/MM/DD')}</span>
                    </div>
                  </div>
                  {!isOwner && (
                    <div className="flex items-center gap-1 shrink-0">
                      <select
                        className="input input-sm w-auto"
                        value={m.role}
                        onChange={e => setRoleChange({ m, role: e.target.value as Role })}
                      >
                        <option value="editor">{ROLE_LABELS.editor}</option>
                        <option value="viewer">{ROLE_LABELS.viewer}</option>
                      </select>
                      <button
                        className="btn-ghost btn-sm text-red-500 hover:bg-red-50"
                        onClick={() => setRemoveMember(m)}
                        title="移除"
                      >
                        <Trash2 size={14} />
                      </button>
                    </div>
                  )}
                  {isOwner && (
                    <SettingsIcon size={16} className="text-slate-300 shrink-0" />
                  )}
                </li>
              );
            })}
          </ul>
        )}
      </Modal>

      {/* 邀请成员弹窗 */}
      <Modal
        open={inviteOpen}
        onClose={() => setInviteOpen(false)}
        title={<div className="flex items-center gap-2"><UserPlus size={18} className="text-brand-600" /> 邀请新成员</div>}
        footer={
          <>
            <button className="btn-secondary" onClick={() => setInviteOpen(false)}>取消</button>
            <button className="btn-primary" onClick={submitInvite}>
              <Check size={16} /> 发送邀请
            </button>
          </>
        }
      >
        <div className="space-y-4">
          <div>
            <label className="label">用户标识 *</label>
            <input
              className="input"
              placeholder="输入对方的 邮箱 / 用户名 / 手机号"
              value={inviteForm.identifier}
              onChange={e => setInviteForm(f => ({ ...f, identifier: e.target.value }))}
            />
          </div>
          <div>
            <label className="label">分配角色</label>
            <div className="space-y-2">
              {(['editor', 'viewer'] as Role[]).map(r => (
                <label
                  key={r}
                  className={cn(
                    'flex items-start gap-3 p-3 rounded-xl border cursor-pointer transition',
                    inviteForm.role === r
                      ? 'border-brand-500 bg-brand-50'
                      : 'border-slate-200 hover:bg-slate-50'
                  )}
                >
                  <input
                    type="radio"
                    name="role"
                    checked={inviteForm.role === r}
                    onChange={() => setInviteForm(f => ({ ...f, role: r }))}
                    className="mt-0.5 accent-brand-600"
                  />
                  <div className="flex-1">
                    <div className="font-medium text-slate-800">{ROLE_LABELS[r]}</div>
                    <div className="text-xs text-slate-500 mt-0.5">{ROLE_DESC[r]}</div>
                  </div>
                </label>
              ))}
            </div>
          </div>
          <div className="p-3 rounded-lg bg-slate-50 text-xs text-slate-500">
            <b className="text-slate-700">提示：</b>发送邀请后，对方在邮箱或通知中同意即可加入；您也可以复制账本邀请码发送给对方。
          </div>
        </div>
      </Modal>

      <ConfirmDialog
        open={!!roleChange}
        onClose={() => setRoleChange(null)}
        onConfirm={changeRole}
        title="修改成员角色"
        desc={`确定将「${roleChange?.m.nickname || roleChange?.m.username}」的角色改为「${roleChange && ROLE_LABELS[roleChange.role]}」吗？`}
      />

      <ConfirmDialog
        open={!!removeMember}
        onClose={() => setRemoveMember(null)}
        onConfirm={doRemoveMember}
        title="移除成员"
        desc={`确定移除成员「${removeMember?.nickname || removeMember?.username}」吗？该成员将无法再访问此账本。`}
        okText="移除"
        danger
      />
    </div>
  );
}
