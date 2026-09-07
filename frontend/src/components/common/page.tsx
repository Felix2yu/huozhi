import { ReactNode } from 'react';
import { useNavigate } from 'react-router-dom';
import { ArrowLeft } from 'lucide-react';
import { cn } from '@/utils';

/**
 * 统一页面头：桌面端显示标题/副标题，移动端标题由 AppLayout 顶栏承担；
 * back / actions 在所有断点可见。
 */
export function PageHeader({
  title, subtitle, actions, back,
}: {
  title?: string;
  subtitle?: ReactNode;
  actions?: ReactNode;
  back?: boolean;
}) {
  const nav = useNavigate();
  if (!title && !actions && !back) return null;
  return (
    <header className="flex items-center gap-2">
      {back && (
        <button
          className="btn-ghost btn-sm shrink-0 -ml-2"
          onClick={() => nav(-1)}
          title="返回"
        >
          <ArrowLeft size={18} />
        </button>
      )}
      {(title || subtitle) && (
        <div className="hidden md:block flex-1 min-w-0">
          {title && (
            <h1 className="text-xl font-bold text-slate-800 dark:text-slate-100 truncate leading-tight">
              {title}
            </h1>
          )}
          {subtitle && (
            <p className="text-sm text-slate-500 dark:text-slate-400 truncate mt-0.5">{subtitle}</p>
          )}
        </div>
      )}
      {actions && <div className={cn('flex items-center gap-2 shrink-0', !title && 'flex-1 justify-end md:justify-end')}>{actions}</div>}
    </header>
  );
}

/** HeroCard 统一渐变汇总大卡：每个页面顶部一块，tone 仅用于语义差异（如预算超支警示） */
const heroTones = {
  brand: 'from-brand-600 via-brand-500 to-emerald-500',
  warning: 'from-red-600 via-rose-500 to-red-500',
} as const;

export function HeroCard({
  tone = 'brand', className, children,
}: {
  tone?: keyof typeof heroTones;
  className?: string;
  children: ReactNode;
}) {
  return (
    <section
      className={cn(
        'relative overflow-hidden rounded-2xl bg-gradient-to-br text-white shadow-soft',
        heroTones[tone],
        className,
      )}
    >
      <div className="absolute -top-16 -right-10 w-52 h-52 rounded-full bg-white/10 blur-2xl pointer-events-none" />
      <div className="absolute -bottom-24 -left-12 w-48 h-48 rounded-full bg-white/10 blur-2xl pointer-events-none" />
      <div className="relative p-5 md:p-6">{children}</div>
    </section>
  );
}

/** HeroCard 内的统计项 */
export function HeroStat({
  label, value, className,
}: {
  label: ReactNode;
  value: ReactNode;
  className?: string;
}) {
  return (
    <div className="min-w-0">
      <div className="text-xs text-white/70 truncate">{label}</div>
      <div className={cn('text-lg md:text-xl font-bold tabular-nums truncate', className)}>{value}</div>
    </div>
  );
}

/** 统一分段 Tab */
export function SegmentedTabs<T extends string>({
  value, onChange, options, className,
}: {
  value: T;
  onChange: (v: T) => void;
  options: { value: T; label: ReactNode; icon?: React.ComponentType<{ size?: number | string }>; activeColor?: string; activeStyle?: React.CSSProperties }[];
  className?: string;
}) {
  return (
    <div
      className={cn('card p-1.5 grid gap-1', className)}
      style={{ gridTemplateColumns: `repeat(${options.length}, minmax(0, 1fr))` }}
    >
      {options.map(opt => {
        const active = opt.value === value;
        return (
          <button
            key={opt.value}
            onClick={() => onChange(opt.value)}
            className={cn(
              'flex items-center justify-center gap-1.5 rounded-lg py-2 text-sm font-medium transition-colors',
              active
                ? cn(opt.activeColor || 'bg-brand-600 text-white', 'shadow-sm')
                : 'text-slate-500 dark:text-slate-400 hover:bg-slate-50 dark:hover:bg-slate-800 hover:text-slate-700 dark:hover:text-slate-200',
            )}
            style={active ? opt.activeStyle : undefined}
          >
            {opt.icon && <opt.icon size={16} />}
            {opt.label}
          </button>
        );
      })}
    </div>
  );
}
