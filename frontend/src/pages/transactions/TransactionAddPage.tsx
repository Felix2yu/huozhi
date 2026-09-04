import { useEffect, useMemo, useRef, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { toast } from 'sonner';
import { useAppStore } from '@/stores/app';
import { txApi, aiApi, uploadApi } from '@/api';
import type { Transaction, TransactionType, Tag as TagType } from '@/types';
import { formatMoney, formatDate, cn } from '@/utils';
import {
  ArrowLeft, ArrowRightLeft, Minus, Plus, Calendar as CalendarIcon,
  Tag, ImagePlus, Save, Repeat1, ChevronDown, Upload, X, Image,
  Sparkles, Wand2,
} from 'lucide-react';
import { Drawer, TagChip } from '@/components/common';

type TabType = 'expense' | 'income' | 'transfer';

const DEFAULT_FORM = {
  type: 'expense' as TransactionType,
  amount: '',
  category_id: 0,
  account_id: 0,
  to_account_id: 0,
  transfer_fee: '',
  tx_date: formatDate(new Date(), 'YYYY-MM-DD'),
  tx_time: formatDate(new Date(), 'HH:mm'),
  description: '',
  tag_ids: [] as number[],
  images: [] as string[],
  merchant: '',
  remark: '',
  include_in_balance: true,
  include_in_budget: true,
  reimburse_status: 'none' as 'none' | 'pending' | 'done',
};

export default function TransactionAddPage() {
  const navigate = useNavigate();
  const [sp] = useSearchParams();
  const editId = Number(sp.get('id') || 0);

  const bookId = useAppStore(s => s.currentBookId);
  const accounts = useAppStore(s => s.accounts);
  const tags = useAppStore(s => s.tags);
  const { expense: expCats, income: incCats } = useAppStore(s => s.categories);

  const [form, setForm] = useState({ ...DEFAULT_FORM });
  const [loading, setLoading] = useState(false);
  const [submitting, setSubmitting] = useState(false);

  // 图片上传
  const [uploading, setUploading] = useState(false);
  const [preview, setPreview] = useState<string | null>(null);
  const fileRef = useRef<HTMLInputElement>(null);

  const [tagDrawerOpen, setTagDrawerOpen] = useState(false);
  const [catDrawerOpen, setCatDrawerOpen] = useState(false);
  const [accDrawerOpen, setAccDrawerOpen] = useState(false);
  const [toAccDrawerOpen, setToAccDrawerOpen] = useState(false);

  // AI 相关状态
  const [aiInput, setAiInput] = useState('');
  const [aiLoading, setAiLoading] = useState(false);
  const [aiClassifying, setAiClassifying] = useState(false);

  // AI 智能记账：自然语言 → 自动填充表单
  const runAiSmartRecord = async () => {
    if (!aiInput.trim()) { toast.error('说点什么吧~'); return; }
    setAiLoading(true);
    try {
      const res = await aiApi.smartRecord({ text: aiInput, book_id: bookId });
      const f: typeof DEFAULT_FORM = { ...DEFAULT_FORM };
      if (res.type === 'income' || res.type === 'expense' || res.type === 'transfer') {
        f.type = res.type as TransactionType;
      }
      f.amount = String(res.amount || '');
      f.description = res.description || aiInput;
      f.category_id = res.category_id || 0;
      if (res.account_id) f.account_id = res.account_id;
      if (res.tx_date) {
        f.tx_date = res.tx_date;
      }
      if (res.tags?.length) {
        // 名字 → id（后续可扩展：前端维护 tag 映射）
      }
      setForm(f);
      setAiInput('');
      toast.success('AI 已帮你填好，确认后保存即可 ✨');
    } catch (e: any) {
      toast.error(e?.message?.includes('503') ? 'AI 未启用，请联系管理员配置 API Key' : 'AI 暂时没猜出来 😅');
    } finally {
      setAiLoading(false);
    }
  };

  // AI 自动分类
  const runAiClassify = async () => {
    if (!form.description.trim()) { toast.info('先填一下描述'); return; }
    setAiClassifying(true);
    try {
      const res = await aiApi.classify({
        description: form.description,
        amount: parseFloat(form.amount) || 0,
        type: tab === 'transfer' ? undefined : tab,
        book_id: bookId,
      });
      if (res.category_id) {
        setForm(prev => ({ ...prev, category_id: res.category_id }));
        toast.success(`AI 推荐：${res.category}${res.confidence ? ` (置信度 ${Math.round(res.confidence * 100)}%)` : ''}`);
      }
    } catch (e: any) {
      toast.error(e?.message?.includes('503') ? 'AI 未启用' : '分类失败');
    } finally {
      setAiClassifying(false);
    }
  };

  const tab = useMemo<TabType>(() => {
    if (form.type === 'income') return 'income';
    if (form.type === 'transfer') return 'transfer';
    return 'expense';
  }, [form.type]);

  const currentCats = tab === 'income' ? incCats : expCats;
  const selectedCat = currentCats.find(c => c.id === form.category_id);
  const selectedAcc = accounts.find(a => a.id === form.account_id);
  const selectedToAcc = accounts.find(a => a.id === form.to_account_id);

  // 编辑模式加载
  useEffect(() => {
    if (!editId) return;
    setLoading(true);
    txApi.get(editId)
      .then((t: Transaction) => {
        const d = new Date(t.tx_date);
        setForm({
          type: t.type,
          amount: String(t.amount),
          category_id: t.category_id || 0,
          account_id: t.account_id || 0,
          to_account_id: t.to_account_id || 0,
          transfer_fee: t.transfer_fee ? String(t.transfer_fee) : '',
          tx_date: formatDate(d, 'YYYY-MM-DD'),
          tx_time: formatDate(d, 'HH:mm'),
          description: t.description || '',
          tag_ids: (t.tags || []).map(x => x.id),
          images: t.images || [],
          merchant: t.merchant || '',
          remark: t.remark || '',
          include_in_balance: t.include_in_balance,
          include_in_budget: t.include_in_budget,
          reimburse_status: t.reimburse_status || 'none',
        });
      })
      .finally(() => setLoading(false));
  }, [editId]);

  const switchTab = (t: TabType) => {
    setForm(f => ({
      ...f,
      type: t === 'transfer' ? 'transfer' : (t === 'income' ? 'income' : 'expense'),
      category_id: 0,
    }));
  };

  const setField = <K extends keyof typeof form>(k: K, v: (typeof form)[K]) => {
    setForm(f => ({ ...f, [k]: v }));
  };

  const toggleTag = (id: number) => {
    setForm(f => {
      const set = new Set(f.tag_ids);
      if (set.has(id)) set.delete(id); else set.add(id);
      return { ...f, tag_ids: [...set] };
    });
  };

  // 上传选中的图片到服务器（本地或 S3），返回路径后追加到 form.images
  const handleFiles = async (files: FileList | null) => {
    if (!files || files.length === 0) return;
    if (form.images.length >= 9) { toast.error('最多上传 9 张'); return; }
    setUploading(true);
    try {
      const urls: string[] = [];
      for (const f of Array.from(files)) {
        const res = await uploadApi.image(f);
        urls.push(res.url);
      }
      setForm(prev => ({ ...prev, images: [...prev.images, ...urls].slice(0, 9) }));
    } catch (e: any) {
      toast.error('上传失败：' + (e?.message || '请稍后重试'));
    } finally {
      setUploading(false);
      if (fileRef.current) fileRef.current.value = '';
    }
  };

  const submit = async (saveAndContinue = false) => {
    if (!bookId) { toast.error('请先选择账本'); return; }
    if (!form.amount || parseFloat(form.amount) <= 0) { toast.error('请输入有效金额'); return; }
    if (tab !== 'transfer' && !form.category_id) { toast.error('请选择分类'); return; }
    if (!form.account_id) { toast.error('请选择账户'); return; }
    if (tab === 'transfer' && !form.to_account_id) { toast.error('请选择转入账户'); return; }
    if (tab === 'transfer' && form.to_account_id === form.account_id) {
      toast.error('转入账户不能与转出账户相同'); return;
    }

    const payload = {
      book_id: bookId,
      type: form.type,
      amount: parseFloat(form.amount),
      category_id: form.category_id,
      account_id: form.account_id,
      to_account_id: tab === 'transfer' ? form.to_account_id : undefined,
      transfer_fee: form.transfer_fee ? parseFloat(form.transfer_fee) : undefined,
      tx_date: `${form.tx_date} ${form.tx_time}:00`,
      description: form.description,
      tag_ids: form.tag_ids,
      images: form.images,
      merchant: form.merchant,
      remark: form.remark,
      include_in_balance: form.include_in_balance,
      include_in_budget: form.include_in_budget,
      reimburse_status: form.reimburse_status,
    };

    setSubmitting(true);
    try {
      if (editId) {
        await txApi.update(editId, payload);
        toast.success('已更新');
        navigate('/transactions');
      } else {
        await txApi.create(payload);
        toast.success('已保存');
        if (saveAndContinue) {
          setForm({ ...DEFAULT_FORM, type: form.type, account_id: form.account_id });
        } else {
          navigate('/transactions');
        }
      }
    } finally {
      setSubmitting(false);
    }
  };

  if (loading) {
    return <div className="py-32 text-center text-slate-400 text-sm">加载中...</div>;
  }

  return (
    <div className="space-y-5 pb-28">
      {/* 顶部操作栏 */}
      <div className="flex items-center justify-between">
        <button className="btn-ghost btn-sm" onClick={() => navigate(-1)}>
          <ArrowLeft size={18} /> 返回
        </button>
        <h1 className="font-semibold text-slate-800">{editId ? '编辑账单' : '记一笔'}</h1>
        <div className="w-16" />
      </div>

      {/* 类型切换 Tab */}
      <section className="card card-body p-1.5">
        <div className="grid grid-cols-3 gap-1.5">
          {[
            { k: 'expense', label: '支出', Icon: Minus, cls: 'text-red-500' },
            { k: 'income', label: '收入', Icon: Plus, cls: 'text-emerald-600' },
            { k: 'transfer', label: '转账', Icon: ArrowRightLeft, cls: 'text-indigo-600' },
          ].map(({ k, label, Icon, cls }) => (
            <button
              key={k}
              onClick={() => switchTab(k as TabType)}
              className={cn(
                'flex items-center justify-center gap-1.5 py-2.5 rounded-lg font-medium transition',
                tab === k
                  ? k === 'expense' ? 'bg-red-50 text-red-600'
                    : k === 'income' ? 'bg-emerald-50 text-emerald-600'
                    : 'bg-indigo-50 text-indigo-600'
                  : 'text-slate-500 hover:bg-slate-50'
              )}
            >
              <Icon size={16} className={tab === k ? cls : ''} />
              {label}
            </button>
          ))}
        </div>
      </section>

      {/* AI 智能记账 */}
      <section className="card card-body !bg-gradient-to-br from-violet-50 via-white to-indigo-50 border-violet-200">
        <div className="flex items-center gap-2 mb-2">
          <Sparkles size={16} className="text-violet-600" />
          <span className="text-sm font-medium text-violet-700">AI 智能记账</span>
          <span className="text-xs text-slate-400">说一句话就能自动填好</span>
        </div>
        <div className="flex gap-2">
          <input
            className="input !bg-white/80 flex-1"
            placeholder="例如：午饭35 / 昨天打车花了20 / 这个月工资8000"
            value={aiInput}
            onChange={e => setAiInput(e.target.value)}
            onKeyDown={e => { if (e.key === 'Enter') runAiSmartRecord(); }}
          />
          <button
            className="btn bg-violet-600 hover:bg-violet-700 text-white whitespace-nowrap"
            disabled={aiLoading}
            onClick={runAiSmartRecord}
          >
            {aiLoading ? (
              <span className="inline-block w-4 h-4 border-2 border-white/40 border-t-white rounded-full animate-spin" />
            ) : <><Wand2 size={14} className="mr-1 inline" />试试</>}
          </button>
        </div>
      </section>

      {/* 金额输入 */}
      <section className="card card-body">
        <label className="label">金额</label>
        <div className="flex items-end gap-3">
          <div className="text-4xl font-bold tabular-nums" style={{
            color: tab === 'income' ? '#10B981' : tab === 'transfer' ? '#6366F1' : '#EF4444'
          }}>¥</div>
          <input
            className="flex-1 !text-4xl !font-bold !tabular-nums !py-2 !border-0 !px-0 focus:!ring-0"
            placeholder="0.00"
            inputMode="decimal"
            value={form.amount}
            onChange={e => {
              const v = e.target.value.replace(/[^0-9.]/g, '');
              const parts = v.split('.');
              setField('amount', parts.length > 2 ? parts[0] + '.' + parts.slice(1).join('') : v);
            }}
          />
          <button
            className="btn-ghost btn-sm text-slate-400"
            onClick={() => setField('amount', String(Math.round(parseFloat(form.amount || '0') / 2 * 100) / 100))}
          >AA</button>
        </div>
        {form.amount && (
          <div className="text-xs text-slate-400 mt-1">
            大写: {formatMoney(form.amount)}
          </div>
        )}
      </section>

      {/* 分类选择 Grid */}
      {tab !== 'transfer' && (
        <section className="card card-body">
          <button
            className="flex items-center justify-between w-full mb-3"
            onClick={() => setCatDrawerOpen(true)}
          >
            <span className="label mb-0">分类</span>
            <span className="text-sm text-brand-600 flex items-center gap-1">
              {selectedCat ? (<span>{selectedCat.icon} {selectedCat.name}</span>) : '请选择'}
              <ChevronDown size={16} />
            </span>
          </button>
          <div className="grid grid-cols-5 md:grid-cols-8 gap-2 max-h-64 overflow-y-auto">
            {currentCats.filter(c => !c.parent_id || c.parent_id === 0).slice(0, 24).map(c => {
              const active = form.category_id === c.id;
              return (
                <button
                  key={c.id}
                  onClick={() => setField('category_id', c.id)}
                  className={cn(
                    'flex flex-col items-center gap-1 p-2 rounded-lg transition',
                    active ? 'bg-brand-50 ring-2 ring-brand-500' : 'hover:bg-slate-50'
                  )}
                >
                  <div
                    className="w-10 h-10 rounded-full grid place-items-center text-xl"
                    style={{ background: (c.color || '#64748b') + '15' }}
                  >
                    {c.icon || '📦'}
                  </div>
                  <span className="text-xs text-slate-600 truncate w-full text-center">{c.name}</span>
                </button>
              );
            })}
            <button
              className="flex flex-col items-center gap-1 p-2 rounded-lg hover:bg-slate-50 text-slate-400"
              onClick={() => setCatDrawerOpen(true)}
            >
              <div className="w-10 h-10 rounded-full bg-slate-100 grid place-items-center">
                <ChevronDown size={18} />
              </div>
              <span className="text-xs">更多</span>
            </button>
          </div>
        </section>
      )}

      {/* 账户选择 */}
      <section className="card card-body space-y-3">
        {tab === 'transfer' ? (
          <>
            <button className="flex items-center justify-between w-full" onClick={() => setAccDrawerOpen(true)}>
              <span className="label mb-0">转出账户</span>
              <span className="text-sm text-slate-700 flex items-center gap-1">
                {selectedAcc?.name || '请选择'}
                <span className="text-slate-400">{selectedAcc ? formatMoney(selectedAcc.balance) : ''}</span>
                <ChevronDown size={16} />
              </span>
            </button>
            <div className="flex justify-center">
              <ArrowRightLeft size={20} className="text-slate-300" />
            </div>
            <button className="flex items-center justify-between w-full" onClick={() => setToAccDrawerOpen(true)}>
              <span className="label mb-0">转入账户</span>
              <span className="text-sm text-slate-700 flex items-center gap-1">
                {selectedToAcc?.name || '请选择'}
                <span className="text-slate-400">{selectedToAcc ? formatMoney(selectedToAcc.balance) : ''}</span>
                <ChevronDown size={16} />
              </span>
            </button>
            <div>
              <label className="label">手续费（可选）</label>
              <input
                className="input"
                placeholder="0.00"
                inputMode="decimal"
                value={form.transfer_fee}
                onChange={e => setField('transfer_fee', e.target.value.replace(/[^0-9.]/g, ''))}
              />
            </div>
          </>
        ) : (
          <button className="flex items-center justify-between w-full" onClick={() => setAccDrawerOpen(true)}>
            <span className="label mb-0">账户</span>
            <span className="text-sm text-slate-700 flex items-center gap-1">
              {selectedAcc?.name || '请选择账户'}
              <span className="text-slate-400">{selectedAcc ? formatMoney(selectedAcc.balance) : ''}</span>
              <ChevronDown size={16} />
            </span>
          </button>
        )}
      </section>

      {/* 日期时间 */}
      <section className="card card-body grid grid-cols-2 gap-3">
        <div>
          <label className="label flex items-center gap-1">
            <CalendarIcon size={14} className="text-slate-400" /> 日期
          </label>
          <input
            type="date" className="input"
            value={form.tx_date}
            onChange={e => setField('tx_date', e.target.value)}
          />
        </div>
        <div>
          <label className="label">时间</label>
          <input
            type="time" className="input"
            value={form.tx_time}
            onChange={e => setField('tx_time', e.target.value)}
          />
        </div>
      </section>

      {/* 描述 / 商户 */}
      <section className="card card-body space-y-3">
        <div>
          <div className="flex items-center justify-between">
            <label className="label mb-0">{tab === 'transfer' ? '备注' : '描述'}</label>
            {tab !== 'transfer' && (
              <button
                type="button"
                className="text-xs text-violet-600 hover:text-violet-700 flex items-center gap-1 disabled:opacity-50"
                disabled={aiClassifying}
                onClick={runAiClassify}
              >
                {aiClassifying
                  ? <span className="inline-block w-3 h-3 border border-violet-300 border-t-violet-600 rounded-full animate-spin" />
                  : <Wand2 size={12} />}
                AI 自动分类
              </button>
            )}
          </div>
          <input
            className="input"
            placeholder={tab === 'transfer' ? '转账备注（可选）' : '简单描述一下这笔...'}
            value={form.description}
            onChange={e => setField('description', e.target.value)}
          />
        </div>
        {tab !== 'transfer' && (
          <div>
            <label className="label">商户/地点（可选）</label>
            <input
              className="input"
              placeholder="如：星巴克 / 公司附近"
              value={form.merchant}
              onChange={e => setField('merchant', e.target.value)}
            />
          </div>
        )}
      </section>

      {/* 标签 */}
      <section className="card card-body">
        <button
          className="flex items-center justify-between w-full mb-3"
          onClick={() => setTagDrawerOpen(true)}
        >
          <span className="label mb-0 flex items-center gap-1">
            <Tag size={14} className="text-slate-400" /> 标签
          </span>
          <span className="text-sm text-brand-600 flex items-center gap-1">
            {form.tag_ids.length > 0 ? `已选 ${form.tag_ids.length} 个` : '添加标签'}
            <ChevronDown size={16} />
          </span>
        </button>
        <div className="flex gap-2 flex-wrap">
          {form.tag_ids.length === 0 ? (
            <span className="text-xs text-slate-400">暂无标签，点击右上角添加</span>
          ) : (
            tags.filter(t => form.tag_ids.includes(t.id)).map(tg => (
              <span key={tg.id} className="inline-flex items-center gap-1">
                <TagChip label={tg.name} color={tg.color} />
                <button
                  className="text-slate-400 hover:text-red-500"
                  onClick={() => toggleTag(tg.id)}
                >
                  <X size={12} />
                </button>
              </span>
            ))
          )}
        </div>
      </section>

      {/* 凭证照片（账单图片） */}
      <section className="card card-body">
        <label className="label flex items-center gap-1">
          <Image size={14} className="text-slate-400" /> 凭证照片
          <span className="text-xs text-slate-400 font-normal">（最多 9 张）</span>
        </label>
        <div className="grid grid-cols-4 md:grid-cols-6 gap-2">
          {form.images.map((img, i) => (
            <div key={i} className="relative aspect-square rounded-lg overflow-hidden bg-slate-100">
              <img
                src={img}
                alt=""
                className="w-full h-full object-cover cursor-pointer"
                onClick={() => setPreview(img)}
              />
              <button
                type="button"
                className="absolute top-1 right-1 w-5 h-5 rounded-full bg-black/60 text-white grid place-items-center"
                onClick={() => setForm(f => ({ ...f, images: f.images.filter((_, idx) => idx !== i) }))}
              >
                <X size={12} />
              </button>
            </div>
          ))}
          {form.images.length < 9 && (
            <button
              type="button"
              disabled={uploading}
              className="aspect-square rounded-lg border-2 border-dashed border-slate-200 grid place-items-center text-slate-400 hover:border-brand-400 hover:text-brand-500 transition disabled:opacity-50"
              onClick={() => fileRef.current?.click()}
            >
              <div className="flex flex-col items-center gap-1">
                {uploading
                  ? <span className="inline-block w-5 h-5 border-2 border-slate-300 border-t-brand-500 rounded-full animate-spin" />
                  : <Upload size={20} />}
                <span className="text-[10px]">{uploading ? '上传中' : '添加'}</span>
              </div>
            </button>
          )}
        </div>
        <input
          ref={fileRef}
          type="file"
          accept="image/*"
          multiple
          className="hidden"
          onChange={e => handleFiles(e.target.files)}
        />
      </section>

      {/* 额外选项 */}
      <section className="card card-body space-y-2">
        <label className="flex items-center justify-between">
          <span className="text-sm text-slate-700">计入账户余额</span>
          <input
            type="checkbox"
            className="w-4 h-4 accent-brand-600"
            checked={form.include_in_balance}
            onChange={e => setField('include_in_balance', e.target.checked)}
          />
        </label>
        {tab === 'expense' && (
          <label className="flex items-center justify-between">
            <span className="text-sm text-slate-700">计入预算</span>
            <input
              type="checkbox"
              className="w-4 h-4 accent-brand-600"
              checked={form.include_in_budget}
              onChange={e => setField('include_in_budget', e.target.checked)}
            />
          </label>
        )}
        <div className="flex items-center justify-between gap-3">
          <span className="text-sm text-slate-700">报销状态</span>
          <select
            className="input"
            value={form.reimburse_status}
            onChange={e => setField('reimburse_status', e.target.value as 'none' | 'pending' | 'done')}
          >
            <option value="none">未报销</option>
            <option value="pending">报销中</option>
            <option value="done">已报销</option>
          </select>
        </div>
      </section>

      {/* 底部固定操作栏 */}
      <div className="fixed inset-x-0 bottom-0 z-30 p-3 md:p-5 bg-white/95 backdrop-blur border-t border-slate-100 safe-bottom">
        <div className="max-w-3xl mx-auto flex gap-2">
          {!editId && (
            <button
              className="btn-secondary flex-1"
              disabled={submitting}
              onClick={() => submit(true)}
            >
              <Repeat1 size={16} /> 保存并再记一笔
            </button>
          )}
          <button
            className="btn-primary flex-1 btn-lg"
            disabled={submitting}
            onClick={() => submit(false)}
          >
            <Save size={16} />
            {editId ? '保存修改' : '保存'}
          </button>
        </div>
      </div>

      {/* 分类选择抽屉 */}
      <Drawer open={catDrawerOpen} onClose={() => setCatDrawerOpen(false)} title="选择分类">
        <div className="space-y-4">
          {currentCats.filter(c => !c.parent_id || c.parent_id === 0).map(p => {
            const children = currentCats.filter(c => c.parent_id === p.id);
            return (
              <div key={p.id}>
                <button
                  onClick={() => { setField('category_id', p.id); setCatDrawerOpen(false); }}
                  className={cn(
                    'w-full text-left px-3 py-2 rounded-lg flex items-center gap-2 mb-2',
                    form.category_id === p.id ? 'bg-brand-50' : 'hover:bg-slate-50'
                  )}
                >
                  <span className="text-xl">{p.icon || '📦'}</span>
                  <span className="font-medium">{p.name}</span>
                </button>
                {children.length > 0 && (
                  <div className="grid grid-cols-3 md:grid-cols-4 gap-2 pl-8">
                    {children.map(c => {
                      const active = form.category_id === c.id;
                      return (
                        <button
                          key={c.id}
                          onClick={() => { setField('category_id', c.id); setCatDrawerOpen(false); }}
                          className={cn(
                            'px-2 py-2 text-sm rounded-lg text-left truncate',
                            active ? 'bg-brand-600 text-white' : 'bg-slate-50 hover:bg-slate-100 text-slate-700'
                          )}
                        >
                          {c.icon} {c.name}
                        </button>
                      );
                    })}
                  </div>
                )}
              </div>
            );
          })}
        </div>
      </Drawer>

      {/* 账户选择抽屉 */}
      <Drawer open={accDrawerOpen} onClose={() => setAccDrawerOpen(false)} title="选择账户">
        <ul className="space-y-2">
          {accounts.filter(a => !a.is_archived).map(a => {
            const active = form.account_id === a.id;
            return (
              <li key={a.id}>
                <button
                  onClick={() => { setField('account_id', a.id); setAccDrawerOpen(false); }}
                  className={cn(
                    'w-full flex items-center gap-3 p-3 rounded-xl border transition',
                    active ? 'border-brand-500 bg-brand-50' : 'border-slate-100 hover:border-slate-200'
                  )}
                >
                  <div className="w-10 h-10 rounded-lg grid place-items-center text-xl" style={{ background: (a.color || '#64748b') + '15' }}>
                    {accountIcon(a.type)}
                  </div>
                  <div className="flex-1 text-left">
                    <div className="text-sm font-medium text-slate-800">{a.name}</div>
                    <div className="text-xs text-slate-400">余额 {formatMoney(a.balance)}</div>
                  </div>
                </button>
              </li>
            );
          })}
        </ul>
      </Drawer>

      <Drawer open={toAccDrawerOpen} onClose={() => setToAccDrawerOpen(false)} title="选择转入账户">
        <ul className="space-y-2">
          {accounts.filter(a => !a.is_archived && a.id !== form.account_id).map(a => {
            const active = form.to_account_id === a.id;
            return (
              <li key={a.id}>
                <button
                  onClick={() => { setField('to_account_id', a.id); setToAccDrawerOpen(false); }}
                  className={cn(
                    'w-full flex items-center gap-3 p-3 rounded-xl border transition',
                    active ? 'border-brand-500 bg-brand-50' : 'border-slate-100 hover:border-slate-200'
                  )}
                >
                  <div className="w-10 h-10 rounded-lg grid place-items-center text-xl" style={{ background: (a.color || '#64748b') + '15' }}>
                    {accountIcon(a.type)}
                  </div>
                  <div className="flex-1 text-left">
                    <div className="text-sm font-medium text-slate-800">{a.name}</div>
                    <div className="text-xs text-slate-400">余额 {formatMoney(a.balance)}</div>
                  </div>
                </button>
              </li>
            );
          })}
        </ul>
      </Drawer>

      {/* 标签抽屉 */}
      <Drawer open={tagDrawerOpen} onClose={() => setTagDrawerOpen(false)} title="选择标签">
        <div className="space-y-2">
          {tags.length === 0 && <div className="text-center text-sm text-slate-400 py-8">暂无标签</div>}
          <div className="flex flex-wrap gap-2">
            {tags.map(tg => {
              const active = form.tag_ids.includes(tg.id);
              return (
                <button
                  key={tg.id}
                  onClick={() => toggleTag(tg.id)}
                  className={cn(
                    'chip !px-3 !py-1.5 text-sm',
                    active ? 'ring-2 ring-brand-500' : ''
                  )}
                  style={{
                    background: (tg.color || '#334155') + (active ? '30' : '15'),
                    color: tg.color || '#334155',
                  }}
                >
                  #{tg.name}
                  {tg.count > 0 && <span className="ml-1 opacity-60">({tg.count})</span>}
                </button>
              );
            })}
          </div>
        </div>
      </Drawer>

      {/* 图片预览 */}
      {preview && (
        <div
          className="fixed inset-0 z-50 bg-black/80 grid place-items-center p-4"
          onClick={() => setPreview(null)}
        >
          <img src={preview} alt="" className="max-w-full max-h-full rounded-lg" />
        </div>
      )}
    </div>
  );
}

function accountIcon(type: string) {
  const map: Record<string, string> = {
    cash: '💵', bank: '🏦', credit: '💳', prepaid: '🎟️',
    investment: '📈', liability: '💸', virtual: '📱',
  };
  return map[type] || '💰';
}
