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

// `virtual:pwa-register` 的类型声明见 src/pwa-register.d.ts：
// 本文件是模块（末尾有 `export {}`），写在模块里的 `declare module` 只是「模块增强」，
// 无法凭空声明一个 ambient 模块，TS 会判 `Cannot find module 'virtual:pwa-register'`。

export {};
