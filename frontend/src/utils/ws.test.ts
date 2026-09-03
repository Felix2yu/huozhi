import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';

// 在导入 ws 模块前定义可控的 WebSocket mock
class MockWebSocket {
  static OPEN = 1;
  static CLOSED = 3;
  static CONNECTING = 0;
  readyState = 0;
  url: string;
  onopen: (() => void) | null = null;
  onmessage: ((ev: any) => void) | null = null;
  onclose: (() => void) | null = null;
  onerror: (() => void) | null = null;
  static last: MockWebSocket | null = null;
  constructor(url: string) {
    this.url = url;
    MockWebSocket.last = this;
  }
  send() {}
  close() {
    this.readyState = MockWebSocket.CLOSED;
    this.onclose?.();
  }
  _open() {
    this.readyState = MockWebSocket.OPEN;
    this.onopen?.();
  }
  _msg(data: string) {
    this.onmessage?.({ data });
  }
}

vi.stubGlobal('WebSocket', MockWebSocket as any);

const { connectWs, disconnectWs, onSync, wsStatus } = await import('./ws');

describe('ws client', () => {
  beforeEach(() => {
    localStorage.clear();
    MockWebSocket.last = null;
    disconnectWs();
  });
  afterEach(() => {
    disconnectWs();
  });

  it('connectWs 建立连接并带 token 参数', () => {
    localStorage.setItem('hz_token', 'abc');
    connectWs();
    expect(MockWebSocket.last).not.toBeNull();
    expect(MockWebSocket.last!.url).toContain('/api/ws?token=abc');
  });

  it('连接打开后 wsStatus 为 true', () => {
    connectWs();
    expect(wsStatus()).toBe(false);
    MockWebSocket.last!._open();
    expect(wsStatus()).toBe(true);
  });

  it('onSync 监听器收到非 ping 消息', () => {
    const fn = vi.fn();
    const unsub = onSync(fn);
    connectWs();
    MockWebSocket.last!._open();
    MockWebSocket.last!._msg(JSON.stringify({ type: 'sync', table: 'accounts', action: 'update', id: 1 }));
    expect(fn).toHaveBeenCalledTimes(1);
    expect(fn.mock.calls[0][0].table).toBe('accounts');
    unsub();
  });

  it('忽略 ping 心跳', () => {
    const fn = vi.fn();
    onSync(fn);
    connectWs();
    MockWebSocket.last!._open();
    MockWebSocket.last!._msg(JSON.stringify({ type: 'ping' }));
    expect(fn).not.toHaveBeenCalled();
  });

  it('disconnectWs 关闭连接且不再重连', () => {
    connectWs();
    MockWebSocket.last!._open();
    expect(wsStatus()).toBe(true);
    disconnectWs();
    expect(wsStatus()).toBe(false);
  });
});
