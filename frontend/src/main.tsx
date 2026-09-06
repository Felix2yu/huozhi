import React from 'react';
import ReactDOM from 'react-dom/client';
import { BrowserRouter } from 'react-router-dom';
import { Toaster } from 'sonner';
import App from './App';
import './styles/index.css';
import { initOfflineQueue } from './utils/offline';
import { applyColorScheme, getColorScheme, applyShowAnimations, getShowAnimations } from './utils/theme';

// 初始化离线请求队列（注册 online/offline 事件 + 启动时重放残留）
initOfflineQueue();

// 应用持久化的收支配色方案（收入绿/支出红 或 反向）
applyColorScheme(getColorScheme());
// 应用持久化的显示动画偏好（默认跟随 prefers-reduced-motion）
applyShowAnimations(getShowAnimations());

ReactDOM.createRoot(document.getElementById('root')!).render(
  <BrowserRouter>
    <App />
    <Toaster
      position="top-right"
      toastOptions={{
        style: { borderRadius: '12px', fontSize: '14px' },
        classNames: {
          error: '!bg-red-50 !text-red-700 !border !border-red-100 dark:!bg-red-950 dark:!text-red-200 dark:!border-red-900',
          success: '!bg-emerald-50 !text-emerald-700 !border !border-emerald-100 dark:!bg-emerald-950 dark:!text-emerald-200 dark:!border-emerald-900',
        },
      }}
    />
  </BrowserRouter>,
);
