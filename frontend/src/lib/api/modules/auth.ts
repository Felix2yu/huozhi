import { http } from '../http';
import type { User } from '$lib/types';

export const authApi = {
	register: (data: { username: string; email?: string; password: string; nickname?: string }) =>
		http.post<{ token: string; expire_in: number; user: User }>('/auth/register', data),
	login: (data: { username: string; password: string }) =>
		http.post<{ token: string; expire_in: number; user: User }>('/auth/login', data),
	me: () => http.get<User>('/auth/me'),
	updateMe: (data: Partial<User>) => http.put<User>('/auth/me', data),
	changePwd: (data: { old_password: string; new_password: string }) =>
		http.post<void>('/auth/password', data),
	logout: () => http.post<void>('/auth/logout')
};
