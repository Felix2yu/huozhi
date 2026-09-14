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
		),

	// B8：全量备份（JSON 快照，含分类/预算/账户/周期等，CSV 导不出的都在这里）
	backup: async () => {
		const res = await fetch('/api/io/backup', {
			headers: { Authorization: `Bearer ${http.getToken()}` || '' }
		});
		const blob = await res.blob();
		const url = URL.createObjectURL(blob);
		const a = document.createElement('a');
		a.href = url;
		a.download = `huozhi-backup-${new Date().toISOString().slice(0, 10)}.json`;
		a.click();
		URL.revokeObjectURL(url);
	},
	// B8：从快照恢复（mode=replace 先清空再导入；merge 按 ID 追加）
	restore: async (file: File, mode: 'replace' | 'merge' = 'replace') => {
		const text = await file.text();
		const res = await fetch(`/api/io/restore?mode=${mode}`, {
			method: 'POST',
			headers: {
				Authorization: `Bearer ${http.getToken()}` || '',
				'Content-Type': 'application/json'
			},
			body: text
		});
		const data = await res.json();
		if (data.code !== 0) throw new Error(data.message || '恢复失败');
		return data.data;
	},

	// B9：清空全部业务数据（需登录密码 + 确认字串；服务端会先落一份安全快照）
	reset: async (password: string) => {
		const res = await fetch('/api/io/reset', {
			method: 'POST',
			headers: {
				Authorization: `Bearer ${http.getToken()}` || '',
				'Content-Type': 'application/json'
			},
			body: JSON.stringify({ password, confirm: 'CLEAR_ALL_DATA' })
		});
		const data = await res.json();
		if (data.code !== 0) throw new Error(data.message || '清空失败');
		return data.data;
	}
};
