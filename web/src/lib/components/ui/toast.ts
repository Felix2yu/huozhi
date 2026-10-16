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
			style: {
				background: 'var(--color-muted)',
				border: '1px solid var(--color-border)'
			}
		})
};

export { toast };
