import { http } from '../http';
import type { Budget, BudgetView } from '$lib/types';

export const budgetApi = {
	list: (params?: any) => http.get<BudgetView[]>('/budgets', { params }),
	create: (data: Partial<Budget>) => http.post<Budget>('/budgets', data),
	update: (id: number, data: Partial<Budget>) =>
		http.put<Budget>(`/budgets/${id}`, data),
	remove: (id: number) => http.delete<void>(`/budgets/${id}`),
	// B2：按流水重算已用金额
	recalc: (id: number) => http.post<any>(`/budgets/${id}/recalc`),
	recalcAll: () => http.post<any>('/budgets/recalc')
};
