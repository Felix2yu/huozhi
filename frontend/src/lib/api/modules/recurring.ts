import { http } from '../http';
import type { Recurring } from '$lib/types';

export const recurringApi = {
	list: () => http.get<Recurring[]>('/recurring'),
	create: (data: any) => http.post<Recurring>('/recurring', data),
	toggle: (id: number) => http.post<Recurring>(`/recurring/${id}/toggle`),
	remove: (id: number) => http.delete<void>(`/recurring/${id}`)
};
