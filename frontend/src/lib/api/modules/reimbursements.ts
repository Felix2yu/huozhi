import { http } from '../http';
import type { Reimbursement } from '$lib/types';

export const reimbApi = {
	list: () => http.get<Reimbursement[]>('/reimbursements'),
	create: (data: any) => http.post<Reimbursement>('/reimbursements', data),
	update: (id: number, data: any) =>
		http.put<Reimbursement>(`/reimbursements/${id}`, data),
	remove: (id: number) => http.delete<void>(`/reimbursements/${id}`)
};
