import { describe, it, expect, beforeEach, vi } from 'vitest';
import {
  isOnline,
  isMutatingMethod,
  enqueue,
  queueCount,
  snapshot,
  replay,
  clearAll,
  subscribe,
} from './offline';

const QUEUE_KEY = 'hz_offline_queue_v1';

function makeRes(ok: boolean, status = 200, jsonBody?: any) {
  const body = jsonBody ?? {};
  const res: any = {
    ok,
    status,
    clone: () => makeRes(ok, status, jsonBody),
    json: async () => body,
  };
  return res;
}

describe('offline queue helpers', () => {
  beforeEach(() => {
    localStorage.clear();
    vi.unstubAllGlobals();
    // 默认在线
    Object.defineProperty(navigator, 'onLine', { value: true, configurable: true });
  });

  it('isOnline 反映 navigator.onLine', () => {
    expect(isOnline()).toBe(true);
    Object.defineProperty(navigator, 'onLine', { value: false, configurable: true });
    expect(isOnline()).toBe(false);
  });

  it('isMutatingMethod 仅对写方法返回 true', () => {
    expect(isMutatingMethod('POST')).toBe(true);
    expect(isMutatingMethod('put')).toBe(true);
    expect(isMutatingMethod('DELETE')).toBe(true);
    expect(isMutatingMethod('PATCH')).toBe(true);
    expect(isMutatingMethod('GET')).toBe(false);
    expect(isMutatingMethod(undefined)).toBe(false);
  });
});

describe('enqueue / queueCount / snapshot', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it('入队后数量增加且可持久化', () => {
    expect(queueCount()).toBe(0);
    const item = enqueue({ method: 'POST', url: '/api/transactions', data: { a: 1 } });
    expect(queueCount()).toBe(1);
    expect(item.id).toBeTruthy();
    expect(item.retries).toBe(0);
    const snap = snapshot();
    expect(snap[0].url).toBe('/api/transactions');
    expect(JSON.parse(localStorage.getItem(QUEUE_KEY)!)[0].url).toBe('/api/transactions'); // 持久化校验
  });

  it('subscribe 在 emit 时触发', () => {
    const fn = vi.fn();
    const unsub = subscribe(fn);
    enqueue({ method: 'PUT', url: '/api/x' });
    expect(fn).toHaveBeenCalledTimes(1);
    unsub();
    enqueue({ method: 'PUT', url: '/api/y' });
    expect(fn).toHaveBeenCalledTimes(1); // 已取消订阅
  });
});

describe('replay', () => {
  beforeEach(() => {
    localStorage.clear();
    vi.unstubAllGlobals();
    Object.defineProperty(navigator, 'onLine', { value: true, configurable: true });
    vi.stubGlobal('fetch', vi.fn());
  });

  it('成功请求后移除队列条目', async () => {
    enqueue({ method: 'POST', url: '/api/transactions', data: { a: 1 } });
    (globalThis.fetch as any).mockResolvedValue(makeRes(true));
    const r = await replay();
    expect(r.ok).toBe(1);
    expect(r.fail).toBe(0);
    expect(queueCount()).toBe(0);
  });

  it('业务错误(code!=0)计入失败并停止重放', async () => {
    enqueue({ method: 'POST', url: '/api/a' });
    enqueue({ method: 'POST', url: '/api/b' });
    (globalThis.fetch as any).mockResolvedValue(makeRes(true, 200, { code: 3001, msg: 'err' }));
    const r = await replay();
    expect(r.fail).toBe(1);
    // 第二条不应被处理（break）；失败条目仍在队列（未达重试上限）
    expect(r.remaining).toBe(2);
  });

  it('网络异常计入失败并停止重放', async () => {
    enqueue({ method: 'POST', url: '/api/a' });
    enqueue({ method: 'POST', url: '/api/b' });
    (globalThis.fetch as any).mockRejectedValue(new Error('network'));
    const r = await replay();
    expect(r.fail).toBe(1);
    expect(r.remaining).toBe(2);
  });

  it('离线时直接返回剩余条目不发送', async () => {
    enqueue({ method: 'POST', url: '/api/a' });
    Object.defineProperty(navigator, 'onLine', { value: false, configurable: true });
    const r = await replay();
    expect(r.ok).toBe(0);
    expect(r.fail).toBe(0);
    expect(r.remaining).toBe(1);
    expect((globalThis.fetch as any)).not.toHaveBeenCalled();
  });

  it('超过最大重试次数后丢弃条目', async () => {
    const item = enqueue({ method: 'POST', url: '/api/a' });
    // 手动把重试次数推到上限前
    (globalThis.fetch as any).mockRejectedValue(new Error('network'));
    for (let i = 0; i < 3; i++) {
      await replay();
    }
    expect(queueCount()).toBe(0);
    expect(item.id).toBeTruthy();
  });
});

describe('clearAll', () => {
  beforeEach(() => localStorage.clear());
  it('清空队列', () => {
    enqueue({ method: 'POST', url: '/api/a' });
    expect(queueCount()).toBe(1);
    clearAll();
    expect(queueCount()).toBe(0);
  });
});
