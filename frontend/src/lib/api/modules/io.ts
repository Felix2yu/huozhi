import { http } from '../http';

export const ioApi = {
	template: () => window.open('/api/io/template'),
	exportCSV: (params?: any) => {
		const q = new URLSearchParams(params as any).toString();
		window.open('/api/io/export?' + q);
	},
	import: async (source: string, book_id: number, file: File) => {
		const fd = new FormData();
		fd.append('file', file);
		return fetch(`/api/io/import?source=${source}&book_id=${book_id}`, {
			method: 'POST',
			headers: { Authorization: `Bearer ${http.getToken()}` || '' },
			body: fd
		}).then((r) => r.json());
	},
	bill: (params: { month: string; book_id?: number }) =>
		http.get<any>(
			`/io/bill?month=${params.month}${params.book_id ? `&book_id=${params.book_id}` : ''}`
		)
};
