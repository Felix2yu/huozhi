import { http } from '../http';
import type { Transaction, TransactionListData } from '$lib/types';

export const txApi = {
	list: (params?: any) =>
		http.get<any>('/transactions', { params }).then((res: any) => res.list ?? res) as Promise<TransactionListData>,
	get: (id: number) => http.get<Transaction>(`/transactions/${id}`),
	create: (data: any) => http.post<Transaction>('/transactions', data),
	update: (id: number, data: any) => http.put<Transaction>(`/transactions/${id}`, data),
	remove: (id: number) => http.delete<void>(`/transactions/${id}`),
	batchRemove: (ids: number[]) =>
		http.post<any>('/transactions/batch-delete', { ids })
};
