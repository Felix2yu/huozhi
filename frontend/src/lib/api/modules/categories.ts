import { http } from '../http';
import type { Category } from '$lib/types';

export const categoryApi = {
	list: (params?: any) =>
		http.get<{ expense: Category[]; income: Category[]; system: Category[] }>('/categories', { params }),
	create: (data: any) => http.post<Category>('/categories', data),
	update: (id: number, data: any) => http.put<Category>(`/categories/${id}`, data),
	remove: (id: number) => http.delete<void>(`/categories/${id}`),
	reorder: (items: { id: number; sort: number }[]) =>
		http.put<void>('/categories/reorder', { items })
};
