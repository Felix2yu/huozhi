import { http } from '../http';

export const aiApi = {
	status: () => http.get<{ enabled: boolean; model?: string; configured: boolean }>('/ai/status'),
	classify: (data: {
		description: string;
		amount?: number;
		type?: string;
		book_id?: number;
	}) =>
		http.post<{
			category_id: number;
			category: string;
			type: string;
			confidence: number;
			explanation?: string;
		}>('/ai/classify', data, { timeout: 120000 }),
	smartRecord: (data: {
		text: string;
		amount?: number;
		type?: string;
		book_id?: number;
	}) =>
		http.post<{
			description: string;
			amount: number;
			type: string;
			category_id: number;
			account_id?: number;
			tx_date: string;
			tags?: string[];
			raw?: string;
		}>('/ai/smart-record', data, { timeout: 120000 })
};
