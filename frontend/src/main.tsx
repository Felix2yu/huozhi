import React from 'react';
import ReactDOM from 'react-dom/client';
import { BrowserRouter } from 'react-router-dom';
import { Toaster } from 'sonner';
import App from './App';
import './styles/index.css';
import { initOfflineQueue } from './utils/offline';

// 初始化离线请求队列（注册 online/offline 事件 + 启动时重放残留）
initOfflineQueue();

ReactDOM.createRoot(document.getElementById('root')!).render(
  <BrowserRouter>
    <App />
    <Toaster
      position="top-right"
      toastOptions={{
        style: { borderRadius: '12px', fontSize: '14px' },
        classNames: {
          error: '!bg-red-50 !text-red-700 !border !border-red-100',
          success: '!bg-emerald-50 !text-emerald-700 !border !border-emerald-100',
        },
      }}
    />
  </BrowserRouter>,
);
