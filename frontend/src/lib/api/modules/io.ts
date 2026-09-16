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

	// B8：全量备份（ZIP 快照，含 backup.json + 图片文件）
	backup: async () => {
		const res = await fetch('/api/io/backup', {
			headers: { Authorization: `Bearer ${http.getToken()}` || '' }
		});
		const blob = await res.blob();
		const url = URL.createObjectURL(blob);
		const a = document.createElement('a');
		a.href = url;
		a.download = `huozhi-backup-${new Date().toISOString().slice(0, 10)}.zip`;
		a.click();
		URL.revokeObjectURL(url);
	},
	// B8：从备份恢复（支持 ZIP 和旧版 JSON，自动检测格式）
	restore: async (file: File, mode: 'replace' | 'merge' = 'replace') => {
		const fd = new FormData();
		fd.append('file', file);
		const res = await fetch(`/api/io/restore?mode=${mode}`, {
			method: 'POST',
			headers: {
				Authorization: `Bearer ${http.getToken()}` || ''
			},
			body: fd
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
	},

	// 自动备份列表
	listAutoBackups: () => http.get<Array<{ name: string; size: number; time: string }>>('/io/auto-backups')
};
