import { http } from '../http';
import type { Tag } from '$lib/types';

export const tagApi = {
	list: () => http.get<Tag[]>('/tags'),
	create: (data: any) => http.post<Tag>('/tags', data),
	update: (id: number, data: any) => http.put<Tag>(`/tags/${id}`, data),
	remove: (id: number) => http.delete<void>(`/tags/${id}`)
};
