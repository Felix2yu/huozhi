// `virtual:pwa-register`（vite-plugin-pwa 提供的虚拟模块）的类型声明。
//
// 为什么单独成文件而不是写进 app.d.ts：ambient 模块声明只在「全局脚本」文件里生效，
// app.d.ts 末尾有 `export {}`（`declare global namespace App` 需要它是模块），
// 在模块内写 `declare module 'virtual:pwa-register'` 会被 TS 当作模块增强，
// 结果是 `Cannot find module 'virtual:pwa-register'`。
//
// 仍采用手写声明而不是 `/// <reference types="vite-plugin-pwa/client" />`：
// 与其它仓库保持一致的写法，也不依赖 pnpm 的 node_modules 提升行为。
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
