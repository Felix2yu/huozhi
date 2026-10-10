import adapter from '@sveltejs/adapter-static';
import tailwindcss from '@tailwindcss/vite';
import { sveltekit } from '@sveltejs/kit/vite';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';
import { VitePWA } from 'vite-plugin-pwa';
import { defineConfig } from 'vite';

export default defineConfig({
	plugins: [
		tailwindcss(),
		// SvelteKit 3 起不再读取 svelte.config.js，配置一律通过 sveltekit() 插件传入。
		// 不属于 SvelteKit 的键（preprocess 等）会透传给 vite-plugin-svelte。
		sveltekit({
			preprocess: vitePreprocess(),
			adapter: adapter({
				pages: 'build',
				assets: 'build',
				fallback: 'index.html',
				precompress: false,
				strict: true
			}),
			alias: {
				$components: 'src/lib/components',
				$api: 'src/lib/api',
				$stores: 'src/lib/stores',
				$utils: 'src/lib/utils',
				$constants: 'src/lib/constants'
			}
		}),
		VitePWA({
			registerType: 'autoUpdate',
			includeAssets: [
				'favicon.ico',
				'favicon.svg',
				'robots.txt',
				'icon-192.png',
				'icon-512.png',
				'icon-maskable-192.png',
				'icon-maskable-512.png',
				'apple-touch-icon.png'
			],
			manifest: {
				id: '/',
				name: '货殖',
				short_name: '货殖',
				description: '一个简洁纯粹的个人记账系统',
				lang: 'zh-CN',
				dir: 'ltr',
				start_url: '/',
				scope: '/',
				display: 'standalone',
				display_override: ['standalone', 'minimal-ui'],
				orientation: 'portrait-primary',
				background_color: '#ffffff',
				theme_color: '#10B981',
				categories: ['finance', 'productivity'],
				icons: [
					{ src: '/icon-192.png', sizes: '192x192', type: 'image/png', purpose: 'any' },
					{ src: '/icon-512.png', sizes: '512x512', type: 'image/png', purpose: 'any' },
					{ src: '/icon-maskable-192.png', sizes: '192x192', type: 'image/png', purpose: 'maskable' },
					{ src: '/icon-maskable-512.png', sizes: '512x512', type: 'image/png', purpose: 'maskable' },
					{ src: '/favicon.svg', sizes: 'any', type: 'image/svg+xml', purpose: 'any' }
				]
			},
			workbox: {
				// 使用默认 globDirectory（Vite outDir），adapter-static 会复制到 build/
				globPatterns: ['**/*.{js,css,html,svg,png,json}'],
				runtimeCaching: [
					// API 请求：NetworkFirst（在线优先，离线回退缓存）
					{
						urlPattern: /^\/api\/(?!uploads)/,
						handler: 'NetworkFirst',
						options: {
							cacheName: 'hz-api-cache',
							expiration: { maxEntries: 500, maxAgeSeconds: 60 * 60 * 24 * 7 },
							cacheableResponse: { statuses: [0, 200] }
						}
					},
					// 账单图片：CacheFirst（不变的资源优先从缓存读取）
					{
						urlPattern: /^\/api\/uploads\//,
						handler: 'CacheFirst',
						options: {
							cacheName: 'hz-uploads-cache',
							expiration: { maxEntries: 200, maxAgeSeconds: 60 * 60 * 24 * 30 }
						}
					},
					// SvelteKit 预构建的 immutable 资源：CacheFirst（带内容哈希）
					{
						urlPattern: /\/_app\/immutable\//,
						handler: 'CacheFirst',
						options: {
							cacheName: 'hz-app-cache',
							expiration: { maxEntries: 200, maxAgeSeconds: 60 * 60 * 24 * 365 }
						}
					}
				]
			}
		})
	],
	resolve: {
		extensions: ['.svelte.ts', '.svelte.js', '.ts', '.js', '.json']
	},
	server: {
		port: 5173,
		host: '0.0.0.0',
		proxy: {
			'/api': {
				target: process.env.VITE_API_TARGET || 'http://localhost:8080',
				changeOrigin: true
			},
			'/uploads': {
				target: process.env.VITE_API_TARGET || 'http://localhost:8080',
				changeOrigin: true
			},
			'/ws': {
				target: process.env.VITE_API_TARGET || 'http://localhost:8080',
				ws: true,
				changeOrigin: true
			}
		}
	},
	build: {
		chunkSizeWarningLimit: 1500
	}
});
