import { http } from '../http';

export const ioApi = {
	template: async () => {
		const res = await fetch('/api/io/template', {
			headers: { Authorization: `Bearer ${http.getToken()}` || '' }
		});
		const blob = await res.blob();
		const url = URL.createObjectURL(blob);
		const a = document.createElement('a');
		a.href = url;
		a.download = 'template.csv';
		a.click();
		URL.revokeObjectURL(url);
	},
	exportCSV: async (params?: any) => {
		const q = new URLSearchParams(params as any).toString();
		const res = await fetch('/api/io/export?' + q, {
			headers: { Authorization: `Bearer ${http.getToken()}` || '' }
		});
		const blob = await res.blob();
		const url = URL.createObjectURL(blob);
		const a = document.createElement('a');
		a.href = url;
		a.download = 'export.csv';
		a.click();
		URL.revokeObjectURL(url);
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
