import { http } from '../http';
import type { SavingPlan } from '$lib/types';

export const savingApi = {
	list: () => http.get<SavingPlan[]>('/saving-plans'),
	create: (data: any) => http.post<SavingPlan>('/saving-plans', data),
	update: (id: number, data: any) =>
		http.put<SavingPlan>(`/saving-plans/${id}`, data),
	remove: (id: number) => http.delete<void>(`/saving-plans/${id}`),
	addRecord: (id: number, data: any) =>
		http.post<any>(`/saving-plans/${id}/records`, data)
};
