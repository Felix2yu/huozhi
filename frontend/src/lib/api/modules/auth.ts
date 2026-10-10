import { http } from '../http';
import type { User } from '#lib/types';

export const authApi = {
	register: (data: { username: string; email?: string; password: string; nickname?: string }) =>
		http.post<{ token: string; expire_in: number; user: User }>('/auth/register', data),
	login: (data: { username: string; password: string }) =>
		http.post<{ token: string; expire_in: number; user: User }>('/auth/login', data),
	me: () => http.get<User>('/auth/me'),
	updateMe: (data: Partial<User>) => http.put<User>('/auth/me', data),
	// 改密码会使其它端 token 立即失效，后端同时返回本端可用的新 token
	changePwd: (data: { old_password: string; new_password: string }) =>
		http.post<{ token: string }>('/auth/password', data),
	// 注销账号：后端会校验密码 + confirm=DELETE_ACCOUNT，并落全量快照
	deleteAccount: (data: { password: string; confirm: string }) =>
		http.delete<void>('/auth/account', { body: JSON.stringify(data) }),
	logout: () => http.post<void>('/auth/logout')
};
