/// <reference types="vitest" />
import { defineConfig } from 'vitest/config';
import { sveltekit } from '@sveltejs/kit/vite';
import path from 'path';

export default defineConfig({
	plugins: [sveltekit()],
	resolve: {
		// Svelte 5 在 Vitest 下默认命中 server 构建，导致 mount() 不可用。
		// 强制 browser 条件，才能对组件做真实的客户端挂载测试。
		conditions: ['browser'],
		// 需要能解析 SvelteKit 的 `.svelte.ts` 运行符模块（如 $lib/stores/app），
		// 否则组件内 `import { appStore } from '$lib/stores/app'` 会在 Vitest 下解析失败。
		extensions: ['.mjs', '.js', '.mts', '.ts', '.svelte.ts', '.svelte.js', '.jsx', '.tsx', '.json'],
		alias: {
			$lib: path.resolve(__dirname, 'src/lib')
		}
	},
	test: {
		globals: true,
		environment: 'jsdom',
		include: ['src/**/*.{test,spec}.{js,ts}'],
		setupFiles: ['./src/__tests__/setup.ts'],
		testTimeout: 10000
	}
});
