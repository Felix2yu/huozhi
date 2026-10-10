import { browser } from '$app/environment';

/**
 * 注册 Service Worker（构建时由 vite-plugin-pwa 生成 `/sw.js`）。
 *
 * 为什么必须显式注册：SvelteKit 的 `src/app.html` 不走 Vite 的 `transformIndexHtml`，
 * 因此 vite-plugin-pwa 自动注入 `registerSW.js` / manifest link 的那一步不会落到最终产物里。
 * SvelteKit 下必须自己声明 manifest link（见 src/app.html）并在此调用 `virtual:pwa-register`。
 * 这是 PWA规范.md 第四节对 SvelteKit 的约定写法（吾身 diarum、青野集 qingye 亦同）。
 */
export function registerServiceWorker(): void {
	if (!browser) return;
	// 仅生产构建注册：dev 下无预缓存产物，注册只会带来噪音
	if (!import.meta.env.PROD) return;
	if (!window.isSecureContext) return;
	if (!('serviceWorker' in navigator)) return;

	void (async () => {
		try {
			const { registerSW } = await import('virtual:pwa-register');
			registerSW({ immediate: true });
		} catch (e) {
			console.warn('[PWA] Service Worker 注册失败', e);
		}
	})();
}
