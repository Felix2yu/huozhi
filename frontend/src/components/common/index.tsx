import { ReactNode, useState } from 'react';
import { cn } from '@/utils';
import { X } from 'lucide-react';

export function Modal({
  open,
  onClose,
  title,
  children,
  footer,
  className,
}: {
  open: boolean;
  onClose: () => void;
  title?: ReactNode;
  children: ReactNode;
  footer?: ReactNode;
  className?: string;
}) {
  if (!open) return null;
  return (
    <div className="fixed inset-0 z-50 grid place-items-center p-4">
      <div className="absolute inset-0 bg-slate-900/50" onClick={onClose} />
      <div className={cn(
        'relative w-full bg-white rounded-xl shadow-soft max-h-[90vh] flex flex-col',
        'max-w-lg', className || ''
      )}>
        {title && (
          <div className="flex items-center justify-between px-5 py-4 border-b border-slate-100">
            <h3 className="font-semibold text-slate-800">{title}</h3>
            <button className="btn-ghost btn-sm" onClick={onClose}>
              <X size={18} />
            </button>
          </div>
        )}
        <div className="flex-1 overflow-y-auto p-5">{children}</div>
        {footer && <div className="px-5 py-3 border-t border-slate-100 flex justify-end gap-2">{footer}</div>}
      </div>
    </div>
  );
}

export function ConfirmDialog({
  open, onClose, onConfirm, title = '确认操作',
  desc = '该操作不可撤销，是否继续？', okText = '确认', danger,
}: {
  open: boolean;
  onClose: () => void;
  onConfirm: () => Promise<void> | void;
  title?: string;
  desc?: ReactNode;
  okText?: string;
  danger?: boolean;
}) {
  const [loading, setLoading] = useState(false);
  return (
    <Modal open={open} onClose={onClose} title={title}
      footer={
        <>
          <button className="btn-secondary" onClick={onClose}>取消</button>
          <button
            className={danger ? 'btn-danger' : 'btn-primary'}
            disabled={loading}
            onClick={async () => {
              setLoading(true);
              try {
                await onConfirm();
                onClose();
              } finally {
                setLoading(false);
              }
            }}
          >{loading ? '处理中...' : okText}</button>
        </>
      }
    >
      <p className="text-sm text-slate-600 leading-relaxed">{desc}</p>
    </Modal>
  );
}

export function Empty({ text = '暂无数据', icon }: { text?: string; icon?: ReactNode }) {
  return (
    <div className="py-16 flex flex-col items-center justify-center text-slate-400">
      <div className="w-16 h-16 rounded-full bg-slate-100 grid place-items-center mb-3">
        {icon || <span className="text-2xl">📭</span>}
      </div>
      <div className="text-sm">{text}</div>
    </div>
  );
}

export function AmountBadge({
  type, amount, showSign = true, currency,
}: {
  type: 'income' | 'expense' | 'transfer' | string;
  amount: number;
  showSign?: boolean;
  currency?: string;
}) {
  const cls =
    type === 'income' || type === 'refund'
      ? 'text-income'
      : type === 'expense' || type === 'reimburse'
        ? 'text-expense'
        : 'text-transfer';
  const sign =
    !showSign ? '' :
      (type === 'income' || type === 'refund' ? '+' :
        (type === 'expense' || type === 'reimburse' ? '-' : ''));
  const n = typeof amount === 'number' ? amount.toLocaleString('zh-CN', {
    minimumFractionDigits: 2, maximumFractionDigits: 2,
  }) : '0.00';
  const sym = currency === 'USD' ? '$' : '¥';
  return <span className={cn('font-semibold tabular-nums', cls)}>{sign}{sym}{n}</span>;
}

export function Progress({
  value, total, alert = 0.8,
}: { value: number; total: number; alert?: number }) {
  const pct = total > 0 ? Math.min(100, (value / total) * 100) : 0;
  const over = pct >= 100;
  const warn = pct >= alert * 100 && !over;
  return (
    <div className="w-full h-2 bg-slate-100 rounded-full overflow-hidden">
      <div
        className={cn(
          'h-full rounded-full transition-all',
          over ? 'bg-red-500' : warn ? 'bg-amber-400' : 'bg-brand-500'
        )}
        style={{ width: pct + '%' }}
      />
    </div>
  );
}

export function TagChip({ label, color }: { label: string; color?: string }) {
  return (
    <span
      className="chip"
      style={{ background: (color || '#334155') + '15', color: color || '#334155' }}
    >
      #{label}
    </span>
  );
}

export function Drawer({
  open, onClose, title, children, side = 'bottom',
}: {
  open: boolean; onClose: () => void; title?: ReactNode;
  children: ReactNode; side?: 'bottom' | 'right';
}) {
  if (!open) return null;
  return (
    <div className="fixed inset-0 z-40">
      <div className="absolute inset-0 bg-slate-900/50" onClick={onClose} />
      <div
        className={cn(
          'absolute bg-white shadow-2xl flex flex-col',
          side === 'bottom'
            ? 'inset-x-0 bottom-0 max-h-[85vh] rounded-t-2xl'
            : 'inset-y-0 right-0 w-full max-w-md'
        )}
      >
        {title && (
          <div className="px-5 py-4 border-b border-slate-100 flex items-center justify-between">
            <h3 className="font-semibold text-slate-800">{title}</h3>
            <button className="btn-ghost btn-sm" onClick={onClose}><X size={18} /></button>
          </div>
        )}
        {side === 'bottom' && (
          <div className="absolute left-1/2 -translate-x-1/2 -top-1 w-12 h-1 bg-slate-200 rounded-full" />
        )}
        <div className="flex-1 overflow-y-auto p-5">{children}</div>
      </div>
    </div>
  );
}
