/**
 * 离线请求队列
 *  - 断网时把 PATCH/POST/PUT/DELETE 请求持久化到 localStorage
 *  - 恢复联网后按顺序自动重放
 *  - 提供队列状态事件供 UI 订阅
 */

const QUEUE_KEY = 'hz_offline_queue_v1';
const MAX_RETRIES = 3;
const RETRY_BASE_DELAY = 1000; // ms

export interface QueueItem {
  id: string;
  method: 'POST' | 'PUT' | 'DELETE' | 'PATCH';
  url: string;            // 相对路径，如 /api/transactions
  data?: any;
  headers?: Record<string, string>;
  timestamp: number;
  retries: number;
}

// ---- 事件总线：队列变化 / 重放结果 ----
type Listener = () => void;
const listeners = new Set<Listener>();

export function subscribe(fn: Listener) {
  listeners.add(fn);
  return () => listeners.delete(fn);
}
function emit() { listeners.forEach(fn => fn()); }

// ---- 持久化 ----
function load(): QueueItem[] {
  try {
    const raw = localStorage.getItem(QUEUE_KEY);
    if (!raw) return [];
    const arr = JSON.parse(raw);
    return Array.isArray(arr) ? arr : [];
  } catch { return []; }
}
function save(items: QueueItem[]) {
  try { localStorage.setItem(QUEUE_KEY, JSON.stringify(items)); } catch {}
}

// ---- 对外 API ----

export function isOnline() {
  return typeof navigator !== 'undefined' && navigator.onLine;
}

/** 当前队列数量 */
export function queueCount(): number {
  return load().length;
}

/** 只读队列副本（用于展示详情） */
export function snapshot(): QueueItem[] {
  return load().map(i => ({ ...i }));
}

/** 是否为会修改数据的 HTTP 方法 */
export function isMutatingMethod(method?: string) {
  return !!method && ['POST', 'PUT', 'DELETE', 'PATCH'].includes(method.toUpperCase());
}

/** 入队（axios 拦截器在网络错误时调用） */
export function enqueue(item: Omit<QueueItem, 'id' | 'timestamp' | 'retries'>) {
  const items = load();
  const full: QueueItem = {
    ...item,
    id: `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
    timestamp: Date.now(),
    retries: 0,
  };
  items.push(full);
  save(items);
  emit();
  return full;
}

/** 移除已成功重放的条目 */
function remove(id: string) {
  const items = load().filter(i => i.id !== id);
  save(items);
  emit();
}

/** 失败条目：增加重试次数；超过上限丢弃 */
function markFailed(id: string, err: unknown) {
  const items = load();
  const idx = items.findIndex(i => i.id === id);
  if (idx < 0) return;
  items[idx].retries += 1;
  if (items[idx].retries >= MAX_RETRIES) {
    console.warn('[offline] 丢弃失败条目', items[idx].url, err);
    items.splice(idx, 1);
  }
  save(items);
  emit();
}

let replaying = false;

/** 重放队列：依次发送，任一失败则后续等待下一轮 */
export async function replay(): Promise<{ ok: number; fail: number; remaining: number }> {
  if (replaying) return { ok: 0, fail: 0, remaining: queueCount() };
  replaying = true;

  const items = load();
  let ok = 0, fail = 0;

  for (const item of items) {
    if (!isOnline()) break;

    // 从 localStorage 重新取最新状态，避免并发修改
    const fresh = load().find(i => i.id === item.id);
    if (!fresh) continue;

    try {
      const token = localStorage.getItem('hz_token');
      const res = await fetch(fresh.url, {
        method: fresh.method,
        headers: {
          'Content-Type': 'application/json',
          ...(token ? { Authorization: `Bearer ${token}` } : {}),
          ...(fresh.headers || {}),
        },
        body: fresh.data != null ? JSON.stringify(fresh.data) : undefined,
      });

      // 业务错误（code !== 0）也算失败，进入重试
      let businessOk = res.ok;
      if (businessOk) {
        try {
          const json = await res.clone().json();
          if (json && typeof json === 'object' && 'code' in json && json.code !== 0) {
            businessOk = false;
          }
        } catch { /* 非 JSON 响应也算成功 */ }
      }

      if (businessOk) {
        remove(fresh.id);
        ok++;
      } else {
        markFailed(fresh.id, `HTTP ${res.status}`);
        fail++;
        // 写操作遇到业务错误就停止后面的，避免重复扣账
        break;
      }
    } catch (err) {
      markFailed(fresh.id, err);
      fail++;
      break; // 网络异常停止本轮，等下一次 online 事件
    }
  }

  replaying = false;
  return { ok, fail, remaining: queueCount() };
}

/** 清空调度器（用户手动操作） */
export function clearAll() {
  save([]);
  emit();
}

/** 初始化：注册 online 事件 + 启动时尝试重放 */
export function initOfflineQueue() {
  if (typeof window === 'undefined') return;

  const onOnline = () => {
    // 延迟一下等网络稳定
    setTimeout(() => {
      if (queueCount() > 0) {
        replay().then(({ ok, remaining }) => {
          if (ok > 0) {
            // 派发事件让页面刷新数据
            window.dispatchEvent(new CustomEvent('hz:data-changed'));
          }
          console.log('[offline] 重放完成 ok=%d remaining=%d', ok, remaining);
        });
      }
    }, 500);
  };

  window.addEventListener('online', onOnline);

  // 启动时如果有残留队列，也试一次
  if (isOnline() && queueCount() > 0) {
    onOnline();
  }

  // 页面隐藏前的兜底：保存状态（其实 enqueue 已经每次 save 了）
  // 这里只做一次事件清理即可
}
