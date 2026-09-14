import { http } from '../http';
import type { Book } from '$lib/types';

export const bookApi = {
	list: () => http.get<Book[]>('/books'),
	get: (id: number) => http.get<Book>(`/books/${id}`),
	create: (data: Partial<Book>) => http.post<Book>('/books', data),
	update: (id: number, data: Partial<Book>) => http.put<Book>(`/books/${id}`, data),
	remove: (id: number) => http.delete<void>(`/books/${id}`),
	listMembers: (id: number) => http.get<any[]>(`/books/${id}/members`),
	// C5：用 identifier（用户名或邮箱）作为统一契约。此前前端传 { email }、
	// 后端只认 username，导致邀请必然失败。
	inviteMember: (id: number, identifier: string, role: 'editor' | 'viewer' = 'viewer') =>
		http.post<any>(`/books/${id}/members`, { identifier, role }),
	removeMember: (bookId: number, memberId: number) =>
		http.delete<void>(`/books/${bookId}/members/${memberId}`),
	// C11：账本归档（停用但保留数据）；删除时可迁移或一并删除子数据
	archive: (id: number, archived = true) =>
		http.put<any>(`/books/${id}/archive?archived=${archived ? 1 : 0}`),
	removeWithOptions: (id: number, opts?: { migrate_to?: number; force?: boolean }) => {
		const q = new URLSearchParams();
		if (opts?.migrate_to) q.set('migrate_to', String(opts.migrate_to));
		if (opts?.force) q.set('force', '1');
		const qs = q.toString();
		return http.delete<any>(`/books/${id}${qs ? '?' + qs : ''}`);
	},
	archived: () => http.get<any[]>('/books/archived')
};
