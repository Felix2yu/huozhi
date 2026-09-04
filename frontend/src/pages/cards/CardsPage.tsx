import { useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { toast } from 'sonner';
import { useAppStore } from '@/stores/app';
import { accountApi } from '@/api';
import type { Account } from '@/types';
import { formatMoney, cn } from '@/utils';
import { Plus, CreditCard, Landmark, AlertCircle, Clock, Layers } from 'lucide-react';
import { Empty } from '@/components/common';

// ============ 卡面组件 ============
interface CardFaceData {
  id: number;
  type: 'bank' | 'credit';
  bank_name?: string;
  name: string;
  card_no4?: string;
  expire_month?: number;
  expire_year?: number;
  credit_limit?: number;
  balance?: number;
}

function bankColorIdx(name: string): number {
  let h = 0;
  for (let i = 0; i < name.length; i++) h = (h * 31 + name.charCodeAt(i)) >>> 0;
  return h % 8;
}

function groupCardNo(full: string): string {
  const digits = full.replace(/\D/g, '').slice(0, 16);
  if (!digits) return '';
  return digits.replace(/(.{4})/g, '$1 ').trim();
}

function CardFace({
  acc, flipped, onFlip,
  fullInfo, loadingFull,
}: {
  acc: CardFaceData;
  flipped: boolean;
  onFlip: () => void;
  fullInfo: { full_card_no?: string; cvv?: string } | null;
  loadingFull: boolean;
}) {
  const isCredit = acc.type === 'credit';
  const bgClass = isCredit
    ? 'card3d-credit'
    : `card3d-bank-${bankColorIdx(acc.bank_name || acc.name)}`;

  const displayFull = fullInfo?.full_card_no
    ? groupCardNo(fullInfo.full_card_no)
    : (acc.card_no4 ? `**** **** **** ${acc.card_no4}` : '');

  return (
    <div className="card3d-wrap">
      <div
        className={cn('card3d', flipped && 'flipped')}
        onClick={onFlip}
        role="button"
        tabIndex={0}
        onKeyDown={e => (e.key === 'Enter' || e.key === ' ') && onFlip()}
      >
        {/* 正面 */}
        <div className={cn('card3d-face front', bgClass)}>
          <div className="card3d-shine" />
          {/* 顶部：银行名 + 卡种标签 */}
          <div className="flex items-start justify-between">
            <div>
              <div className="card3d-small">{isCredit ? '信用卡' : '储蓄卡'}</div>
              <div className="text-sm font-semibold tracking-wide mt-0.5">
                {acc.bank_name || acc.name}
              </div>
            </div>
            <div className="card3d-chip" />
          </div>

          {/* 中部：卡号 */}
          <div className="card3d-number">{displayFull}</div>

          {/* 底部：持卡人 / 有效期 */}
          <div className="flex items-end justify-between gap-3">
            <div className="min-w-0">
              <div className="card3d-small">持卡人</div>
              <div className="card3d-name truncate">{acc.name}</div>
            </div>
            <div className="text-right shrink-0">
              <div className="card3d-small">有效期</div>
              <div className="card3d-name tabular-nums">
                {acc.expire_month && acc.expire_year
                  ? `${String(acc.expire_month).padStart(2, '0')}/${String(acc.expire_year).padStart(2, '0')}`
                  : '—'}
              </div>
            </div>
          </div>
        </div>

        {/* 背面 */}
        <div className="card3d-face back">
          <div className="card3d-magstripe" />
          <div className="card3d-sigstrip">
            <div className="flex-1 text-[10px] tracking-widest text-slate-500">
              授权签名
            </div>
            <div className="card3d-cvv-box">
              {loadingFull ? '···' : (fullInfo?.cvv ? fullInfo.cvv.replace(/./g, '·') : acc.type === 'credit' ? '安全码' : '—')}
            </div>
          </div>
          <div className="px-[22px] pb-[18px] mt-[14px] text-[11px] text-slate-400 leading-relaxed">
            点击卡片查看完整卡号和安全码（<span className="text-amber-300">敏感信息 · 请注意保密</span>）
          </div>
        </div>
      </div>
    </div>
  );
}

// ============ 主页面 ============
export default function CardsPage() {
  const bookId = useAppStore(s => s.currentBookId);
  const allAccounts = useAppStore(s => s.accounts);
  const navigate = useNavigate();

  const cards = useMemo(
    () => allAccounts.filter(a => a.type === 'bank' || a.type === 'credit'),
    [allAccounts],
  );
  const credits = useMemo(() => cards.filter(c => c.type === 'credit'), [cards]);
  const debits = useMemo(() => cards.filter(c => c.type === 'bank'), [cards]);

  // 按需解密缓存
  const [fullInfoMap, setFullInfoMap] = useState<Record<number, { full_card_no?: string; cvv?: string } | null>>({});
  const [loadingMap, setLoadingMap] = useState<Record<number, boolean>>({});
  const [flipped, setFlipped] = useState<Record<number, boolean>>({});

  const flip = async (acc: Account) => {
    const nowFlipped = flipped[acc.id];
    setFlipped(prev => ({ ...prev, [acc.id]: !prev[acc.id] }));
    if (nowFlipped) return;

    // 首次翻转去拉完整卡信息
    if (fullInfoMap[acc.id] === undefined) {
      setLoadingMap(prev => ({ ...prev, [acc.id]: true }));
      try {
        const data = await accountApi.getFullCardNo(acc.id);
        setFullInfoMap(prev => ({ ...prev, [acc.id]: data || null }));
      } catch {
        setFullInfoMap(prev => ({ ...prev, [acc.id]: null }));
        toast.warning('该卡未保存完整卡号或安全码');
      } finally {
        setLoadingMap(prev => ({ ...prev, [acc.id]: false }));
      }
    }
  };

  // 顶部汇总
  // 后端语义：信用卡 balance > 0 表示已用/应还额度（负债），< 0 表示溢缴
  const creditSummary = useMemo(() => {
    const total = credits.reduce((s, c) => s + (c.credit_limit || 0), 0);
    const used = credits.reduce((s, c) => s + Math.max(0, c.balance || 0), 0);
    const available = Math.max(0, total - used);
    return { total, used, available };
  }, [credits]);

  const debitTotal = useMemo(
    () => debits.reduce((s, d) => s + Math.max(0, d.balance || 0), 0),
    [debits],
  );

  // 到期提醒：卡即将过期（90 天内）
  const expiringSoon = useMemo(() => {
    const now = new Date();
    return cards.filter(c => {
      if (!c.expire_month || !c.expire_year) return false;
      const exp = new Date(2000 + c.expire_year, c.expire_month - 1, 1);
      const diff = (exp.getTime() - now.getTime()) / 86400000;
      return diff > 0 && diff < 90;
    });
  }, [cards]);

  // 自动刷新账户列表
  const loadAccounts = useAppStore(s => s.loadDictionaries);
  useEffect(() => {
    if (bookId) loadAccounts();
  }, [bookId, loadAccounts]);

  return (
    <div className="max-w-6xl mx-auto space-y-8 pb-16">
      {/* 头部 */}
      <header className="flex items-center justify-between pt-6">
        <div>
          <h1 className="text-2xl font-bold text-slate-900 flex items-center gap-2">
            <CreditCard className="text-brand-600" size={26} />
            我的银行卡
          </h1>
          <p className="text-sm text-slate-500 mt-1">
            储蓄卡 {debits.length} 张 · 信用卡 {credits.length} 张 · 点击卡面翻转查看背面
          </p>
        </div>
        <button className="btn-primary" onClick={() => navigate('/accounts')}>
          <Plus size={16} /> 新增账户
        </button>
      </header>

      {/* 汇总条 */}
      <section className="grid md:grid-cols-3 gap-4">
        <div className="card card-body">
          <div className="flex items-center gap-2 text-xs text-slate-500 mb-1">
            <Landmark size={13} className="text-slate-400" /> 储蓄卡余额
          </div>
          <div className="text-2xl font-bold tabular-nums text-slate-800">
            {formatMoney(debitTotal)}
          </div>
          <div className="text-[11px] text-slate-400 mt-1">共 {debits.length} 张卡</div>
        </div>
        <div className="card card-body bg-gradient-to-br from-slate-900 to-slate-700 text-white border-0 shadow-lg">
          <div className="flex items-center gap-2 text-xs text-white/70 mb-1">
            <CreditCard size={13} /> 信用卡总额度
          </div>
          <div className="text-2xl font-bold tabular-nums">{formatMoney(creditSummary.total)}</div>
          <div className="mt-2 h-1.5 bg-white/20 rounded-full overflow-hidden">
            <div
              className={cn(
                'h-full rounded-full transition-all',
                creditSummary.used / creditSummary.total > 0.8 ? 'bg-red-400' :
                creditSummary.used / creditSummary.total > 0.6 ? 'bg-amber-400' : 'bg-emerald-400',
              )}
              style={{ width: `${creditSummary.total ? Math.min(100, creditSummary.used / creditSummary.total * 100) : 0}%` }}
            />
          </div>
          <div className="flex justify-between text-[11px] mt-1.5">
            <span className="text-white/60">已用 {formatMoney(creditSummary.used)}</span>
            <span className="text-white/80 font-medium">可用 {formatMoney(creditSummary.available)}</span>
          </div>
        </div>
        <div className="card card-body">
          <div className="flex items-center gap-2 text-xs text-slate-500 mb-1">
            <AlertCircle size={13} className="text-amber-500" /> 卡到期提醒
          </div>
          {expiringSoon.length > 0 ? (
            <>
              <div className="text-2xl font-bold text-amber-600">{expiringSoon.length} 张</div>
              <div className="text-[11px] text-slate-400 mt-1 truncate">
                {expiringSoon.map(c =>
                  `${c.bank_name || c.name} · ${String(c.expire_month).padStart(2, '0')}/${String(c.expire_year).padStart(2, '0')}`
                ).join('，')}
              </div>
            </>
          ) : (
            <>
              <div className="text-2xl font-bold text-emerald-600">全部正常</div>
              <div className="text-[11px] text-slate-400 mt-1">未来 90 天内无到期卡</div>
            </>
          )}
        </div>
      </section>

      {/* 信用卡专区 */}
      {credits.length > 0 && (
        <section>
          <h2 className="text-sm font-semibold text-slate-500 uppercase tracking-wider mb-4 flex items-center gap-2">
            <Layers size={14} /> 信用卡 · 额度管理
          </h2>
          <div className="grid sm:grid-cols-2 lg:grid-cols-3 gap-6">
            {credits.map(c => (
              <div key={c.id} className="space-y-3">
                <CardFace
                  acc={{
                    id: c.id, type: 'credit',
                    bank_name: c.bank_name, name: c.name,
                    card_no4: c.card_no4, expire_month: c.expire_month, expire_year: c.expire_year,
                    credit_limit: c.credit_limit, balance: c.balance,
                  }}
                  flipped={!!flipped[c.id]}
                  onFlip={() => flip(c)}
                  fullInfo={fullInfoMap[c.id] ?? null}
                  loadingFull={!!loadingMap[c.id]}
                />
                {/* 卡下额度信息 */}
                <div className="px-1 flex items-center justify-between gap-2">
                  <span className="text-xs text-slate-500 tabular-nums">
                    额度 {formatMoney(c.credit_limit || 0)} · 已用 {formatMoney(Math.max(0, c.balance || 0))}
                  </span>
                  <button
                    onClick={() => navigate(`/accounts?account=${c.id}`)}
                    className="text-xs text-brand-600 hover:underline shrink-0"
                  >
                    账户详情 →
                  </button>
                </div>
              </div>
            ))}
          </div>
        </section>
      )}

      {/* 储蓄卡专区 */}
      {debits.length > 0 && (
        <section>
          <h2 className="text-sm font-semibold text-slate-500 uppercase tracking-wider mb-4 flex items-center gap-2">
            <Landmark size={14} /> 储蓄卡 · 现金池
          </h2>
          <div className="grid sm:grid-cols-2 lg:grid-cols-3 gap-6">
            {debits.map(d => (
              <div key={d.id} className="space-y-3">
                <CardFace
                  acc={{
                    id: d.id, type: 'bank',
                    bank_name: d.bank_name, name: d.name,
                    card_no4: d.card_no4, expire_month: d.expire_month, expire_year: d.expire_year,
                    balance: d.balance,
                  }}
                  flipped={!!flipped[d.id]}
                  onFlip={() => flip(d)}
                  fullInfo={fullInfoMap[d.id] ?? null}
                  loadingFull={!!loadingMap[d.id]}
                />
                <div className="px-1 flex items-center justify-between gap-2">
                  <span className="text-xs text-slate-500 tabular-nums">
                    {d.bank_name || '储蓄卡'} · 余额 {formatMoney(d.balance || 0)}
                  </span>
                  <button
                    onClick={() => navigate(`/accounts?account=${d.id}`)}
                    className="text-xs text-brand-600 hover:underline shrink-0"
                  >
                    账户详情 →
                  </button>
                </div>
              </div>
            ))}
          </div>
        </section>
      )}

      {/* 空状态 */}
      {cards.length === 0 && (
        <div className="pt-8">
          <Empty
            icon={<CreditCard size={40} />}
            text="还没有银行卡账户"
            hint="到「账户资产」页面新增储蓄卡或信用卡，这里会自动生成炫酷卡面"
          />
        </div>
      )}

      {/* 安全提示 */}
      <div className="mt-8 rounded-xl border border-amber-200 bg-amber-50/70 p-4 flex items-start gap-3 text-[12px] text-amber-900">
        <AlertCircle size={16} className="mt-0.5 shrink-0" />
        <div>
          <div className="font-semibold mb-0.5">安全提示</div>
          <p className="leading-relaxed">
            完整卡号和安全码仅在你的设备上按需解密显示，后端仅存 AES-256-GCM 密文且默认不返回。
            查看完毕后请及时关闭本页或点击卡面翻回正面，避免他人窥视。
          </p>
        </div>
      </div>
    </div>
  );
}
