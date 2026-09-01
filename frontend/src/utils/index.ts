import { clsx, type ClassValue } from 'clsx';
import { twMerge } from 'tailwind-merge';

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

/** 金额格式化（千分位 + 保留2位小数） */
export function formatMoney(value: number | string, currency = 'CNY') {
  const n = typeof value === 'string' ? parseFloat(value || '0') : value || 0;
  const formatted = n.toLocaleString('zh-CN', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  });
  const symbol = currency === 'CNY' ? '¥' : currency === 'USD' ? '$' : '';
  return `${symbol}${formatted}`;
}

/** 日期格式化 */
export function formatDate(d: Date | string | number, pattern = 'YYYY-MM-DD') {
  const date = new Date(d);
  if (isNaN(date.getTime())) return '';
  const pad = (n: number) => String(n).padStart(2, '0');
  const map: Record<string, string> = {
    YYYY: String(date.getFullYear()),
    MM: pad(date.getMonth() + 1),
    DD: pad(date.getDate()),
    HH: pad(date.getHours()),
    mm: pad(date.getMinutes()),
    ss: pad(date.getSeconds()),
  };
  return pattern.replace(/YYYY|MM|DD|HH|mm|ss/g, (m) => map[m]);
}

/** 获取本月的起止时间 */
export function getMonthRange(date = new Date()) {
  const y = date.getFullYear();
  const m = date.getMonth();
  const start = new Date(y, m, 1);
  const end = new Date(y, m + 1, 0);
  return { start, end };
}

/** 计算进度百分比 */
export function pct(done: number, total: number, digits = 1) {
  if (!total) return 0;
  return Math.round((done / total) * 100 * 10 ** digits) / 10 ** digits;
}
