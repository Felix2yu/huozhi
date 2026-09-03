// WebSocket 实时同步客户端
// 连接 /api/ws?token=xxx，接收变更推送并派发 sync 事件

export interface WsMessage {
  type: 'sync' | 'ping' | 'alert' | 'error';
  table?: string;     // transactions / accounts / categories / tags / budgets / books / recurring
  action?: string;    // create / update / delete / over_budget
  id?: number;
  data?: Record<string, any>;
  version?: number;
  timestamp?: string;
}

type Listener = (msg: WsMessage) => void;

let ws: WebSocket | null = null;
let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
let reconnectDelay = 1000;
let listeners: Listener[] = [];
let intentionalClose = false;

function buildUrl(): string {
  const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
  const token = localStorage.getItem('hz_token') || '';
  return `${proto}//${location.host}/api/ws?token=${encodeURIComponent(token)}`;
}

export function connectWs() {
  intentionalClose = false;
  if (ws && ws.readyState === WebSocket.OPEN) return;
  if (reconnectTimer) {
    clearTimeout(reconnectTimer);
    reconnectTimer = null;
  }

  try {
    ws = new WebSocket(buildUrl());

    ws.onopen = () => {
      reconnectDelay = 1000;
      console.log('[WS] connected');
    };

    ws.onmessage = (ev) => {
      try {
        const msg: WsMessage = JSON.parse(ev.data);
        if (msg.type === 'ping') return; // 忽略心跳
        // 派发
        for (const fn of listeners) {
          try { fn(msg); } catch {}
        }
        // 同时派发 DOM event（方便非 React 代码监听）
        window.dispatchEvent(new CustomEvent('hz:sync', { detail: msg }));
      } catch {}
    };

    ws.onclose = () => {
      console.log('[WS] closed');
      ws = null;
      if (!intentionalClose) scheduleReconnect();
    };

    ws.onerror = () => {
      ws?.close();
    };
  } catch (e) {
    console.warn('[WS] 连接失败', e);
    scheduleReconnect();
  }
}

function scheduleReconnect() {
  if (reconnectTimer) return;
  reconnectDelay = Math.min(reconnectDelay * 2, 30_000);
  reconnectTimer = setTimeout(() => {
    reconnectTimer = null;
    connectWs();
  }, reconnectDelay);
}

export function disconnectWs() {
  intentionalClose = true;
  if (reconnectTimer) {
    clearTimeout(reconnectTimer);
    reconnectTimer = null;
  }
  if (ws) {
    ws.close();
    ws = null;
  }
}

export function onSync(fn: Listener): () => void {
  listeners.push(fn);
  return () => {
    listeners = listeners.filter((f) => f !== fn);
  };
}

export function wsStatus(): boolean {
  return !!ws && ws.readyState === WebSocket.OPEN;
}
