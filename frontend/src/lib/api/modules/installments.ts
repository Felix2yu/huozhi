import { http } from '../http';
import type { Installment } from '$lib/types';

export const installmentApi = {
	list: () => http.get<Installment[]>('/installments'),
	create: (data: any) => http.post<Installment>('/installments', data),
	remove: (id: number) => http.delete<void>(`/installments/${id}`)
};
