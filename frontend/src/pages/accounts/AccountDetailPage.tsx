import { useCallback, useEffect, useMemo, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useAppStore } from '@/stores/app';
import { accountApi, txApi } from '@/api';
import type { Account, AccountType, DayGroup } from '@/types';
import { formatMoney, formatDate, cn, pct } from '@/utils';
import {
  ArrowLeftRight, Calendar, Edit3, StickyNote, CreditCard,
  Image as ImageIcon,
} from 'lucide-react';
import { AmountBadge, Empty } from '@/components/common';
import { PageHeader, HeroCard, HeroStat } from '@/components/common/page';

const TYPE_LABELS: Record<AccountType, string> = {
  cash: '现金', bank: '储蓄卡', credit: '信用卡', prepaid: '储值卡',
  investment: '投资账户', liability: '负债', virtual: '虚拟账户',
};

const PAGE_SIZE = 50;

export default function AccountDetailPage() {
  const { id } = useParams();
  const accountId = Number(id);
  const navigate = useNavigate();
  const bookId = useAppStore(s => s.currentBookId);
  const { expense: expCats, income: incCats } = useAppStore(s => s.categories);

  const [account, setAccount] = useState<Account | null>(null);
  const [groups, setGroups] = useState<DayGroup[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [error, setError] = useState('');

  const allCats = useMemo(() => [...expCats, ...incCats], [expCats, incCats]);
  const catOf = (cid: number) => allCats.find(c => c.id === cid);

  useEffect(() => {
    if (!accountId) return;
    (async () => {
      setLoading(true);
      setError('');
      try {
        const acc = await accountApi.get(accountId).catch(() => null);
        if (!acc) {
          setError('账户不存在或已删除');
          return;
        }
        setAccount(acc);
        const res = await txApi.listPaged({ account_id: accountId, page: 1, page_size: PAGE_SIZE });
        setGroups(res.list?.grouped || []);
        setTotal(res.pagination?.total || 0);
        setPage(1);
      } catch (e: any) {
        setError(e?.message || '加载失败');
      } finally {
        setLoading(false);
      }
    })();
  }, [accountId]);

  const loadMore = useCallback(async () => {
    if (loadingMore || groups.length === 0) return;
    setLoadingMore(true);
    try {
      const res = await txApi.listPaged({ account_id: accountId, page: page + 1, page_size: PAGE_SIZE });
      setGroups(prev => [...prev, ...(res.list?.grouped || [])]);
      setTotal(res.pagination?.total || 0);
      setPage(p => p + 1);
    } finally {
      setLoadingMore(false);
    }
  }, [accountId, page, loadingMore, groups.length]);

  const loadedCount = groups.reduce((s, g) => s + g.transactions.length, 0);
  const hasMore = loadedCount < total;

  const isDebt = account?.type === 'credit' || account?.type === 'liability';

  if (loading) {
    return <div className="py-32 text-center text-slate-400 text-sm">加载中...</div>;
  }
  if (error || !account) {
    return (
      <div className="space-y-5">
        <PageHeader back />
        <Empty text={error || '账户不存在'} />
      </div>
    );
  }

  return (
    <div className="space-y-5">
      <PageHeader
        back
        title={account.name}
        subtitle={`${TYPE_LABELS[account.type]}${account.bank_name ? ` · ${account.bank_name}` : ''}${account.card_no4 ? ` · 尾号 ${account.card_no4}` : ''}`}
      />

      {/* 账户概览 Hero */}
      <HeroCard>
        <div className="text-xs text-white/70">
          {isDebt ? (account.balance > 0 ? '当前应还' : '当前溢缴') : '当前余额'}
        </div>
        <div className={cn(
          'text-3xl md:text-4xl font-bold tabular-nums mt-2 tracking-tight',
          isDebt && account.balance > 0 && 'text-red-200',
        )}>
          {formatMoney(account.balance)}
        </div>
        <div className="mt-4 grid grid-cols-2 md:grid-cols-4 gap-4">
          <HeroStat label="交易笔数" value={`${total} 笔`} />
          <HeroStat label="初始金额" value={formatMoney(account.initial_amount || 0)} />
          {account.type === 'credit' && account.credit_limit ? (
            <>
              <HeroStat label="信用额度" value={formatMoney(account.credit_limit)} />
              <HeroStat
                label="可用额度"
                value={formatMoney(Math.max(0, account.credit_limit - Math.max(0, account.balance)))}
              />
            </>
          ) : (
            <HeroStat label="币种" value={account.currency || 'CNY'} />
          )}
        </div>
        {account.type === 'credit' && account.credit_limit ? (
          <div className="mt-4">
            <div className="flex justify-between text-[11px] text-white/70 mb-1">
              <span>额度使用 {pct(Math.max(0, account.balance), account.credit_limit)}%</span>
              <span>{formatMoney(Math.max(0, account.balance))} / {formatMoney(account.credit_limit)}</span>
            </div>
            <div className="h-1.5 bg-white/20 rounded-full overflow-hidden">
              <div
                className="h-full rounded-full bg-white/80"
                style={{ width: `${Math.min(100, pct(Math.max(0, account.balance), account.credit_limit))}%` }}
              />
            </div>
          </div>
        ) : null}
      </HeroCard>

      {/* 交易流水 */}
      <section className="card card-body">
        <div className="flex items-center justify-between mb-4">
          <h3 className="font-semibold text-slate-800 flex items-center gap-2">
            <CreditCard size={16} className="text-brand-600" /> 全部流水
          </h3>
          <span className="text-xs text-slate-400">含转入/转出 · 共 {total} 笔</span>
        </div>

        {groups.length === 0 ? (
          <Empty text="该账户还没有交易记录" />
        ) : (
          <div className="space-y-6">
            {groups.map(g => (
              <div key={g.date}>
                <div className="flex items-center justify-between mb-2 px-1">
                  <div className="flex items-center gap-2">
                    <Calendar size={14} className="text-slate-400" />
                    <span className="font-medium text-slate-700">{g.date}</span>
                    <span className="text-xs text-slate-400">
                      ({['周日', '周一', '周二', '周三', '周四', '周五', '周六'][new Date(g.date).getDay()]})
                    </span>
                  </div>
                  <div className="text-xs tabular-nums space-x-3">
                    {g.day_income > 0 && <span className="text-income">+{formatMoney(g.day_income)}</span>}
                    {g.day_expense > 0 && <span className="text-expense">-{formatMoney(g.day_expense)}</span>}
                  </div>
                </div>
                <ul className="divide-y divide-slate-50 rounded-xl border border-slate-100 overflow-hidden">
                  {g.transactions.map(t => {
                    const cat = catOf(t.category_id);
                    return (
                      <li
                        key={t.id}
                        className="flex items-center gap-3 p-3 hover:bg-slate-50 dark:hover:bg-slate-800 transition cursor-pointer"
                        onClick={() => navigate(`/transactions/add?id=${t.id}`)}
                        title="点击编辑该账单"
                      >
                        <div
                          className="w-10 h-10 rounded-lg grid place-items-center text-xl shrink-0"
                          style={{ background: (cat?.color || '#64748b') + '15' }}
                        >
                          {t.type === 'transfer' ? <ArrowLeftRight size={18} className="text-indigo-600" /> : (cat?.icon || '📦')}
                        </div>
                        <div className="flex-1 min-w-0">
                          <div className="flex items-center gap-2 flex-wrap">
                            <span className="text-sm font-medium text-slate-800 dark:text-slate-100 truncate">
                              {t.description || cat?.name || (t.type === 'transfer' ? '转账' : '未分类')}
                            </span>
                            {t.merchant && <span className="text-xs text-slate-400">{t.merchant}</span>}
                            {!!t.images?.length && (
                              <span className="inline-flex items-center gap-0.5 text-[11px] text-slate-400">
                                <ImageIcon size={12} /> {t.images.length}
                              </span>
                            )}
                            {t.remark && <StickyNote size={12} className="text-amber-400 shrink-0" />}
                          </div>
                          <div className="text-xs text-slate-400 mt-0.5 flex items-center gap-2 flex-wrap">
                            <span>{formatDate(t.tx_date, 'HH:mm')}</span>
                            {t.type === 'transfer' && <span className="text-indigo-500">转账</span>}
                            {t.type !== 'transfer' && cat?.name && <span>{cat.name}</span>}
                            {t.tags?.map(tg => (
                              <span key={tg.id} className="text-[11px] text-slate-400">#{tg.name}</span>
                            ))}
                          </div>
                        </div>
                        <div className="flex items-center gap-2 shrink-0">
                          <AmountBadge type={t.type} amount={t.amount} />
                          <Edit3 size={13} className="text-slate-300" />
                        </div>
                      </li>
                    );
                  })}
                </ul>
              </div>
            ))}

            {hasMore && (
              <button
                className="btn-secondary w-full"
                disabled={loadingMore}
                onClick={loadMore}
              >
                {loadingMore ? '加载中...' : `加载更多（已显示 ${loadedCount}/${total} 笔）`}
              </button>
            )}
          </div>
        )}
      </section>
    </div>
  );
}
