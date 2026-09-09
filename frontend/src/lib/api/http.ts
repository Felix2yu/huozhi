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
	const { params, timeout = 30000, headers, ...rest } = options;

	// 构建 URL
	let url = path.startsWith('/api') ? path : `/api${path}`;
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

	// 超时控制
	const controller = new AbortController();
	const timer = setTimeout(() => controller.abort(), timeout);

	try {
		const res = await fetch(url, {
			...rest,
			headers: finalHeaders,
			signal: controller.signal
		});
		clearTimeout(timer);

		// 401
		if (res.status === 401) {
			removeToken();
			if (typeof window !== 'undefined' && !location.pathname.startsWith('/login')) {
				location.href = `/login?redirect=${encodeURIComponent(location.pathname)}`;
			}
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

		// 离线写操作入队
		// 只在真正的网络断开（fetch 抛出非 ApiError 的错误，或 navigator.onLine=false）时入队
		// HTTP 4xx/5xx 错误是服务端正常返回的业务/状态错误，不入队
		const isOffline = !isOnline();
		const isNetworkError = !(err instanceof ApiError);
		if (
			(isOffline || isNetworkError) &&
			isMutating(options.method || 'GET')
		) {
			enqueueOffline({
				method: (options.method as any) || 'POST',
				url,
				data: options.body ? JSON.parse(String(options.body)) : undefined
			});
			throw new ApiError(-1, 'OFFLINE_QUEUED');
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
