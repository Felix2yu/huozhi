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
	// C15：查看完整卡号需要重新输入登录密码（二次验证），
	// 后端已从 GET 改为 POST，避免密码出现在 URL 与访问日志里
	getFullCardNo: (id: number, password: string) =>
		http.post<{ full_card_no: string; expire_month: number; expire_year: number }>(
			`/accounts/${id}/full-card`,
			{ password }
		),
	// B7：按流水重算账户余额（对账修复）
	recalc: (id: number) => http.post<any>(`/accounts/${id}/recalc`),
	// B7：数据体检 —— 各账户「存储余额 vs 按流水重算」的差额
	audit: () =>
		http.get<{
			accounts: Array<{
				account_id: number;
				name: string;
				type: string;
				balance: number;
				computed: number;
				diff: number;
				tx_count: number;
				need_fix: boolean;
			}>;
			checked: number;
		}>('/accounts/audit'),
	creditSummary: () => http.get<CreditRepayItem[]>('/accounts/credit-summary')
};
