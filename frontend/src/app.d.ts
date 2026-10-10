// See https://svelte.dev/docs/kit/types#app.d.ts
// for information about these interfaces
declare global {
	namespace App {
		// interface Error {}
		// interface Locals {}
		// interface PageData {}
		// interface PageState {}
		// interface Platform {}
	}
}

// `virtual:pwa-register` 的类型声明。
// 手写而非 `/// <reference types="vite-plugin-pwa/client" />`：与其它仓库保持一致的写法，
// 也不依赖 pnpm 的 node_modules 提升行为。
declare module 'virtual:pwa-register' {
	export interface RegisterSWOptions {
		immediate?: boolean;
		onNeedRefresh?: () => void;
		onOfflineReady?: () => void;
		onRegistered?: (registration: ServiceWorkerRegistration | undefined) => void;
		onRegisterError?: (error: unknown) => void;
	}

	export function registerSW(options?: RegisterSWOptions): (reload?: boolean) => Promise<void>;
}

export {};
