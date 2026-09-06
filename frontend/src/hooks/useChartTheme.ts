import { useEffect, useReducer, type CSSProperties } from 'react';
import { COLOR_SCHEME_EVENT } from '@/utils/theme';

/**
 * 主题感知的 Recharts 配色。
 * 坐标轴文字、网格线、tooltip 底色/边框/文字从 CSS 变量读取（styles/index.css 中
 * 定义浅色默认值，@media (prefers-color-scheme: dark) 自动切换）；
 * 收支线颜色跟随收支配色方案。
 * 监听系统配色变化与配色方案事件，变化时强制重渲染以刷新图表颜色。
 */
export function useChartTheme() {
  const [, force] = useReducer((x: number) => x + 1, 0);

  useEffect(() => {
    const mq = window.matchMedia('(prefers-color-scheme: dark)');
    const onChange = () => force();
    mq.addEventListener('change', onChange);
    window.addEventListener(COLOR_SCHEME_EVENT, onChange);
    return () => {
      mq.removeEventListener('change', onChange);
      window.removeEventListener(COLOR_SCHEME_EVENT, onChange);
    };
  }, []);

  const cs = getComputedStyle(document.documentElement);
  const read = (name: string, fallback: string) => cs.getPropertyValue(name).trim() || fallback;

  const income = read('--income', '#10B981');
  const expense = read('--expense', '#EF4444');

  return {
    /** 坐标轴/图例文字色 */
    tick: read('--chart-tick', '#94a3b8'),
    /** 网格线颜色 */
    grid: read('--chart-grid', '#f1f5f9'),
    /** Tooltip 完整样式（borderRadius/border/背景/文字） */
    tooltipStyle: {
      borderRadius: 10,
      border: `1px solid ${read('--chart-tip-border', '#e2e8f0')}`,
      backgroundColor: read('--chart-tip-bg', '#ffffff'),
      color: read('--chart-tip-text', '#334155'),
    } as React.CSSProperties,
    /** 收支配色（随方案切换） */
    income,
    expense,
  };
}
