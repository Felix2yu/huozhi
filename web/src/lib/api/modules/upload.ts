export const uploadApi = {
	image: async (file: File): Promise<{ url: string }> => {
		const fd = new FormData();
		fd.append('file', file);
		const res = await fetch('/api/upload', {
			method: 'POST',
			body: fd,
			headers: {
				Authorization: `Bearer ${localStorage.getItem('hz_token') || ''}`
			}
		});
		const data = await res.json();
		if (data.code !== 0) throw new Error(data.message || '上传失败');
		return data.data;
	}
};
