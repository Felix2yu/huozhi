import { http } from '../http';
import type { FxSnapshot } from '$lib/types';

export const fxApi = {
	/**
	 * 当前汇率快照。
	 * 后端在缓存为空时才同步打上游，其余情况直接返回库里的快照，
	 * 因此这里可以放心地在打开表单时调用。
	 */
	list: (base?: string) =>
		http.get<FxSnapshot>('/exchange-rates', { params: { base } }),

	/** 强制刷新（后端有 10 秒节流，狂点会被拒） */
	refresh: (base?: string) =>
		http.post<FxSnapshot>('/exchange-rates/refresh', { base }),

	/** 服务端折算：用于表单实时预览与对账，避免前端口径与后端不一致 */
	convert: (params: { amount: number; from: string; base?: string }) =>
		http.get<{ base: string; from: string; amount: number; rate: number; converted: number; resolved: boolean }>(
			'/exchange-rates/convert',
			{ params }
		)
};
