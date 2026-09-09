import { http } from '../http';
import type { Budget, BudgetView } from '$lib/types';

export const budgetApi = {
	list: (params?: any) => http.get<BudgetView[]>('/budgets', { params }),
	create: (data: Partial<Budget>) => http.post<Budget>('/budgets', data),
	update: (id: number, data: Partial<Budget>) =>
		http.put<Budget>(`/budgets/${id}`, data),
	remove: (id: number) => http.delete<void>(`/budgets/${id}`)
};
