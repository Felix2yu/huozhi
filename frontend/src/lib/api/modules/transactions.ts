import { http } from '../http';
import type { Transaction, TransactionListData } from '$lib/types';

export const txApi = {
	/**
	 * 交易列表。
	 * - 后端 PagedOK 返回 { list: {...业务数据}, pagination: {...} }，
	 *   这里把业务字段与分页信息合并后再返回，否则前端拿不到 total 无法分页（A1）
	 * - signal：上层可传入 AbortController.signal 取消在途请求，
	 *   用于消除筛选条件快速切换时的响应乱序覆盖（F-03）
	 */
	list: (params?: any, signal?: AbortSignal) =>
		http
			.get<any>('/transactions', { params, signal })
			.then((res: any) => ({
				...(res?.list ?? res ?? {}),
				pagination: res?.pagination
			})) as Promise<TransactionListData>,
	get: (id: number) => http.get<Transaction>(`/transactions/${id}`),
	create: (data: any) => http.post<Transaction>('/transactions', data),
	update: (id: number, data: any) => http.put<Transaction>(`/transactions/${id}`, data),
	remove: (id: number) => http.delete<void>(`/transactions/${id}`),
	batchRemove: (ids: number[]) =>
		http.post<any>('/transactions/batch-delete', { ids }),
	// B3：回收站 —— 后端本就是软删除，此前前端既无删除入口也无恢复入口
	recover: (id: number) => http.post<any>(`/transactions/${id}/recover`),
	deleted: () => http.get<Transaction[]>('/transactions/deleted')
};
