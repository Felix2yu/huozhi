/**
 * HTTP 客户端 - fetch 封装
 * - 自动注入 token
 * - 统一错误处理
 * - 离线请求队列
 */

export interface ApiResp<T = unknown> {
	code: number;
	message?: string;
	data?: T;
}

export interface QueueItem {
	id: string;
	method: 'POST' | 'PUT' | 'DELETE' | 'PATCH';
	url: string;
	data?: unknown;
	timestamp: number;
	retries: number;
}

const QUEUE_KEY = 'hz_offline_queue_v1';
const TOKEN_KEY = 'hz_token';

// 认证检查专用短超时（5秒），避免卡住页面
const AUTH_TIMEOUT = 5000;
const DEFAULT_TIMEOUT = 30000;

function getToken(): string | null {
	if (typeof localStorage === 'undefined') return null;
	return localStorage.getItem(TOKEN_KEY);
}

function setToken(token: string): void {
	localStorage.setItem(TOKEN_KEY, token);
}

function removeToken(): void {
	localStorage.removeItem(TOKEN_KEY);
}

function isOnline(): boolean {
	return typeof navigator !== 'undefined' && navigator.onLine;
}

function isMutating(method: string): boolean {
	return ['POST', 'PUT', 'DELETE', 'PATCH'].includes(method.toUpperCase());
}

// 负数错误码表示「本地产生」，与服务端返回的 code 区分开
export const ERR_OFFLINE_QUEUED = -1;
export const ERR_ABORTED = -2;
export const ERR_TIMEOUT = -3;
export const ERR_BAD_RESPONSE = -4;

// 不进离线队列的端点前缀。
//
// 判定标准不是「重不重要」，而是「暂存重放有没有意义」：
//   - /api-key：结果必须当场展示给用户（key 只出现一次），且重放会重新生成、
//     让已配置到客户端的旧 key 失效；
//   - /auth：登录/注册/改密的响应是一次性凭证，重放无意义；
//   - /io/import：重放会造成账单重复导入。
const NON_QUEUEABLE_PREFIXES = ['/api/api-key', '/api/auth', '/api/io/import'];

function isQueueable(url: string): boolean {
	const path = url.split('?')[0];
	return !NON_QUEUEABLE_PREFIXES.some((p) => path.startsWith(p));
}

// 离线队列 -----------------------------

function loadQueue(): QueueItem[] {
	try {
		const raw = localStorage.getItem(QUEUE_KEY);
		return raw ? JSON.parse(raw) : [];
	} catch {
		return [];
	}
}

function saveQueue(items: QueueItem[]): void {
	try {
		localStorage.setItem(QUEUE_KEY, JSON.stringify(items));
	} catch {}
}

function enqueueOffline(item: Omit<QueueItem, 'id' | 'timestamp' | 'retries'>): void {
	const items = loadQueue();
	items.push({
		...item,
		id: `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
		timestamp: Date.now(),
		retries: 0
	});
	saveQueue(items);
	window.dispatchEvent(new CustomEvent('hz:queue-changed'));
}

export function queueCount(): number {
	return loadQueue().length;
}

export function clearQueue(): void {
	saveQueue([]);
	window.dispatchEvent(new CustomEvent('hz:queue-changed'));
}

export function subscribeQueue(cb: () => void): () => void {
	const handler = () => cb();
	window.addEventListener('hz:queue-changed', handler);
	return () => window.removeEventListener('hz:queue-changed', handler);
}

export async function replayQueue(): Promise<{ ok: number; remaining: number }> {
	const items = loadQueue();
	let ok = 0;

	for (const item of items) {
		if (!isOnline()) break;
		try {
			const res = await fetch(item.url, {
				method: item.method,
				headers: {
					'Content-Type': 'application/json',
					...(getToken() ? { Authorization: `Bearer ${getToken()}` } : {})
				},
				body: item.data != null ? JSON.stringify(item.data) : undefined
			});
			if (res.ok) {
				const fresh = loadQueue().find((i) => i.id === item.id);
				if (fresh) {
					saveQueue(loadQueue().filter((i) => i.id !== item.id));
				}
				ok++;
			}
		} catch {
			break;
		}
	}
	window.dispatchEvent(new CustomEvent('hz:queue-changed'));
	return { ok, remaining: queueCount() };
}

// 请求函数 -----------------------------

export interface RequestOptions extends RequestInit {
	params?: Record<string, unknown>;
	timeout?: number;
}

export class ApiError extends Error {
	constructor(
		public code: number,
		message: string,
		public status?: number
	) {
		super(message);
		this.name = 'ApiError';
	}
}

async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
	const { params, timeout = DEFAULT_TIMEOUT, headers, ...rest } = options;

	// 认证端点使用短超时，避免卡死页面
	const isAuthEndpoint = path === '/auth/me' || path === '/api/auth/me';
	const effectiveTimeout = isAuthEndpoint ? AUTH_TIMEOUT : timeout;

	// 构建 URL
	// 必须用 '/api/'（带尾斜杠）判断：'/api-key' 同样满足 startsWith('/api')，
	// 漏掉尾斜杠会让所有 /api-* 开头的请求拼成 /api-key 打到根路径，
	// 命中后端 SPA fallback 返回 index.html，最终被误判成断网（详见 OFFLINE_QUEUED 事故）。
	let url = path.startsWith('/api/') ? path : `/api${path}`;
	if (params && Object.keys(params).length) {
		const searchParams = new URLSearchParams();
		for (const [k, v] of Object.entries(params)) {
			if (v !== undefined && v !== null && v !== '') {
				searchParams.append(k, String(v));
			}
		}
		const qs = searchParams.toString();
		if (qs) url += (url.includes('?') ? '&' : '?') + qs;
	}

	// 构建 headers
	const finalHeaders: Record<string, string> = {
		'Content-Type': 'application/json',
		...(headers as Record<string, string>)
	};
	const token = getToken();
	if (token) finalHeaders['Authorization'] = `Bearer ${token}`;

	// 超时控制 + 外部（上层）取消。
	// 后者是请求竞态治理的关键：列表页切换筛选条件时必须能掐掉上一个在途请求，
	// 否则网络抖动会让旧响应的结果覆盖新条件的结果（详见 F-03）。
	const external: AbortSignal | null = options.signal ?? null;
	const controller = new AbortController();
	const onExternalAbort = () => controller.abort(external?.reason);
	if (external) {
		if (external.aborted) controller.abort(external.reason);
		else external.addEventListener('abort', onExternalAbort, { once: true });
	}
	let timedOut = false;
	const timer = setTimeout(() => {
		timedOut = true;
		controller.abort();
	}, effectiveTimeout);

	try {
		const res = await fetch(url, {
			...rest,
			headers: finalHeaders,
			signal: controller.signal
		});
		clearTimeout(timer);

		// 401 - 不做硬跳转，抛出错误让上层处理
		if (res.status === 401) {
			removeToken();
			throw new ApiError(401, '登录已过期', res.status);
		}

		const data = (await res.json()) as ApiResp<T>;

		// 业务错误
		if (data.code !== 0) {
			throw new ApiError(data.code, data.message || '请求失败', res.status);
		}

		return data.data as T;
	} catch (err) {
		clearTimeout(timer);
		if (external) external.removeEventListener('abort', onExternalAbort);

		// 被上层主动取消：既不是网络故障也不是业务错误，
		// 绝不能入离线队列（否则一次筛选切换会凭空产生一串待同步请求）。
		if (external?.aborted) {
			throw new ApiError(ERR_ABORTED, 'ABORTED');
		}

		// 超时：请求**可能已经到达服务端并执行完毕**，只是响应没赶回来。
		// 这种情况一旦入队重放就会造成重复写入（重复记一笔账），因此绝不能入队。
		if (timedOut) {
			throw new ApiError(
				ERR_TIMEOUT,
				`请求超时（${Math.round(effectiveTimeout / 1000)} 秒），请稍后重试`
			);
		}

		// 响应体不是合法 JSON：多半是反向代理返回了 HTML 错误页，或后端压根没起来。
		// 这属于服务端故障而非断网，入队只会把错误拖到恢复后再次爆发。
		if (err instanceof SyntaxError) {
			throw new ApiError(ERR_BAD_RESPONSE, '服务响应异常，请检查后端服务是否正常');
		}

		// 离线写操作入队
		// 只在真正的网络断开（fetch 抛出非 ApiError 的错误，或 navigator.onLine=false）时入队
		// HTTP 4xx/5xx 错误是服务端正常返回的业务/状态错误，不入队
		const isOffline = !isOnline();
		const isNetworkError = !(err instanceof ApiError);
		if (
			(isOffline || isNetworkError) &&
			isMutating(options.method || 'GET') &&
			isQueueable(url)
		) {
			// 原始错误只进控制台，便于用 DevTools 区分「真断网」与「代理/证书/跨域」等伪离线
			console.warn('[http] 请求失败，已暂存到离线队列：', url, err);
			enqueueOffline({
				method: (options.method as any) || 'POST',
				url,
				data: options.body ? JSON.parse(String(options.body)) : undefined
			});
			throw new ApiError(ERR_OFFLINE_QUEUED, '网络不可用，操作已暂存，恢复网络后会自动同步');
		}

		throw err;
	}
}

// 快捷方法
export const http = {
	get: <T>(path: string, options: RequestOptions = {}) =>
		request<T>(path, { ...options, method: 'GET' }),

	post: <T>(path: string, data?: unknown, options: RequestOptions = {}) =>
		request<T>(path, {
			...options,
			method: 'POST',
			body: data != null ? JSON.stringify(data) : undefined
		}),

	put: <T>(path: string, data?: unknown, options: RequestOptions = {}) =>
		request<T>(path, {
			...options,
			method: 'PUT',
			body: data != null ? JSON.stringify(data) : undefined
		}),

	patch: <T>(path: string, data?: unknown, options: RequestOptions = {}) =>
		request<T>(path, {
			...options,
			method: 'PATCH',
			body: data != null ? JSON.stringify(data) : undefined
		}),

	delete: <T>(path: string, options: RequestOptions = {}) =>
		request<T>(path, { ...options, method: 'DELETE' }),

	setToken,
	removeToken,
	getToken,
	replayQueue,
	clearQueue,
	queueCount
};

export default http;
