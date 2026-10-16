import { toast } from 'svelte-sonner';

export const hzToast = {
	success: (message: string, description?: string) =>
		toast.success(message, { description }),
	error: (message: string, description?: string) =>
		toast.error(message, { description }),
	warning: (message: string, description?: string) =>
		toast.warning(message, { description }),
	info: (message: string, description?: string) =>
		toast(message, { description }),
	offline: (message: string) =>
		toast(message, {
			duration: 4000,
			// svelte-sonner v1 的 toastOptions.style 是「CSS 字符串」，会被拼进
			// style={`${restProps.style} ${toast.style}`}；传对象只会得到 "[object Object]"，
			// 浏览器直接忽略，所以这里必须写成声明串。
			style: 'background: var(--color-muted); border: 1px solid var(--color-border)'
		})
};

export { toast };
