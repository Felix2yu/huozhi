import tailwindcss from '@tailwindcss/vite';
import { sveltekit } from '@sveltejs/kit/vite';
import { VitePWA } from 'vite-plugin-pwa';
import { defineConfig } from 'vite';

export default defineConfig({
	plugins: [
		tailwindcss(),
		sveltekit(),
		VitePWA({
			registerType: 'autoUpdate',
			includeAssets: ['favicon.svg', 'icon-192.svg', 'icon-512.svg', 'robots.txt'],
			manifest: {
				name: '货殖',
				short_name: '货殖',
				description: '一个简洁纯粹的个人记账系统',
				start_url: '/',
				display: 'standalone',
				background_color: '#ffffff',
				theme_color: '#10B981',
				lang: 'zh-CN',
				scope: '/',
				icons: [
					{ src: '/icon-192.svg', sizes: '192x192', type: 'image/svg+xml', purpose: 'any maskable' },
					{ src: '/icon-512.svg', sizes: '512x512', type: 'image/svg+xml', purpose: 'any maskable' }
				]
			},
			workbox: {
				// 使用默认 globDirectory（Vite outDir），adapter-static 会复制到 build/
				globPatterns: ['**/*.{js,css,html,svg,json}'],
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
