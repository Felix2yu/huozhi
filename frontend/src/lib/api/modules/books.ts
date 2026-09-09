import { http } from '../http';
import type { Book } from '$lib/types';

export const bookApi = {
	list: () => http.get<Book[]>('/books'),
	get: (id: number) => http.get<Book>(`/books/${id}`),
	create: (data: Partial<Book>) => http.post<Book>('/books', data),
	update: (id: number, data: Partial<Book>) => http.put<Book>(`/books/${id}`, data),
	remove: (id: number) => http.delete<void>(`/books/${id}`),
	listMembers: (id: number) => http.get<any[]>(`/books/${id}/members`),
	inviteMember: (id: number, data: any) =>
		http.post<any>(`/books/${id}/members`, data)
};
