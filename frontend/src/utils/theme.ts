/**
 * 收支配色方案
 * - default       : 收入绿 / 支出红（中国记账惯例）
 * - expense-green : 收入红 / 支出绿（反向）
 * 方案持久化于 localStorage，并在 <html> 上写入 data-color-scheme，
 * 由 styles/index.css 中的 CSS 变量联动驱动全站配色。
 */

export type ColorScheme = 'default' | 'expense-green';

export const COLOR_SCHEME_KEY = 'hz_color_scheme';

export const COLOR_SCHEMES: Array<{
  key: ColorScheme;
  label: string;
  income: string;
  expense: string;
}> = [
  { key: 'default', label: '收入绿 · 支出红', income: '#10B981', expense: '#EF4444' },
  { key: 'expense-green', label: '收入红 · 支出绿', income: '#EF4444', expense: '#10B981' },
];

export function getColorScheme(): ColorScheme {
  const v = localStorage.getItem(COLOR_SCHEME_KEY);
  return v === 'expense-green' ? 'expense-green' : 'default';
}

/** 配色方案变更时派发的全局事件名，供图表 hook 等监听刷新 */
export const COLOR_SCHEME_EVENT = 'hz:color-scheme';

/** 将当前配色方案同步到 <html data-color-scheme>，供 CSS 变量联动 */
export function applyColorScheme(scheme: ColorScheme): void {
  const root = document.documentElement;
  if (scheme === 'expense-green') {
    root.dataset.colorScheme = 'expense-green';
  } else {
    // default：移除以回退到 :root 默认值，避免冗余属性
    delete root.dataset.colorScheme;
  }
  localStorage.setItem(COLOR_SCHEME_KEY, scheme);
  // 通知订阅方（如 Recharts 主题 hook）立即刷新
  window.dispatchEvent(new Event(COLOR_SCHEME_EVENT));
}

/** 供图表等需要十六进制颜色的场景使用（与 CSS 变量保持一致） */
export function getFinanceColors(): { income: string; expense: string; transfer: string } {
  const s = getColorScheme();
  if (s === 'expense-green') {
    return { income: '#EF4444', expense: '#10B981', transfer: '#6366F1' };
  }
  return { income: '#10B981', expense: '#EF4444', transfer: '#6366F1' };
}

/**
 * 显示动画偏好
 * 默认值跟随系统 prefers-reduced-motion：系统要求减弱动态时默认关闭动画。
 * 关闭时在 <html> 上添加 no-anim 类，由 CSS 全局禁用 transition/animation。
 */
export const SHOW_ANIMATIONS_KEY = 'hz_show_animations';

export function getShowAnimations(): boolean {
  const v = localStorage.getItem(SHOW_ANIMATIONS_KEY);
  if (v === 'on') return true;
  if (v === 'off') return false;
  return !window.matchMedia('(prefers-reduced-motion: reduce)').matches;
}

export function applyShowAnimations(on: boolean): void {
  localStorage.setItem(SHOW_ANIMATIONS_KEY, on ? 'on' : 'off');
  document.documentElement.classList.toggle('no-anim', !on);
}
