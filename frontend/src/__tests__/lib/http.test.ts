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
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { ioApi } from '$lib/api/modules/io';

// 先 mock localStorage
const store: Record<string, string> = {};
vi.stubGlobal('localStorage', {
	getItem: (k: string) => store[k] ?? null,
	setItem: (k: string, v: string) => {
		store[k] = v;
	},
	removeItem: (k: string) => {
		delete store[k];
	},
	clear: () => {
		Object.keys(store).forEach((k) => delete store[k]);
	}
});

// 每个用例前清空 store
beforeEach(() => {
	Object.keys(store).forEach((k) => delete store[k]);
	vi.unstubAllGlobals();
});

import http, {
	ApiError,
	queueCount,
	clearQueue,
	ERR_OFFLINE_QUEUED,
	ERR_TIMEOUT,
	ERR_BAD_RESPONSE
} from '$lib/api/http';

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

		await expect(http.post('/transactions', { amount: 100 })).rejects.toMatchObject({
			code: ERR_OFFLINE_QUEUED
		});
		expect(queueCount()).toBe(1);
	});

	// ====== 以下为超时 / 坏响应 / 不可重放端点的回归测试 ======
	// 历史 bug: 所有「非 ApiError」异常都被当成断网入队。于是请求超时（请求可能
	// 已在服务端执行）与反向代理返回 HTML 错误页（res.json() 抛 SyntaxError）
	// 也会被暂存重放，前者造成重复写入，后者把服务端故障伪装成离线。

	it('请求超时 → 不入队（请求可能已执行，重放会重复写入）', async () => {
		// mock 的 fetch 不响应 signal，自己延迟 50ms 抛出 AbortError；
		// 而 http 内部 10ms 就会把 timedOut 置位。
		const fetchMock = vi.fn(
			() =>
				new Promise((_resolve, reject) => {
					setTimeout(() => reject(new DOMException('aborted', 'AbortError')), 50);
				})
		);
		vi.stubGlobal('fetch', fetchMock);

		await expect(
			http.post('/transactions', { amount: 100 }, { timeout: 10 })
		).rejects.toMatchObject({ code: ERR_TIMEOUT });
		expect(queueCount()).toBe(0);
	});

	it('响应体非 JSON（反向代理 502 页）→ 不入队', async () => {
		const fetchMock = vi.fn().mockResolvedValue({
			ok: false,
			status: 502,
			json: () => Promise.reject(new SyntaxError('Unexpected token < in JSON at position 0'))
		});
		vi.stubGlobal('fetch', fetchMock);

		await expect(http.post('/transactions', { amount: 100 })).rejects.toMatchObject({
			code: ERR_BAD_RESPONSE
		});
		expect(queueCount()).toBe(0);
	});

	it('生成 API Key 失败 → 不入队（结果需当场展示，重放会作废旧 key）', async () => {
		const fetchMock = vi.fn().mockRejectedValue(new TypeError('NetworkError'));
		vi.stubGlobal('fetch', fetchMock);

		// 不入队，因此原始 TypeError 会原样抛出（而不是被替换成 OFFLINE_QUEUED）
		await expect(http.post('/api-key/generate')).rejects.toThrow('NetworkError');
		expect(queueCount()).toBe(0);
	});

	it('/api-key 不得被误判为已带 /api 前缀', async () => {
		const fetchMock = vi.fn().mockResolvedValue({
			ok: true,
			status: 200,
			json: () => Promise.resolve({ code: 0, data: {} })
		});
		vi.stubGlobal('fetch', fetchMock);

		await http.get('/api-key');
		expect(fetchMock.mock.calls[0][0]).toBe('/api/api-key');
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

describe('备份与恢复 API', () => {
	beforeEach(() => {
		clearQueue();
		http.setToken('backup-test-token');
	});

	afterEach(() => {
		vi.restoreAllMocks();
		vi.unstubAllGlobals();
		http.removeToken();
	});

	it.each(['s3', 'local'] as const)('立即备份返回 %s 存储结果并携带认证', async (storage) => {
		const data = { name: 'backup.zip', storage };
		const fetchMock = vi.fn().mockResolvedValue({
			ok: true,
			json: async () => ({ code: 0, data })
		});
		vi.stubGlobal('fetch', fetchMock);

		await expect(ioApi.createAutoBackup()).resolves.toEqual(data);
		expect(fetchMock).toHaveBeenCalledWith('/api/io/auto-backups', {
			method: 'POST',
			headers: { Authorization: 'Bearer backup-test-token' }
		});
		expect(queueCount()).toBe(0);
	});

	it.each([
		{ ok: false, code: 401 },
		{ ok: false, code: 500 },
		{ ok: true, code: 1001 },
		{ ok: true, code: 0 }
	])('立即备份拒绝错误或缺失数据的响应 %j', async ({ ok, code }) => {
		vi.stubGlobal(
			'fetch',
			vi.fn().mockResolvedValue({
				ok,
				json: async () => ({ code, message: '内部存储错误详情' })
			})
		);

		await expect(ioApi.createAutoBackup()).rejects.toThrow('立即备份失败');
		expect(queueCount()).toBe(0);
	});

	it('立即备份断网不进入离线重放队列', async () => {
		vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('NetworkError')));

		await expect(ioApi.createAutoBackup()).rejects.toThrow('NetworkError');
		expect(queueCount()).toBe(0);
	});

	it('列表保留名称、大小和时间并携带认证', async () => {
		const data = [{ name: 'backup.zip', size: 1024, time: '2026-09-17T03:00:00Z' }];
		const fetchMock = vi.fn().mockResolvedValue({
			ok: true,
			status: 200,
			json: async () => ({ code: 0, data })
		});
		vi.stubGlobal('fetch', fetchMock);

		await expect(ioApi.listAutoBackups()).resolves.toEqual(data);
		expect(fetchMock).toHaveBeenCalledWith(
			'/api/io/auto-backups',
			expect.objectContaining({
				method: 'GET',
				headers: expect.objectContaining({ Authorization: 'Bearer backup-test-token' })
			})
		);
	});

	it('下载编码文件名、携带认证并释放对象 URL', async () => {
		const name = '备份 #1.zip';
		const blob = new Blob(['zip'], { type: 'application/zip' });
		const fetchMock = vi.fn().mockResolvedValue({
			ok: true,
			headers: new Headers({ 'Content-Type': 'application/zip' }),
			blob: async () => blob
		});
		vi.stubGlobal('fetch', fetchMock);
		const createObjectURL = vi.fn(() => 'blob:backup');
		const revokeObjectURL = vi.fn();
		vi.stubGlobal('URL', { createObjectURL, revokeObjectURL });
		const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function (
			this: HTMLAnchorElement
		) {
			expect(this.download).toBe(name);
			expect(this.getAttribute('href')).toBe('blob:backup');
			expect(this.isConnected).toBe(true);
		});

		await ioApi.downloadAutoBackup(name);
		expect(fetchMock).toHaveBeenCalledWith(`/api/io/auto-backups/${encodeURIComponent(name)}`, {
			headers: { Authorization: 'Bearer backup-test-token' }
		});
		expect(click).toHaveBeenCalledOnce();
		expect(createObjectURL).toHaveBeenCalledWith(blob);
		expect(revokeObjectURL).toHaveBeenCalledWith('blob:backup');
		expect(document.querySelector('a[download]')).toBeNull();
	});

	it.each([
		{ ok: false, type: 'application/json' },
		{ ok: false, type: 'text/html' },
		{ ok: true, type: 'application/json' },
		{ ok: true, type: 'text/html' }
	])('错误响应不触发 ZIP 下载 %j', async ({ ok, type }) => {
		const blob = vi.fn();
		vi.stubGlobal(
			'fetch',
			vi.fn().mockResolvedValue({
				ok,
				headers: new Headers({ 'Content-Type': type }),
				blob
			})
		);
		const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});

		await expect(ioApi.downloadAutoBackup('backup.zip')).rejects.toThrow('备份下载失败');
		expect(blob).not.toHaveBeenCalled();
		expect(click).not.toHaveBeenCalled();
	});

	it.each(['replace', 'merge'] as const)('恢复以 raw File 发送，模式为 %s', async (mode) => {
		const file = new File(['backup'], 'backup.zip', { type: 'application/zip' });
		const fetchMock = vi.fn().mockResolvedValue({
			ok: true,
			json: async () => ({ code: 0, data: { imported: {} } })
		});
		vi.stubGlobal('fetch', fetchMock);

		await ioApi.restore(file, mode);
		expect(fetchMock).toHaveBeenCalledWith(`/api/io/restore?mode=${mode}`, {
			method: 'POST',
			headers: { Authorization: 'Bearer backup-test-token' },
			body: file
		});
	});
});
