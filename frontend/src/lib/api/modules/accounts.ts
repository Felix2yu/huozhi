import { http } from '../http';
import type { Account, AccountSummary, CreditRepayItem } from '$lib/types';

export const accountApi = {
	list: (params?: any) =>
		http.get<{ accounts: Account[]; summary: AccountSummary }>('/accounts', { params }),
	get: (id: number) => http.get<Account>(`/accounts/${id}`),
	create: (data: any) => http.post<Account>('/accounts', data),
	update: (id: number, data: any) => http.put<Account>(`/accounts/${id}`, data),
	remove: (id: number) => http.delete<void>(`/accounts/${id}`),
	adjust: (id: number, data: any) => http.post<any>(`/accounts/${id}/adjust`, data),
	listGroups: () => http.get<any[]>('/accounts/groups'),
	createGroup: (data: any) => http.post<any>('/accounts/groups', data),
	removeGroup: (id: number) => http.delete<void>(`/accounts/groups/${id}`),
	getFullCardNo: (id: number) =>
		http.get<{ full_card_no: string }>(`/accounts/${id}/full-card`),
	creditSummary: () => http.get<CreditRepayItem[]>('/accounts/credit-summary')
};
