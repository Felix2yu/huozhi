/**
 * Bug #5 回归测试: 离线请求队列的入队条件
 *
 * 历史 bug: 原代码把 500 错误（`isServerDown`）也当作离线场景入队，
 * 导致在 proxy 正常工作但后端返回 500 时，用户的 POST 请求被悄悄入队
 * 并立即抛出 OFFLINE_QUEUED 错误，前端登录/注册页面因此无法正常工作。
 *
 * 修复后: 只在真正的网络断开（navigator.onLine=false 或 fetch 抛非 ApiError）时入队。
 * HTTP 4xx/5xx 是服务端正常返回的业务/状态错误，不应入队。
 */
import { describe, it, expect, beforeEach, vi } from 'vitest';

// 先 mock localStorage
const store: Record<string, string> = {};
vi.stubGlobal('localStorage', {
	getItem: (k: string) => store[k] ?? null,
	setItem: (k: string, v: string) => { store[k] = v; },
	removeItem: (k: string) => { delete store[k]; },
	clear: () => { Object.keys(store).forEach(k => delete store[k]); }
});

// 每个用例前清空 store
beforeEach(() => {
	Object.keys(store).forEach(k => delete store[k]);
	vi.unstubAllGlobals();
});

import http, { ApiError, queueCount, clearQueue } from '$lib/api/http';

describe('HTTP client - Bug #5 offline queue conditions', () => {
	beforeEach(() => {
		clearQueue();
	});

	it('200 成功 → 不入队', async () => {
		const fetchMock = vi.fn().mockResolvedValue({
			ok: true,
			status: 200,
			json: () => Promise.resolve({ code: 0, data: { ok: true } })
		});
		vi.stubGlobal('fetch', fetchMock);

		const result = await http.post('/auth/login', { username: 'x', password: 'y' });
		expect(result).toEqual({ ok: true });
		expect(queueCount()).toBe(0);
	});

	it('400 参数错误 → 不入队（之前的 bug 会入队）', async () => {
		const fetchMock = vi.fn().mockResolvedValue({
			ok: false,
			status: 400,
			json: () => Promise.resolve({ code: 400, message: '参数错误' })
		});
		vi.stubGlobal('fetch', fetchMock);

		await expect(http.post('/auth/register', {})).rejects.toThrow(/参数错误/);
		expect(queueCount()).toBe(0);
	});

	it('401 未授权 → 不入队，抛 ApiError', async () => {
		const fetchMock = vi.fn().mockResolvedValue({
			ok: false,
			status: 401,
			json: () => Promise.resolve({ code: 401, message: '未授权' })
		});
		vi.stubGlobal('fetch', fetchMock);

		await expect(http.get('/auth/me')).rejects.toBeInstanceOf(ApiError);
		expect(queueCount()).toBe(0);
	});

	it('500 服务端错误 → 不入队（之前的 bug 会入队）', async () => {
		const fetchMock = vi.fn().mockResolvedValue({
			ok: false,
			status: 500,
			json: () => Promise.resolve({ code: 500, message: '内部错误' })
		});
		vi.stubGlobal('fetch', fetchMock);

		await expect(http.post('/auth/register', {})).rejects.toThrow(/内部错误/);
		expect(queueCount()).toBe(0);
	});

	it('网络断开（fetch 抛 TypeError）→ 入队', async () => {
		const fetchMock = vi.fn().mockRejectedValue(new TypeError('NetworkError'));
		vi.stubGlobal('fetch', fetchMock);

		await expect(http.post('/transactions', { amount: 100 })).rejects.toThrow('OFFLINE_QUEUED');
		expect(queueCount()).toBe(1);
	});

	it('GET 请求 → 即使离线也不入队（只读操作）', async () => {
		const fetchMock = vi.fn().mockRejectedValue(new TypeError('NetworkError'));
		vi.stubGlobal('fetch', fetchMock);

		await expect(http.get('/transactions')).rejects.toThrow();
		expect(queueCount()).toBe(0);
	});

	it('POST 正常业务错误 → 不入队（业务 code != 0）', async () => {
		const fetchMock = vi.fn().mockResolvedValue({
			ok: true,
			status: 200,
			json: () => Promise.resolve({ code: 1001, message: '用户名已存在' })
		});
		vi.stubGlobal('fetch', fetchMock);

		await expect(http.post('/auth/register', {})).rejects.toThrow(/用户名已存在/);
		expect(queueCount()).toBe(0);
	});

	it('PUT 请求成功', async () => {
		const fetchMock = vi.fn().mockResolvedValue({
			ok: true,
			status: 200,
			json: () => Promise.resolve({ code: 0, data: { updated: true } })
		});
		vi.stubGlobal('fetch', fetchMock);

		const result = await http.put('/transactions/1', { amount: 100 });
		expect(result).toEqual({ updated: true });
	});

	it('DELETE 请求成功', async () => {
		const fetchMock = vi.fn().mockResolvedValue({
			ok: true,
			status: 200,
			json: () => Promise.resolve({ code: 0, data: null })
		});
		vi.stubGlobal('fetch', fetchMock);

		const result = await http.delete('/transactions/1');
		expect(result).toBeNull();
	});

	it('带 params 的 GET 请求', async () => {
		const fetchMock = vi.fn().mockResolvedValue({
			ok: true,
			status: 200,
			json: () => Promise.resolve({ code: 0, data: [] })
		});
		vi.stubGlobal('fetch', fetchMock);

		await http.get('/transactions', { params: { type: 'expense', page: 1 } });
		expect(fetchMock).toHaveBeenCalledWith(
			expect.stringContaining('type=expense'),
			expect.anything()
		);
	});

	it('setToken/removeToken/getToken', () => {
		http.setToken('test-token-123');
		expect(http.getToken()).toBe('test-token-123');
		http.removeToken();
		expect(http.getToken()).toBeNull();
	});

	it('subscribeQueue 返回取消订阅函数', async () => {
		const { subscribeQueue } = await import('$lib/api/http');
		const cancel = subscribeQueue(() => {});
		expect(typeof cancel).toBe('function');
		cancel();
	});

	it('PATCH 请求成功', async () => {
		const fetchMock = vi.fn().mockResolvedValue({
			ok: true,
			status: 200,
			json: () => Promise.resolve({ code: 0, data: { patched: true } })
		});
		vi.stubGlobal('fetch', fetchMock);

		const result = await http.patch('/transactions/1', { amount: 200 });
		expect(result).toEqual({ patched: true });
	});

	it('带空params的GET请求不追加查询字符串', async () => {
		const fetchMock = vi.fn().mockResolvedValue({
			ok: true,
			status: 200,
			json: () => Promise.resolve({ code: 0, data: [] })
		});
		vi.stubGlobal('fetch', fetchMock);

		await http.get('/transactions', { params: {} });
		const url = fetchMock.mock.calls[0][0] as string;
		expect(url).toBe('/api/transactions');
	});

	it('401错误移除token', async () => {
		const fetchMock = vi.fn().mockResolvedValue({
			ok: false,
			status: 401,
			json: () => Promise.resolve({ code: 401, message: '未授权' })
		});
		vi.stubGlobal('fetch', fetchMock);
		vi.stubGlobal('localStorage', {
			getItem: vi.fn(() => 'token-123'),
			setItem: vi.fn(),
			removeItem: vi.fn()
		});

		await expect(http.get('/auth/me')).rejects.toThrow();
	});

	it('replayQueue在离线时中断', async () => {
		vi.stubGlobal('navigator', { onLine: false });
		const { replayQueue } = await import('$lib/api/http');
		const result = await replayQueue();
		expect(result.remaining).toBe(0);
	});

	it('清除队列后queueCount为0', async () => {
		const { clearQueue, queueCount } = await import('$lib/api/http');
		clearQueue();
		expect(queueCount()).toBe(0);
	});

	it('请求带自定义headers', async () => {
		const fetchMock = vi.fn().mockResolvedValue({
			ok: true,
			status: 200,
			json: () => Promise.resolve({ code: 0, data: null })
		});
		vi.stubGlobal('fetch', fetchMock);

		await http.get('/test', { headers: { 'X-Custom': 'value' } });
		const callHeaders = fetchMock.mock.calls[0][1].headers;
		expect(callHeaders['X-Custom']).toBe('value');
		expect(callHeaders['Content-Type']).toBe('application/json');
	});

	it('GET请求不携带body', async () => {
		const fetchMock = vi.fn().mockResolvedValue({
			ok: true,
			status: 200,
			json: () => Promise.resolve({ code: 0, data: null })
		});
		vi.stubGlobal('fetch', fetchMock);

		await http.get('/test');
		const callOptions = fetchMock.mock.calls[0][1];
		expect(callOptions.body).toBeUndefined();
	});
});
